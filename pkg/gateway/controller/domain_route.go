// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	envoycluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	grpcreversebridge "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/grpc_http1_reverse_bridge/v3"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"google.golang.org/protobuf/types/known/wrapperspb"

	kusciacrypt "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_crypt/v3"
	headerDecorator "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_header_decorator/v3"
	kusciapoller "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_poller/v3"
	kusciareceiver "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_receiver/v3"
	kusciatokenauth "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_token_auth/v3"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	clientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciaextv1alpha1 "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/clusters"
	"github.com/secretflow/kuscia/pkg/gateway/controller/interconn"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

const (
	controllerName        = "domain-route-controller"
	maxRetries            = 16
	domainRouteSyncPeriod = 10 * time.Minute
	domainRouteQueueName  = "domain-route-queue"
	grpcDegradeLabel      = "kuscia.secretflow/grpc-degrade"
)

type DomainRouteConfig struct {
	Namespace       string
	MasterNamespace string
	IsMaster        bool
	CAKey           *rsa.PrivateKey
	CACert          *x509.Certificate
	Prikey          *rsa.PrivateKey
	PrikeyData      []byte
	HandshakePort   uint32
}

type DomainRouteController struct {
	gateway         *kusciaapisv1alpha1.Gateway
	masterNamespace string
	isMaser         bool
	CaCertData      []byte
	CaCert          *x509.Certificate
	CaKey           *rsa.PrivateKey
	prikey          *rsa.PrivateKey
	prikeyData      []byte

	kubeClient              kubernetes.Interface
	kusciaClient            clientset.Interface
	domainRouteLister       kuscialistersv1alpha1.DomainRouteLister
	domainRouteListerSynced cache.InformerSynced
	workqueue               workqueue.RateLimitingInterface

	drCache sync.Map

	handshakeCache  *gocache.Cache
	handshakeServer *http.Server
	handshakePort   uint32

	drHeartbeat map[string]time.Time
}

// NewDomainRouteController create a new endpoints controller.
func NewDomainRouteController(
	drConfig *DomainRouteConfig,
	kubeClient kubernetes.Interface,
	kusciaClient clientset.Interface,
	DomainRouteInformer kusciaextv1alpha1.DomainRouteInformer) *DomainRouteController {

	hostname := utils.GetHostname()
	pubPem := tls.EncodePKCS1PublicKey(drConfig.Prikey)

	gateway := &kusciaapisv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostname,
			Namespace: drConfig.Namespace,
		},
		Status: kusciaapisv1alpha1.GatewayStatus{
			PublicKey: base64.StdEncoding.EncodeToString(pubPem),
		},
	}
	c := &DomainRouteController{
		gateway:                 gateway,
		masterNamespace:         drConfig.MasterNamespace,
		CaCertData:              drConfig.CACert.Raw,
		CaCert:                  drConfig.CACert,
		CaKey:                   drConfig.CAKey,
		isMaser:                 drConfig.IsMaster,
		prikey:                  drConfig.Prikey,
		prikeyData:              drConfig.PrikeyData,
		kubeClient:              kubeClient,
		kusciaClient:            kusciaClient,
		domainRouteLister:       DomainRouteInformer.Lister(),
		domainRouteListerSynced: DomainRouteInformer.Informer().HasSynced,
		workqueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), domainRouteQueueName),
		handshakePort:           drConfig.HandshakePort,
		drCache:                 sync.Map{},
		handshakeCache:          gocache.New(5*time.Minute, 10*time.Minute),
		drHeartbeat:             make(map[string]time.Time, 0),
	}

	DomainRouteInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.addDomainRoute,
			UpdateFunc: func(_, newObj interface{}) {
				c.addDomainRoute(newObj)
			},
			DeleteFunc: c.enqueueDomainRoute,
		},
		domainRouteSyncPeriod,
	)

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *DomainRouteController) Run(ctx context.Context, threadiness int, stopCh <-chan struct{}) {
	var err error

	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	defer func() {
		if err != nil {
			nlog.Fatalf("domain route controller exit with: %v", err)
		}
	}()

	// Start the informer factories to begin populating the informer caches
	nlog.Info("Starting DomainRoute controller")

	// Wait for the caches to be synced before starting workers
	nlog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.domainRouteListerSynced); !ok {
		err = fmt.Errorf("failed to wait for caches to sync")
	}

	go c.startHandShakeServer(c.handshakePort)
	go c.checkConnectionHealthy(ctx, stopCh)
	nlog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	nlog.Info("Started workers")

	<-stopCh
	c.handshakeServer.Close()
	nlog.Info("Shutting down workers")
}

