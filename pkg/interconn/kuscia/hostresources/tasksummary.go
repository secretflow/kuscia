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
	"reflect"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runTaskSummaryWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runTaskSummaryWorker() {
	for queue.HandleQueueItem(context.Background(), c.taskSummaryQueueName, c.taskSummaryQueue,
		c.syncTaskSummaryHandler, maxRetries) {
	}
}

// handleAddedorDeletedTaskSummary is used to handle added or deleted taskSummary.
func (c *hostResourcesController) handleAddedorDeletedTaskSummary(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.taskSummaryQueue)
}

// handleUpdatedTaskSummary is used to handle updated taskSummary.
func (c *hostResourcesController) handleUpdatedTaskSummary(oldObj, newObj interface{}) {
	oldTs, ok := oldObj.(*kusciaapisv1alpha1.KusciaTaskSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaTaskSummary", oldObj)
		return
	}

	newTs, ok := newObj.(*kusciaapisv1alpha1.KusciaTaskSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaTaskSummary", newObj)
		return
	}

	if oldTs.ResourceVersion == newTs.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newTs, c.taskSummaryQueue)
}

// TODO: Abstract into common interface
// syncTaskSummaryHandler is used to sync task between host and member cluster.
func (c *hostResourcesController) syncTaskSummaryHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split taskSummary key %v, %v, skip processing it", key, err)
		return nil
	}

	hTs, err := c.hostTaskSummaryLister.KusciaTaskSummaries(namespace).Get(name)
	if err != nil {
		// TaskSummary is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("TaskSummary %v may be deleted under host %v cluster, skip processing it", key, c.host)
			return nil
		}
		return err
	}

	if hTs.Spec.Alias == "" || hTs.Spec.JobID == "" {
		nlog.Errorf("Task alias and job id can't be empty in taskSummary %v under host %v cluster, skip processing it", key, c.host)
		return nil
	}

	taskSummary := hTs.DeepCopy()
	updated, err := c.updateMemberJobByTaskSummary(ctx, taskSummary)
	if err != nil {
		return err
	}
	if updated {
		return nil
	}

	originalTask, err := c.memberTaskLister.KusciaTasks(common.KusciaCrossDomain).Get(taskSummary.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			originalTask, err = c.memberKusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(context.Background(),
				taskSummary.Name, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Failed to get member task %v, %v", taskSummary.Name, err)
			return err
		}
	}

	selfDomainIDs := ikcommon.GetSelfClusterPartyDomainIDs(originalTask)
	if len(selfDomainIDs) == 0 {
		nlog.Errorf("Task %v annotation %v is empty, skip processing it",
			ikcommon.GetObjectNamespaceName(originalTask), common.InterConnSelfPartyAnnotationKey)
		return nil
	}

	if err = c.updateMemberTask(ctx, taskSummary, originalTask.DeepCopy(), selfDomainIDs); err != nil {
		return err
	}

	return c.updateMemberTaskResource(ctx, taskSummary, selfDomainIDs)
}

func (c *hostResourcesController) updateMemberJobByTaskSummary(ctx context.Context, taskSummary *kusciaapisv1alpha1.KusciaTaskSummary) (bool, error) {
	originalJob, err := c.memberJobLister.KusciaJobs(common.KusciaCrossDomain).Get(taskSummary.Spec.JobID)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			originalJob, err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Get(ctx,
				taskSummary.Spec.JobID, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Failed to Get member job %v, %v", taskSummary.Spec.JobID, err)
			return false, err
		}
	}

	job := originalJob.DeepCopy()
	needUpdate := false
	for i, task := range job.Spec.Tasks {
		if task.Alias == taskSummary.Spec.Alias {
			if task.TaskID == "" {
				needUpdate = true
				job.Spec.Tasks[i].TaskID = taskSummary.Name
			}
			break
		}
	}

	if needUpdate {
		_, err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(job.Namespace).Update(ctx, job, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
	}
	return needUpdate, nil
}

