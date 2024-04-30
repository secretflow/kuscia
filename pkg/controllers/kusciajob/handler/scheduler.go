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

package handler

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/uuid"
	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	intercommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	updateRetries = 3
)

// JobScheduler will compute current kuscia job and do scheduling until the kuscia job finished.
type JobScheduler struct {
	kusciaClient          versioned.Interface
	kusciaTaskLister      kuscialistersv1alpha1.KusciaTaskLister
	domainLister          kuscialistersv1alpha1.DomainLister
	namespaceLister       corelisters.NamespaceLister
	enableWorkloadApprove bool
}

// NewJobScheduler return kuscia job scheduler.
func NewJobScheduler(deps *Dependencies) *JobScheduler {
	return &JobScheduler{
		kusciaClient:          deps.KusciaClient,
		kusciaTaskLister:      deps.KusciaTaskLister,
		namespaceLister:       deps.NamespaceLister,
		domainLister:          deps.DomainLister,
		enableWorkloadApprove: deps.EnableWorkloadApprove,
	}
}

// handleStageCommand handle stage command.
func (h *JobScheduler) handleStageCommand(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) (hasReconciled bool, err error) {
	stageCmd, cmdTrigger, ok := h.getStageCmd(job)
	if !ok {
		return
	}

	nlog.Infof("Handle stage trigger: %s send command: %s.", cmdTrigger, stageCmd)
	switch kusciaapisv1alpha1.JobStage(stageCmd) {
	case kusciaapisv1alpha1.JobStartStage:
		return h.handleStageCmdStart(now, job)
	case kusciaapisv1alpha1.JobStopStage:
		err = h.handleStageCmdStop(now, job)
		return true, err
	case kusciaapisv1alpha1.JobCancelStage:
		err = h.handleStageCmdCancelled(now, job)
		return true, err
	case kusciaapisv1alpha1.JobRestartStage:
		return h.handleStageCmdRestart(now, job)
	case kusciaapisv1alpha1.JobSuspendStage:
		return h.handleStageCmdSuspend(now, job)
	}
	return
}

// handleStageCmdStart handles job-start stage.
func (h *JobScheduler) handleStageCmdStart(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) (hasReconciled bool, err error) {
	// get own party
	ownP, _, _ := h.getAllParties(job)
	if job.Status.StageStatus == nil {
		job.Status.StageStatus = make(map[string]kusciaapisv1alpha1.JobStagePhase)
	}
	// set own stage status
	for p := range ownP {
		if v, ok := job.Status.StageStatus[p]; ok && v == kusciaapisv1alpha1.JobStartStageSucceeded {
			continue
		}
		job.Status.StageStatus[p] = kusciaapisv1alpha1.JobStartStageSucceeded
		hasReconciled = true
		// print command log
		cmd, cmdTrigger, _ := h.getStageCmd(job)
		nlog.Infof("Job: %s party: %s execute the cmd: %s, set own party: %s stage to 'startSuccess'.", job.Name, cmdTrigger, cmd, p)
	}
	return hasReconciled, nil
}

// handleStageCmdReStart handles job-restart stage.
func (h *JobScheduler) handleStageCmdRestart(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) (hasReconciled bool, err error) {
	// print command log
	cmd, cmdTrigger, _ := h.getStageCmd(job)
	nlog.Infof("Job: %s party: %s execute the cmd: %s.", job.Name, cmdTrigger, cmd)
	// handle 'failed' and 'suspend' phase
	if job.Status.Phase == kusciaapisv1alpha1.KusciaJobFailed || job.Status.Phase == kusciaapisv1alpha1.KusciaJobSuspended {
		// set job phase to 'running'
		job.Status.Phase = kusciaapisv1alpha1.KusciaJobRunning
		partyStage := kusciaapisv1alpha1.JobRestartStageSucceeded
		job.Status.Message = fmt.Sprintf("This job is restarted by %s", cmdTrigger)
		job.Status.Reason = fmt.Sprintf("Party: %s execute the cmd: %s.", cmdTrigger, cmd)
		// delete the failed task
		if err := h.deleteNotSuccessTasks(job); err != nil {
			// delete the failed task failed
			partyStage = kusciaapisv1alpha1.JobRestartStageFailed
			job.Status.Message = fmt.Sprintf("Restart this job and delete the 'failed' phase task failed, error: %s.", err.Error())
		}
		// set own party stage status to 'restartSuccess' or 'restartFailed'
		ownP, _, _ := h.getAllParties(job)
		if job.Status.StageStatus == nil {
			job.Status.StageStatus = make(map[string]kusciaapisv1alpha1.JobStagePhase)
		}
		for p := range ownP {
			job.Status.StageStatus[p] = partyStage
		}
		return true, nil
	}
	// handle running phase
	if job.Status.Phase == kusciaapisv1alpha1.KusciaJobRunning {
		// begin schedule tasks if all party restart success
		isComplete := h.isAllPartyRestartComplete(job)
		// not all party  complete handling 'restart' stage
		if !isComplete {
			// wait for all party restart success or any party restart failed
			return true, nil
		}
		// Since a Job can be restarted more than once, we need to set the job's status to 'start' so that the job can be restarted again.
		// To avoid conflicts, only the initiator sets the stage
		if ok, _ := intercommon.SelfClusterIsInitiator(h.domainLister, job); ok {
			if err = h.setJobStage(job.Name, job.Spec.Initiator, string(kusciaapisv1alpha1.JobStartStage)); err != nil {
				nlog.Errorf("Set job stage to 'start' failed, error: %s.", err.Error())
				// wait for all party restart success or any party restart failed
				return true, err
			}
			return true, nil
		}
		return false, nil
	}
	nlog.Errorf("Unexpect phase: %s of job: %s.", job.Status.Phase, job.Name)
	return false, nil
}