func (c *DomainRouteController) checkConnectionHealthy(ctx context.Context, stopCh <-chan struct{}) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			drs, err := c.domainRouteLister.DomainRoutes(c.gateway.Namespace).List(labels.Everything())
			if err != nil {
				nlog.Error(err)
				break
			}
			for _, dr := range drs {
				if dr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationToken && dr.Spec.Source == c.gateway.Namespace &&
					dr.Status.TokenStatus.RevisionInitializer == c.gateway.Name && dr.Status.TokenStatus.RevisionToken.Token != "" {
					nlog.Debugf("checkConnectionHealthy of dr(%s)", dr.Name)
					err := c.checkConnectionStatus(dr, c.getDefaultClusterNameByDomainRoute(dr))
					if err != nil {
						nlog.Warn(err)
					}
				}
			}
		case <-stopCh:
			return
		}
	}
}

func (c *DomainRouteController) runWorker() {
	for queue.HandleQueueItem(context.Background(), domainRouteQueueName, c.workqueue, c.syncHandler, maxRetries) {
	}
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the DomainRoute resource
// with the current status of the resource.
func (c *DomainRouteController) syncHandler(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the DomainRoute resource with this namespace/name
	dr, err := c.domainRouteLister.DomainRoutes(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return c.deleteDomainRoute(key)
		}

		return err
	}

	// try to update poller && receiver rule first
	if err := c.updatePollerReceiverFilter(dr); err != nil {
		nlog.Warnf("Failed to update poller or receiver filter: %v", err)
	}

	is3rdParty := utils.IsThirdPartyTransit(dr.Spec.Transit)
	if dr.Spec.Source == c.gateway.Namespace && !is3rdParty && dr.Spec.Destination != c.masterNamespace {
		if err := c.addClusterWithEnvoy(dr); err != nil {
			return fmt.Errorf("add envoy cluster failed with %s", err.Error())
		}
	}

	if (dr.Spec.BodyEncryption != nil || (dr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationToken && !is3rdParty)) &&
		(dr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenMethodRSA || dr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenUIDRSA) {
		if dr.Spec.Source == c.gateway.Namespace && dr.Status.TokenStatus.RevisionInitializer == c.gateway.Name {
			if dr.Status.TokenStatus.RevisionToken.Token == "" {
				_, ok := c.handshakeCache.Get(dr.Name)
				if !ok {
					c.handshakeCache.Add(dr.Name, dr.Name, 2*time.Minute)
					defer c.handshakeCache.Delete(dr.Name)
					if err := func() error {
						if !is3rdParty {
							if err := c.setKeepAliveForDstClusters(dr, false); err != nil {
								return fmt.Errorf("disable keep-alive fail for DomainRoute: %s err: %v", key, err)
							}
						}
						nlog.Infof("DomainRoute %s starts handshake, the last revision is %d", key, dr.Status.TokenStatus.RevisionToken.Revision)

						return c.sourceInitiateHandShake(dr, c.getDefaultClusterNameByDomainRoute(dr))
					}(); err != nil {
						nlog.Error(err)
						return err
					}
				}
				return nil
			}
		}
	}

	return c.updateDomainRoute(dr)
}

func (c *DomainRouteController) enqueueDomainRoute(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
	nlog.Debugf("enqueue domainRoute:%s", key)
}

func (c *DomainRouteController) addDomainRoute(obj interface{}) {
	c.enqueueDomainRoute(obj)

	// find transit domain routes whose Transit.Domain.DomainID equals to obj's destination and update them
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return
	}
	newDomainRoute, err := c.domainRouteLister.DomainRoutes(namespace).Get(name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("can't find DomainRoute: %s", key))
		return
	}

	if newDomainRoute.Spec.Source == c.gateway.Namespace && !utils.IsThirdPartyTransit(newDomainRoute.Spec.Transit) {
		drs, err := c.domainRouteLister.List(labels.Everything())
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("list DomainRoute failed with: %s", err.Error()))
		}
		for _, dr := range drs {
			if utils.IsThirdPartyTransit(dr.Spec.Transit) && dr.Spec.Transit.Domain.DomainID == newDomainRoute.Spec.Destination {
				c.workqueue.Add(fmt.Sprintf("%s/%s", dr.Namespace, dr.Name))
			}
		}
	}
}

