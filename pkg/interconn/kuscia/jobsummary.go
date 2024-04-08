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
	"reflect"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

// runJobSummaryWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runJobSummaryWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, jobSummaryQueueName, c.jobSummaryQueue, c.syncJobSummaryHandler, maxRetries) {
	}
}

// handleAddedJobSummary is used to handle added jobSummary.
func (c *Controller) handleAddedJobSummary(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.jobSummaryQueue)
}

// handleUpdatedJobSummary is used to handle updated jobSummary.
func (c *Controller) handleUpdatedJobSummary(oldObj, newObj interface{}) {
	oldJs, ok := oldObj.(*v1alpha1.KusciaJobSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJobSummary", oldObj)
		return
	}

	newJs, ok := newObj.(*v1alpha1.KusciaJobSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJobSummary", newObj)
		return
	}

	if oldJs.ResourceVersion == newJs.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newJs, c.jobSummaryQueue)
}

// syncJobSummaryHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncJobSummaryHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split jobSummary key %v, %v, skip processing it", key, err)
		return nil
	}

	originalJs, err := c.jobSummaryLister.KusciaJobSummaries(namespace).Get(name)
	if err != nil {
		// jobSummary was deleted in cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get jobSummary %v, maybe it was deleted, skip processing it", key)
			return nil
		}
		return err
	}

	return c.updateJob(ctx, originalJs.DeepCopy())
}

func (c *Controller) updateJob(ctx context.Context, jobSummary *v1alpha1.KusciaJobSummary) error {
	originalJob, err := c.jobLister.KusciaJobs(common.KusciaCrossDomain).Get(jobSummary.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			originalJob, err = c.kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Get(ctx, jobSummary.Name, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Failed to get job %v, %v", jobSummary.Name, err)
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}

	partyDomainIDs := ikcommon.GetInterConnKusciaPartyDomainIDs(jobSummary)
	if partyDomainIDs == nil {
		nlog.Warnf("Failed to get interconn kuscia party domain ids from jobSummary %v, skip processing it", ikcommon.GetObjectNamespaceName(jobSummary))
		return nil
	}

	job := originalJob.DeepCopy()
	needUpdate := false
	if updated := ikcommon.UpdateJobStage(job, jobSummary); updated {
		job.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = c.kusciaClient.KusciaV1alpha1().KusciaJobs(job.Namespace).Update(ctx, job, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return nil
	}

	if updated := updateJobStatusInfo(job, jobSummary, partyDomainIDs); updated {
		needUpdate = true
	}

	if needUpdate {
		job.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = c.kusciaClient.KusciaV1alpha1().KusciaJobs(job.Namespace).UpdateStatus(ctx, job, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func updateJobStatusInfo(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, domainIDs []string) bool {
	updated := false
	if job.Status.ApproveStatus == nil && len(jobSummary.Status.ApproveStatus) > 0 {
		updated = true
		job.Status.ApproveStatus = jobSummary.Status.ApproveStatus
	}

	if job.Status.StageStatus == nil && len(jobSummary.Status.StageStatus) > 0 {
		updated = true
		job.Status.StageStatus = jobSummary.Status.StageStatus
	}

	if job.Status.PartyTaskCreateStatus == nil && len(jobSummary.Status.PartyTaskCreateStatus) > 0 {
		updated = true
		job.Status.PartyTaskCreateStatus = jobSummary.Status.PartyTaskCreateStatus
	}

	for _, domainID := range domainIDs {
		if jobSummary.Status.ApproveStatus[domainID] != job.Status.ApproveStatus[domainID] {
			job.Status.ApproveStatus[domainID] = jobSummary.Status.ApproveStatus[domainID]
			updated = true
		}

		if jobSummary.Status.StageStatus[domainID] != job.Status.StageStatus[domainID] {
			job.Status.StageStatus[domainID] = jobSummary.Status.StageStatus[domainID]
			updated = true
		}

		if !reflect.DeepEqual(jobSummary.Status.PartyTaskCreateStatus[domainID], job.Status.PartyTaskCreateStatus[domainID]) {
			job.Status.PartyTaskCreateStatus[domainID] = jobSummary.Status.PartyTaskCreateStatus[domainID]
			updated = true
		}
	}
	return updated
}
