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
)

// runJobSummaryWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runJobSummaryWorker() {
	for queue.HandleQueueItem(context.Background(), c.jobSummaryQueueName, c.jobSummaryQueue, c.syncJobSummaryHandler, maxRetries) {
	}
}

// handleAddedorDeletedJobSummary is used to handle added or deleted jobSummary.
func (c *hostResourcesController) handleAddedorDeletedJobSummary(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.jobSummaryQueue)
}

// handleUpdatedJobSummary is used to handle updated jobSummary.
func (c *hostResourcesController) handleUpdatedJobSummary(oldObj, newObj interface{}) {
	oldJs, ok := oldObj.(*kusciaapisv1alpha1.KusciaJobSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJobSummary", oldObj)
		return
	}

	newJs, ok := newObj.(*kusciaapisv1alpha1.KusciaJobSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJobSummary", newObj)
		return
	}

	if oldJs.ResourceVersion == newJs.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newJs, c.jobSummaryQueue)
}

// TODO: Abstract into common interface
// syncJobSummaryHandler is used to sync jobSummary between host and member cluster.
func (c *hostResourcesController) syncJobSummaryHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split jobSummary key %v, %v, skip processing it", key, err)
		return nil
	}

	hJs, err := c.hostJobSummaryLister.KusciaJobSummaries(namespace).Get(name)
	if err != nil {
		// JobSummary is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			_, jobErr := c.hostKusciaClient.KusciaV1alpha1().KusciaJobs(namespace).Get(ctx, name, metav1.GetOptions{})
			if jobErr != nil {
				if k8serrors.IsNotFound(jobErr) {
					nlog.Infof("Host job %s/%s is not found, delete job %v", namespace, name, name)
					err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Delete(ctx, name, metav1.DeleteOptions{})
					if k8serrors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return fmt.Errorf("failed to get host job %s/%s, %v", namespace, name, jobErr)
			}
		}
		return err
	}

	return c.updateMemberJobByJobSummary(ctx, hJs.DeepCopy())
}

func (c *hostResourcesController) updateMemberJobByJobSummary(ctx context.Context, jobSummary *kusciaapisv1alpha1.KusciaJobSummary) error {
	originalJob, err := c.memberJobLister.KusciaJobs(common.KusciaCrossDomain).Get(jobSummary.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			originalJob, err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Get(ctx, jobSummary.Name, metav1.GetOptions{})
			if err != nil && k8serrors.IsNotFound(err) {
				nlog.Infof("Job %s/%s is not found, skip processing it", common.KusciaCrossDomain, jobSummary.Name)
				return nil
			}
		}
		if err != nil {
			return err
		}
	}

	selfDomainIDs := ikcommon.GetSelfClusterPartyDomainIDs(originalJob)
	if len(selfDomainIDs) == 0 {
		nlog.Infof("Party domain ids from job %v not found, skip processing it", ikcommon.GetObjectNamespaceName(originalJob))
		return nil
	}

	job := originalJob.DeepCopy()
	domainIDMap := make(map[string]struct{})
	for _, domainID := range selfDomainIDs {
		domainIDMap[domainID] = struct{}{}
	}

	needUpdate := false
	if ikcommon.UpdateJobStage(job, jobSummary) {
		job.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(job.Namespace).Update(ctx, job, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return nil
	}

	if updateJobApproveStatus(job, jobSummary, domainIDMap) {
		needUpdate = true
	}

	if updateJobStageStatus(job, jobSummary, domainIDMap) {
		needUpdate = true
	}

	if updateJobPartyTaskCreateStatus(job, jobSummary, domainIDMap) {
		needUpdate = true
	}

	if updateJobStatusPhase(job, jobSummary) {
		needUpdate = true
	}

	if needUpdate {
		job.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(job.Namespace).UpdateStatus(ctx, job, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func updateJobStatusPhase(job *kusciaapisv1alpha1.KusciaJob, jobSummary *kusciaapisv1alpha1.KusciaJobSummary) bool {
	if jobSummary.Status.Phase == kusciaapisv1alpha1.KusciaJobFailed &&
		jobSummary.Status.Phase != job.Status.Phase &&
		jobSummary.Status.CompletionTime != nil &&
		jobSummary.Status.Reason != "" &&
		jobSummary.Status.Reason != job.Status.Reason {
		switch jobSummary.Status.Reason {
		case string(kusciaapisv1alpha1.ValidateFailed), string(kusciaapisv1alpha1.CreateTaskFailed):
			job.Status.Phase = jobSummary.Status.Phase
			job.Status.Reason = jobSummary.Status.Reason
			job.Status.Message = jobSummary.Status.Message
			return true
		}
	}
	return false
}

func updateJobApproveStatus(job *kusciaapisv1alpha1.KusciaJob, jobSummary *kusciaapisv1alpha1.KusciaJobSummary, domainIDMap map[string]struct{}) bool {
	if len(job.Status.ApproveStatus) == 0 && len(jobSummary.Status.ApproveStatus) > 0 {
		job.Status.ApproveStatus = jobSummary.Status.ApproveStatus
		return true
	}

	updated := false
	for domainID, status := range jobSummary.Status.ApproveStatus {
		if _, exist := domainIDMap[domainID]; exist {
			continue
		}

		if !reflect.DeepEqual(status, job.Status.ApproveStatus[domainID]) {
			job.Status.ApproveStatus[domainID] = status
			updated = true
		}
	}
	return updated
}

func updateJobStageStatus(job *kusciaapisv1alpha1.KusciaJob, jobSummary *kusciaapisv1alpha1.KusciaJobSummary, domainIDMap map[string]struct{}) bool {
	if len(job.Status.StageStatus) == 0 && len(jobSummary.Status.StageStatus) > 0 {
		job.Status.StageStatus = jobSummary.Status.StageStatus
		return true
	}

	updated := false
	for domainID, status := range jobSummary.Status.StageStatus {
		if _, exist := domainIDMap[domainID]; exist {
			continue
		}

		if !reflect.DeepEqual(status, job.Status.StageStatus[domainID]) {
			job.Status.StageStatus[domainID] = status
			updated = true
		}
	}
	return updated
}

func updateJobPartyTaskCreateStatus(job *kusciaapisv1alpha1.KusciaJob, jobSummary *kusciaapisv1alpha1.KusciaJobSummary, domainIDMap map[string]struct{}) bool {
	if len(job.Status.PartyTaskCreateStatus) == 0 && len(jobSummary.Status.PartyTaskCreateStatus) > 0 {
		job.Status.PartyTaskCreateStatus = jobSummary.Status.PartyTaskCreateStatus
		return true
	}

	updated := false
	for domainID, status := range jobSummary.Status.PartyTaskCreateStatus {
		if _, exist := domainIDMap[domainID]; exist {
			continue
		}

		if !reflect.DeepEqual(status, job.Status.PartyTaskCreateStatus[domainID]) {
			job.Status.PartyTaskCreateStatus[domainID] = status
			updated = true
		}
	}
	return updated
}
