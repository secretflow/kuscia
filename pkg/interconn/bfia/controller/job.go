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
	"reflect"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/adapter"
	bfiacommon "github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

// handleAddedOrDeletedKusciaJob handles added or deleted kuscia job.
func (c *Controller) handleAddedOrDeletedKusciaJob(obj interface{}) {
	kj, ok := obj.(*kusciaapisv1alpha1.KusciaJob)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaJob", obj)
		return
	}

	if utilsres.SelfClusterAsInitiator(c.nsLister, kj.Spec.Initiator, kj.Annotations) {
		queue.EnqueueObjectWithKey(obj, c.kjQueue)
	} else {
		c.kjStatusSyncQueue.AddAfter(strings.Join([]string{kj.Namespace, kj.Name}, "/"), jobStatusSyncInterval)
	}
}

// handleUpdatedKusciaJob handles updated kuscia job.
func (c *Controller) handleUpdatedKusciaJob(oldObj, newObj interface{}) {
	oldKj, ok := oldObj.(*kusciaapisv1alpha1.KusciaJob)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaJob", oldObj)
		return
	}

	newKj, ok := newObj.(*kusciaapisv1alpha1.KusciaJob)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaJob", newObj)
		return
	}

	if oldKj.ResourceVersion == newKj.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newKj, c.kjQueue)
}

// runJobWorker is a long-running function that will read and process a event on the work queue.
func (c *Controller) runJobWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, kusciaJobQueueName, c.kjQueue, c.syncJobHandler, maxRetries) {
	}
}

// runJobStatusSyncWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runJobStatusSyncWorker(ctx context.Context) {
	for c.processJobStatusSyncNextWorkItem(ctx) {
	}
}

// processJobStatusSyncNextWorkItem process job status sync queue item.
func (c *Controller) processJobStatusSyncNextWorkItem(ctx context.Context) bool {
	key, quit := c.kjStatusSyncQueue.Get()
	if quit {
		return false
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		nlog.Errorf("Failed to split job key %v, %v, skip processing it", key, err)
		return false
	}

	rawKj, err := c.kjLister.KusciaJobs(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Kuscia job %v maybe deleted, ignore it", key)
			c.kjStatusSyncQueue.Done(key)
			return true
		}
		nlog.Errorf("Failed to get kuscia job %v, %v", key, err)
		c.kjStatusSyncQueue.Done(key)
		return true
	}

	if rawKj.Status.Phase == kusciaapisv1alpha1.KusciaJobFailed || rawKj.Status.Phase == kusciaapisv1alpha1.KusciaJobSucceeded {
		nlog.Infof("Kuscia job %v status is %v, skip query job status from party %v", key, rawKj.Status.Phase, rawKj.Spec.Initiator)
		c.kjStatusSyncQueue.Done(key)
		return true
	}

	defer func() {
		c.kjStatusSyncQueue.Done(key)
		c.kjStatusSyncQueue.AddAfter(key, jobStatusSyncInterval)
	}()

	if rawKj.Labels == nil || rawKj.Labels[common.LabelJobStage] != string(kusciaapisv1alpha1.JobStartStage) {
		return true
	}

	cacheKey := getCacheKeyName(reqTypeQueryJobStatus, resourceTypeKusciaJob, rawKj.Name)
	if _, ok := c.inflightRequestCache.Get(cacheKey); ok {
		return true
	}

	_ = c.inflightRequestCache.Add(cacheKey, "", jobStatusSyncInterval)

	kj := rawKj.DeepCopy()
	now := metav1.Now().Rfc3339Copy()
	cond, _ := utilsres.GetKusciaJobCondition(&kj.Status, kusciaapisv1alpha1.JobStatusSynced, true)

	resp, err := c.bfiaClient.QueryJobStatusAll(ctx, c.getReqDomainIDFromKusciaJob(kj), buildHostFor(kj.Spec.Initiator), kj.Name)
	if err != nil {
		utilsres.SetKusciaJobCondition(now, cond, corev1.ConditionFalse, "ErrorQueryJobStatus", err.Error())
		if err = c.updateJobStatus(kj, false, true); err != nil {
			nlog.Errorf("Update kuscia job %v status condition failed, %v", kj.Name, err)
		}
		return true
	}

	taskStatus := make(map[string]string)
	for _, v := range resp.Data.Fields {
		vs := v.GetStructValue()
		if vs == nil {
			break
		}

		for taskID, taskPhase := range vs.Fields {
			taskStatus[taskID] = taskPhase.GetStringValue()
		}
	}

	if hasSet := setKusciaJobTaskStatus(kj, taskStatus); !hasSet {
		return true
	}

	utilsres.SetKusciaJobCondition(now, cond, corev1.ConditionTrue, "", "")
	if err = c.updateJobStatus(kj, true, true); err != nil {
		nlog.Errorf("Update kuscia job %v status failed, %v", kj.Name, err)
	}
	return true
}

// syncJobHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncJobHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split job key %v, %v, skip processing it", key, err)
		return nil
	}

	rawKj, err := c.kjLister.KusciaJobs(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Kuscia job %v maybe deleted, skip to handle it", key)
			c.cleanCacheData(reqTypeCreateJob, resourceTypeKusciaJob, key)
			c.cleanCacheData(reqTypeStopJob, resourceTypeKusciaJob, key)
			c.cleanCacheData(reqTypeStartJob, resourceTypeKusciaJob, key)
			return nil
		}
		return err
	}

	if !utilsres.SelfClusterAsInitiator(c.nsLister, rawKj.Spec.Initiator, rawKj.Annotations) {
		return nil
	}

	if rawKj.DeletionTimestamp != nil {
		nlog.Infof("Kuscia job %v is terminating, skip to handle it", key)
		return nil
	}

	if rawKj.Status.CompletionTime != nil {
		nlog.Infof("Kuscia job %s is finished, skip to handle it", key)
		return nil
	}

	kj := rawKj.DeepCopy()
	if rawKj.Status.Phase == kusciaapisv1alpha1.KusciaJobFailed {
		cond, exist := utilsres.GetKusciaJobCondition(&kj.Status, kusciaapisv1alpha1.JobStopSucceeded, false)
		if exist && cond.Status == corev1.ConditionTrue {
			return nil
		}
		c.stopJob(ctx, kj)
		return nil
	}

	nlog.Infof("Handle kuscia job %v with stage %v", kj.Name, kj.Labels[common.LabelJobStage])
	switch kj.Labels[common.LabelJobStage] {
	case "", string(kusciaapisv1alpha1.JobCreateStage):
		c.handleJobCreateStage(ctx, kj)
	case string(kusciaapisv1alpha1.JobStopStage):
		c.handleJobStopStage(ctx, kj)
	case string(kusciaapisv1alpha1.JobStartStage):
		c.handleJobStartStage(ctx, kj)
	}

	return nil
}

// handleJobCreateStage handles kuscia job with create stage.
func (c *Controller) handleJobCreateStage(ctx context.Context, kj *kusciaapisv1alpha1.KusciaJob) {
	// check self stage status to avoid duplicate creation,
	if _, ok := kj.Status.StageStatus[kj.Spec.Initiator]; ok {
		return
	}

	c.createJob(ctx, kj)
}

// createJob creates job.
func (c *Controller) createJob(ctx context.Context, kj *kusciaapisv1alpha1.KusciaJob) {
	if _, ok := c.inflightRequestCache.Get(getCacheKeyName(reqTypeCreateJob, resourceTypeKusciaJob, kj.Name)); ok {
		return
	}

	nlog.Infof("Create job %v", kj.Name)
	interConnJobSpec, err := adapter.GenerateInterConnJobInfoFrom(kj, c.appImageLister)
	if err != nil {
		nlog.Errorf("Generate interconn bfia job spec %v failed, %v", kj.Name, err)
		utilsres.SetJobPartiesStageStatus(kj, kj.Spec.Initiator, kusciaapisv1alpha1.JobCreateStageFailed)
		if err = c.updateJobStatus(kj, false, true); err != nil {
			nlog.Errorf("Update kuscia job %v status condition failed, %v", kj.Name, err)
		}
		return
	}
	utilsres.SetJobPartiesStageStatus(kj, kj.Spec.Initiator, kusciaapisv1alpha1.JobCreateStageSucceeded)

	cacheKeyName := getCacheKeyName(reqTypeCreateJob, resourceTypeKusciaJob, kj.Name)
	_ = c.inflightRequestCache.Add(cacheKeyName, "", inflightRequestCacheExpiration)

	var wg sync.WaitGroup
	var errs errorcode.Errs
	for domainID := range c.getPartiesDomainInfo(kj) {
		wg.Add(1)
		go func(domainID string, kj *kusciaapisv1alpha1.KusciaJob) {
			createJobErr := c.bfiaClient.CreateJob(ctx, kj.Spec.Initiator, buildHostFor(domainID), interConnJobSpec.JodID, interConnJobSpec.FlowID, interConnJobSpec.DAG, interConnJobSpec.Config)
			if createJobErr != nil {
				errs.AppendErr(createJobErr)
				utilsres.SetJobPartiesStageStatus(kj, domainID, kusciaapisv1alpha1.JobCreateStageFailed)
			} else {
				utilsres.SetJobPartiesStageStatus(kj, domainID, kusciaapisv1alpha1.JobCreateStageSucceeded)
			}
			defer wg.Done()
		}(domainID, kj)
	}

	go func(cacheKeyName string) {
		defer c.inflightRequestCache.Set(cacheKeyName, "", finishedInflightRequestCacheExpiration)
		wg.Wait()
		if err = c.updateJobStatus(kj, false, true); err != nil {
			nlog.Errorf("Update kuscia job %v status condition failed, %v", kj.Name, err)
		}
	}(cacheKeyName)
}

