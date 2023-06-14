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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

// runTaskResourceWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runTaskResourceWorker(ctx context.Context) {
	for queue.HandleQueueItem(context.Background(), taskResourceQueueName, c.taskResourceQueue, c.syncTaskResourceHandler, maxRetries) {
	}
}

// handleAddedTaskResource is used to handle added task resource.
func (c *Controller) handleAddedTaskResource(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.taskResourceQueue)
}

// handleUpdatedTaskResource is used to handle updated task resource.
func (c *Controller) handleUpdatedTaskResource(oldObj, newObj interface{}) {
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

	queue.EnqueueObjectWithKey(newTr, c.taskResourceQueue)
}

// handleDeletedTaskResource is used to handle deleted task resource.
func (c *Controller) handleDeletedTaskResource(obj interface{}) {
	tr, ok := obj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		return
	}

	host := tr.Labels[common.LabelTaskInitiator]
	hra := c.hostResourceManager.GetHostResourceAccessor(host, tr.Namespace)
	if hra == nil {
		nlog.Warnf("Host resource accessor for task resource %v is empty of host/member %v/%v, skip this event...", tr.Name, host, tr.Namespace)
		return
	}

	hra.EnqueueTaskResource(fmt.Sprintf("%s/%s", tr.Namespace, tr.Name))
}

// Todo: Abstract into common interface
// syncTaskResourceHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncTaskResourceHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Split pod key %v failed, %v", key, err.Error())
		return nil
	}

	mTr, err := c.taskResourceLister.TaskResources(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Warnf("Can't get task resource %v under member cluster, skip this event...", key)
			return nil
		}
		return err
	}

	host := mTr.Labels[common.LabelTaskInitiator]
	hra := c.hostResourceManager.GetHostResourceAccessor(host, namespace)
	if hra == nil {
		nlog.Warnf("Host resource accessor for task resource %v is empty of host/member %v/%v, skip this event", key, host, namespace)
		return nil
	}

	if !hra.HasSynced() {
		return fmt.Errorf("host %v resource accessor has not synced, retry", host)
	}

	hTr, err := hra.HostTaskResourceLister().TaskResources(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Warnf("Task resource %v may be deleted under host %v cluster, skip this event...", key, host)
			return nil
		}
		nlog.Errorf("Get task resource %v under host %v cluster failed, %v", key, host, err.Error())
		return err
	}

	rvInLabel := mTr.Labels[common.LabelResourceVersionUnderHostCluster]
	if utilscommon.CompareResourceVersion(hTr.ResourceVersion, rvInLabel) {
		return fmt.Errorf("task resource %v label resource version %v in member cluster is less than the value %v in host cluster, retry", key, rvInLabel, hTr.ResourceVersion)
	}

	hExtractedTrStatus := utilscommon.ExtractTaskResourceStatus(hTr.DeepCopy())
	mExtractedTrStatus := utilscommon.ExtractTaskResourceStatus(mTr.DeepCopy())
	if err = utilscommon.PatchTaskResource(ctx, hra.HostKusciaClient(), hExtractedTrStatus, mExtractedTrStatus); err != nil {
		return fmt.Errorf("patch member task resource  %v status to host cluster failed, %v", key, err.Error())
	}
	return nil
}