// handleStageCmdStop handles job-stop stage.
func (h *JobScheduler) handleStageCmdStop(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) (err error) {
	// print command log
	cmd, cmdTrigger, _ := h.getStageCmd(job)
	nlog.Infof("Job: %s party: %s execute the cmd: %s.", job.Name, cmdTrigger, cmd)
	stageStatus := kusciaapisv1alpha1.JobStopStageSucceeded
	ownP, _, _ := h.getAllParties(job)
	if job.Status.StageStatus == nil {
		job.Status.StageStatus = make(map[string]kusciaapisv1alpha1.JobStagePhase)
	}
	// stop the running task
	if err = h.stopTasks(now, job); err != nil {
		nlog.Errorf("Stop 'runnning' task of job: %s  failed, error: %s.", job.Name, err.Error())
		return err
	}
	// set own stage status
	for p := range ownP {
		job.Status.StageStatus[p] = stageStatus
	}
	// set job phase to failed
	reason := fmt.Sprintf("Party: %s execute the cmd: %s.", cmdTrigger, cmd)
	setKusciaJobStatus(now, &job.Status, kusciaapisv1alpha1.KusciaJobFailed, reason, reason)
	setRunningTaskStatusToFailed(&job.Status)
	return nil
}

// handleStageCmdCancelled handles job 'cancel' stage.
func (h *JobScheduler) handleStageCmdCancelled(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) (err error) {
	// print command log
	cmd, cmdTrigger, _ := h.getStageCmd(job)
	nlog.Infof("Job: %s party: %s execute the cmd: %s.", job.Name, cmdTrigger, cmd)
	stageStatus := kusciaapisv1alpha1.JobCancelStageSucceeded
	// get own party
	ownP, _, _ := h.getAllParties(job)
	if job.Status.StageStatus == nil {
		job.Status.StageStatus = make(map[string]kusciaapisv1alpha1.JobStagePhase)
	}
	// stop the running task
	if err = h.stopTasks(now, job); err != nil {
		nlog.Errorf("Stop 'runnning' task of job: %s  failed, error: %s.", job.Name, err.Error())
		return err
	}
	// set own stage status
	for p := range ownP {
		job.Status.StageStatus[p] = stageStatus
	}
	// set job phase to cancelled
	reason := fmt.Sprintf("Party: %s execute the cmd: %s.", cmdTrigger, cmd)
	setKusciaJobStatus(now, &job.Status, kusciaapisv1alpha1.KusciaJobCancelled, reason, "")
	return nil
}

// handleStageCmdSuspend handles job 'suspend' stage.
func (h *JobScheduler) handleStageCmdSuspend(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) (hasReconciled bool, err error) {
	if job.Status.Phase != kusciaapisv1alpha1.KusciaJobRunning {
		return false, nil
	}
	// print command log
	cmd, cmdTrigger, _ := h.getStageCmd(job)
	nlog.Infof("Job: %s party: %s execute the cmd: %s.", job.Name, cmdTrigger, cmd)
	stageStatus := kusciaapisv1alpha1.JobSuspendStageSucceeded
	ownP, _, _ := h.getAllParties(job)
	if job.Status.StageStatus == nil {
		job.Status.StageStatus = make(map[string]kusciaapisv1alpha1.JobStagePhase)
	}
	// stop the running task
	if err = h.stopTasks(now, job); err != nil {
		nlog.Errorf("Stop 'runnning' task of job: %s  failed, error: %s.", job.Name, err.Error())
		return false, err
	}
	// set own stage status
	for p := range ownP {
		job.Status.StageStatus[p] = stageStatus
	}
	// set job phase to suspended
	reason := fmt.Sprintf("Party: %s execute the cmd: %s.", cmdTrigger, cmd)
	setKusciaJobStatus(now, &job.Status, kusciaapisv1alpha1.KusciaJobSuspended, reason, "")
	return true, nil
}

func (h *JobScheduler) getStageCmd(job *kusciaapisv1alpha1.KusciaJob) (stageCmd, cmdTrigger string, ok bool) {
	stageCmd, ok = job.Labels[common.LabelJobStage]
	cmdTrigger, ok = job.Labels[common.LabelJobStageTrigger]
	return
}

func updateJobTime(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) {
	if job.Status.StartTime == nil {
		job.Status.StartTime = &now
	}
	job.Status.LastReconcileTime = &now
}

func (h *JobScheduler) validateJob(now metav1.Time, job *kusciaapisv1alpha1.KusciaJob) (needUpdateStatus, validatePass bool) {
	jobValidatedCond, _ := utilsres.GetKusciaJobCondition(&job.Status, kusciaapisv1alpha1.JobValidated, true)
	// have finished validate
	if jobValidatedCond.Status == corev1.ConditionTrue {
		return false, true
	}
	// validate job
	err := h.kusciaJobValidate(job)
	if err == nil {
		// validate pass
		utilsres.SetKusciaJobCondition(now, jobValidatedCond, corev1.ConditionTrue, "", "")
		return true, true
	}
	// validate failed
	utilsres.SetKusciaJobCondition(now, jobValidatedCond, corev1.ConditionFalse, "ValidateFailed", fmt.Sprintf("Validate job failed, %v", err.Error()))
	setKusciaJobStatus(now, &job.Status, kusciaapisv1alpha1.KusciaJobFailed, "KusciaJobValidateFailed", "")
	return true, false
}

// someApprovalReject:  All of the parties accept means the job is accepted
func (h *JobScheduler) allApprovalAccept(job *kusciaapisv1alpha1.KusciaJob) (ture bool, err error) {
	for _, party := range h.getParties(job) {
		domain, err := h.domainLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("Check approval failed, error: %s.", err.Error())
			return false, err
		}
		// initiator auto approval as accepted
		if party.DomainID == job.Spec.Initiator {
			continue
		}
		//  Todo: check whether need to approval
		if result, ok := job.Status.ApproveStatus[domain.Name]; ok && result == kusciaapisv1alpha1.JobAccepted {
			continue
		}
		return false, nil
	}
	return true, nil
}