func (c *DomainRouteController) updateDomainRoute(dr *kusciaapisv1alpha1.DomainRoute) error {
	key, _ := cache.MetaNamespaceKeyFunc(dr)
	nlog.Infof("Update DomainRoute %s revision:%s", key, dr.ResourceVersion)

	var tokens []*Token
	// for non-token author
	tokens, err := c.parseToken(dr, key)
	// Swallow all errors to avoid requeuing
	if err != nil {
		nlog.Warn(err)
		return nil
	}

	if len(tokens) == 0 {
		nlog.Debugf("DomainRoute %s has no available token", key)
		return nil
	}

	if err := c.updateEnvoyRule(dr, tokens); err != nil {
		return err
	}

	c.drCache.Store(key, dr)

	// update effective instances
	return c.checkAndUpdateTokenInstances(dr)
}

func (c *DomainRouteController) deleteDomainRoute(key string) error {
	nlog.Infof("Delete DomainRoute %s", key)

	val, ok := c.drCache.Load(key)
	if !ok {
		return nil
	}
	dr, ok := val.(*kusciaapisv1alpha1.DomainRoute)
	if !ok {
		return fmt.Errorf("cache[%s] cannit cast to DomainRoute", key)
	}

	if err := c.deleteEnvoyRule(dr); err != nil {
		return err
	}
	delete(c.drHeartbeat, dr.Name)
	c.drCache.Delete(key)
	return nil
}

