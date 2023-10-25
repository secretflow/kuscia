// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type K8sNodeDependence struct {
	BaseNodeDependence
	BkClient    clientset.Interface
	BkNamespace string
}

type K8sNodeProvider struct {
	bkClient    clientset.Interface
	bkNamespace string

	*BaseNode
}

func NewK8sNodeProvider(dep *K8sNodeDependence) *K8sNodeProvider {
	knp := &K8sNodeProvider{
		bkClient:    dep.BkClient,
		bkNamespace: dep.BkNamespace,
	}

	knp.BaseNode = newBaseNode(&dep.BaseNodeDependence)

	return knp
}

func (knp *K8sNodeProvider) Ping(ctx context.Context) error {
	_, err := knp.bkClient.CoreV1().Pods(knp.bkNamespace).Get(ctx, "test", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (knp *K8sNodeProvider) ConfigureNode(ctx context.Context, name string) *v1.Node {
	nlog.Infof("Configuring k8s node %q successfully", name)
	return knp.configureCommonNode(ctx, name)
}

func (knp *K8sNodeProvider) RefreshNodeStatus(ctx context.Context, nodeStatus *v1.NodeStatus) bool {
	return false
}

func (knp *K8sNodeProvider) SetStatusUpdateCallback(ctx context.Context, cb func(*v1.Node)) {
	return
}
