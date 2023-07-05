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

//nolint:dulp
package hostresources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runPodWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runPodWorker() {
	ctx := context.Background()
	queueName := generateQueueName(hostPodsQueueName, c.host, c.member)
	for queue.HandleQueueItem(ctx, queueName, c.podsQueue, c.syncPodHandler, maxRetries) {
	}
}

// handleAddedOrDeletedPod is used to handle added or deleted pod.
func (c *hostResourcesController) handleAddedOrDeletedPod(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.podsQueue)
}

// handleUpdatedPod is used to handle updated pod.
func (c *hostResourcesController) handleUpdatedPod(oldObj, newObj interface{}) {
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
	queue.EnqueueObjectWithKey(newPod, c.podsQueue)
}

// Todo: Abstract into common interface
// syncPodHandler is used to sync pod between host and member cluster.
func (c *hostResourcesController) syncPodHandler(ctx context.Context, key string) error {
	namespace, name, err := getNamespaceAndNameFromKey(key, c.member)
	if err != nil {
		nlog.Error(err)
		return nil
	}

	hPod, err := c.hostPodLister.Pods(namespace).Get(name)
	if err != nil {
		// pod is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Pod %v may be deleted under host %v cluster, so delete the pod under member cluster", key, c.host)
			if err = c.memberKubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			return nil
		}
		return err
	}

	hPodCopy := hPod.DeepCopy()
	hrv := hPodCopy.ResourceVersion

	hExtractedPodSpec := utilsres.ExtractPodSpec(hPodCopy)
	hExtractedPodSpec.Labels[common.LabelResourceVersionUnderHostCluster] = hrv
	hExtractedPodSpec.Spec.SchedulerName = common.KusciaSchedulerName

	mPod, err := c.memberPodLister.Pods(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Failed to get pod %v under member cluster, create it...", key)
			if _, err = c.memberKubeClient.CoreV1().Pods(namespace).Create(ctx, hExtractedPodSpec, metav1.CreateOptions{}); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					return nil
				}
				return fmt.Errorf("create pod %v from host cluster failed, %v", key, err.Error())
			}
			return nil
		}
		return err
	}

	mPodCopy := mPod.DeepCopy()
	mrv := mPodCopy.Labels[common.LabelResourceVersionUnderHostCluster]
	if !utilsres.CompareResourceVersion(hrv, mrv) {
		nlog.Infof("Pod %v resource version %v under host cluster is not greater than the value %v under member cluster, skip this event...", key, hrv, mrv)
		return nil
	}

	mExtractedPodSpec := utilsres.ExtractPodSpec(mPodCopy)
	if err = utilsres.PatchPod(ctx, c.memberKubeClient, mExtractedPodSpec, hExtractedPodSpec); err != nil {
		return fmt.Errorf("patch member pod %v failed, %v", key, err.Error())
	}
	return nil
}