func (c *DomainRouteController) addClusterWithEnvoy(dr *kusciaapisv1alpha1.DomainRoute) error {
	var transportSocket *core.TransportSocket
	if dr.Spec.MTLSConfig != nil {
		srcCertdata, err := base64.StdEncoding.DecodeString(dr.Spec.MTLSConfig.SourceClientCert)
		if err != nil {
			return err
		}

		var srcPrivateKeyData []byte
		if len(dr.Spec.MTLSConfig.SourceClientPrivateKey) > 0 {
			srcPrivateKeyData, err = base64.StdEncoding.DecodeString(dr.Spec.MTLSConfig.SourceClientPrivateKey)
			if err != nil {
				return err
			}
		} else if dr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationMTLS {
			srcPrivateKeyData = c.prikeyData
		}

		srcTLSCAdata, err := base64.StdEncoding.DecodeString(dr.Spec.MTLSConfig.TLSCA)
		if err != nil {
			return err
		}
		transportSocket, err = xds.GenerateUpstreamTLSConfigByCertStr(srcCertdata, srcPrivateKeyData, srcTLSCAdata)
		if err != nil {
			return fmt.Errorf("verify mtls config of %s failed with %s", dr.Name, err.Error())
		}
	}

	for _, dp := range dr.Spec.Endpoint.Ports {
		nlog.Infof("add cluster %s-to-%s name:%s protocol:%s port:%d", dr.Spec.Source, dr.Spec.Destination, dp.Name, dp.Protocol, dp.Port)
		err := addClusterForDstGateway(dr, dp, transportSocket)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *DomainRouteController) updatePollerReceiverFilter(dr *kusciaapisv1alpha1.DomainRoute) error {
	if dr.Spec.Source == c.gateway.Namespace { // internal
		if utils.IsGatewayTceTransit(dr.Spec.Transit) {
			rule := kusciareceiver.ReceiverRule{
				Source:      dr.Spec.Source,
				Destination: dr.Spec.Destination,
			}
			if err := xds.UpdateReceiverRules(&rule, true); err != nil {
				return err
			}
		}

	} else if dr.Spec.Destination == c.gateway.Namespace { // external
		if !utils.IsThirdPartyTransit(dr.Spec.Transit) {
			if utils.IsGatewayTceTransit(dr.Spec.Transit) {
				pollHeader := generatePollHeaders(dr)
				if err := xds.UpdatePoller(pollHeader, true); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *DomainRouteController) updateEnvoyRule(dr *kusciaapisv1alpha1.DomainRoute, tokens []*Token) error {
	// TODO domain route from current ns to master can't be transit route, is it correct, or else why?
	if dr.Spec.Destination == c.masterNamespace && dr.Spec.Source == c.gateway.Namespace {
		token := tokens[len(tokens)-1]
		cl, err := xds.QueryCluster(clusters.GetMasterClusterName())
		if err != nil {
			nlog.Error(err)
			return err
		}
		pathPrefix := utils.GetPrefixIfPresent(dr.Spec.Endpoint)
		if err := clusters.AddMasterProxyVirtualHost(cl.Name, pathPrefix, utils.ServiceMasterProxy, c.gateway.Namespace, token.Token); err != nil {
			nlog.Error(err)
			return err
		}
		if err = xds.SetKeepAliveForDstCluster(cl, true); err != nil {
			nlog.Error(err)
			return err
		}
		if err = xds.AddOrUpdateCluster(cl); err != nil {
			nlog.Error(err)
			return err
		}
		nlog.Info("Update rule to master success")
		return nil
	} else if dr.Spec.Source == c.gateway.Namespace { // internal
		token := tokens[len(tokens)-1]
		grpcDegrade := false
		if dr.Labels[grpcDegradeLabel] == "True" {
			grpcDegrade = true
		}

		if dr.Spec.BodyEncryption != nil {
			if err := updateEncryptRule(dr, token); err != nil {
				return err
			}
		}

		// next step with two cases
		// case1: transit route, just clone routing rule  from source-to-transitDomainID
		if utils.IsThirdPartyTransit(dr.Spec.Transit) {
			return c.updateRoutingRule(dr)
		}

		// case2: direct route, add virtualhost: source-to-dest-Protocol
		if err := xds.AddOrUpdateVirtualHost(generateInternalVirtualHost(dr, token.Token, grpcDegrade),
			xds.InternalRoute); err != nil {
			return err
		}

		return c.setKeepAliveForDstClusters(dr, true)
	} else if dr.Spec.Destination == c.gateway.Namespace { // external
		if !utils.IsThirdPartyTransit(dr.Spec.Transit) {
			var tokenVals []string
			for _, token := range tokens {
				tokenVals = append(tokenVals, token.Token)
			}
			// for DomainAuthenticationMTLS, DomainAuthenticationNone auth type, use NoopToken
			sourceToken := &kusciatokenauth.TokenAuth_SourceToken{
				Source: dr.Spec.Source,
				Tokens: tokenVals,
			}
			sourceHeader := generateRequestHeaders(dr)

			if err := xds.UpdateTokenAuthAndHeaderDecorator(sourceToken, sourceHeader, true); err != nil {
				return err
			}
		}

		if dr.Spec.BodyEncryption != nil {
			return updateDecryptFilter(dr, tokens)
		}
	}
	return nil
}

func (c *DomainRouteController) deleteEnvoyRule(dr *kusciaapisv1alpha1.DomainRoute) error {
	name := fmt.Sprintf("%s-to-%s", dr.Spec.Source, dr.Spec.Destination)
	if dr.Spec.Source == c.gateway.Namespace {
		if err := xds.DeleteVirtualHost(name, xds.InternalRoute); err != nil {
			return fmt.Errorf("delete virtual host %s failed with %v", name, err)
		}
		if utils.IsGatewayTceTransit(dr.Spec.Transit) {
			rule := kusciareceiver.ReceiverRule{
				Source:      dr.Spec.Source,
				Destination: dr.Spec.Destination,
			}
			if err := xds.UpdateReceiverRules(&rule, false); err != nil {
				return err
			}
		}
		if dr.Spec.BodyEncryption != nil {
			rule := &kusciacrypt.CryptRule{
				Source:      dr.Spec.Source,
				Destination: dr.Spec.Destination,
			}
			// directly return, why?
			return xds.UpdateEncryptRules(rule, false)
		}
		if !utils.IsThirdPartyTransit(dr.Spec.Transit) {
			return xds.DeleteCluster(name)
		}
	} else if dr.Spec.Destination == c.gateway.Namespace {
		if dr.Spec.BodyEncryption != nil {
			rule := &kusciacrypt.CryptRule{
				Source:      dr.Spec.Source,
				Destination: dr.Spec.Destination,
			}
			if err := xds.UpdateDecryptRules(rule, false); err != nil {
				return err
			}
		}
		if !utils.IsThirdPartyTransit(dr.Spec.Transit) {
			sourceToken := &kusciatokenauth.TokenAuth_SourceToken{
				Source: dr.Spec.Source,
			}
			sourceHeader := &headerDecorator.HeaderDecorator_SourceHeader{
				Source: dr.Spec.Source,
			}
			nlog.Debugf("delete token and sourceHeaders, source is %s", sourceToken.Source)
			if err := xds.UpdateTokenAuthAndHeaderDecorator(sourceToken, sourceHeader, false); err != nil {
				return err
			}
			if utils.IsGatewayTceTransit(dr.Spec.Transit) {
				sourceHeader := &kusciapoller.Poller_SourceHeader{
					Source: dr.Spec.Source,
				}
				if err := xds.UpdatePoller(sourceHeader, false); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *DomainRouteController) updateRoutingRule(dr *kusciaapisv1alpha1.DomainRoute) error {
	ns := dr.Spec.Transit.Domain.DomainID
	vhName := fmt.Sprintf("%s-to-%s", dr.Spec.Source, ns)
	if ns == c.masterNamespace && c.masterNamespace != c.gateway.Namespace {
		vhName = fmt.Sprintf("%s-internal", clusters.GetMasterClusterName())
	}
	vh, err := xds.QueryVirtualHost(vhName, xds.InternalRoute)
	if err != nil {
		return fmt.Errorf("failed to query virtual host with %s", err.Error())
	}
	if vh == nil {
		return fmt.Errorf("failed to get virtual host (%s) in route(%s)", vhName, xds.InternalRoute)
	}

	vhNew, ok := proto.Clone(vh).(*route.VirtualHost)
	if !ok {
		return fmt.Errorf("proto cannot cast to VirtualHost")
	}

	// new vh vs old vh
	vhNew.Name = fmt.Sprintf("%s-to-%s", dr.Spec.Source, dr.Spec.Destination)
	vhNew.Domains = []string{fmt.Sprintf("*.%s.svc", dr.Spec.Destination)}
	if err = xds.AddOrUpdateVirtualHost(vhNew, xds.InternalRoute); err != nil {
		return err
	}
	return nil
}

func updateEncryptRule(dr *kusciaapisv1alpha1.DomainRoute, token *Token) error {
	// update encrypt filter config
	rule := &kusciacrypt.CryptRule{
		Source:           dr.Spec.Source,
		Destination:      dr.Spec.Destination,
		Algorithm:        string(dr.Spec.BodyEncryption.Algorithm),
		SecretKey:        token.Token,
		SecretKeyVersion: fmt.Sprint(token.Version),
	}
	return xds.UpdateEncryptRules(rule, true)
}

func updateDecryptFilter(dr *kusciaapisv1alpha1.DomainRoute, tokens []*Token) error {
	n := len(tokens)
	rule := &kusciacrypt.CryptRule{
		Source:           dr.Spec.Source,
		Destination:      dr.Spec.Destination,
		Algorithm:        string(dr.Spec.BodyEncryption.Algorithm),
		SecretKey:        tokens[n-1].Token,
		SecretKeyVersion: fmt.Sprint(tokens[n-1].Version),
	}
	if n >= 2 {
		rule.ReserveKey = tokens[n-2].Token
		rule.ReserveKeyVersion = fmt.Sprint(tokens[n-2].Version)
	}

	return xds.UpdateDecryptRules(rule, true)
}

func generateInternalRoute(dr *kusciaapisv1alpha1.DomainRoute, dp kusciaapisv1alpha1.DomainPort, token string, isDefaultRoute bool,
	grpcDegrade bool) []*route.Route {
	httpRoutes := interconn.Decorator.GenerateInternalRoute(dr, dp, token)
	for _, httpRoute := range httpRoutes {
		if !isDefaultRoute && dp.Protocol == "GRPC" {
			httpRoute.Match.Headers = []*route.HeaderMatcher{
				{
					Name: "content-type",
					HeaderMatchSpecifier: &route.HeaderMatcher_StringMatch{
						StringMatch: &matcher.StringMatcher{
							MatchPattern: &matcher.StringMatcher_Prefix{
								Prefix: "application/grpc",
							},
						},
					},
				},
			}
		}

		if !grpcDegrade || dp.Protocol == "GRPC" {
			disable := &grpcreversebridge.FilterConfigPerRoute{
				Disabled: true,
			}
			b, err := proto.Marshal(disable)
			if err != nil {
				nlog.Errorf("Marshal grpc reverse bridge config failed with %v", err)
			} else {
				httpRoute.TypedPerFilterConfig = map[string]*anypb.Any{
					"envoy.filters.http.grpc_http1_reverse_bridge": {
						TypeUrl: "type.googleapis.com/envoy.extensions.filters.http.grpc_http1_reverse_bridge.v3.FilterConfigPerRoute",
						Value:   b,
					},
				}
			}
		}
	}
	return httpRoutes
}

func generateInternalRoutes(dr *kusciaapisv1alpha1.DomainRoute, token string, grpcDegrade bool) []*route.Route {
	dps := sortDomainPorts(dr.Spec.Endpoint.Ports)
	var routes []*route.Route
	n := len(dps)
	for _, dp := range dps {
		isDefaultRoute := false
		if n == 1 || dp.Protocol == "HTTP" {
			isDefaultRoute = true
		}
		routes = append(routes, generateInternalRoute(dr, dp, token, isDefaultRoute, grpcDegrade)...)
	}
	return routes
}

func generateInternalVirtualHost(dr *kusciaapisv1alpha1.DomainRoute, token string, grpcDegrade bool) *route.VirtualHost {
	routes := generateInternalRoutes(dr, token, grpcDegrade)

	connectRoute := &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_ConnectMatcher_{
				ConnectMatcher: &route.RouteMatch_ConnectMatcher{},
			},
		},
		Action: &route.Route_Route{
			Route: xds.AddDefaultTimeout(
				&route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: "internal-cluster", // send to internal listener again so we can add headers
					},
					UpgradeConfigs: []*route.RouteAction_UpgradeConfig{
						{
							UpgradeType: "CONNECT",
							// this config terminates the connect request and forward payload as raw tcp data to upstream
							ConnectConfig: &route.RouteAction_UpgradeConfig_ConnectConfig{},
						},
					},
				},
			),
		},
	}

	routes = append(routes, connectRoute)

	vh := &route.VirtualHost{
		Name:    fmt.Sprintf("%s-to-%s", dr.Spec.Source, dr.Spec.Destination),
		Domains: []string{fmt.Sprintf("*.%s.svc", dr.Spec.Destination)},
		Routes:  routes,
	}

	return vh
}

func (c *DomainRouteController) setKeepAliveForDstClusters(dr *kusciaapisv1alpha1.DomainRoute, enable bool) error {
	clusterNames := c.getClusterNamesByDomainRoute(dr)
	for _, cn := range clusterNames {
		c, err := xds.QueryCluster(cn)
		if err != nil {
			return err
		}
		if err := xds.SetKeepAliveForDstCluster(c, enable); err != nil {
			return err
		}
		if err := xds.AddOrUpdateCluster(c); err != nil {
			return err
		}
	}
	return nil
}

func addClusterForDstGateway(dr *kusciaapisv1alpha1.DomainRoute, dp kusciaapisv1alpha1.DomainPort,
	transportSocket *core.TransportSocket) error {
	var protocolOptions *envoyhttp.HttpProtocolOptions
	var protocol string
	if dr.Labels[grpcDegradeLabel] == "True" && dp.Protocol == kusciaapisv1alpha1.DomainRouteProtocolGRPC {
		// use http1.1
		protocolOptions = xds.GenerateHTTP2UpstreamHTTPOptions(true)
		protocol = xds.GenerateProtocol(dp.IsTLS, true)
	} else {
		// use same protocol with downstream
		protocolOptions = xds.GenerateSimpleUpstreamHTTPOptions(true)
		protocol = xds.GenerateProtocol(dp.IsTLS, false)
	}

	clusterName := common.GenerateClusterName(dr.Spec.Source, dr.Spec.Destination, dp.Name)

	// before token take effect, we disable keep-alive for DstEnvoy
	if dr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationToken {
		preProtocolOptions, preCluster, _ := xds.GetClusterHTTPProtocolOptions(clusterName)
		if preCluster == nil {
			// next action is handshake
			protocolOptions.CommonHttpProtocolOptions.MaxRequestsPerConnection = &wrapperspb.UInt32Value{
				Value: uint32(1),
			}
			nlog.Infof("disable keep-alive for cluster:%s ", clusterName)
		} else if preProtocolOptions != nil && preProtocolOptions.CommonHttpProtocolOptions != nil {
			// do not change keep-alive options when changing cluster
			protocolOptions.CommonHttpProtocolOptions = preProtocolOptions.CommonHttpProtocolOptions
		}
	}

	b, err := proto.Marshal(protocolOptions)
	if err != nil {
		nlog.Errorf("Marshal protocolOptions failed with %s", err.Error())
		return err
	}

	cluster := &envoycluster.Cluster{
		Name: clusterName,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: clusterName,
			Endpoints: []*endpoint.LocalityLbEndpoints{
				{
					LbEndpoints: []*endpoint.LbEndpoint{
						{
							HostIdentifier: &endpoint.LbEndpoint_Endpoint{
								Endpoint: &endpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Address: dr.Spec.Endpoint.Host,
												PortSpecifier: &core.SocketAddress_PortValue{
													PortValue: uint32(dp.Port),
												},
											},
										},
									},
									Hostname: dr.Spec.Endpoint.Host,
								},
							},
						},
					},
				},
			},
		},
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": {
				TypeUrl: "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
				Value:   b,
			},
		},
		TransportSocket: transportSocket,
	}

	if err := xds.DecorateRemoteUpstreamCluster(cluster, protocol); err != nil {
		return err
	}

	interconn.Decorator.UpdateDstCluster(dr, cluster)

	return xds.AddOrUpdateCluster(cluster)
}

