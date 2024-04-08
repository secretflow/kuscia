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

// runJobWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runJobWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, jobQueueName, c.jobQueue, c.syncJobHandler, maxRetries) {
	}
}

// handleAddedJob is used to handle added job.
func (c *Controller) handleAddedJob(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.jobQueue)
}

// handleUpdatedJob is used to handle updated job.
func (c *Controller) handleUpdatedJob(oldObj, newObj interface{}) {
	oldJob, ok := oldObj.(*v1alpha1.KusciaJob)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJob", oldObj)
		return
	}

	newJob, ok := newObj.(*v1alpha1.KusciaJob)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJob", newObj)
		return
	}

	if oldJob.ResourceVersion == newJob.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newJob, c.jobQueue)
}

// handleDeletedJob is used to handle deleted job.
func (c *Controller) handleDeletedJob(obj interface{}) {
	job, ok := obj.(*v1alpha1.KusciaJob)
	if !ok {
		return
	}

	isInitiator, err := ikcommon.SelfClusterIsInitiator(c.domainLister, job)
	if err != nil {
		nlog.Errorf("Failed to handle kuscia job %v delete event, %v", ikcommon.GetObjectNamespaceName(job), err)
		return
	}

	if !isInitiator {
		return
	}

	masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, job)
	if err != nil {
		nlog.Errorf("Failed to handle kuscia job %v delete event, %v", ikcommon.GetObjectNamespaceName(job), err)
		return
	}

	for masterDomain := range masterDomains {
		c.jobQueue.Add(fmt.Sprintf("%v%v/%v", ikcommon.DeleteEventKeyPrefix, masterDomain, job.Name))
	}
}

// syncJobHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncJobHandler(ctx context.Context, key string) (err error) {
	key, deleteEvent := ikcommon.IsOriginalResourceDeleteEvent(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split job key %v, %v, skip processing it", key, err)
		return nil
	}

	if deleteEvent {
		nlog.Infof("Delete job %v cascaded resources", name)
		return c.deleteJobCascadedResources(ctx, namespace, name)
	}

	originalJob, err := c.jobLister.KusciaJobs(namespace).Get(name)
	if err != nil {
		// job was deleted in cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get job %v, maybe it's deleted, skip processing it", key)
			return nil
		}
		return err
	}

	selfIsInitiator, err := ikcommon.SelfClusterIsInitiator(c.domainLister, originalJob)
	if err != nil {
		return err
	}

	job := originalJob.DeepCopy()
	if selfIsInitiator {
		return c.processJobAsInitiator(ctx, job)
	}
	return c.processJobAsPartner(ctx, job)
}