// handleJobCreateStage handles kuscia job with stop stage.
func (c *Controller) handleJobStopStage(ctx context.Context, kj *kusciaapisv1alpha1.KusciaJob) {
	initializedCond, found := utilsres.GetKusciaJobCondition(&kj.Status, kusciaapisv1alpha1.JobStopInitialized, false)
	if !found || initializedCond.Status != corev1.ConditionTrue {
		return
	}

	c.stopJob(ctx, kj)
}

// stopJob stops job.
func (c *Controller) stopJob(ctx context.Context, kj *kusciaapisv1alpha1.KusciaJob) {
	if _, ok := c.inflightRequestCache.Get(getCacheKeyName(reqTypeStopJob, resourceTypeKusciaJob, kj.Name)); ok {
		return
	}

	nlog.Infof("Stop job %v", kj.Name)
	cacheKeyName := getCacheKeyName(reqTypeStopJob, resourceTypeKusciaJob, kj.Name)
	_ = c.inflightRequestCache.Add(cacheKeyName, "", inflightRequestCacheExpiration)

	var wg sync.WaitGroup
	var errs errorcode.Errs
	for domainID := range c.getPartiesDomainInfo(kj) {
		wg.Add(1)
		go func(domainID string) {
			stopJobErr := c.bfiaClient.StopJob(ctx, kj.Spec.Initiator, buildHostFor(domainID), kj.Name)
			if stopJobErr != nil {
				errs.AppendErr(stopJobErr)
			}
			defer wg.Done()
		}(domainID)
	}

	go func(cacheKeyName string) {
		defer c.inflightRequestCache.Set(cacheKeyName, "", finishedInflightRequestCacheExpiration)
		wg.Wait()
		now := metav1.Now().Rfc3339Copy()
		succeededCond, _ := utilsres.GetKusciaJobCondition(&kj.Status, kusciaapisv1alpha1.JobStopSucceeded, true)
		if len(errs) > 0 {
			err := fmt.Errorf("stop interconn job %v request failed, %v", kj.Name, errs.String())
			nlog.Error(err)
			utilsres.SetKusciaJobCondition(now, succeededCond, corev1.ConditionFalse, "ErrorStopJobRequest", err.Error())
		} else {
			utilsres.SetKusciaJobCondition(now, succeededCond, corev1.ConditionTrue, "", "")
		}

		c.stopPartyTasks(ctx, kj)

		if err := c.updateJobStatus(kj, false, true); err != nil {
			nlog.Errorf("Update kuscia job %v status condition failed, %v", kj.Name, err)
		}
	}(cacheKeyName)
}