func generateRequestHeaders(dr *kusciaapisv1alpha1.DomainRoute) *headerDecorator.HeaderDecorator_SourceHeader {
	if len(dr.Spec.RequestHeadersToAdd) == 0 {
		return nil
	}
	sourceHeader := &headerDecorator.HeaderDecorator_SourceHeader{
		Source: dr.Spec.Source,
	}
	for k, v := range dr.Spec.RequestHeadersToAdd {
		entry := &headerDecorator.HeaderDecorator_HeaderEntry{
			Key:   k,
			Value: v,
		}
		sourceHeader.Headers = append(sourceHeader.Headers, entry)
	}
	return sourceHeader
}

func generatePollHeaders(dr *kusciaapisv1alpha1.DomainRoute) *kusciapoller.Poller_SourceHeader {
	if len(dr.Spec.RequestHeadersToAdd) == 0 {
		return nil
	}
	sourceHeader := &kusciapoller.Poller_SourceHeader{
		Source: dr.Spec.Source,
	}
	for k, v := range dr.Spec.RequestHeadersToAdd {
		entry := &kusciapoller.Poller_HeaderEntry{
			Key:   k,
			Value: v,
		}
		sourceHeader.Headers = append(sourceHeader.Headers, entry)
	}
	return sourceHeader
}