// someApprovalReject:  One of the parties reject means the job is rejected
func (h *JobScheduler) someApprovalReject(job *kusciaapisv1alpha1.KusciaJob) (ture bool, rejectParty string, err error) {
	for _, party := range h.getParties(job) {
		domain, err := h.domainLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("Check approval failed, error: %s.", err.Error())
			return false, "", err
		}
		// initiator auto approval as accepted
		if party.DomainID == job.Spec.Initiator {
			continue
		}
		//  Todo: check whether need to approval
		if result, ok := job.Status.ApproveStatus[domain.Name]; ok && result == kusciaapisv1alpha1.JobRejected {
			return true, domain.Name, nil
		}
	}
	return false, "", nil
}

func (h *JobScheduler) allPartyCreateSuccess(job *kusciaapisv1alpha1.KusciaJob) (ture bool, err error) {
	for _, party := range h.getParties(job) {
		domain, err := h.domainLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("Check party create success status failed, error: %s.", err.Error())
			return false, err
		}
		if stageStatus, ok := job.Status.StageStatus[domain.Name]; ok && stageStatus == kusciaapisv1alpha1.JobCreateStageSucceeded ||
			stageStatus == kusciaapisv1alpha1.JobStartStageSucceeded {
			continue
		}
		return false, nil
	}
	return true, nil
}

func (h *JobScheduler) somePartyCreateFailed(job *kusciaapisv1alpha1.KusciaJob) (ture bool, rejectParty string, err error) {
	for _, party := range h.getParties(job) {
		domain, err := h.domainLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("Check party create fail status failed, error: %s.", err.Error())
			return false, "", err
		}
		if stageStatus, ok := job.Status.StageStatus[domain.Name]; ok && stageStatus == kusciaapisv1alpha1.JobCreateStageFailed {
			return true, domain.Name, nil
		}
	}
	return false, "", nil
}

func (h *JobScheduler) allPartyStartSuccess(job *kusciaapisv1alpha1.KusciaJob) (ture bool, err error) {
	for _, party := range h.getParties(job) {
		domain, err := h.domainLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("Check party create success status failed, error: %s.", err.Error())
			return false, err
		}
		if stageStatus, ok := job.Status.StageStatus[domain.Name]; ok && stageStatus == kusciaapisv1alpha1.JobStartStageSucceeded {
			continue
		}
		return false, nil
	}
	return true, nil
}

func (h *JobScheduler) somePartyStartFailed(job *kusciaapisv1alpha1.KusciaJob) (ture bool, failedParty string, err error) {
	for _, party := range h.getParties(job) {
		domain, err := h.domainLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("Check party create fail status failed, error: %s.", err.Error())
			return false, "", err
		}
		if stageStatus, ok := job.Status.StageStatus[domain.Name]; ok && stageStatus == kusciaapisv1alpha1.JobStartStageFailed ||
			stageStatus == kusciaapisv1alpha1.JobCreateStageFailed {
			return true, domain.Name, nil
		}
	}
	return false, "", nil
}

func (h *JobScheduler) isAllPartyRestartSuccess(job *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	for _, party := range h.getParties(job) {
		domain, err := h.domainLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("Check party 'restartSuccess' status failed, error: %s.", err.Error())
			return false, err
		}
		if stageStatus, ok := job.Status.StageStatus[domain.Name]; ok && stageStatus == kusciaapisv1alpha1.JobRestartStageSucceeded {
			continue
		}
		return false, nil
	}
	return true, nil
}

func (h *JobScheduler) isAllPartyRestartComplete(job *kusciaapisv1alpha1.KusciaJob) bool {

	for _, party := range h.getParties(job) {
		stageStatus, ok := job.Status.StageStatus[party.DomainID]
		if !ok {
			return false
		}
		if stageStatus != kusciaapisv1alpha1.JobRestartStageFailed && stageStatus != kusciaapisv1alpha1.JobRestartStageSucceeded {
			nlog.Infof("Party: %s is not complete to handle 'restart' stage.", party.DomainID)
			return false
		}
	}
	return true
}
func (h *JobScheduler) getAllParties(job *kusciaapisv1alpha1.KusciaJob) (ownParties, otherParties map[string]kusciaapisv1alpha1.Party, err error) {
	partyMap := make(map[string]kusciaapisv1alpha1.Party)
	ownParties = make(map[string]kusciaapisv1alpha1.Party)
	otherParties = make(map[string]kusciaapisv1alpha1.Party)
	for _, t := range job.Spec.Tasks {
		for _, p := range t.Parties {
			partyMap[p.DomainID] = p
		}
	}
	for k, v := range partyMap {
		domain, err := h.domainLister.Get(k)
		if err != nil {
			nlog.Errorf("getAllParties failed, get domain: %s, error: %s.", k, err.Error())
			return nil, nil, err
		}
		if domain.Spec.Role == kusciaapisv1alpha1.Partner {
			otherParties[k] = v
			continue
		}
		ownParties[k] = v
	}
	return
}

func (h *JobScheduler) getParties(job *kusciaapisv1alpha1.KusciaJob) map[string]kusciaapisv1alpha1.Party {
	partyMap := make(map[string]kusciaapisv1alpha1.Party)
	for _, t := range job.Spec.Tasks {
		for _, p := range t.Parties {
			partyMap[p.DomainID] = p
		}
	}
	return partyMap
}

