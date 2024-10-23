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
package kuscia

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runTaskWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runTaskWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, taskQueueName, c.taskQueue, c.syncTaskHandler, maxRetries) {
	}
}

// handleAddedTask is used to handle added task.
func (c *Controller) handleAddedTask(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.taskQueue)
}

// handleUpdatedTask is used to handle updated task.
func (c *Controller) handleUpdatedTask(oldObj, newObj interface{}) {
	oldTask, ok := oldObj.(*v1alpha1.KusciaTask)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaTask", oldObj)
		return
	}

	newTask, ok := newObj.(*v1alpha1.KusciaTask)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaTask", newObj)
		return
	}

	if oldTask.ResourceVersion == newTask.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newTask, c.taskQueue)
}

// handleDeletedTask is used to handle deleted task.
func (c *Controller) handleDeletedTask(obj interface{}) {
	task, ok := obj.(*v1alpha1.KusciaTask)
	if !ok {
		return
	}

	masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, task)
	if err != nil {
		nlog.Errorf("Failed to delete task summaries of task %v, %v", ikcommon.GetObjectNamespaceName(task), err)
		return
	}

	for masterDomain := range masterDomains {
		c.taskQueue.Add(fmt.Sprintf("%v%v/%v", ikcommon.DeleteEventKeyPrefix, masterDomain, task.Name))
	}
}

// syncTaskHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncTaskHandler(ctx context.Context, key string) (err error) {
	key, deleteEvent := ikcommon.IsOriginalResourceDeleteEvent(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split task key %v, %v, skip processing it", key, err)
		return nil
	}

	if deleteEvent {
		nlog.Infof("Delete task %v cascaded resources", name)
		return c.deleteTaskCascadedResources(ctx, namespace, name)
	}

	originalTask, err := c.taskLister.KusciaTasks(namespace).Get(name)
	if err != nil {
		// task was deleted in cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get task %v, maybe it's deleted, skip processing it", key)
			return nil
		}
		return err
	}

	selfIsInitiator, err := ikcommon.SelfClusterIsInitiator(c.domainLister, originalTask)
	if err != nil {
		return err
	}

	task := originalTask.DeepCopy()
	if selfIsInitiator {
		return c.processTaskAsInitiator(ctx, task)
	}

	return c.processTaskAsPartner(ctx, task)
}