func (c *DomainRouteController) getClusterNamesByDomainRoute(dr *kusciaapisv1alpha1.DomainRoute) []string {
	var names []string
	if dr.Spec.Destination == c.masterNamespace && c.masterNamespace != c.gateway.Namespace {
		names = append(names, clusters.GetMasterClusterName())
	}
	for _, dp := range dr.Spec.Endpoint.Ports {
		names = append(names, common.GenerateClusterName(dr.Spec.Source, dr.Spec.Destination, dp.Name))
	}
	return names
}

func (c *DomainRouteController) getDefaultClusterNameByDomainRoute(dr *kusciaapisv1alpha1.DomainRoute) string {
	if dr.Spec.Destination == c.masterNamespace && c.masterNamespace != c.gateway.Namespace {
		return clusters.GetMasterClusterName()
	}
	dps := sortDomainPorts(dr.Spec.Endpoint.Ports)
	n := len(dps)
	for _, dp := range dps {
		if n == 1 || dp.Protocol == "HTTP" {
			return common.GenerateClusterName(dr.Spec.Source, dr.Spec.Destination, dp.Name)
		}
	}
	if n > 0 {
		return common.GenerateClusterName(dr.Spec.Source, dr.Spec.Destination, dps[0].Name)
	}
	return ""
}

func getHandshakeHost(dr *kusciaapisv1alpha1.DomainRoute) string {
	ns := dr.Spec.Destination
	if utils.IsThirdPartyTransit(dr.Spec.Transit) {
		ns = dr.Spec.Transit.Domain.DomainID
	}
	return fmt.Sprintf("%s.%s.svc", utils.ServiceHandshake, ns)
}