// stopPartyTasks stops party tasks.
func (c *Controller) stopPartyTasks(ctx context.Context, kj *kusciaapisv1alpha1.KusciaJob) {
	if _, ok := c.inflightRequestCache.Get(getCacheKeyName(reqTypeStopTask, resourceTypeKusciaJob, kj.Name)); ok {
		return
	}

	cacheKeyName := getCacheKeyName(reqTypeStopTask, resourceTypeKusciaJob, kj.Name)
	_ = c.inflightRequestCache.Add(cacheKeyName, "", inflightRequestCacheExpiration)

	taskDomainStopMap := make(map[string]map[string]string)
	for taskID, phase := range kj.Status.TaskStatus {
		if phase != kusciaapisv1alpha1.TaskSucceeded && phase != kusciaapisv1alpha1.TaskFailed {
			taskDomainStopMap[taskID] = map[string]string{}
		}
	}

	if len(taskDomainStopMap) == 0 {
		return
	}

	var wg sync.WaitGroup
	domainRolesInfo := c.getPartiesDomainInfo(kj)
	for taskID := range taskDomainStopMap {
		for domainID := range domainRolesInfo {
			wg.Add(1)
			go func(taskID, domainID string) {
				domainStopInfo := map[string]string{}
				if err := c.bfiaClient.StopTask(ctx, kj.Spec.Initiator, buildHostFor(domainID), taskID); err != nil {
					domainStopInfo[domainID] = err.Error()
				} else {
					domainStopInfo[domainID] = ""
				}
				taskDomainStopMap[taskID] = domainStopInfo
				defer wg.Done()
			}(taskID, domainID)
		}
	}

	go func(cacheKeyName string) {
		defer c.inflightRequestCache.Set(cacheKeyName, "", finishedInflightRequestCacheExpiration)
		wg.Wait()
		for taskID, domainStopInfo := range taskDomainStopMap {
			needUpdate := false
			task, getErr := c.ktLister.KusciaTasks(common.KusciaCrossDomain).Get(taskID)
			copyTask := task.DeepCopy()
			if getErr == nil {
				for domainID, message := range domainStopInfo {
					for _, role := range domainRolesInfo[domainID] {
						updated := setPartyTaskStatus(&copyTask.Status, domainID, role, message, kusciaapisv1alpha1.TaskFailed)
						if updated {
							needUpdate = true
						}
					}
				}
			} else {
				nlog.Errorf("Update task %q party status failed, %v", taskID, getErr)
			}

			if needUpdate {
				_ = c.updatePartyTaskStatus(copyTask)
			}
		}
	}(cacheKeyName)
}

// handleJobCreateStage handles kuscia job with start stage.
func (c *Controller) handleJobStartStage(ctx context.Context, kj *kusciaapisv1alpha1.KusciaJob) {
	// check self stage status to avoid duplicate start
	nlog.Infof("Check job %s stage status %+v", kj.Name, kj.Status.StageStatus)
	stageStatus, ok := kj.Status.StageStatus[kj.Spec.Initiator]
	if ok && (stageStatus == kusciaapisv1alpha1.JobStartStageSucceeded || stageStatus == kusciaapisv1alpha1.JobStartStageFailed) {
		return
	}

	c.startJob(ctx, kj)
}

// startJob starts job.
func (c *Controller) startJob(ctx context.Context, kj *kusciaapisv1alpha1.KusciaJob) {
	if _, ok := c.inflightRequestCache.Get(getCacheKeyName(reqTypeStartJob, resourceTypeKusciaJob, kj.Name)); ok {
		nlog.Infof("Bfia job %v start request cache entered, skip start", kj.Name)
		return
	}
	nlog.Infof("Start job %v", kj.Name)
	utilsres.SetJobPartiesStageStatus(kj, kj.Spec.Initiator, kusciaapisv1alpha1.JobStartStageSucceeded)

	cacheKeyName := getCacheKeyName(reqTypeStartJob, resourceTypeKusciaJob, kj.Name)
	_ = c.inflightRequestCache.Add(cacheKeyName, "", inflightRequestCacheExpiration)

	var wg sync.WaitGroup
	var errs errorcode.Errs
	for domainID := range c.getPartiesDomainInfo(kj) {
		wg.Add(1)
		go func(domainID string, kj *kusciaapisv1alpha1.KusciaJob) {
			startJobErr := c.bfiaClient.StartJob(ctx, kj.Spec.Initiator, buildHostFor(domainID), kj.Name)
			if startJobErr != nil {
				errs.AppendErr(startJobErr)
				utilsres.SetJobPartiesStageStatus(kj, domainID, kusciaapisv1alpha1.JobStartStageFailed)
			} else {
				utilsres.SetJobPartiesStageStatus(kj, domainID, kusciaapisv1alpha1.JobStartStageSucceeded)
			}
			defer wg.Done()
		}(domainID, kj)
	}

	go func(cacheKeyName string) {
		defer c.inflightRequestCache.Set(cacheKeyName, "", finishedInflightRequestCacheExpiration)
		wg.Wait()
		if err := c.updateJobStatus(kj, false, true); err != nil {
			nlog.Errorf("Update kuscia job %v status condition failed, %v", kj.Name, err)
		}
	}(cacheKeyName)
}

