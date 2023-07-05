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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runTaskResourceWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runTaskResourceWorker() {
	ctx := context.Background()
	queueName := generateQueueName(hostTaskResourcesQueueName, c.host, c.member)
	for queue.HandleQueueItem(ctx, queueName, c.taskResourcesQueue, c.syncTaskResourceHandler, maxRetries) {
	}
}

// handleAddedOrDeletedTaskResource is used to handle added or deleted task resource.
func (c *hostResourcesController) handleAddedOrDeletedTaskResource(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.taskResourcesQueue)
}

// handleUpdatedTaskResource is used to handle updated task resource.
func (c *hostResourcesController) handleUpdatedTaskResource(oldObj, newObj interface{}) {
	oldTr, ok := oldObj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		nlog.Errorf("Object %#v is not a TaskResource", oldObj)
		return
	}

	newTr, ok := newObj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		nlog.Errorf("Object %#v is not a TaskResource", newObj)
		return
	}

	if oldTr.ResourceVersion == newTr.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newTr, c.taskResourcesQueue)
}

// Todo: Abstract into common interface
// syncTaskResourceHandler is used to sync task resource between host and member cluster.
func (c *hostResourcesController) syncTaskResourceHandler(ctx context.Context, key string) error {
	namespace, name, err := getNamespaceAndNameFromKey(key, c.member)
	if err != nil {
		nlog.Error(err)
		return nil
	}

	hTr, err := c.hostTrLister.TaskResources(namespace).Get(name)
	if err != nil {
		// tr is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Task resource %v may be deleted under host %v cluster, so delete the task resource under member cluster", key, c.host)
			if err = c.memberKusciaClient.KusciaV1alpha1().TaskResources(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			return nil
		}
		return err
	}

	hTrCopy := hTr.DeepCopy()
	hrv := hTrCopy.ResourceVersion

	hExtractedTr := utilsres.ExtractTaskResource(hTrCopy)
	hExtractedTr.Labels[common.LabelResourceVersionUnderHostCluster] = hrv

	mTr, err := c.memberTrLister.TaskResources(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Failed to get task resource %v under member cluster, create it...", key)
			if _, err = c.memberKusciaClient.KusciaV1alpha1().TaskResources(namespace).Create(ctx, hExtractedTr, metav1.CreateOptions{}); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					return nil
				}
				return fmt.Errorf("create task resource %v from host cluster failed, %v", key, err.Error())
			}
			return nil
		}
		return err
	}

	mTrCopy := mTr.DeepCopy()
	mrv := mTrCopy.Labels[common.LabelResourceVersionUnderHostCluster]
	if !utilsres.CompareResourceVersion(hrv, mrv) {
		nlog.Infof("Task resource %v resource version %v under host cluster is not greater than the value %v under member cluster, skip this event...", key, hrv, mrv)
		return nil
	}

	mExtractedTr := utilsres.ExtractTaskResource(mTrCopy)
	if err = utilsres.PatchTaskResource(ctx, c.memberKusciaClient, mExtractedTr, hExtractedTr); err != nil {
		return fmt.Errorf("patch member task resource %v failed, %v", key, err.Error())
	}
	return nil
}