func (h *JobScheduler) stopTasks(now metav1.Time, kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	for taskID, phase := range kusciaJob.Status.TaskStatus {
		if phase == kusciaapisv1alpha1.TaskFailed || phase == kusciaapisv1alpha1.TaskSucceeded {
			continue
		}

		kt, err := h.kusciaTaskLister.KusciaTasks(common.KusciaCrossDomain).Get(taskID)
		if err != nil && !k8serrors.IsNotFound(err) {
			nlog.Errorf("Get kuscia task %v failed, so skip stopping this task", taskID)
			return err
		}

		copyKt := kt.DeepCopy()
		setKusciaTaskStatus(now, &copyKt.Status, kusciaapisv1alpha1.TaskFailed, "KusciaJobStopped", "Job was stopped")
		for _, party := range copyKt.Spec.Parties {
			if utilsres.IsOuterBFIAInterConnDomain(h.namespaceLister, party.DomainID) {
				continue
			}
			if utilsres.IsPartnerDomain(h.namespaceLister, party.DomainID) {
				continue
			}
			found := false
			for i := range copyKt.Status.PartyTaskStatus {
				if copyKt.Status.PartyTaskStatus[i].DomainID == party.DomainID && copyKt.Status.PartyTaskStatus[i].Role == party.Role {
					found = true
					copyKt.Status.PartyTaskStatus[i].Phase = kusciaapisv1alpha1.TaskFailed
				}
			}

			if !found {
				partyTaskStatus := kusciaapisv1alpha1.PartyTaskStatus{DomainID: party.DomainID, Role: party.Role, Phase: kusciaapisv1alpha1.TaskFailed}
				copyKt.Status.PartyTaskStatus = append(copyKt.Status.PartyTaskStatus, partyTaskStatus)
			}
		}

		if err = utilsres.UpdateKusciaTaskStatus(h.kusciaClient, kt, copyKt); err != nil {
			return err
		}
	}
	return nil
}

func (h *JobScheduler) deleteNotSuccessTasks(kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	var tasks []string
	for taskID, phase := range kusciaJob.Status.TaskStatus {
		if phase == kusciaapisv1alpha1.TaskSucceeded {
			continue
		}
		kt, err := h.kusciaTaskLister.KusciaTasks(common.KusciaCrossDomain).Get(taskID)
		if err != nil && !k8serrors.IsNotFound(err) {
			nlog.Warnf("Get kuscia task %v failed, so skip delete this task, error: %s.", taskID, err.Error())
			return err
		}
		err = h.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Delete(context.Background(), kt.Name, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			nlog.Warnf("Delete kuscia task %v failed,  so skip delete this task, error: %s.", taskID, err.Error())
			return err
		}
		tasks = append(tasks, taskID)
	}
	for _, v := range tasks {
		delete(kusciaJob.Status.TaskStatus, v)
	}
	return nil
}

// kusciaJobValidate check whether kusciaJob is valid.
func (h *JobScheduler) kusciaJobValidate(kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	if _, err := h.namespaceLister.Get(kusciaJob.Spec.Initiator); err != nil {
		return fmt.Errorf("can't find initiator namespace %v under cluster, %v", kusciaJob.Spec.Initiator, err)
	}

	if len(kusciaJob.Spec.Tasks) == 0 {
		return fmt.Errorf("kuscia job should include at least one of task")
	}

	findInitiator := false
	for _, party := range kusciaJob.Spec.Tasks[0].Parties {
		if _, err := h.namespaceLister.Get(party.DomainID); err != nil {
			return fmt.Errorf("can't find party namespace %v under cluster, %v", party.DomainID, err)
		}

		if party.DomainID == kusciaJob.Spec.Initiator {
			findInitiator = true
		}
	}
	if !findInitiator {
		return fmt.Errorf("initiator should be one of task parties")
	}

	if err := kusciaJobDependenciesExits(kusciaJob); err != nil {
		return err
	}
	return kusciaJobHasTaskCycle(kusciaJob)
}

// annotateKusciaJob preprocess the kuscia job.
// update labels and annotations value
// 1 inter connection protocol
// 2 SelfClusterAsInitiator
func (h *JobScheduler) annotateKusciaJob(job *kusciaapisv1alpha1.KusciaJob) (hasUpdate bool, err error) {
	if job.Annotations != nil {
		if _, ok := job.Annotations[common.InterConnSelfPartyAnnotationKey]; ok {
			return false, nil
		}
	}
	// annotate SelfClusterAsInitiator true or false
	if err = h.annotateSelfClusterAsInitiator(job); err != nil {
		return false, err
	}
	// annotate InterConn
	if err = h.annotateInterConn(job); err != nil {
		return false, err
	}

	// update label to k8s apiserver
	update := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) {
		mergeAnnotations(kusciaJob, job)
	}

	hasUpdated := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) bool {
		if job.Annotations != nil {
			if _, ok := job.Annotations[common.SelfClusterAsInitiatorAnnotationKey]; ok {
				return true
			}
		}
		return false
	}
	return true, utilsres.UpdateKusciaJob(h.kusciaClient, job, hasUpdated, update, updateRetries)
}

func mergeAnnotations(originJob, currentJob *kusciaapisv1alpha1.KusciaJob) {
	if originJob.Annotations == nil {
		originJob.Annotations = make(map[string]string)
	}
	for k, v := range currentJob.Annotations {
		originJob.Annotations[k] = v
	}
}

// annotateSelfClusterAsInitiator checks if self cluster domain is scheduling party.
func (h *JobScheduler) annotateSelfClusterAsInitiator(job *kusciaapisv1alpha1.KusciaJob) (err error) {
	ns, err := h.namespaceLister.Get(job.Spec.Initiator)
	if err != nil {
		nlog.Errorf("setSelfClusterAsInitiator failed, Get domain error: %s.", err.Error())
		return err
	}
	if job.Annotations == nil {
		job.Annotations = make(map[string]string)
	}
	if ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) {
		job.Annotations[common.SelfClusterAsInitiatorAnnotationKey] = "false"
		return
	}
	job.Annotations[common.SelfClusterAsInitiatorAnnotationKey] = "true"
	job.Annotations[common.InitiatorAnnotationKey] = job.Spec.Initiator
	return
}

