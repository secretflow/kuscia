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

package interop

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	podHasSynced = "true"
)

// runPodWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runPodWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, podQueueName, c.podQueue, c.syncPodHandler, maxRetries) {
	}
}

// handleAddedPod is used to handle added pod.
func (c *Controller) handleAddedPod(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.podQueue)
}

// handleUpdatedPod is used to handle updated pod.
func (c *Controller) handleUpdatedPod(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*corev1.Pod)
	if !ok {
		nlog.Errorf("Object %#v is not a Pod", oldObj)
		return
	}

	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		nlog.Errorf("Object %#v is not a Pod", newObj)
		return
	}

	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newPod, c.podQueue)
}

// handleDeletedPod is used to handle deleted pod.
func (c *Controller) handleDeletedPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	host := pod.Labels[common.LabelTaskInitiator]
	hra := c.hostResourceManager.GetHostResourceAccessor(host, pod.Namespace)
	if hra == nil {
		nlog.Warnf("Host resource accessor for pod %v is empty of host/member %v/%v, skip this event...", pod.Name, host, pod.Namespace)
		return
	}

	hra.EnqueuePod(fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
}

// Todo: Abstract into common interface
// syncPodHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncPodHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Split pod key %v failed, %v", key, err.Error())
		return nil
	}

	mPod, err := c.podLister.Pods(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Warnf("Can't get pod %v under member cluster, skip this event...", key)
			return nil
		}
		return err
	}

	host := mPod.Labels[common.LabelTaskInitiator]
	hra := c.hostResourceManager.GetHostResourceAccessor(host, namespace)
	if hra == nil {
		nlog.Warnf("Host resource accessor for pod %v is empty of host/member %v/%v, skip this event", key, host, namespace)
		return nil
	}

	if !hra.HasSynced() {
		return fmt.Errorf("host %v resource accessor has not synced, retry", host)
	}

	hPod, err := hra.HostPodLister().Pods(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Warnf("Pod %v may be deleted under host %v cluster, skip this event...", key, host)
			return nil
		}
		nlog.Errorf("Get pod %v under host %v cluster failed, %v", key, host, err.Error())
		return err
	}

	rvInLabel := mPod.Labels[common.LabelResourceVersionUnderHostCluster]
	if utilscommon.CompareResourceVersion(hPod.ResourceVersion, rvInLabel) {
		return fmt.Errorf("pod %v label resource version %v in member cluster is less than the value %v under host cluster, retry", key, rvInLabel, hPod.ResourceVersion)
	}

	hPodCopy := hPod.DeepCopy()
	if hPodCopy.Labels == nil || (hPodCopy.Labels != nil && hPodCopy.Labels[common.LabelPodHasSynced] != podHasSynced) {
		extractedHPodLabels := utilscommon.ExtractPodLabels(hPodCopy)
		if extractedHPodLabels.Labels == nil {
			extractedHPodLabels.Labels = make(map[string]string)
		}

		extractedHPodLabelsCopy := extractedHPodLabels.DeepCopy()
		extractedHPodLabelsCopy.Labels[common.LabelPodHasSynced] = podHasSynced
		if err = utilscommon.PatchPod(ctx, hra.HostKubeClient(), extractedHPodLabels, extractedHPodLabelsCopy); err != nil {
			return fmt.Errorf("patch pod %v label %v under host cluster failed, %v", key, common.LabelPodHasSynced, err.Error())
		}
	}

	mPodCopy := mPod.DeepCopy()
	extractedMPodStatus := utilscommon.ExtractPodStatus(mPodCopy)
	extractedHPodStatus := utilscommon.ExtractPodStatus(hPodCopy)
	if err = utilscommon.PatchPodStatus(ctx, hra.HostKubeClient(), extractedHPodStatus, extractedMPodStatus); err != nil {
		return fmt.Errorf("patch member pod %v status to host cluster failed, %v", key, err.Error())
	}
	return nil
}
