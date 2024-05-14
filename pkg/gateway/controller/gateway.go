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
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciaextv1alpha1 "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
)

const (
	heartbeatPeriod = 15 * time.Second
)

// GatewayController sync gateway status periodically to master.
type GatewayController struct {
	namespace string
	publicKey []byte
	hostname  string
	address   string
	uptime    time.Time

	lock sync.Mutex

	kusciaClient        kusciaclientset.Interface
	gatewayLister       kuscialistersv1alpha1.GatewayLister
	gatewayListerSynced cache.InformerSynced
	networkStatus       []kusciaapisv1alpha1.GatewayEndpointStatus
}

// NewGatewayController returns a new GatewayController.
func NewGatewayController(namespace string, prikey *rsa.PrivateKey, kusciaClient kusciaclientset.Interface, informer kusciaextv1alpha1.GatewayInformer) (*GatewayController, error) {
	hostname, address, err := getHostnameAndHostIP()
	if err != nil {
		return nil, err
	}

	pubPemData := tlsutils.EncodePKCS1PublicKey(prikey)

	controller := &GatewayController{
		namespace:           namespace,
		publicKey:           pubPemData,
		hostname:            hostname,
		address:             address,
		uptime:              time.Now(),
		kusciaClient:        kusciaClient,
		gatewayLister:       informer.Lister(),
		gatewayListerSynced: informer.Informer().HasSynced,
	}

	return controller, nil
}

func (c *GatewayController) GatewayName() string {
	return c.hostname
}

// Run begins watching and syncing.
func (c *GatewayController) Run(threadiness int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Start the informer factories to begin populating the informer caches
	nlog.Info("Starting Gateway controller")

	// Wait for the caches to be synced before starting workers
	if ok := cache.WaitForCacheSync(stopCh, c.gatewayListerSynced); !ok {
		nlog.Fatal("failed to wait for caches to sync")
	}

	// Update gateway heartbeat immediately
	if err := c.syncHandler(); err != nil {
		nlog.Errorf("sync gateway error: %v", err)
	}
	ticker := time.NewTicker(heartbeatPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.syncHandler(); err != nil {
				nlog.Errorf("sync gateway error: %v", err)
			}
		case <-stopCh:
			return
		}
	}
}

func (c *GatewayController) syncHandler() error {
	client := c.kusciaClient.KusciaV1alpha1().Gateways(c.namespace)
	gateway, err := c.gatewayLister.Gateways(c.namespace).Get(c.hostname)
	if k8serrors.IsNotFound(err) {
		gateway = &kusciaapisv1alpha1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.hostname,
				Namespace: c.namespace,
			},
		}

		gateway, err = client.Create(context.Background(), gateway, metav1.CreateOptions{})
		if err != nil {
			nlog.Errorf("create gateway(name:%s namespace:%s) fail: %v", c.hostname, c.namespace, err)
			return err
		}
		nlog.Infof("create gateway(name:%s namespace:%s) success", c.hostname, c.namespace)
	}

	if err != nil {
		return err
	}

	status := kusciaapisv1alpha1.GatewayStatus{
		Address: c.address,
		UpTime: metav1.Time{
			Time: c.uptime,
		},
		HeartbeatTime: metav1.Time{
			Time: time.Now(),
		},
		PublicKey: base64.StdEncoding.EncodeToString(c.publicKey),
		Version:   meta.KusciaVersionString(),
	}

	{
		c.lock.Lock()
		defer c.lock.Unlock()

		status.NetworkStatus = append(status.NetworkStatus, c.networkStatus...)
	}

	gatewayCopy := gateway.DeepCopy()
	gatewayCopy.Status = status

	_, err = client.UpdateStatus(context.Background(), gatewayCopy, metav1.UpdateOptions{})
	if err != nil {
		nlog.Errorf("update gateway(name:%s namespace:%s) fail: %v", c.hostname, c.namespace, err)
	}
	return err
}

func (c *GatewayController) UpdateStatus(status []*kusciaapisv1alpha1.GatewayEndpointStatus) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.networkStatus = c.networkStatus[:0]
	for _, s := range status {
		c.networkStatus = append(c.networkStatus, *s)
	}
}

func getHostnameAndHostIP() (string, string, error) {
	hostname := utils.GetHostname()

	address, err := network.GetHostIP()
	if err != nil {
		return "", "", fmt.Errorf("get host IP error: %v", err)
	}
	return hostname, address, nil
}