func (h *JobScheduler) annotateInterConn(job *kusciaapisv1alpha1.KusciaJob) (err error) {
	var (
		bfiaDomainList   []string
		kusciaDomainList []string
		selfDomainList   []string
	)
	for _, party := range h.getParties(job) {
		ns, err := h.namespaceLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("labelInterConn failed to get domain %v namespace, %v", party.DomainID, err)
			return err
		}
		if ns.Labels != nil && ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) {
			// only initiator need to label kusciaParty and bfiaParty
			if !utilsres.SelfClusterAsInitiator(h.namespaceLister, job.Spec.Initiator, job.Annotations) {
				continue
			}
			switch ns.Labels[common.LabelInterConnProtocols] {
			case string(kusciaapisv1alpha1.InterConnBFIA):
				bfiaDomainList = append(bfiaDomainList, ns.Name)
			case string(kusciaapisv1alpha1.InterConnKuscia):
				kusciaDomainList = append(kusciaDomainList, ns.Name)
			default:
				kusciaDomainList = append(kusciaDomainList, ns.Name)
			}
		} else {
			selfDomainList = append(selfDomainList, ns.Name)
		}
	}

	if len(bfiaDomainList) > 0 {
		value := domainListToString(bfiaDomainList)
		job.Annotations[common.InterConnBFIAPartyAnnotationKey] = value
	}
	if len(kusciaDomainList) > 0 {
		value := domainListToString(kusciaDomainList)
		job.Annotations[common.InterConnKusciaPartyAnnotationKey] = value
	}
	if len(selfDomainList) > 0 {
		value := domainListToString(selfDomainList)
		job.Annotations[common.InterConnSelfPartyAnnotationKey] = value
	}
	return
}

func (h *JobScheduler) setJobStage(jobID string, trigger, stage string) (err error) {
	job, err := h.kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Get(context.Background(), jobID, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Get job: %s failed, error: %s.", jobID, err.Error())
		return err
	}
	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}
	job.Labels[common.LabelJobStage] = stage
	job.Labels[common.LabelJobStageTrigger] = trigger
	jobVersion := "1"
	if v, ok := job.Labels[common.LabelJobStageVersion]; ok {
		if iV, err := strconv.Atoi(v); err == nil {
			jobVersion = strconv.Itoa(iV + 1)
		}
	}
	job.Labels[common.LabelJobStageVersion] = jobVersion
	_, err = h.kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain).Update(context.Background(), job, metav1.UpdateOptions{})
	if err != nil {
		nlog.Errorf("Update job: %s failed, error: %s.", job.Name, err.Error())
		return err
	}
	return nil
}

func domainListToString(domains []string) (labelValue string) {
	for _, v := range domains {
		if labelValue == "" {
			labelValue = v
			continue
		}
		labelValue += "_" + v
	}
	return
}

// setJobTaskID sets job task id.
func (h *JobScheduler) setJobTaskID(kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	needSetTaskID := false
	for i := range kusciaJob.Spec.Tasks {
		if kusciaJob.Spec.Tasks[i].TaskID == "" {
			needSetTaskID = true
		}
	}

	if !needSetTaskID {
		return false, nil
	}

	update := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) {
		for i := range kusciaJob.Spec.Tasks {
			if kusciaJob.Spec.Tasks[i].TaskID == "" {
				kusciaJob.Spec.Tasks[i].TaskID = generateTaskID(kusciaJob.Name)
			}
		}
	}

	update(kusciaJob)

	hasUpdated := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) bool {
		for i := range kusciaJob.Spec.Tasks {
			if kusciaJob.Spec.Tasks[i].TaskID == "" {
				return false
			}
		}
		return true
	}
	return true, utilsres.UpdateKusciaJob(h.kusciaClient, kusciaJob, hasUpdated, update, updateRetries)
}

// isBFIAInterConnJob checks if the job is interconn with BFIA protocol.
func isBFIAInterConnJob(nsLister corelisters.NamespaceLister, kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	if kusciaJob.Labels != nil {
		if _, ok := kusciaJob.Annotations[common.InterConnBFIAPartyAnnotationKey]; ok {
			return true, nil
		}
	}
	return false, nil
}

func isInterConnJob(kusciaJob *kusciaapisv1alpha1.KusciaJob) bool {
	_, existBFIA := kusciaJob.Annotations[common.InterConnBFIAPartyAnnotationKey]
	_, existKuscia := kusciaJob.Annotations[common.InterConnKusciaPartyAnnotationKey]
	if existBFIA || existKuscia {
		return true
	}
	return false
}

// kusciaJobHasTaskCycle check whether kusciaJob's tasks has cycles.
func kusciaJobHasTaskCycle(kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	// check dependencies cycle.
	copyKusciaJob := kusciaJob.DeepCopy()
	for {
		// 1. we remove all 0 dependencies subtasks.
		removeSubtasks := make(map[string]bool, 0)
		copyKusciaJob.Spec.Tasks = kusciaTaskTemplateFilter(copyKusciaJob.Spec.Tasks,
			func(t kusciaapisv1alpha1.KusciaTaskTemplate, i int) bool {
				noDependencies := len(t.Dependencies) == 0
				if noDependencies {
					removeSubtasks[t.Alias] = true
				}
				return !noDependencies
			})

		// 2. then we remove all removed-task dependencies from other subtasks.
		for i, t := range copyKusciaJob.Spec.Tasks {
			copyKusciaJob.Spec.Tasks[i].Dependencies = stringFilter(t.Dependencies,
				func(taskName string, i int) bool {
					_, exist := removeSubtasks[taskName]
					return !exist
				})
		}

		// when len(removeSubtasks) == 0, it
		// - has no cycle when all sub-tasks has no dependencies.
		// - has cycles when any sub-tasks has dependencies.
		if len(removeSubtasks) == 0 {
			for _, t := range copyKusciaJob.Spec.Tasks {
				if len(t.Dependencies) != 0 {
					return fmt.Errorf("validate failed: cycled dependencies")
				}
			}
			break
		}
	}

	return nil
}

// kusciaJobDependenciesExits check whether all kusciaJob's tasks dependencies exists.
func kusciaJobDependenciesExits(kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	copyKusciaJob := kusciaJob.DeepCopy()
	taskIDSet := make(map[string]bool, 0)
	for _, t := range copyKusciaJob.Spec.Tasks {
		taskIDSet[t.Alias] = true
	}

	for _, t := range copyKusciaJob.Spec.Tasks {
		for _, d := range t.Dependencies {
			if _, exits := taskIDSet[d]; !exits {
				return fmt.Errorf("validate failed: task %s has not exist dependency task %s", t.Alias, d)
			}
		}
	}

	return nil
}