func (c *Controller) deleteTaskCascadedResources(ctx context.Context, namespace, name string) error {
	ts, err := c.taskSummaryLister.KusciaTaskSummaries(namespace).Get(name)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if ts == nil {
		return nil
	}

	err = c.kusciaClient.KusciaV1alpha1().KusciaTaskSummaries(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Controller) processTaskAsInitiator(ctx context.Context, task *v1alpha1.KusciaTask) error {
	return c.createOrUpdateTaskSummary(ctx, task)
}

// createOrUpdateTaskSummary creates or updates job summary.
func (c *Controller) createOrUpdateTaskSummary(ctx context.Context, task *v1alpha1.KusciaTask) error {
	masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, task)
	if err != nil {
		nlog.Errorf("Failed to create or update taskSummary for task %v, %v", ikcommon.GetObjectNamespaceName(task), err)
		return nil
	}

	for masterDomainID, partyDomainIDs := range masterDomains {
		taskSummary, err := c.taskSummaryLister.KusciaTaskSummaries(masterDomainID).Get(task.Name)
		if err != nil {
			// create taskSummary
			if k8serrors.IsNotFound(err) {
				if err = c.createTaskSummary(ctx, task, masterDomainID, partyDomainIDs); err != nil {
					return err
				}
				continue
			}
			return err
		}

		if taskSummary.Status.CompletionTime != nil {
			continue
		}
		// update taskSummary
		if err = c.updateTaskSummaryByTask(ctx, task, taskSummary.DeepCopy()); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) createTaskSummary(ctx context.Context, task *v1alpha1.KusciaTask, masterDomainID string, partyDomainIDs []string) error {
	taskSummary := &v1alpha1.KusciaTaskSummary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      task.Name,
			Namespace: masterDomainID,
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:            ikcommon.GetObjectAnnotation(task, common.InitiatorAnnotationKey),
				common.InterConnKusciaPartyAnnotationKey: strings.Join(partyDomainIDs, "_"),
			},
		},
		Spec: v1alpha1.KusciaTaskSummarySpec{
			Alias: ikcommon.GetObjectAnnotation(task, common.TaskAliasAnnotationKey),
			JobID: ikcommon.GetObjectAnnotation(task, common.JobIDAnnotationKey),
		},
		Status: v1alpha1.KusciaTaskSummaryStatus{
			KusciaTaskStatus: v1alpha1.KusciaTaskStatus{
				Phase:           task.Status.Phase,
				PartyTaskStatus: task.Status.PartyTaskStatus,
				StartTime:       ikcommon.GetCurrentTime(),
			},
			ResourceStatus: make(map[string][]*v1alpha1.TaskSummaryResourceStatus),
		},
	}
	_, err := c.kusciaClient.KusciaV1alpha1().KusciaTaskSummaries(masterDomainID).Create(ctx, taskSummary, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *Controller) updateTaskSummaryByTask(ctx context.Context, task *v1alpha1.KusciaTask, taskSummary *v1alpha1.KusciaTaskSummary) error {
	now := ikcommon.GetCurrentTime()
	needUpdate := false
	if task.Status.Phase != taskSummary.Status.Phase {
		needUpdate = true
		taskSummary.Status.Phase = task.Status.Phase
	}

	if task.Status.CompletionTime != nil && taskSummary.Status.CompletionTime == nil {
		needUpdate = true
		taskSummary.Status.CompletionTime = task.Status.CompletionTime
	}

	if updateTaskSummaryPartyTaskStatus(task, taskSummary, true) {
		needUpdate = true
	}

	if needUpdate {
		taskSummary.Status.LastReconcileTime = now
		if _, err := c.kusciaClient.KusciaV1alpha1().KusciaTaskSummaries(taskSummary.Namespace).Update(ctx, taskSummary, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update taskSummary %v resource status based on task %v, %v",
				ikcommon.GetObjectNamespaceName(taskSummary), ikcommon.GetObjectNamespaceName(task), err)
		}
	}

	return nil
}

func updateTaskSummaryPartyTaskStatus(task *v1alpha1.KusciaTask, taskSummary *v1alpha1.KusciaTaskSummary, isHost bool) bool {
	if len(task.Status.PartyTaskStatus) == 0 || reflect.DeepEqual(task.Status.PartyTaskStatus, taskSummary.Status.PartyTaskStatus) {
		return false
	}

	if len(taskSummary.Status.PartyTaskStatus) == 0 {
		taskSummary.Status.PartyTaskStatus = task.Status.PartyTaskStatus
		return true
	}

	ptsInTaskSummaryMap := make(map[string]v1alpha1.PartyTaskStatus)
	for _, pts := range taskSummary.Status.PartyTaskStatus {
		key := fmt.Sprintf("%s:%s", pts.DomainID, pts.Role)
		ptsInTaskSummaryMap[key] = pts
	}

	updated := false
	if isHost {
		for _, pts := range task.Status.PartyTaskStatus {
			if pts.DomainID == taskSummary.Namespace {
				continue
			}
			key := fmt.Sprintf("%s:%s", pts.DomainID, pts.Role)
			if !reflect.DeepEqual(ptsInTaskSummaryMap[key], pts) {
				updated = true
				ptsInTaskSummaryMap[key] = pts
			}
		}
	} else {
		domainIDs := ikcommon.GetSelfClusterPartyDomainIDs(task)
		if len(domainIDs) == 0 {
			nlog.Warnf("Failed to get self cluster party domain ids from task %v, skip updating host task summary party task status", ikcommon.GetObjectNamespaceName(task))
			return false
		}

		ptsInTaskMap := make(map[string]v1alpha1.PartyTaskStatus)
		for _, domainID := range domainIDs {
			for _, pts := range task.Status.PartyTaskStatus {
				if domainID == pts.DomainID {
					key := fmt.Sprintf("%s:%s", pts.DomainID, pts.Role)
					ptsInTaskMap[key] = pts
				}
			}
		}

		for key, pts := range ptsInTaskMap {
			if !reflect.DeepEqual(ptsInTaskSummaryMap[key], pts) {
				updated = true
				ptsInTaskSummaryMap[key] = pts
			}
		}
	}

	if updated {
		taskSummary.Status.PartyTaskStatus = nil
		for _, status := range ptsInTaskSummaryMap {
			taskSummary.Status.PartyTaskStatus = append(taskSummary.Status.PartyTaskStatus, status)
		}
	}

	return updated
}