func (c *hostResourcesController) updateMemberTask(ctx context.Context,
	taskSummary *kusciaapisv1alpha1.KusciaTaskSummary,
	task *kusciaapisv1alpha1.KusciaTask,
	domainIDs []string) error {
	if updateTaskPartyTaskStatus(task, taskSummary, domainIDs) {
		_, err := c.memberKusciaClient.KusciaV1alpha1().KusciaTasks(task.Namespace).UpdateStatus(ctx, task, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func updateTaskPartyTaskStatus(task *kusciaapisv1alpha1.KusciaTask,
	taskSummary *kusciaapisv1alpha1.KusciaTaskSummary,
	domainIDs []string) bool {
	selfDomainIDMap := make(map[string]struct{})
	for _, domainID := range domainIDs {
		selfDomainIDMap[domainID] = struct{}{}
	}

	updated := false
	for i, ptsInTask := range task.Status.PartyTaskStatus {
		for j, ptsInTaskSummary := range taskSummary.Status.PartyTaskStatus {
			if ptsInTask.DomainID == ptsInTaskSummary.DomainID && ptsInTask.Role == ptsInTaskSummary.Role {
				if _, exist := selfDomainIDMap[ptsInTask.DomainID]; exist {
					if ptsInTaskSummary.Phase == kusciaapisv1alpha1.TaskFailed &&
						ptsInTaskSummary.Phase != ptsInTask.Phase {
						updated = true
						task.Status.PartyTaskStatus[i].Phase = ptsInTaskSummary.Phase
						task.Status.PartyTaskStatus[i].Message = ptsInTaskSummary.Message
					}
				} else {
					if !reflect.DeepEqual(ptsInTask, ptsInTaskSummary) {
						updated = true
						task.Status.PartyTaskStatus[i] = taskSummary.Status.PartyTaskStatus[j]
					}
				}
			}
		}
	}

	var addedPartyTaskStatus []kusciaapisv1alpha1.PartyTaskStatus
	for i, ptsInTaskSummary := range taskSummary.Status.PartyTaskStatus {
		found := false
		for _, ptsInTask := range task.Status.PartyTaskStatus {
			if ptsInTaskSummary.DomainID == ptsInTask.DomainID && ptsInTaskSummary.Role == ptsInTask.Role {
				found = true
				break
			}
		}

		if !found {
			addedPartyTaskStatus = append(addedPartyTaskStatus, taskSummary.Status.PartyTaskStatus[i])
		}
	}

	if len(addedPartyTaskStatus) > 0 {
		updated = true
		task.Status.PartyTaskStatus = append(task.Status.PartyTaskStatus, addedPartyTaskStatus...)
	}

	return updated
}

func (c *hostResourcesController) updateMemberTaskResource(ctx context.Context, taskSummary *kusciaapisv1alpha1.KusciaTaskSummary, domainIDs []string) error {
	for _, domainID := range domainIDs {
		statuses, ok := taskSummary.Status.ResourceStatus[domainID]
		if !ok {
			continue
		}

		for _, status := range statuses {
			if status.MemberTaskResourceName == "" {
				continue
			}

			originalTr, err := c.memberTaskResourceLister.TaskResources(domainID).Get(status.MemberTaskResourceName)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					originalTr, err = c.memberKusciaClient.KusciaV1alpha1().TaskResources(domainID).Get(ctx, status.MemberTaskResourceName, metav1.GetOptions{})
				}
				if err != nil {
					if k8serrors.IsNotFound(err) {
						return nil
					}
					nlog.Errorf("Failed to get taskResource %v, %v", fmt.Sprintf("%v/%v", domainID, status.MemberTaskResourceName), err)
					return err
				}
			}

			tsRvInTaskResource := ikcommon.GetObjectAnnotation(originalTr, common.TaskSummaryResourceVersionAnnotationKey)
			if tsRvInTaskResource != "" && !utilsres.CompareResourceVersion(taskSummary.ResourceVersion, tsRvInTaskResource) {
				nlog.Infof("TaskSummary resource version is not greater than the annotation value in taskResource %v, skip updating taskResource",
					ikcommon.GetObjectNamespaceName(originalTr))
				return nil
			}

			updated := false
			tr := originalTr.DeepCopy()
			switch status.Phase {
			case kusciaapisv1alpha1.TaskResourcePhaseReserving,
				kusciaapisv1alpha1.TaskResourcePhaseFailed,
				kusciaapisv1alpha1.TaskResourcePhaseSchedulable:
				updated = updateTaskResourceStatusByHostTaskSummary(status, tr)
			default:
			}

			if updated {
				if tr.Annotations == nil {
					tr.Annotations = make(map[string]string)
				}
				tr.Annotations[common.TaskSummaryResourceVersionAnnotationKey] = taskSummary.ResourceVersion
				_, err = c.memberKusciaClient.KusciaV1alpha1().TaskResources(tr.Namespace).Update(ctx, tr, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("failed to update taskResource %v status based on host taskSummary %v, %v",
						ikcommon.GetObjectNamespaceName(tr), ikcommon.GetObjectNamespaceName(taskSummary), err)
				}
			}
		}
	}

	return nil
}

func updateTaskResourceStatusByHostTaskSummary(statusInTaskSummary *kusciaapisv1alpha1.TaskSummaryResourceStatus,
	taskResource *kusciaapisv1alpha1.TaskResource) bool {
	updated := false
	switch statusInTaskSummary.Phase {
	case kusciaapisv1alpha1.TaskResourcePhaseReserving:
		if taskResource.Status.Phase == kusciaapisv1alpha1.TaskResourcePhaseFailed &&
			statusInTaskSummary.LastTransitionTime.Sub(taskResource.Status.LastTransitionTime.Time) >= 0 {
			updated = true
		}
	case kusciaapisv1alpha1.TaskResourcePhaseFailed:
		if statusInTaskSummary.Phase != taskResource.Status.Phase &&
			statusInTaskSummary.LastTransitionTime.Sub(taskResource.Status.LastTransitionTime.Time) >= 0 {
			updated = true
		}
	case kusciaapisv1alpha1.TaskResourcePhaseSchedulable:
		if statusInTaskSummary.Phase != taskResource.Status.Phase {
			updated = true
		}
	}

	if updated {
		taskResource.Status.Phase = statusInTaskSummary.Phase
		taskResource.Status.LastTransitionTime = statusInTaskSummary.LastTransitionTime
		taskResource.Status.CompletionTime = statusInTaskSummary.CompletionTime
	}
	return updated
}
