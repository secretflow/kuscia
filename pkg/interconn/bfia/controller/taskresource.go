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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// handleAddedOrDeletedTaskResource handles added or deleted task resource.
func (c *Controller) handleAddedOrDeletedTaskResource(obj interface{}) {
	_, ok := obj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResource", obj)
		return
	}

	queue.EnqueueObjectWithKey(obj, c.trQueue)
}

// handleUpdatedTaskResource handles updated task resource.
func (c *Controller) handleUpdatedTaskResource(oldObj, newObj interface{}) {
	oldTr, ok := oldObj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResource", oldObj)
		return
	}

	newTr, ok := newObj.(*kusciaapisv1alpha1.TaskResource)
	if !ok {
		nlog.Warnf("Object %#v is not a TaskResource", newObj)
		return
	}

	if oldTr.ResourceVersion == newTr.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newTr, c.trQueue)
}

// runTaskResourceWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runTaskResourceWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, taskResourceQueueName, c.trQueue, c.syncTaskResourceHandler, maxRetries) {
	}
}

// syncTaskResourceHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncTaskResourceHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Split meta namespace key %v failed: %v", key, err.Error())
		return nil
	}
	rawTr, err := c.trLister.TaskResources(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("TaskResource %v maybe deleted, skip to handle it", key)
			return nil
		}
		return err
	}

	if rawTr.DeletionTimestamp != nil {
		nlog.Infof("TaskResource %v is terminating, skip to handle it", key)
		return nil
	}

	if rawTr.Status.CompletionTime != nil {
		nlog.Infof("TaskResource %s is finished, skip to handle it", key)
		return nil
	}

	return c.handleTaskResource(ctx, rawTr.DeepCopy(), key)
}

// handleTaskResource handles task resource.
func (c *Controller) handleTaskResource(ctx context.Context, tr *kusciaapisv1alpha1.TaskResource, key string) error {
	var jobID, taskID, taskName string
	if tr.Annotations != nil {
		jobID = tr.Annotations[common.JobIDAnnotationKey]
		taskID = tr.Annotations[common.TaskIDAnnotationKey]
		taskName = tr.Annotations[common.TaskAliasAnnotationKey]
	}

	if jobID == "" || taskID == "" || taskName == "" {
		return fmt.Errorf("task resource %v/%v labels job id/task id/task alias can't be empty", tr.Namespace, tr.Name)
	}

	cacheKey := getCacheKeyName(reqTypeStartTask, resourceTypeTaskResource, fmt.Sprintf("%v/%v", tr.Namespace, taskID))
	if _, ok := c.inflightRequestCache.Get(cacheKey); ok {
		c.trQueue.AddAfter(key, 2*time.Second)
		return nil
	}

	_ = c.inflightRequestCache.Add(cacheKey, "", inflightRequestCacheExpiration)

	go func() {
		defer c.inflightRequestCache.Delete(cacheKey)
		rawKt, err := c.ktLister.KusciaTasks(common.KusciaCrossDomain).Get(taskID)
		if err != nil {
			message := fmt.Sprintf("get kuscia task %v failed, %v", taskID, err)
			_ = c.updateTaskResourceStatus(tr, kusciaapisv1alpha1.TaskResourcePhaseFailed, kusciaapisv1alpha1.TaskResourceCondReserved, corev1.ConditionFalse, message)
			return
		}

		kt := rawKt.DeepCopy()
		nlog.Infof("Send task %s:%s start request", tr.Namespace, taskID)
		_, startTaskErr := c.bfiaClient.StartTask(ctx, c.getReqDomainIDFromKusciaTask(kt), buildHostFor(tr.Namespace), jobID, taskID, taskName)
		if startTaskErr != nil {
			nlog.Errorf("Send task %s:%s start request failed, error: %v", tr.Namespace, taskID, startTaskErr)
			message := fmt.Sprintf("start task request failed, %v", startTaskErr)
			c.setPartyTaskStatuses(kt, tr.Namespace, message, kusciaapisv1alpha1.TaskFailed)
			return
		}

		nlog.Infof("Set party task %s and taskresource %s:%s status", kt.Name, tr.Namespace, tr.Name)
		c.setPartyTaskStatuses(kt, tr.Namespace, "", kusciaapisv1alpha1.TaskPending)
		err = c.updateTaskResourceStatus(tr, kusciaapisv1alpha1.TaskResourcePhaseReserved, kusciaapisv1alpha1.TaskResourceCondReserved, corev1.ConditionTrue, "Start interconn task succeeded")
		if err != nil {
			nlog.Errorf("Update partner taskresources %s:%s to reserved failed, %v", tr.Namespace, tr.Name, err)
		}
	}()

	return nil
}

// setPartyTaskStatuses sets party task statuses.
func (c *Controller) setPartyTaskStatuses(task *kusciaapisv1alpha1.KusciaTask, domainID, message string, targetPhase kusciaapisv1alpha1.KusciaTaskPhase) {
	hasSet := false
	for _, party := range task.Spec.Parties {
		if party.DomainID == domainID {
			updated := setPartyTaskStatus(&task.Status, party.DomainID, party.Role, message, targetPhase)
			if updated {
				hasSet = true
			}
		}
	}

	if hasSet {
		_ = c.updatePartyTaskStatus(task)
	}
}

// updateTaskResourcesStatus updates task resources status.
func (c *Controller) updateTaskResourceStatus(tr *kusciaapisv1alpha1.TaskResource,
	phase kusciaapisv1alpha1.TaskResourcePhase,
	condType kusciaapisv1alpha1.TaskResourceConditionType,
	condStatus corev1.ConditionStatus,
	reason string) error {

	now := metav1.Now().Rfc3339Copy()
	trCopy := tr.DeepCopy()
	trCopy.Status.LastTransitionTime = &now
	trCopy.Status.Phase = phase
	trCopy.Status.CompletionTime = &now
	trCond := utilsres.GetTaskResourceCondition(&trCopy.Status, condType)
	trCond.LastTransitionTime = &now
	trCond.Status = condStatus
	trCond.Reason = reason
	if err := utilsres.PatchTaskResource(context.Background(), c.kusciaClient, utilsres.ExtractTaskResourceStatus(tr), utilsres.ExtractTaskResourceStatus(trCopy)); err != nil {
		return fmt.Errorf("patch interconn task resource %v/%v status failed, %v", trCopy.Namespace, trCopy.Name, err)
	}

	return nil
}