func (c *Controller) deleteJobCascadedResources(ctx context.Context, namespace, name string) error {
	mirrorKj, err := c.jobLister.KusciaJobs(namespace).Get(name)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if mirrorKj != nil {
		err = c.kusciaClient.KusciaV1alpha1().KusciaJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	js, err := c.jobSummaryLister.KusciaJobSummaries(namespace).Get(name)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if js != nil {
		err = c.kusciaClient.KusciaV1alpha1().KusciaJobSummaries(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *Controller) processJobAsInitiator(ctx context.Context, job *v1alpha1.KusciaJob) error {
	if err := c.createMirrorJobs(ctx, job); err != nil {
		return err
	}

	return c.createOrUpdateJobSummary(ctx, job)
}

func (c *Controller) createMirrorJobs(ctx context.Context, job *v1alpha1.KusciaJob) error {
	mirrorJobs, err := c.buildMirrorJobs(job)
	if err != nil {
		return err
	}

	if len(mirrorJobs) == 0 {
		return nil
	}

	for domain, mirrorJob := range mirrorJobs {
		_, err = c.kusciaClient.KusciaV1alpha1().KusciaJobs(domain).Create(ctx, mirrorJob, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (c *Controller) buildMirrorJobs(job *v1alpha1.KusciaJob) (map[string]*v1alpha1.KusciaJob, error) {
	jobStage := ikcommon.GetObjectLabel(job, common.LabelJobStage)
	if jobStage == string(v1alpha1.JobStopStage) {
		nlog.Infof("Kuscia job %v stage is %v, skip building it's mirror job", job.Name, jobStage)
		return nil, nil
	}

	masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, job)
	if err != nil {
		nlog.Errorf("Failed to build mirror job %v, %v", ikcommon.GetObjectNamespaceName(job), err)
		return nil, nil
	}

	mirrorJobs := make(map[string]*v1alpha1.KusciaJob)
	for masterDomainID, partyDomainIDs := range masterDomains {
		mirrorJob, err := c.jobLister.KusciaJobs(masterDomainID).Get(job.Name)
		if err != nil && !k8serrors.IsNotFound(err) {
			return nil, err
		}

		if mirrorJob != nil {
			continue
		}

		annotations := map[string]string{
			common.InitiatorAnnotationKey:            ikcommon.GetObjectAnnotation(job, common.InitiatorAnnotationKey),
			common.InterConnKusciaPartyAnnotationKey: strings.Join(partyDomainIDs, "_"),
		}

		labels := make(map[string]string)
		for k, v := range job.Labels {
			if strings.Contains(k, common.JobCustomFieldsLabelPrefix) {
				labels[k] = v
			}
		}

		mirrorJobs[masterDomainID] = &v1alpha1.KusciaJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:        job.Name,
				Namespace:   masterDomainID,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: *buildMirrorJobSpec(job),
		}

	}
	return mirrorJobs, nil
}

func buildMirrorJobSpec(job *v1alpha1.KusciaJob) *v1alpha1.KusciaJobSpec {
	spec := job.Spec.DeepCopy()
	for i := range spec.Tasks {
		if spec.Tasks[i].TaskID != "" {
			spec.Tasks[i].TaskID = ""
		}
	}
	return spec
}

func (c *Controller) createOrUpdateJobSummary(ctx context.Context, job *v1alpha1.KusciaJob) error {
	masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, job)
	if err != nil {
		nlog.Errorf("Failed to create or update jobSummary for job %v, %v, skip processing it",
			ikcommon.GetObjectNamespaceName(job), err)
		return nil
	}

	for masterDomainID, partyDomainIDs := range masterDomains {
		jobSummary, err := c.jobSummaryLister.KusciaJobSummaries(masterDomainID).Get(job.Name)
		if err != nil {
			// create jobSummary
			if k8serrors.IsNotFound(err) {
				if err = c.createJobSummary(ctx, job, masterDomainID, partyDomainIDs); err != nil {
					return err
				}
				continue
			}
			return err
		}

		// update jobSummary
		if err = c.updateJobSummary(ctx, job, jobSummary.DeepCopy()); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) createJobSummary(ctx context.Context, job *v1alpha1.KusciaJob, masterDomainID string, partyDomainIDs []string) error {
	jobStage := ikcommon.GetJobStage(ikcommon.GetObjectLabel(job, common.LabelJobStage))
	jobSummary := &v1alpha1.KusciaJobSummary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.Name,
			Namespace: masterDomainID,
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:            ikcommon.GetObjectAnnotation(job, common.InitiatorAnnotationKey),
				common.InterConnKusciaPartyAnnotationKey: strings.Join(partyDomainIDs, "_"),
			},
		},
		Spec: v1alpha1.KusciaJobSummarySpec{
			Stage:        jobStage,
			StageTrigger: ikcommon.GetObjectAnnotation(job, common.InitiatorAnnotationKey),
		},
		Status: v1alpha1.KusciaJobStatus{
			Phase:                 job.Status.Phase,
			ApproveStatus:         job.Status.ApproveStatus,
			StageStatus:           job.Status.StageStatus,
			PartyTaskCreateStatus: job.Status.PartyTaskCreateStatus,
			TaskStatus:            job.Status.TaskStatus,
			StartTime:             ikcommon.GetCurrentTime(),
		},
	}
	_, err := c.kusciaClient.KusciaV1alpha1().KusciaJobSummaries(masterDomainID).Create(ctx, jobSummary, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *Controller) updateJobSummary(ctx context.Context, job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary) error {
	domainIDs := ikcommon.GetSelfClusterPartyDomainIDs(job)
	if domainIDs == nil {
		nlog.Errorf("Failed to get self cluster party domain ids from job %v, skip processing it", ikcommon.GetObjectNamespaceName(job))
		return nil
	}

	needUpdate := false
	if job.Status.Phase != jobSummary.Status.Phase {
		needUpdate = true
		jobSummary.Status.Phase = job.Status.Phase
	}

	if updateJobSummaryStage(job, jobSummary) {
		needUpdate = true
	}

	if updateJobSummaryApproveStatus(job, jobSummary, domainIDs) {
		needUpdate = true
	}

	if updateJobSummaryStageStatus(job, jobSummary, domainIDs) {
		needUpdate = true
	}

	if updateJobSummaryPartyTaskCreateStatus(job, jobSummary, domainIDs) {
		needUpdate = true
	}

	if updateJobSummaryTaskStatus(job, jobSummary) {
		needUpdate = true
	}

	now := ikcommon.GetCurrentTime()
	if job.Status.CompletionTime != jobSummary.Status.CompletionTime {
		needUpdate = true
		jobSummary.Status.CompletionTime = now
	}

	if needUpdate {
		jobSummary.Status.LastReconcileTime = now
		if _, err := c.kusciaClient.KusciaV1alpha1().KusciaJobSummaries(jobSummary.Namespace).Update(ctx, jobSummary, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func updateJobSummaryStage(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary) bool {
	// if the stage of job changed to stop, we will prevent updating the job stage
	if jobSummary.Spec.Stage == v1alpha1.JobCancelStage {
		return false
	}
	jobStage := ikcommon.GetJobStage(ikcommon.GetObjectLabel(job, common.LabelJobStage))
	jobStageTrigger := ikcommon.GetObjectLabel(job, common.LabelJobStageTrigger)
	jobStageVersion := ikcommon.GetObjectLabel(job, common.LabelJobStageVersion)
	jobSummaryStageVersion := ikcommon.GetObjectLabel(jobSummary, common.LabelJobStageVersion)
	if jobStageVersion <= jobSummaryStageVersion {
		return false
	}
	if jobSummary.Labels == nil {
		jobSummary.Labels = make(map[string]string)
	}
	jobSummary.Labels[common.LabelJobStageVersion] = jobStageVersion
	if jobStage != jobSummary.Spec.Stage {
		jobSummary.Spec.Stage = jobStage
		jobSummary.Spec.StageTrigger = jobStageTrigger
		return true
	}
	return false
}

func updateJobSummaryApproveStatus(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, domainIDs []string) bool {
	if len(job.Status.ApproveStatus) == 0 || reflect.DeepEqual(job.Status.ApproveStatus, jobSummary.Status.ApproveStatus) {
		return false
	}

	if len(jobSummary.Status.ApproveStatus) == 0 {
		jobSummary.Status.ApproveStatus = job.Status.ApproveStatus
		return true
	}

	updated := false
	for _, domainID := range domainIDs {
		if job.Status.ApproveStatus[domainID] != jobSummary.Status.ApproveStatus[domainID] {
			updated = true
			jobSummary.Status.ApproveStatus[domainID] = job.Status.ApproveStatus[domainID]
		}
	}

	return updated
}

func updateJobSummaryStageStatus(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, domainIDs []string) bool {
	if len(job.Status.StageStatus) == 0 || reflect.DeepEqual(job.Status.StageStatus, jobSummary.Status.StageStatus) {
		return false
	}

	if len(jobSummary.Status.StageStatus) == 0 {
		jobSummary.Status.StageStatus = job.Status.StageStatus
		return true
	}

	updated := false
	for _, domainID := range domainIDs {
		if job.Status.StageStatus[domainID] != jobSummary.Status.StageStatus[domainID] {
			updated = true
			jobSummary.Status.StageStatus[domainID] = job.Status.StageStatus[domainID]
		}
	}

	return updated
}

func updateJobSummaryPartyTaskCreateStatus(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, domainIDs []string) bool {
	if len(job.Status.PartyTaskCreateStatus) == 0 || reflect.DeepEqual(job.Status.PartyTaskCreateStatus, jobSummary.Status.PartyTaskCreateStatus) {
		return false
	}

	if len(jobSummary.Status.PartyTaskCreateStatus) == 0 {
		jobSummary.Status.PartyTaskCreateStatus = job.Status.PartyTaskCreateStatus
		return true
	}

	updated := false
	for _, domainID := range domainIDs {
		if !reflect.DeepEqual(job.Status.PartyTaskCreateStatus[domainID], jobSummary.Status.PartyTaskCreateStatus[domainID]) {
			updated = true
			jobSummary.Status.PartyTaskCreateStatus[domainID] = job.Status.PartyTaskCreateStatus[domainID]
		}
	}

	return updated
}

func updateJobSummaryTaskStatus(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary) bool {
	if len(job.Status.TaskStatus) == 0 || reflect.DeepEqual(job.Status.TaskStatus, jobSummary.Status.TaskStatus) {
		return false
	}

	jobSummary.Status.TaskStatus = job.Status.TaskStatus
	return true
}

func (c *Controller) processJobAsPartner(ctx context.Context, job *v1alpha1.KusciaJob) error {
	initiator := ikcommon.GetObjectAnnotation(job, common.InitiatorAnnotationKey)
	if initiator == "" {
		nlog.Errorf("Failed to get initiator from job %v, skip processing it", ikcommon.GetObjectNamespaceName(job))
		return nil
	}

	masterDomainID := ikcommon.GetObjectAnnotation(job, common.KusciaPartyMasterDomainAnnotationKey)
	if masterDomainID == "" {
		nlog.Errorf("Failed to get master domain id from job %v, skip processing it",
			ikcommon.GetObjectNamespaceName(job))
		return nil
	}

	hostMasterDomainID, err := utilsres.GetMasterDomain(c.domainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}

	hra := c.hostResourceManager.GetHostResourceAccessor(hostMasterDomainID, masterDomainID)
	if hra == nil {
		return fmt.Errorf("host resource accessor for job %v is empty of host/member %v/%v, retry",
			ikcommon.GetObjectNamespaceName(job), initiator, masterDomainID)
	}

	if !hra.HasSynced() {
		return fmt.Errorf("host %v resource accessor has not synced, retry", initiator)
	}

	hJobSummary, err := hra.HostJobSummaryLister().KusciaJobSummaries(masterDomainID).Get(job.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			hJobSummary, err = hra.HostKusciaClient().KusciaV1alpha1().KusciaJobSummaries(masterDomainID).Get(ctx, job.Name, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Get JobSummary %v from host %v cluster failed, %v", job.Name, initiator, err)
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}

	return c.updateHostJobSummary(ctx, masterDomainID, hra.HostKusciaClient(), job, hJobSummary.DeepCopy())
}

func (c *Controller) updateHostJobSummary(ctx context.Context,
	masterDomainID string,
	kusciaClient kusciaclientset.Interface,
	job *v1alpha1.KusciaJob,
	jobSummary *v1alpha1.KusciaJobSummary) error {

	domainIDs := ikcommon.GetSelfClusterPartyDomainIDs(job)
	if domainIDs == nil {
		nlog.Warnf("Failed to get self cluster party domain ids from job %v, skip processing it", ikcommon.GetObjectNamespaceName(job))
		return nil
	}

	needUpdate := false
	if updateHostJobSummaryStage(job, jobSummary, masterDomainID) {
		needUpdate = true
	}

	if updateHostJobSummaryApproveStatus(job, jobSummary, domainIDs) {
		needUpdate = true
	}

	if updateHostJobSummaryStageStatus(job, jobSummary, domainIDs) {
		needUpdate = true
	}

	if updateHostJobSummaryPartyTaskCreateStatus(job, jobSummary, domainIDs) {
		needUpdate = true
	}

	if needUpdate {
		jobSummary.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err := kusciaClient.KusciaV1alpha1().KusciaJobSummaries(jobSummary.Namespace).Update(ctx, jobSummary, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func updateHostJobSummaryStage(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, masterDomainID string) bool {
	jobStageVersion := ikcommon.GetObjectLabel(job, common.LabelJobStageVersion)
	jobSummaryStageVersion := ikcommon.GetObjectLabel(jobSummary, common.LabelJobStageVersion)
	if jobStageVersion <= jobSummaryStageVersion {
		return false
	}
	jobStage := ikcommon.GetJobStage(ikcommon.GetObjectLabel(job, common.LabelJobStage))
	if jobSummary.Labels == nil {
		jobSummary.Labels = make(map[string]string)
	}
	jobSummary.Labels[common.LabelJobStageVersion] = jobStageVersion
	jobSummary.Spec.Stage = jobStage
	jobSummary.Spec.StageTrigger = masterDomainID
	return true
}

func updateHostJobSummaryApproveStatus(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, domainIDs []string) bool {
	return updateJobSummaryApproveStatus(job, jobSummary, domainIDs)
}

func updateHostJobSummaryStageStatus(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, domainIDs []string) bool {
	return updateJobSummaryStageStatus(job, jobSummary, domainIDs)
}

func updateHostJobSummaryPartyTaskCreateStatus(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary, domainIDs []string) bool {
	return updateJobSummaryPartyTaskCreateStatus(job, jobSummary, domainIDs)
}