// buildJobSubTaskStatus returns current subtask status.
func buildJobSubTaskStatus(currentSubTasks []*kusciaapisv1alpha1.KusciaTask, job *kusciaapisv1alpha1.KusciaJob) (map[string]kusciaapisv1alpha1.KusciaTaskPhase, map[string]kusciaapisv1alpha1.KusciaTaskPhase) {
	subTaskStatusWithAlias := make(map[string]kusciaapisv1alpha1.KusciaTaskPhase, 0)
	subTaskStatusWithID := make(map[string]kusciaapisv1alpha1.KusciaTaskPhase, 0)
	for idx := range currentSubTasks {
		for _, task := range job.Spec.Tasks {
			if task.TaskID == currentSubTasks[idx].Name {
				subTaskStatusWithAlias[task.Alias] = currentSubTasks[idx].Status.Phase
				subTaskStatusWithID[task.TaskID] = currentSubTasks[idx].Status.Phase
			}
		}
	}
	return subTaskStatusWithAlias, subTaskStatusWithID
}

// buildJobStatus builds kuscia job status.
func buildJobStatus(now metav1.Time,
	kjStatus *kusciaapisv1alpha1.KusciaJobStatus,
	currentJobStatusPhase kusciaapisv1alpha1.KusciaJobPhase,
	currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase) bool {
	needUpdate := false
	if kjStatus.Phase != currentJobStatusPhase {
		needUpdate = true
		kjStatus.Phase = currentJobStatusPhase
	}

	if currentJobStatusPhase == kusciaapisv1alpha1.KusciaJobSucceeded || currentJobStatusPhase == kusciaapisv1alpha1.KusciaJobFailed {
		if kjStatus.CompletionTime == nil {
			needUpdate = true
			kjStatus.CompletionTime = &now
		}
	}

	if !reflect.DeepEqual(kjStatus.TaskStatus, currentSubTasksStatus) {
		needUpdate = true
		kjStatus.TaskStatus = currentSubTasksStatus
	}

	return needUpdate
}

// jobStatusPhaseFrom will computer this job status from currentSubTasksStatus.
// Finished task means the task is succeeded or failed.
// Ready task means it can be scheduled but not scheduled.
// The job status phase means:
// Pending: this job has been submitted, but has no subtasks.
// Running : least one subtask is running.
// Succeeded : all subtasks are finished and all critical subtasks are succeeded.
// Failed:
//   - BestEffort: least one critical subtasks is failed. But has no readyTask subtasks and running subtasks.
//   - Strict: least one critical subtasks is failed. But some scheduled subtasks may be not scheduled.
func jobStatusPhaseFrom(job *kusciaapisv1alpha1.KusciaJob, currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase) (phase kusciaapisv1alpha1.KusciaJobPhase) {
	tasks := currentTaskMapFrom(job, currentSubTasksStatus)

	// no subtasks mean the job is pending.
	if tasks.AllMatch(taskNotExists) {
		return kusciaapisv1alpha1.KusciaJobRunning
	}

	// Critical task means the task is not tolerable.
	// Ready task means it can be scheduled but not scheduled.
	criticalTasks := tasks.criticalTaskMap()
	readyTasks := tasks.readyTaskMap()
	nlog.Infof("jobStatusPhaseFrom readyTasks=%+v, tasks=%+v, kusciaJobId=%s",
		readyTasks.ToShortString(), tasks.ToShortString(), job.Name)

	// all subtasks succeed and all critical subtasks succeeded means the job is succeeded.
	if tasks.AllMatch(taskFinished) && criticalTasks.AllMatch(taskSucceeded) {
		return kusciaapisv1alpha1.KusciaJobSucceeded
	}

	switch job.Spec.ScheduleMode {
	case kusciaapisv1alpha1.KusciaJobScheduleModeStrict:
		// in Strict mode, any critical subtasks failed means the job is failed.
		if criticalTasks.AnyMatch(taskFailed) {
			return kusciaapisv1alpha1.KusciaJobFailed
		}
	case kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort:
		// in BestEffort mode, has no readyTask subtasks and running subtasks, and least one critical subtasks is failed.
		if len(readyTasks) == 0 && !tasks.AnyMatch(taskRunning) &&
			criticalTasks.AnyMatch(taskFailed) {
			nlog.Infof("jobStatusPhaseFrom failed readyTasks=%+v, tasks=%+v, kusciaJobId=%s",
				readyTasks.ToShortString(), tasks.ToShortString(), job.Name)
			return kusciaapisv1alpha1.KusciaJobFailed
		}
	default:
		// unreachable code, return Failed. Mode will be validated when this job committed.
		return kusciaapisv1alpha1.KusciaJobFailed
	}

	// else job is running
	return kusciaapisv1alpha1.KusciaJobRunning
}

// ShouldReconcile return whether this job need to reconcile again.
func ShouldReconcile(job *kusciaapisv1alpha1.KusciaJob) bool {
	if job.Status.Phase == kusciaapisv1alpha1.KusciaJobCancelled || job.Status.Phase == kusciaapisv1alpha1.KusciaJobSucceeded {
		return false
	}
	return true
}

