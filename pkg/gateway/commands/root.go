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

package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/gateway/clusters"
	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/gateway/controller"
	"github.com/secretflow/kuscia/pkg/gateway/metrics"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

const (
	gatewayName     = "gateway"
	concurrentSyncs = 8

	defaultHandshakeRetryCount    = 10
	defaultHandshakeRetryInterval = 100 * time.Millisecond
)

var (
	ReadyChan = make(chan struct{})
)

func Run(ctx context.Context, gwConfig *config.GatewayConfig, clients *kubeconfig.KubeClients, afterRegisterHook controller.AfterRegisterDomainHook) error {
	prikey := gwConfig.DomainKey
	priKeyData := tls.EncodePKCS1PublicKey(gwConfig.DomainKey)

	// start xds server and envoy
	err := StartXds(gwConfig)
	if err != nil {
		return fmt.Errorf("start xds server fail with err: %v", err)
	}
	nlog.Infof("Start xds success")

	// add master config
	masterConfig, err := config.LoadMasterConfig(gwConfig.MasterConfig, clients.Kubeconfig)
	nlog.Debugf("masterConfig is: %v", masterConfig)

	if err != nil {
		return fmt.Errorf("failed to load masterConfig, detail-> %v", err)
	}

	// add master Clusters
	err = clusters.AddMasterClusters(ctx, gwConfig.DomainID, masterConfig)
	if err != nil {
		return fmt.Errorf("add master clusters fail, detail-> %v", err)
	}
	if !masterConfig.Master {
		err = controller.RegisterDomain(gwConfig.DomainID, masterConfig.MasterProxy.Path, gwConfig.CsrData, prikey, afterRegisterHook)
		if err != nil {
			return fmt.Errorf("register self domain [%s] cert to master error: %s", gwConfig.DomainID, err.Error())
		}

		for i := 1; i <= defaultHandshakeRetryCount; i++ {
			err = controller.HandshakeToMaster(gwConfig.DomainID, masterConfig.MasterProxy.Path, prikey)
			if err == nil {
				break
			}
			nlog.Warnf("HandshakeToMaster error: %v", err)
			if i == defaultHandshakeRetryCount {
				return err
			}
			time.Sleep(defaultHandshakeRetryInterval)
		}
		checkMasterProxyReady(ctx, gwConfig.DomainID, clients.KubeClient)
	}
	nlog.Infof("Add master clusters success")

	// add interconn cluster
	interConnClusterConfig, err := config.LoadInterConnClusterConfig(gwConfig.TransportConfig,
		gwConfig.InterConnSchedulerConfig)
	if err != nil {
		return fmt.Errorf("failed to load interConnClusterConfig, detail-> %v", err)
	}
	err = clusters.AddInterConnClusters(gwConfig.DomainID, interConnClusterConfig)
	if err != nil {
		return fmt.Errorf("add interConn clusters fail, detail-> %v", err)
	}
	nlog.Infof("Add interconn clusters success")

	// create informer factory
	defaultResync := time.Duration(gwConfig.ResyncPeriod) * time.Second
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(clients.KubeClient, defaultResync,
		kubeinformers.WithNamespace(gwConfig.DomainID))
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(clients.KusciaClient, defaultResync,
		informers.WithNamespace(gwConfig.DomainID))

	// start GatewayController
	gwc, err := controller.NewGatewayController(gwConfig.DomainID, prikey, clients.KusciaClient, kusciaInformerFactory.Kuscia().V1alpha1().Gateways())
	if err != nil {
		return fmt.Errorf("failed to new gateway controller, detail-> %v", err)
	}
	go gwc.Run(concurrentSyncs, ctx.Done())

	// start endpoints controller
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()

	clientCert, err := config.LoadTLSCertByTLSConfig(gwConfig.InnerClientTLS)
	if err != nil {
		return fmt.Errorf("load innerClientTLS fail, detail-> %v", err)
	}
	ec, err := controller.NewEndpointsController(masterConfig.Master, clients.KubeClient, serviceInformer,
		endpointsInformer,
		gwConfig.WhiteListFile,
		clientCert)
	if err != nil {
		return fmt.Errorf("failed to new endpoints controller, detail-> %v", err)
	}
	go ec.Run(concurrentSyncs, ctx.Done())

	// start DomainRoute controller
	drInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()
	drConfig := &controller.DomainRouteConfig{
		Namespace:       gwConfig.DomainID,
		MasterNamespace: masterConfig.Namespace,
		MasterConfig:    masterConfig,
		CAKey:           gwConfig.CAKey,
		CACert:          gwConfig.CACert,
		Prikey:          prikey,
		PrikeyData:      priKeyData,
		HandshakePort:   gwConfig.HandshakePort,
	}
	drc := controller.NewDomainRouteController(drConfig, clients.KubeClient, clients.KusciaClient, drInformer)
	go drc.Run(ctx, concurrentSyncs*2, ctx.Done())

	// start runtime metrics collector
	go metrics.MonitorRuntimeMetrics(ctx.Done())

	// start cluster metrics collector
	envoyStatsEndpoint := fmt.Sprintf("http://127.0.0.1:%d", gwConfig.EnvoyAdminPort)
	mc := metrics.NewClusterMetricsCollector(serviceInformer.Lister(), endpointsInformer.Lister(),
		drInformer.Lister(), gwc, envoyStatsEndpoint)
	go mc.MonitorClusterMetrics(ctx.Done())

	// Notice that there is no need to run Start methods in a separate goroutine.
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(ctx.Done())
	kusciaInformerFactory.Start(ctx.Done())
	nlog.Info("Gateway running")
	close(ReadyChan)
	<-ctx.Done()
	nlog.Info("Gateway shutdown")
	return nil
}

func checkMasterProxyReady(ctx context.Context, domainID string, kubeClient kubernetes.Interface) {
	var err error
	times := 5
	for i := 0; i < times; i++ {
		if _, err = kubeClient.CoreV1().Pods(domainID).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
			nlog.Info("Check MasterProxy ready")
			return
		}
		time.Sleep(time.Second)
	}
	nlog.Fatalf("Check MasterProxy failed: %v", err.Error())
}

func StartXds(gwConfig *config.GatewayConfig) error {
	// set route idle timeout
	xds.IdleTimeout = gwConfig.IdleTimeout

	xds.NewXdsServer(gwConfig.XDSPort, gwConfig.GetEnvoyNodeID())

	externalCert, err := config.LoadTLSCertByTLSConfig(gwConfig.ExternalTLS)
	if err != nil {
		return err
	}
	internalCert, err := config.LoadTLSCertByTLSConfig(gwConfig.InnerServerTLS)
	if err != nil {
		return err
	}

	xdsConfig := &xds.InitConfig{
		Basedir:      gwConfig.ConfBasedir,
		XDSPort:      gwConfig.XDSPort,
		ExternalPort: gwConfig.ExternalPort,
		ExternalCert: externalCert,
		InternalCert: internalCert,
		Logdir:       filepath.Join(gwConfig.RootDir, "var/logs/envoy/"),
	}

	xds.InitSnapshot(gwConfig.DomainID, utils.GetHostname(), xdsConfig)
	return nil
}
