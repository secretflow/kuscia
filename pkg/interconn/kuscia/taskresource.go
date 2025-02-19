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

//nolint:dupl
package kuscia

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runTaskResourceWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runTaskResourceWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, taskResourceQueueName, c.taskResourceQueue, c.syncTaskResourceHandler, maxRetries) {
	}
}

// handleAddedTaskResource is used to handle added taskResource.
func (c *Controller) handleAddedTaskResource(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.taskResourceQueue)
}

// handleUpdatedTaskResource is used to handle updated taskResource.
func (c *Controller) handleUpdatedTaskResource(oldObj, newObj interface{}) {
	oldTr, ok := oldObj.(*v1alpha1.TaskResource)
	if !ok {
		nlog.Errorf("Object %#v is not a TaskResource", oldObj)
		return
	}

	newTr, ok := newObj.(*v1alpha1.TaskResource)
	if !ok {
		nlog.Errorf("Object %#v is not a TaskResource", newObj)
		return
	}

	if oldTr.ResourceVersion == newTr.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newTr, c.taskResourceQueue)
}

// syncTaskResourceHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncTaskResourceHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split taskResource key %v, %v, skip processing it", key, err)
		return nil
	}

	originalTr, err := c.taskResourceLister.TaskResources(namespace).Get(name)
	if err != nil {
		// taskResource was deleted in cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get taskResource %v, maybe it was deleted, skip processing it", key)
			return nil
		}
		return err
	}

	selfIsInitiator, err := ikcommon.SelfClusterIsInitiator(c.domainLister, originalTr)
	if err != nil {
		return err
	}

	tr := originalTr.DeepCopy()
	if selfIsInitiator {
		return c.processTaskSummaryAsInitiator(ctx, tr)
	}

	return c.processTaskSummaryAsPartner(ctx, tr)
}

func (c *Controller) processTaskSummaryAsInitiator(ctx context.Context, taskResource *v1alpha1.TaskResource) error {
	return c.updateTaskSummaryByTaskResource(ctx, taskResource)
}

func (c *Controller) updateTaskSummaryByTaskResource(ctx context.Context, taskResource *v1alpha1.TaskResource) error {
	taskID := ikcommon.GetObjectAnnotation(taskResource, common.TaskIDAnnotationKey)
	if taskID == "" {
		nlog.Warnf("Skip updating taskSummary status based on taskResource %v since task id is empty", ikcommon.GetObjectNamespaceName(taskResource))
		return nil
	}

	domain, err := c.domainLister.Get(taskResource.Namespace)
	if err != nil {
		return fmt.Errorf("failed to updating taskSummary resource status based on taskResource %v since getting master domain fail, %v", ikcommon.GetObjectNamespaceName(taskResource), err)
	}

	// only update partner taskResource status
	if domain.Spec.Role != v1alpha1.Partner {
		return nil
	}

	masterDomain := domain.Spec.MasterDomain
	if masterDomain == "" {
		masterDomain = domain.Name
	}

	originalTaskSummary, err := c.taskSummaryLister.KusciaTaskSummaries(masterDomain).Get(taskID)
	if err != nil {
		return fmt.Errorf("failed to updating taskSummary status based on taskResource %v since getting taskSummary fail, %v", ikcommon.GetObjectNamespaceName(taskResource), err)
	}

	updated := false
	taskSummary := originalTaskSummary.DeepCopy()
	statusInTaskSummary := getTaskSummaryResourceStatus(taskResource.Namespace, taskResource.Spec.Role, taskSummary)
	switch taskResource.Status.Phase {
	case v1alpha1.TaskResourcePhaseReserving:
		if statusInTaskSummary == nil {
			updated = true
			initTaskSummaryResourceStatus(taskResource, taskSummary)
			break
		}

		if skipHandlingTaskResource(taskResource, statusInTaskSummary) {
			break
		}

		if statusInTaskSummary.Phase == "" ||
			statusInTaskSummary.Phase == v1alpha1.TaskResourcePhasePending ||
			statusInTaskSummary.Phase == v1alpha1.TaskResourcePhaseFailed {
			nlog.Infof("Set taskSummary %v status from %q to %q", ikcommon.GetObjectNamespaceName(taskSummary), statusInTaskSummary.Phase, taskResource.Status.Phase)
			updated = true
			setTaskSummaryResourceStatus(taskResource, statusInTaskSummary)
		}
	case v1alpha1.TaskResourcePhaseFailed,
		v1alpha1.TaskResourcePhaseSchedulable:
		if statusInTaskSummary == nil {
			updated = true
			initTaskSummaryResourceStatus(taskResource, taskSummary)
			break
		}

		if skipHandlingTaskResource(taskResource, statusInTaskSummary) {
			break
		}

		nlog.Infof("Set taskSummary %v status from %q to %q", ikcommon.GetObjectNamespaceName(taskSummary), statusInTaskSummary.Phase, taskResource.Status.Phase)
		updated = true
		setTaskSummaryResourceStatus(taskResource, statusInTaskSummary)
	default:
	}

	if statusInTaskSummary != nil && statusInTaskSummary.HostTaskResourceName == "" {
		updated = true
		statusInTaskSummary.HostTaskResourceName = taskResource.Name
		statusInTaskSummary.HostTaskResourceVersion = taskResource.ResourceVersion
		statusInTaskSummary.LastTransitionTime = taskResource.Status.LastTransitionTime
		nlog.Infof("Filling host taskResource %v name and version", ikcommon.GetObjectNamespaceName(taskResource))
	}

	if updated {
		_, err = c.kusciaClient.KusciaV1alpha1().KusciaTaskSummaries(taskSummary.Namespace).Update(ctx, taskSummary, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update taskSummary %v status based on taskResource %v, %v",
				ikcommon.GetObjectNamespaceName(taskSummary), ikcommon.GetObjectNamespaceName(taskResource), err)
		}
	}

	return nil
}