// readyTasksOf return subtasks that has been ready to create.
// NOTE: Regardless of the job mode, readyTasksOf always return subtasks that has been ready to create,
// even though the subtask shouldn't be scheduled.
func readyTasksOf(kusciaJob *kusciaapisv1alpha1.KusciaJob, currentTasks map[string]kusciaapisv1alpha1.KusciaTaskPhase) []kusciaapisv1alpha1.KusciaTaskTemplate {
	if currentTasks == nil {
		currentTasks = make(map[string]kusciaapisv1alpha1.KusciaTaskPhase, 0)
	}

	// we copy kusciaJob to prevent modification.
	copyKusciaJob := kusciaJob.DeepCopy()
	// we remove all finished dependencies from subtasks,
	// then all 0 dependencies subtasks are created or failed or ready to create.
	for i, t := range copyKusciaJob.Spec.Tasks {
		copyKusciaJob.Spec.Tasks[i].Dependencies = stringFilter(t.Dependencies,
			func(t string, i int) bool {
				return !(currentTasks[t] == kusciaapisv1alpha1.TaskSucceeded)
			})
	}
	noDependenciesTasks := kusciaTaskTemplateFilter(copyKusciaJob.Spec.Tasks,
		func(t kusciaapisv1alpha1.KusciaTaskTemplate, i int) bool {
			return len(t.Dependencies) == 0 && t.TaskID != ""
		})

	// ready task are sub-tasks that they are uncreated.
	readyTasks := kusciaTaskTemplateFilter(noDependenciesTasks,
		func(t kusciaapisv1alpha1.KusciaTaskTemplate, i int) bool {
			_, exist := currentTasks[t.Alias]
			return !exist
		})

	if len(readyTasks) == 0 {
		return nil
	}

	// task with higher priority should run early.
	sort.Slice(readyTasks, func(i, j int) bool {
		return readyTasks[i].Priority > readyTasks[j].Priority
	})

	// return origin tasks
	kusciaJobTaskMap := make(map[string]kusciaapisv1alpha1.KusciaTaskTemplate)
	for _, t := range kusciaJob.Spec.Tasks {
		kusciaJobTaskMap[t.Alias] = t
	}

	originReadyTasks := make([]kusciaapisv1alpha1.KusciaTaskTemplate, len(readyTasks))
	for i, t := range readyTasks {
		originReadyTasks[i] = kusciaJobTaskMap[t.Alias]
	}
	return originReadyTasks
}

// generateTaskID is used to generate task id.
func generateTaskID(jobName string) string {
	uid := strings.Split(string(uuid.NewUUID()), "-")
	return jobName + "-" + uid[len(uid)-1]
}

// willStartTasksOf returns will-start subtasks according to job schedule config.
// KusciaJob.Spec.MaxParallelism defines the maximum number of tasks in the Running state.
func willStartTasksOf(kusciaJob *kusciaapisv1alpha1.KusciaJob, readyTasks []kusciaapisv1alpha1.KusciaTaskTemplate, status map[string]kusciaapisv1alpha1.KusciaTaskPhase) []kusciaapisv1alpha1.KusciaTaskTemplate {
	count := 0
	for _, phase := range status {
		if phase == kusciaapisv1alpha1.TaskRunning || phase == kusciaapisv1alpha1.TaskPending || phase == "" {
			count++
		}
	}

	if *kusciaJob.Spec.MaxParallelism <= count {
		return nil
	}

	willStartTasks := readyTasks
	if len(readyTasks) > (*kusciaJob.Spec.MaxParallelism - count) {
		willStartTasks = readyTasks[:*kusciaJob.Spec.MaxParallelism-count]
	}

	return willStartTasks
}

// buildWillStartKusciaTask build KusciaTask CR from job's will-start sub-tasks.
func buildWillStartKusciaTask(nsLister corelisters.NamespaceLister, kusciaJob *kusciaapisv1alpha1.KusciaJob, willStartTask []kusciaapisv1alpha1.KusciaTaskTemplate) []*kusciaapisv1alpha1.KusciaTask {
	createdTasks := make([]*kusciaapisv1alpha1.KusciaTask, 0)

	isIcJob := isInterConnJob(kusciaJob)
	for _, t := range willStartTask {
		var taskObject = &kusciaapisv1alpha1.KusciaTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.TaskID,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(kusciaJob,
						kusciaapisv1alpha1.SchemeGroupVersion.WithKind(KusciaJobKind)),
				},
				Annotations: map[string]string{
					common.JobIDAnnotationKey:     kusciaJob.Name,
					common.TaskAliasAnnotationKey: t.Alias,
				},
				Labels: map[string]string{
					common.LabelController: LabelControllerValueKusciaJob,
					common.LabelJobUID:     string(kusciaJob.UID),
				},
			},
			Spec: createTaskSpec(kusciaJob.Spec.Initiator, t),
		}

		if isIcJob {
			// todo delete LabelInterConnProtocolType label
			taskObject.Annotations[common.InterConnBFIAPartyAnnotationKey] = kusciaJob.Annotations[common.InterConnBFIAPartyAnnotationKey]
			taskObject.Annotations[common.InterConnKusciaPartyAnnotationKey] = kusciaJob.Annotations[common.InterConnKusciaPartyAnnotationKey]
			taskObject.Annotations[common.InterConnSelfPartyAnnotationKey] = kusciaJob.Annotations[common.InterConnSelfPartyAnnotationKey]
			taskObject.Annotations[common.InitiatorAnnotationKey] = kusciaJob.Annotations[common.InitiatorAnnotationKey]
			if kusciaJob.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnBFIA) {
				// todo
				taskObject.Labels[common.LabelTaskUnschedulable] = common.True
			}
			if kusciaJob.Annotations[common.KusciaPartyMasterDomainAnnotationKey] != "" {
				taskObject.Annotations[common.KusciaPartyMasterDomainAnnotationKey] = kusciaJob.Annotations[common.KusciaPartyMasterDomainAnnotationKey]
			}
			if kusciaJob.Annotations[common.SelfClusterAsInitiatorAnnotationKey] != "" {
				taskObject.Annotations[common.SelfClusterAsInitiatorAnnotationKey] = kusciaJob.Annotations[common.SelfClusterAsInitiatorAnnotationKey]
			}
		}

		createdTasks = append(createdTasks, taskObject)
	}
	return createdTasks
}

// createTaskSpec will make kuscia task spec for kuscia job.
func createTaskSpec(initiator string, t kusciaapisv1alpha1.KusciaTaskTemplate) kusciaapisv1alpha1.KusciaTaskSpec {
	result := kusciaapisv1alpha1.KusciaTaskSpec{
		Initiator:       initiator,
		TaskInputConfig: t.TaskInputConfig,
		Parties:         buildPartiesFromTaskInputConfig(t),
	}
	if t.ScheduleConfig != nil {
		result.ScheduleConfig = *t.ScheduleConfig
	}
	return result
}

