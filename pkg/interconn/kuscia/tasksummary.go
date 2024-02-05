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

// runTaskSummaryWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runTaskSummaryWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, taskSummaryQueueName, c.taskSummaryQueue, c.syncTaskSummaryHandler, maxRetries) {
	}
}

// handleAddedTaskSummary is used to handle added taskSummary.
func (c *Controller) handleAddedTaskSummary(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.taskSummaryQueue)
}

// handleUpdatedTaskSummary is used to handle updated taskSummary.
func (c *Controller) handleUpdatedTaskSummary(oldObj, newObj interface{}) {
	oldTs, ok := oldObj.(*v1alpha1.KusciaTaskSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaTaskSummary", oldObj)
		return
	}

	newTs, ok := newObj.(*v1alpha1.KusciaTaskSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaTaskSummary", newObj)
		return
	}

	if oldTs.ResourceVersion == newTs.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newTs, c.taskSummaryQueue)
}

// syncTaskSummaryHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncTaskSummaryHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split taskSummary key %v, %v, skip processing it", key, err)
		return nil
	}

	originalTs, err := c.taskSummaryLister.KusciaTaskSummaries(namespace).Get(name)
	if err != nil {
		// taskSummary was deleted in cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get taskSummary %v, maybe it was deleted, skip processing it", key)
			return nil
		}
		return err
	}

	partyDomainIDs := ikcommon.GetInterConnKusciaPartyDomainIDs(originalTs)
	if partyDomainIDs == nil {
		nlog.Warnf("Failed to get interconn kuscia party domain ids from taskSummary %v, skip processing it", ikcommon.GetObjectNamespaceName(originalTs))
		return nil
	}

	taskSummary := originalTs.DeepCopy()
	if updateErr := c.updateTask(ctx, taskSummary, partyDomainIDs); updateErr != nil {
		err = updateErr
	}

	if updateErr := c.updateTaskResource(ctx, taskSummary, partyDomainIDs); updateErr != nil {
		err = updateErr
	}

	return err
}

func (c *Controller) updateTask(ctx context.Context, taskSummary *v1alpha1.KusciaTaskSummary, domainIDs []string) error {
	originalTask, err := c.taskLister.KusciaTasks(common.KusciaCrossDomain).Get(taskSummary.Name)
	if err != nil {
		return err
	}

	tsRvInTask := ikcommon.GetObjectAnnotation(originalTask, common.TaskSummaryResourceVersionAnnotationKey)
	if !utilsres.CompareResourceVersion(taskSummary.ResourceVersion, tsRvInTask) {
		nlog.Infof("TaskSummary resource version is not greater than the annotation value in task %v, skip updating task", originalTask.Name)
		return nil
	}

	task := originalTask.DeepCopy()
	needUpdate := false
	if updated := updateTaskPartyStatus(task, taskSummary, domainIDs); updated {
		task.Annotations[common.TaskSummaryResourceVersionAnnotationKey] = taskSummary.ResourceVersion
		needUpdate = true
	}

	if needUpdate {
		task.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = c.kusciaClient.KusciaV1alpha1().KusciaTasks(task.Namespace).UpdateStatus(ctx, task, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func updateTaskPartyStatus(task *v1alpha1.KusciaTask, taskSummary *v1alpha1.KusciaTaskSummary, domainIDs []string) bool {
	if len(taskSummary.Status.PartyTaskStatus) == 0 {
		return false
	}

	if len(task.Status.PartyTaskStatus) == 0 {
		task.Status.PartyTaskStatus = taskSummary.Status.PartyTaskStatus
		return true
	}

	var partyTaskStatusInTaskSummary []v1alpha1.PartyTaskStatus
	for _, domainID := range domainIDs {
		for j := range taskSummary.Status.PartyTaskStatus {
			if domainID == taskSummary.Status.PartyTaskStatus[j].DomainID {
				partyTaskStatusInTaskSummary = append(partyTaskStatusInTaskSummary, taskSummary.Status.PartyTaskStatus[j])
			}
		}
	}

	updated := false
	for i, pts := range partyTaskStatusInTaskSummary {
		found := false
		for j, taskPts := range task.Status.PartyTaskStatus {
			if pts.DomainID == taskPts.DomainID && pts.Role == taskPts.Role {
				found = true
				if pts.Phase != taskPts.Phase {
					task.Status.PartyTaskStatus[j].Phase = pts.Phase
					updated = true
				}

				if pts.Message != taskPts.Message {
					task.Status.PartyTaskStatus[j].Message = pts.Message
					updated = true
				}
			}
		}

		if !found {
			task.Status.PartyTaskStatus = append(task.Status.PartyTaskStatus, partyTaskStatusInTaskSummary[i])
			updated = true
		}
	}

	return updated
}

func (c *Controller) updateTaskResource(ctx context.Context, taskSummary *v1alpha1.KusciaTaskSummary, domainIDs []string) error {
	for _, domainID := range domainIDs {
		statuses, ok := taskSummary.Status.ResourceStatus[domainID]
		if !ok {
			continue
		}

		for _, status := range statuses {
			originalTr, err := c.taskResourceLister.TaskResources(domainID).Get(status.HostTaskResourceName)
			if err != nil {
				return fmt.Errorf("failed to update taskResource based on taskSummary %v, %v", ikcommon.GetObjectNamespaceName(taskSummary), err)
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
			case v1alpha1.TaskResourcePhaseReserved:
				if tr.Status.Phase == "" ||
					tr.Status.Phase == v1alpha1.TaskResourcePhasePending ||
					tr.Status.Phase == v1alpha1.TaskResourcePhaseReserving {
					updated = true
					tr.Status.Phase = v1alpha1.TaskResourcePhaseReserved
					tr.Status.LastTransitionTime = status.LastTransitionTime
				}
			case v1alpha1.TaskResourcePhaseFailed:
				if tr.Status.Phase == "" ||
					tr.Status.Phase == v1alpha1.TaskResourcePhasePending ||
					(tr.Status.Phase == v1alpha1.TaskResourcePhaseReserving &&
						status.LastTransitionTime.Sub(tr.Status.LastTransitionTime.Time) >= 0) {
					updated = true
					tr.Status.Phase = v1alpha1.TaskResourcePhaseFailed
					tr.Status.LastTransitionTime = status.LastTransitionTime
				}
			}

			if updated {
				if tr.Annotations == nil {
					tr.Annotations = make(map[string]string)
				}
				tr.Annotations[common.TaskSummaryResourceVersionAnnotationKey] = taskSummary.ResourceVersion
				_, err = c.kusciaClient.KusciaV1alpha1().TaskResources(tr.Namespace).Update(ctx, tr, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