func skipHandlingTaskResource(taskResource *v1alpha1.TaskResource, statusInTaskSummary *v1alpha1.TaskSummaryResourceStatus) bool {
	if taskResource.Status.Phase == statusInTaskSummary.Phase ||
		!utilsres.CompareResourceVersion(taskResource.ResourceVersion, statusInTaskSummary.HostTaskResourceVersion) {
		return true
	}
	return false
}

func initTaskSummaryResourceStatus(taskResource *v1alpha1.TaskResource, taskSummary *v1alpha1.KusciaTaskSummary) {
	statusInTaskSummary := &v1alpha1.TaskSummaryResourceStatus{}
	setTaskSummaryResourceStatus(taskResource, statusInTaskSummary)
	if taskSummary.Status.ResourceStatus == nil {
		taskSummary.Status.ResourceStatus = make(map[string][]*v1alpha1.TaskSummaryResourceStatus)
	}
	taskSummary.Status.ResourceStatus[taskResource.Namespace] = append(taskSummary.Status.ResourceStatus[taskResource.Namespace], statusInTaskSummary)
}

func setTaskSummaryResourceStatus(taskResource *v1alpha1.TaskResource, status *v1alpha1.TaskSummaryResourceStatus) {
	status.Role = taskResource.Spec.Role
	status.HostTaskResourceName = taskResource.Name
	status.HostTaskResourceVersion = taskResource.ResourceVersion
	status.Phase = taskResource.Status.Phase
	status.LastTransitionTime = taskResource.Status.LastTransitionTime

	if status.Phase == v1alpha1.TaskResourcePhaseSchedulable {
		status.CompletionTime = taskResource.Status.CompletionTime
	}
}

func (c *Controller) processTaskSummaryAsPartner(ctx context.Context, taskResource *v1alpha1.TaskResource) error {
	return c.updateHostTaskSummaryByTaskResource(ctx, taskResource)
}