func (c *Controller) processTaskAsPartner(ctx context.Context, task *v1alpha1.KusciaTask) error {
	initiator := ikcommon.GetObjectAnnotation(task, common.InitiatorAnnotationKey)
	if initiator == "" {
		nlog.Warnf("Failed to get initiator from task %v, skip processing it", ikcommon.GetObjectNamespaceName(task))
		return nil
	}

	masterDomainID := ikcommon.GetObjectAnnotation(task, common.KusciaPartyMasterDomainAnnotationKey)
	if masterDomainID == "" {
		nlog.Warnf("Failed to get master domain id from task %v, skip processing it", ikcommon.GetObjectNamespaceName(task))
		return nil
	}

	hostMasterDomainID, err := utilsres.GetMasterDomain(c.domainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}

	hra := c.hostResourceManager.GetHostResourceAccessor(hostMasterDomainID, masterDomainID)
	if hra == nil {
		return fmt.Errorf("host resource accessor for task %v is empty of host/member %v/%v, retry",
			ikcommon.GetObjectNamespaceName(task), initiator, masterDomainID)
	}

	if !hra.HasSynced() {
		return fmt.Errorf("host %v resource accessor has not synced, retry", initiator)
	}

	hTaskSummary, err := hra.HostTaskSummaryLister().KusciaTaskSummaries(masterDomainID).Get(task.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			hTaskSummary, err = hra.HostKusciaClient().KusciaV1alpha1().KusciaTaskSummaries(masterDomainID).Get(ctx, task.Name, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Failed to get taskSummary %v from host %v cluster, %v", task.Name, initiator, err)
			if k8serrors.IsNotFound(err) {
				nlog.Infof("Host taskSummary %s/%s is not found, delete task %v", masterDomainID, task.Name, ikcommon.GetObjectNamespaceName(task))
				err = c.kusciaClient.KusciaV1alpha1().KusciaTasks(task.Namespace).Delete(ctx, task.Name, metav1.DeleteOptions{})
				if k8serrors.IsNotFound(err) {
					return nil
				}
			}
			return err
		}
	}

	if hTaskSummary.Status.CompletionTime != nil {
		return nil
	}

	return c.updateHostTaskSummary(ctx, hra.HostKusciaClient(), task, hTaskSummary.DeepCopy())
}

func (c *Controller) updateHostTaskSummary(ctx context.Context,
	kusciaClient kusciaclientset.Interface,
	task *v1alpha1.KusciaTask,
	taskSummary *v1alpha1.KusciaTaskSummary) error {

	needUpdate := false
	if updateTaskSummaryPartyTaskStatus(task, taskSummary, false) {
		needUpdate = true
	}

	if needUpdate {
		taskSummary.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err := kusciaClient.KusciaV1alpha1().KusciaTaskSummaries(taskSummary.Namespace).Update(ctx, taskSummary, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update host taskSummary %v resource status based on task %v, %v",
				ikcommon.GetObjectNamespaceName(taskSummary), ikcommon.GetObjectNamespaceName(task), err)
		}
	}

	return nil
}