// getPartiesDomainID gets domain id and role of parties.
func (c *Controller) getPartiesDomainInfo(kusciaJob *kusciaapisv1alpha1.KusciaJob) map[string][]string {
	domainRoleMap := make(map[string][]string)
	for _, party := range kusciaJob.Spec.Tasks[0].Parties {
		if utilsres.IsOuterBFIAInterConnDomain(c.nsLister, party.DomainID) {
			if roles, exist := domainRoleMap[party.DomainID]; exist {
				roles = append(roles, party.Role)
				domainRoleMap[party.DomainID] = roles
			} else {
				domainRoleMap[party.DomainID] = []string{party.Role}
			}
		}
	}

	return domainRoleMap
}

// getReqDomainIDFromKusciaJob gets request domain id from kuscia job.
func (c *Controller) getReqDomainIDFromKusciaJob(kj *kusciaapisv1alpha1.KusciaJob) string {
	for _, party := range kj.Spec.Tasks[0].Parties {
		if !utilsres.IsOuterBFIAInterConnDomain(c.nsLister, party.DomainID) {
			return party.DomainID
		}
	}
	return ""
}

// updateJobStatus updates job status.
func (c *Controller) updateJobStatus(kj *kusciaapisv1alpha1.KusciaJob, taskStatusUpdated, stageStatusUpdated bool) (err error) {
	var originalTaskStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase
	if taskStatusUpdated {
		originalTaskStatus = make(map[string]kusciaapisv1alpha1.KusciaTaskPhase)
		for k, v := range kj.Status.TaskStatus {
			originalTaskStatus[k] = v
		}
	}

	var originalStageStatus map[string]kusciaapisv1alpha1.JobStagePhase
	if stageStatusUpdated {
		originalStageStatus = make(map[string]kusciaapisv1alpha1.JobStagePhase)
		for k, v := range kj.Status.StageStatus {
			originalStageStatus[k] = v
		}
	}

	jobName := kj.Name
	for i, curJob := 0, kj; ; i++ {
		nlog.Infof("Start updating kuscia job %q status, %+v", jobName, curJob.Status)
		if _, err = c.kusciaClient.KusciaV1alpha1().KusciaJobs(curJob.Namespace).UpdateStatus(context.Background(), curJob, metav1.UpdateOptions{}); err == nil {
			nlog.Infof("Finish updating kuscia job %q status", jobName)
			return nil
		}

		nlog.Warnf("Failed to update kuscia job %q status, %v", jobName, err)
		if i >= statusUpdateRetries {
			break
		}

		curJob, err = c.kusciaClient.KusciaV1alpha1().KusciaJobs(curJob.Namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get the newest kuscia job %q, %v", jobName, err)
		}

		needUpdate := false
		if taskStatusUpdated && !reflect.DeepEqual(curJob.Status.TaskStatus, originalTaskStatus) {
			needUpdate = true
			curJob.Status.TaskStatus = originalTaskStatus
		}

		if stageStatusUpdated && !reflect.DeepEqual(curJob.Status.StageStatus, originalStageStatus) {
			needUpdate = true
			curJob.Status.StageStatus = originalStageStatus
		}

		if !needUpdate {
			return nil
		}
	}
	return err
}

// setKusciaJobTaskStatus sets kuscia job task status.
func setKusciaJobTaskStatus(kj *kusciaapisv1alpha1.KusciaJob, status map[string]string) bool {
	if len(status) == 0 {
		return false
	}

	taskPhase := make(map[string]kusciaapisv1alpha1.KusciaTaskPhase)
	for taskID, icTaskPhase := range status {
		taskPhase[taskID] = bfiacommon.InterConnTaskPhaseToKusciaTaskPhase[icTaskPhase]
	}

	return utilsres.MergeKusciaJobTaskStatus(kj, taskPhase)
}