func (c *Controller) updateHostTaskSummaryByTaskResource(ctx context.Context, taskResource *v1alpha1.TaskResource) error {
	taskID := ikcommon.GetObjectAnnotation(taskResource, common.TaskIDAnnotationKey)
	if taskID == "" {
		nlog.Warnf("Skip updating taskSummary status based on taskResource %v since label %v is empty",
			ikcommon.GetObjectNamespaceName(taskResource), common.TaskIDAnnotationKey)
		return nil
	}

	initiator := ikcommon.GetObjectAnnotation(taskResource, common.InitiatorAnnotationKey)
	if initiator == "" {
		nlog.Errorf("Skip updating taskSummary status based on taskResource %v since annotation %v is empty",
			ikcommon.GetObjectNamespaceName(taskResource), common.InitiatorAnnotationKey)
		return nil
	}

	masterDomainID := ikcommon.GetObjectAnnotation(taskResource, common.KusciaPartyMasterDomainAnnotationKey)
	if masterDomainID == "" {
		nlog.Errorf("Skip updating taskSummary status based on taskResource %v since annotation %v is empty",
			ikcommon.GetObjectNamespaceName(taskResource), common.KusciaPartyMasterDomainAnnotationKey)
		return nil
	}

	if c.hostResourceManager == nil {
		return fmt.Errorf("host resource manager for taskResource %v is empty of host/member %v/%v, retry",
			ikcommon.GetObjectNamespaceName(taskResource), initiator, masterDomainID)
	}

	hostMasterDomainID, err := utilsres.GetMasterDomain(c.domainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}
	hra := c.hostResourceManager.GetHostResourceAccessor(hostMasterDomainID, masterDomainID)
	if hra == nil {
		return fmt.Errorf("host resource accessor for taskResource %v is empty of host/member %v/%v, retry",
			ikcommon.GetObjectNamespaceName(taskResource), initiator, masterDomainID)
	}

	if !hra.HasSynced() {
		return fmt.Errorf("host %v resource accessor has not synced, retry", initiator)
	}

	hTaskSummary, err := hra.HostTaskSummaryLister().KusciaTaskSummaries(masterDomainID).Get(taskID)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			hTaskSummary, err = hra.HostKusciaClient().KusciaV1alpha1().KusciaTaskSummaries(masterDomainID).Get(ctx, taskID, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Failed to get taskSummary %v under host %v cluster, %v", taskID, initiator, err)
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}

	updated := false
	taskSummary := hTaskSummary.DeepCopy()
	statusInTaskSummary := getTaskSummaryResourceStatus(taskResource.Namespace, taskResource.Spec.Role, taskSummary)
	switch taskResource.Status.Phase {
	case v1alpha1.TaskResourcePhaseReserving,
		v1alpha1.TaskResourcePhaseReserved,
		v1alpha1.TaskResourcePhaseFailed:
		if statusInTaskSummary == nil {
			updated = true
			statusInTaskSummary = &v1alpha1.TaskSummaryResourceStatus{}
			setHostTaskSummaryResourceStatus(taskResource, statusInTaskSummary)
			if taskSummary.Status.ResourceStatus == nil {
				taskSummary.Status.ResourceStatus = make(map[string][]*v1alpha1.TaskSummaryResourceStatus)
			}
			taskSummary.Status.ResourceStatus[taskResource.Namespace] = append(taskSummary.Status.ResourceStatus[taskResource.Namespace], statusInTaskSummary)
			break
		}

		if statusInTaskSummary.Phase == taskResource.Status.Phase ||
			!utilsres.CompareResourceVersion(taskResource.ResourceVersion, statusInTaskSummary.MemberTaskResourceVersion) {
			return nil
		}

		if statusInTaskSummary.Phase == v1alpha1.TaskResourcePhaseReserving {
			if taskResource.Status.Phase == v1alpha1.TaskResourcePhaseReserved ||
				taskResource.Status.Phase == v1alpha1.TaskResourcePhaseFailed {
				updated = true
			}
		}
		if statusInTaskSummary.Phase == v1alpha1.TaskResourcePhaseReserved {
			if taskResource.Status.Phase == v1alpha1.TaskResourcePhaseFailed {
				updated = true
			}
		}
		if updated {
			nlog.Infof("Set host taskSummary %v status from %q to %q", ikcommon.GetObjectNamespaceName(taskSummary), statusInTaskSummary.Phase, taskResource.Status.Phase)
			setHostTaskSummaryResourceStatus(taskResource, statusInTaskSummary)
		}
	default:
	}

	if statusInTaskSummary != nil && statusInTaskSummary.MemberTaskResourceName == "" {
		updated = true
		statusInTaskSummary.MemberTaskResourceName = taskResource.Name
		statusInTaskSummary.MemberTaskResourceVersion = taskResource.ResourceVersion
		nlog.Infof("Filling member taskResource %v name and version", ikcommon.GetObjectNamespaceName(taskResource))
	}

	if updated {
		_, err = hra.HostKusciaClient().KusciaV1alpha1().KusciaTaskSummaries(taskSummary.Namespace).Update(ctx, taskSummary, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update host taskSummary %v status based on taskResource %v, %v",
				ikcommon.GetObjectNamespaceName(taskSummary), ikcommon.GetObjectNamespaceName(taskResource), err)
		}
	}
	return nil
}

func setHostTaskSummaryResourceStatus(taskResource *v1alpha1.TaskResource, status *v1alpha1.TaskSummaryResourceStatus) {
	status.Role = taskResource.Spec.Role
	status.MemberTaskResourceName = taskResource.Name
	status.MemberTaskResourceVersion = taskResource.ResourceVersion
	status.Phase = taskResource.Status.Phase
	status.StartTime = taskResource.Status.StartTime
	status.LastTransitionTime = taskResource.Status.LastTransitionTime
}

func getTaskSummaryResourceStatus(domainID, role string, taskSummary *v1alpha1.KusciaTaskSummary) *v1alpha1.TaskSummaryResourceStatus {
	if len(taskSummary.Status.ResourceStatus) == 0 {
		return nil
	}

	statuses, exist := taskSummary.Status.ResourceStatus[domainID]
	if !exist {
		return nil
	}

	for index, status := range statuses {
		if status.Role == role {
			return statuses[index]
		}
	}

	return nil
}