// buildPartiesFromTaskInputConfig will make kuscia task parties for kuscia job.
func buildPartiesFromTaskInputConfig(template kusciaapisv1alpha1.KusciaTaskTemplate) []kusciaapisv1alpha1.PartyInfo {
	taskPartyInfos := make([]kusciaapisv1alpha1.PartyInfo, len(template.Parties))
	for i, p := range template.Parties {
		taskPartyInfos[i] = kusciaapisv1alpha1.PartyInfo{
			DomainID:    p.DomainID,
			AppImageRef: template.AppImage,
			Role:        p.Role,
		}
	}
	return taskPartyInfos
}

// jobTaskSelector will make selector of kuscia task which kuscia job generate.
func jobTaskSelector(jobUID string) (labels.Selector, error) {
	controllerEquals, err :=
		labels.NewRequirement(common.LabelController, selection.Equals, []string{LabelControllerValueKusciaJob})
	if err != nil {
		return nil, err
	}
	ownerEquals, err :=
		labels.NewRequirement(common.LabelJobUID, selection.Equals, []string{jobUID})
	if err != nil {
		return nil, err
	}

	return labels.NewSelector().Add(*controllerEquals, *ownerEquals), nil
}

type currentTask struct {
	kusciaapisv1alpha1.KusciaTaskTemplate
	Phase *kusciaapisv1alpha1.KusciaTaskPhase
}

type currentTaskMap map[string]currentTask

// criticalTaskMap return critical task.
func (c currentTaskMap) criticalTaskMap() currentTaskMap {
	criticalMap := currentTaskMap{}
	for k, t := range c {
		if t.Tolerable == nil || !*t.Tolerable {
			criticalMap[k] = t
		}
	}
	return criticalMap
}

// readyTaskMap return ready task but not scheduled.
func (c currentTaskMap) readyTaskMap() currentTaskMap {
	readyTaskMap := currentTaskMap{}
	for k, t := range c {
		if taskNotExists(t) && (len(t.Dependencies) == 0 || stringAllMatch(t.Dependencies, func(taskId string) bool {
			return c[taskId].Phase != nil && *c[taskId].Phase == kusciaapisv1alpha1.TaskSucceeded
		})) {
			readyTaskMap[k] = t
		}
	}
	return readyTaskMap
}

func (c currentTaskMap) ToShortString() string {
	var taskString = make([]string, 0)
	for _, t := range c {
		var phase = "nil"
		if t.Phase != nil {
			phase = string(*t.Phase)
		}
		tolerable := false
		if t.Tolerable != nil {
			tolerable = *t.Tolerable
		}
		taskString = append(taskString, fmt.Sprintf(
			"{taskId=%s, dependencies=%+v, tolerable=%+v, phase=%s}",
			t.TaskID, t.Dependencies, tolerable, phase))
	}
	return "{" + strings.Join(taskString, ",") + "}"
}

// AllMatch will return true if every item match predicate.
func (c currentTaskMap) AllMatch(p func(v currentTask) bool) bool {
	for _, v := range c {
		if !p(v) {
			return false
		}
	}
	return true
}

// AnyMatch will return true if any item match predicate.
func (c currentTaskMap) AnyMatch(p func(v currentTask) bool) bool {
	for _, v := range c {
		if p(v) {
			return true
		}
	}
	return false
}

func taskFinished(v currentTask) bool {
	return v.Phase != nil && (*v.Phase == kusciaapisv1alpha1.TaskSucceeded || *v.Phase == kusciaapisv1alpha1.TaskFailed)
}

func taskNotExists(v currentTask) bool {
	return v.Phase == nil
}

func taskSucceeded(v currentTask) bool {
	return v.Phase != nil && *v.Phase == kusciaapisv1alpha1.TaskSucceeded
}

func taskFailed(v currentTask) bool {
	return v.Phase != nil && *v.Phase == kusciaapisv1alpha1.TaskFailed
}

func taskRunning(v currentTask) bool {
	return v.Phase != nil && (*v.Phase == kusciaapisv1alpha1.TaskRunning || *v.Phase == kusciaapisv1alpha1.TaskPending || *v.Phase == "")
}

// currentTaskMapFrom make currentTaskMap from the kuscia job and current task status.
func currentTaskMapFrom(kusciaJob *kusciaapisv1alpha1.KusciaJob, currentTaskStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase) currentTaskMap {
	currentTasks := make(map[string]currentTask, 0)
	for _, t := range kusciaJob.Spec.Tasks {
		c := currentTask{
			KusciaTaskTemplate: t,
			Phase:              nil,
		}

		if phase, exist := currentTaskStatus[t.Alias]; exist {
			c.Phase = &phase
		}

		currentTasks[t.TaskID] = c
	}
	return currentTasks
}

// setKusciaJobStatus sets the kuscia job status.
func setKusciaJobStatus(now metav1.Time, status *kusciaapisv1alpha1.KusciaJobStatus, phase kusciaapisv1alpha1.KusciaJobPhase, reason, message string) {
	status.Phase = phase
	status.Reason = reason
	status.Message = message
	status.LastReconcileTime = &now
	if status.StartTime == nil {
		status.StartTime = &now
	}
}

// setRunningTaskStatusToFailed
func setRunningTaskStatusToFailed(status *kusciaapisv1alpha1.KusciaJobStatus) {
	for k, v := range status.TaskStatus {
		if v == kusciaapisv1alpha1.TaskPending || v == kusciaapisv1alpha1.TaskRunning {
			status.TaskStatus[k] = kusciaapisv1alpha1.TaskFailed
		}
	}
}

func setKusciaTaskStatus(now metav1.Time, status *kusciaapisv1alpha1.KusciaTaskStatus, phase kusciaapisv1alpha1.KusciaTaskPhase, reason, message string) {
	status.Phase = phase
	status.LastReconcileTime = &now
	status.Reason = reason
	status.Message = message
	if status.StartTime == nil {
		status.StartTime = &now
	}
}
