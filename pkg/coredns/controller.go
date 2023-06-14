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

package coredns

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	concurrentSyncs = 4
	heartbeatPeriod = 30 * time.Second
)

// Start create and run controllers to list-watch resources.
func (n *KusciaCoreDNS) Start(ctx context.Context, kubeClient kubernetes.Interface) error {
	go startEndpointsController(ctx, kubeClient, n, n.Namespace)
	go startPodController(ctx, kubeClient, n, n.Namespace)
	return nil
}

func startEndpointsController(ctx context.Context, kubeClient kubernetes.Interface, n *KusciaCoreDNS, namespace string) {
	defer runtime.HandleCrash()
	c := NewEndpointsController(ctx, kubeClient, n, n.Namespace)
	nlog.Infof("Starting endpoint controller, namespace: %s", n.Namespace)
	go c.Run(concurrentSyncs)
	<-ctx.Done()
	nlog.Infof("Shutting down endpoint controller, namespace: %s", n.Namespace)
	c.Stop()
}

func startPodController(ctx context.Context, kubeClient kubernetes.Interface, n *KusciaCoreDNS, namespace string) {
	defer runtime.HandleCrash()
	c := NewPodController(ctx, kubeClient, n, namespace)
	nlog.Infof("Starting pod controller, namespace: %s", namespace)
	go c.Run(concurrentSyncs)
	<-ctx.Done()
	nlog.Infof("Shutting down pod controller, namespace: %s", namespace)
	c.Stop()
}
