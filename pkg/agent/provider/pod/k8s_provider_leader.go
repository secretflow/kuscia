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

package pod

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/secretflow/kuscia/pkg/agent/resource"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/agent/utils/nodeutils"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type nodeState int

const (
	nodeStateNotExist = iota
	nodeStateUnReady
	nodeStateReady
)

type nodeStore struct {
	rm     *resource.KubeResourceManager
	states map[string]nodeState
}

func newNodeStore(rm *resource.KubeResourceManager) *nodeStore {
	return &nodeStore{
		rm:     rm,
		states: map[string]nodeState{},
	}
}

func (n *nodeStore) isNodeReady(nodeName string) (bool, error) {
	if state, ok := n.states[nodeName]; ok {
		return state == nodeStateReady, nil
	}

	state := nodeStateNotExist
	node, err := n.rm.GetNode(nodeName)
	if err != nil && !k8serrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to get node %q, detail-> %v", nodeName, err)
	}

	if err == nil {
		if nodeutils.IsNodeReady(node) {
			state = nodeStateReady
		} else {
			state = nodeStateUnReady
		}
	}

	n.states[nodeName] = nodeState(state)
	return state == nodeStateReady, nil
}

func (kp *K8sProvider) cleanupZombieResources(ctx context.Context) error {
	nlog.Debugf("Start cleaning up zombie resources ...")

	ns := newNodeStore(kp.resourceManager)

	if err := kp.cleanupZombiePods(ctx, ns); err != nil {
		nlog.Errorf("Failed to cleanup zombie pods, detail-> %v", err)
	}

	// TODO cleanup zombie leases

	return nil
}

func (kp *K8sProvider) cleanupZombiePods(ctx context.Context, ns *nodeStore) error {
	bkPodList, err := kp.podLister.List(labels.SelectorFromSet(labels.Set{common.LabelNodeNamespace: kp.namespace}))
	if err != nil {
		return fmt.Errorf("failed to list pod, detail-> %v", err)
	}

	for _, bkPod := range bkPodList {
		isZombie, err := kp.isZombiePod(bkPod, ns)
		if err != nil {
			nlog.Errorf("Check if pod %v is zombie failed, detail-> %v", format.Pod(bkPod), err)
			continue
		}

		if !isZombie {
			continue
		}

		if err := kp.deleteZombiePod(ctx, bkPod); err != nil {
			nlog.Errorf("Failed to delete zombie pod %v: %v", format.Pod(bkPod), err)
		}

		nlog.Infof("Delete zombie backend pod %v successfully", format.Pod(bkPod))
	}

	return nil
}

func (kp *K8sProvider) isZombiePod(bkPod *v1.Pod, ns *nodeStore) (bool, error) {
	if bkPod.Labels == nil || bkPod.Labels[common.LabelNodeName] == "" {
		return false, nil
	}

	ownerNodeName := bkPod.Labels[common.LabelNodeName]

	ready, err := ns.isNodeReady(ownerNodeName)
	if err != nil {
		return false, fmt.Errorf("failed to check node %q is ready: %v", ownerNodeName, err)
	}
	if ready {
		return false, nil
	}

	masterPod, err := kp.resourceManager.GetPod(bkPod.Name)
	if k8serrors.IsNotFound(err) {
		nlog.Infof("Node %v is not exist or unready and master pod is not exist, backend pod %v is zombie pod", ownerNodeName, format.Pod(bkPod))
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to get master pod %v: %v", format.Pod(bkPod), err)
	}

	if masterPod.DeletionTimestamp != nil {
		nlog.Infof("Node %v is not exist or unready and master pod is terminating, backend pod %v is zombie pod", ownerNodeName, format.Pod(bkPod))
		return true, nil
	}

	return false, nil
}

func (kp *K8sProvider) deleteZombiePod(ctx context.Context, bkPod *v1.Pod) error {
	if err := kp.deleteBackendPod(ctx, bkPod.Namespace, bkPod.Name, nil); err != nil {
		return err
	}

	return nil
}

func (kp *K8sProvider) cleanupSubResources(ctx context.Context) error {
	configMaps, err := kp.configMapLister.List(labels.SelectorFromSet(labels.Set{common.LabelNodeNamespace: kp.namespace}))
	if err != nil {
		return fmt.Errorf("failed to list configmap, detail-> %v", err)
	}

	secrets, err := kp.secretLister.List(labels.SelectorFromSet(labels.Set{common.LabelNodeNamespace: kp.namespace}))
	if err != nil {
		return fmt.Errorf("failed to list secret, detail-> %v", err)
	}

	for _, configMap := range configMaps {
		if err := cleanupSubResource[*v1.ConfigMap](ctx, configMap, kp.bkClient.CoreV1().Pods(kp.bkNamespace), kp.bkClient.CoreV1().ConfigMaps(kp.bkNamespace)); err != nil {
			nlog.Warnf("Failed to cleanup configmap %q: %v", configMap.Name, err)
		}
	}

	for _, secret := range secrets {
		if err := cleanupSubResource[*v1.Secret](ctx, secret, kp.bkClient.CoreV1().Pods(kp.bkNamespace), kp.bkClient.CoreV1().Secrets(kp.bkNamespace)); err != nil {
			nlog.Warnf("Failed to cleanup secret %q: %v", secret.Name, err)
		}
	}

	return nil
}

// onNewLeader is executed when leader is changed.
func (kp *K8sProvider) onNewLeader(identity string) {
	nlog.Infof("New leader has been elected: %s", identity)
}

// onStartedLeading is executed when leader started.
func (kp *K8sProvider) onStartedLeading(ctx context.Context) {
	nlog.Infof("K8s provider %v start leading", kp.nodeName)

	ticker := time.NewTicker(time.Minute)

	for {
		select {
		case <-ctx.Done():
			nlog.Infof("Context is done, exit leading")
			return
		case <-ticker.C:
			if err := kp.cleanupZombieResources(context.Background()); err != nil {
				nlog.Errorf("Failed to cleanup zombie pods: %v", err)
			}

			if err := kp.cleanupSubResources(context.Background()); err != nil {
				nlog.Errorf("Failed to cleanup sub resource: %v", err)
			}
		}
	}
}

// onStoppedLeading is executed when leader stopped.
func (kp *K8sProvider) onStoppedLeading() {
	nlog.Warnf("K8s provider %v Leading stopped", kp.nodeName)
}
