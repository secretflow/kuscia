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
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	updateRetries = 3
)

// JobScheduler will compute current kuscia job and do scheduling until the kuscia job finished.
type JobScheduler struct {
	kusciaClient     versioned.Interface
	kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister
	namespaceLister  corelisters.NamespaceLister
}

// NewJobScheduler return kuscia job scheduler.
func NewJobScheduler(kusciaClient versioned.Interface, namespaceLister corelisters.NamespaceLister, kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister) *JobScheduler {
	return &JobScheduler{
		kusciaClient:     kusciaClient,
		kusciaTaskLister: kusciaTaskLister,
		namespaceLister:  namespaceLister,
	}
}

// push will do scheduling for current kuscia job.
func (h *JobScheduler) push(kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	defer func() {
		if needUpdate {
			if kusciaJob.Status.StartTime == nil {
				kusciaJob.Status.StartTime = &now
			}

			if !kusciaJob.Status.LastReconcileTime.Equal(&now) {
				kusciaJob.Status.LastReconcileTime = &now
			}
		}
	}()

	jobUpdated, err := h.preprocessKusciaJob(now, kusciaJob)
	if err != nil {
		return true, nil
	}

	if jobUpdated {
		return false, nil
	}

	switch kusciaJob.Spec.Stage {
	case "", kusciaapisv1alpha1.JobCreateStage:
		needUpdate, err = h.handleJobCreateStage(now, kusciaJob)
	case kusciaapisv1alpha1.JobStartStage:
		needUpdate, err = h.handleJobStartStage(now, kusciaJob)
	case kusciaapisv1alpha1.JobStopStage:
		needUpdate, err = h.handleJobStopStage(now, kusciaJob)
	default:
		nlog.Errorf("KusciaJob %v stage type %v is unknown", kusciaJob.Name, kusciaJob.Spec.Stage)
		setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "KusciaJobStageUnknown", fmt.Sprintf("Invalid Job Stage %v", kusciaJob.Spec.Stage))
		needUpdate = true
	}

	return needUpdate, err
}

// handleJobCreateStage handles job-create stage.
func (h *JobScheduler) handleJobCreateStage(now metav1.Time, kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	// validate job
	jobValidatedCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobValidated, true)
	if jobValidatedCond.Status != corev1.ConditionTrue {
		if err = h.kusciaJobValidate(kusciaJob); err != nil {
			utilsres.SetKusciaJobCondition(now, jobValidatedCond, corev1.ConditionFalse, "ValidateFailed", fmt.Sprintf("Validate job failed, %v", err.Error()))
			setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "KusciaJobValidateFailed", "")
			return true, nil
		}
		needUpdate = true
		utilsres.SetKusciaJobCondition(now, jobValidatedCond, corev1.ConditionTrue, "", "")
	}
	if !utilsres.SelfClusterAsInitiator(h.namespaceLister, kusciaJob.Spec.Initiator, kusciaJob.Labels) {
		return false, nil
	}

	// process job as initiator
	jobCreateSucceededCond, exist := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobCreateSucceeded, false)
	if exist && jobCreateSucceededCond.Status == corev1.ConditionTrue {
		return false, utilsres.UpdateKusciaJobStage(h.kusciaClient, kusciaJob, kusciaapisv1alpha1.JobStartStage, updateRetries)
	}

	jobCreateInitializedCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobCreateInitialized, true)
	if jobCreateInitializedCond.Status != corev1.ConditionTrue {
		needUpdate = true
		utilsres.SetKusciaJobCondition(now, jobCreateInitializedCond, corev1.ConditionTrue, "", "")
	}

	isICJob, err := isBFIAInterConnJob(h.namespaceLister, kusciaJob)
	if err != nil {
		setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "CheckJobTypeFailed", err.Error())
		return true, nil
	}
	switch isICJob {
	case true:
		if exist && jobCreateSucceededCond.Status == corev1.ConditionFalse {
			setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "KusciaJobCreateFailed", "")
			return true, nil
		}
	default:
		jobCreateSucceededCond, _ = utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobCreateSucceeded, true)
		if jobCreateSucceededCond.Status != corev1.ConditionTrue {
			needUpdate = true
			utilsres.SetKusciaJobCondition(now, jobCreateSucceededCond, corev1.ConditionTrue, "", "")
		}
	}
	return needUpdate, nil
}

// handleJobStopStage handles job-stop stage.
func (h *JobScheduler) handleJobStopStage(now metav1.Time, kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	needStopTask := false
	if utilsres.SelfClusterAsInitiator(h.namespaceLister, kusciaJob.Spec.Initiator, kusciaJob.Labels) {
		jobStopInitializedCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStopInitialized, true)
		if jobStopInitializedCond.Status != corev1.ConditionTrue {
			needUpdate = true
			utilsres.SetKusciaJobCondition(now, jobStopInitializedCond, corev1.ConditionTrue, "", "")
		}

		isICJob, err := isBFIAInterConnJob(h.namespaceLister, kusciaJob)
		if err != nil {
			setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "CheckJobTypeFailed", err.Error())
			return true, nil
		}
		switch isICJob {
		case true:
			_, exist := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStopSucceeded, false)
			if exist {
				needUpdate = true
				needStopTask = true
				setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "", "")
			}
		default:
			jobStopSucceededCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStopSucceeded, true)
			if jobStopSucceededCond.Status != corev1.ConditionTrue {
				utilsres.SetKusciaJobCondition(now, jobStopSucceededCond, corev1.ConditionTrue, "", "")
				needUpdate = true
				needStopTask = true
				setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "", "")
			}
		}
	} else {
		jobStopSucceededCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStopSucceeded, true)
		if jobStopSucceededCond.Status != corev1.ConditionTrue {
			utilsres.SetKusciaJobCondition(now, jobStopSucceededCond, corev1.ConditionTrue, "", "")
			needUpdate = true
			needStopTask = true
			setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "", "")
		}
	}

	if needStopTask {
		taskStoppedCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.TaskStopped, true)
		if taskStoppedCond.Status != corev1.ConditionTrue {
			message := ""
			condStatus := corev1.ConditionTrue
			if err = h.stopTasks(now, kusciaJob); err != nil {
				message = err.Error()
				condStatus = corev1.ConditionFalse
			}
			utilsres.SetKusciaJobCondition(now, taskStoppedCond, condStatus, "", message)
		}
	}

	return needUpdate, nil
}

func (h *JobScheduler) stopTasks(now metav1.Time, kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	for taskID, phase := range kusciaJob.Status.TaskStatus {
		if phase == kusciaapisv1alpha1.TaskFailed || phase == kusciaapisv1alpha1.TaskSucceeded {
			continue
		}

		kt, err := h.kusciaTaskLister.Get(taskID)
		if err != nil && !k8serrors.IsNotFound(err) {
			nlog.Errorf("Get kuscia task %v failed, so skip stopping this task", taskID)
			return err
		}

		copyKt := kt.DeepCopy()
		setKusciaTaskStatus(now, &copyKt.Status, kusciaapisv1alpha1.TaskFailed, "KusciaJobStopped", "")
		for _, party := range copyKt.Spec.Parties {
			if utilsres.IsOuterBFIAInterConnDomain(h.namespaceLister, party.DomainID) {
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

		if err = utilsres.UpdateKusciaTaskStatus(h.kusciaClient, kt, copyKt, updateRetries); err != nil {
			return err
		}
	}
	return nil
}

// handleJobStartStage handles job-start stage.
func (h *JobScheduler) handleJobStartStage(now metav1.Time, kusciaJob *kusciaapisv1alpha1.KusciaJob) (needUpdate bool, err error) {
	asInitiator := false
	if utilsres.SelfClusterAsInitiator(h.namespaceLister, kusciaJob.Spec.Initiator, kusciaJob.Labels) {
		asInitiator = true
		jobStartInitializedCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStartInitialized, true)
		if jobStartInitializedCond.Status != corev1.ConditionTrue {
			needUpdate = true
			utilsres.SetKusciaJobCondition(now, jobStartInitializedCond, corev1.ConditionTrue, "", "")
		}

		isICJob, err := isBFIAInterConnJob(h.namespaceLister, kusciaJob)
		if err != nil {
			setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "CheckJobTypeFailed", err.Error())
			return true, nil
		}
		switch isICJob {
		case true:
			jobStartSucceededCond, exist := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStartSucceeded, false)
			if exist && jobStartSucceededCond.Status != corev1.ConditionTrue {
				setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "KusciaJobStartFailed", "")
				return true, nil
			}
		default:
			jobStartSucceededCond, _ := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStartSucceeded, true)
			if jobStartSucceededCond.Status != corev1.ConditionTrue {
				needUpdate = true
				utilsres.SetKusciaJobCondition(now, jobStartSucceededCond, corev1.ConditionTrue, "", "")
			}
		}

		if needUpdate {
			return needUpdate, nil
		}

		if hasSet, setErr := h.setJobTaskID(kusciaJob); hasSet {
			return false, setErr
		}
	}

	if asInitiator {
		jobStartSucceededCond, exist := utilsres.GetKusciaJobCondition(&kusciaJob.Status, kusciaapisv1alpha1.JobStartSucceeded, false)
		if !exist || jobStartSucceededCond.Status != corev1.ConditionTrue {
			nlog.Infof("Waiting job start-stage condition %q status changed to %q", kusciaapisv1alpha1.JobStartSucceeded, corev1.ConditionTrue)
			return false, nil
		}
	}

	selector, err := jobTaskSelector(kusciaJob.Name)
	if err != nil {
		nlog.Errorf("Create job sub-tasks selector failed: %s", err)
		return false, err
	}

	// get all sub-tasks
	subTasks, err := h.kusciaTaskLister.List(selector)
	if err != nil {
		nlog.Errorf("List job sub-tasks selector: %s", err)
		return false, err
	}

	// compute current status.
	// NOTE: We don't believe kusciaJob.TaskStatus, we rebuild it from current sub-task status.
	// MayBe some tasks have been created, but updateStatus failed Or first task creation has been happened,
	// but updateStatus is delayed.
	currentSubTasksStatusWithAlias, currentSubTasksStatusWithID := buildJobSubTaskStatus(asInitiator, subTasks, kusciaJob)
	currentJobPhase := jobStatusPhaseFrom(kusciaJob, currentSubTasksStatusWithAlias)

	// compute ready task and push job when needed.
	readyTask := readyTasksOf(kusciaJob, currentSubTasksStatusWithAlias)
	if currentJobPhase != kusciaapisv1alpha1.KusciaJobFailed && currentJobPhase != kusciaapisv1alpha1.KusciaJobSucceeded {
		willStartTask := willStartTasksOf(kusciaJob, readyTask, currentSubTasksStatusWithAlias)
		willStartKusciaTasks := buildWillStartKusciaTask(h.namespaceLister, kusciaJob, willStartTask)
		// then we will start KusciaTask
		for _, t := range willStartKusciaTasks {
			nlog.Infof("Create kuscia tasks: %s", t.ObjectMeta.Name)
			_, err = h.kusciaClient.KusciaV1alpha1().KusciaTasks().Create(context.Background(), t, metav1.CreateOptions{})
			if err != nil {
				if k8serrors.IsAlreadyExists(err) {
					existTask, err := h.kusciaTaskLister.Get(t.Name)
					if err != nil {
						if k8serrors.IsNotFound(err) {
							existTask, err = h.kusciaClient.KusciaV1alpha1().KusciaTasks().Get(context.Background(), t.Name, metav1.GetOptions{})
						}

						if err != nil {
							nlog.Errorf("Get exist task %v failed: %v", t.Name, err)
							setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "CreateTaskFailed", err.Error())
							return true, nil
						}
					}

					if existTask.Labels == nil || existTask.Labels[common.LabelJobID] != kusciaJob.Name {
						message := fmt.Sprintf("Failed to create task %v because a task with the same name already exists", t.Name)
						nlog.Error(message)
						setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "CreateTaskFailed", message)
						return true, nil
					}
				} else {
					nlog.Errorf("Create kuscia task %s failed, %v", t.Name, err)
					return true, err
				}
			}
		}
	}

	needUpdate = buildJobStatus(now, &kusciaJob.Status, currentJobPhase, currentSubTasksStatusWithID)
	return needUpdate, nil
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

// preprocessKusciaJob preprocess the kuscia job.
func (h *JobScheduler) preprocessKusciaJob(now metav1.Time, kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	isIcJob, err := isInterConnJob(h.namespaceLister, kusciaJob)
	if err != nil {
		setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "CheckJobTypeFailed", err.Error())
		return false, err
	}

	if isIcJob {
		isBFIAIcJob, err := isBFIAInterConnJob(h.namespaceLister, kusciaJob)
		if err != nil {
			setKusciaJobStatus(now, &kusciaJob.Status, kusciaapisv1alpha1.KusciaJobFailed, "CheckInterConnJobTypeFailed", err.Error())
			return false, err
		}

		protocolType := string(kusciaapisv1alpha1.InterConnKuscia)
		if isBFIAIcJob {
			protocolType = string(kusciaapisv1alpha1.InterConnBFIA)
		}

		if utilsres.SelfClusterAsInitiator(h.namespaceLister, kusciaJob.Spec.Initiator, kusciaJob.Labels) {
			hasUpdated := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) bool {
				if kusciaJob.Labels != nil &&
					kusciaJob.Labels[common.LabelInterConnProtocolType] == protocolType &&
					kusciaJob.Labels[common.LabelSelfClusterAsInitiator] == common.True {
					return true
				}
				return false
			}

			if hasUpdated(kusciaJob) {
				return false, nil
			}

			update := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) {
				if kusciaJob.Labels == nil {
					kusciaJob.Labels = map[string]string{}
				}
				kusciaJob.Labels[common.LabelInterConnProtocolType] = protocolType
				kusciaJob.Labels[common.LabelSelfClusterAsInitiator] = common.True
			}

			update(kusciaJob)

			return true, utilsres.UpdateKusciaJob(h.kusciaClient, kusciaJob, hasUpdated, update, updateRetries)
		}
	}

	return false, nil
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

// changeJobStageToStart changes kuscia job stage to start.
func (h *JobScheduler) changeJobStageToStart(kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	update := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) {
		kusciaJob.Spec.Stage = kusciaapisv1alpha1.JobStartStage
	}

	hasUpdated := func(kusciaJob *kusciaapisv1alpha1.KusciaJob) bool {
		if kusciaJob.Spec.Stage == kusciaapisv1alpha1.JobStartStage {
			return true
		}
		return false
	}
	update(kusciaJob)

	return utilsres.UpdateKusciaJob(h.kusciaClient, kusciaJob, hasUpdated, update, updateRetries)
}

// isBFIAInterConnJob checks if the job is interconn with BFIA protocol.
func isBFIAInterConnJob(nsLister corelisters.NamespaceLister, kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	if kusciaJob.Labels != nil {
		if kusciaJob.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnBFIA) {
			return true, nil
		}

		if kusciaJob.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnKuscia) {
			return false, nil
		}
	}

	for _, party := range kusciaJob.Spec.Tasks[0].Parties {
		ns, err := nsLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("failed to get domain %v namespace, %v", party.DomainID, err)
			return false, err
		}

		if ns.Labels != nil &&
			ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) &&
			ns.Labels[common.LabelInterConnProtocols] == string(kusciaapisv1alpha1.InterConnBFIA) {
			return true, nil
		}
	}
	return false, nil
}

// isInterConnJob checks if the job is interconn job.
func isInterConnJob(nsLister corelisters.NamespaceLister, kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	if kusciaJob.Labels != nil &&
		(kusciaJob.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnBFIA) ||
			kusciaJob.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnKuscia)) {
		return true, nil
	}

	for _, party := range kusciaJob.Spec.Tasks[0].Parties {
		ns, err := nsLister.Get(party.DomainID)
		if err != nil {
			nlog.Errorf("failed to get domain %v namespace, %v", party.DomainID, err)
			return false, err
		}

		if ns.Labels != nil && ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) {
			return true, nil
		}
	}
	return false, nil
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
func buildJobSubTaskStatus(asInitiator bool, currentSubTasks []*kusciaapisv1alpha1.KusciaTask, job *kusciaapisv1alpha1.KusciaJob) (map[string]kusciaapisv1alpha1.KusciaTaskPhase, map[string]kusciaapisv1alpha1.KusciaTaskPhase) {
	subTaskStatusWithAlias := make(map[string]kusciaapisv1alpha1.KusciaTaskPhase, 0)
	subTaskStatusWithID := make(map[string]kusciaapisv1alpha1.KusciaTaskPhase, 0)
	for idx := range currentSubTasks {
		for _, task := range job.Spec.Tasks {
			if task.TaskID == currentSubTasks[idx].Name {
				if asInitiator {
					subTaskStatusWithAlias[task.Alias] = currentSubTasks[idx].Status.Phase
					subTaskStatusWithID[task.TaskID] = currentSubTasks[idx].Status.Phase
				} else {
					subTaskStatusWithAlias[task.Alias] = job.Status.TaskStatus[task.TaskID]
					subTaskStatusWithID[task.TaskID] = job.Status.TaskStatus[task.TaskID]
				}
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
		return kusciaapisv1alpha1.KusciaJobPending
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
	if job.DeletionTimestamp != nil {
		nlog.Infof("KusciaJob %s was deleted, skipping", job.Name)
		return false
	}

	if job.Status.CompletionTime != nil {
		nlog.Infof("KusciaJob %s was finished, skipping", job.Name)
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

	isIcJob, _ := isInterConnJob(nsLister, kusciaJob)
	selfClusterAsInitiator := false
	if utilsres.SelfClusterAsInitiator(nsLister, kusciaJob.Spec.Initiator, kusciaJob.Labels) {
		selfClusterAsInitiator = true
	}

	for _, t := range willStartTask {
		var taskObject = &kusciaapisv1alpha1.KusciaTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: t.TaskID,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(kusciaJob,
						kusciaapisv1alpha1.SchemeGroupVersion.WithKind(KusciaJobKind)),
				},
				Labels: map[string]string{
					common.LabelController: LabelControllerValueKusciaJob,
					common.LabelJobID:      kusciaJob.Name,
					common.LabelTaskAlias:  t.Alias,
				},
			},
			Spec: createTaskSpec(kusciaJob.Spec.Initiator, t),
		}

		if isIcJob {
			taskObject.Labels[common.LabelInterConnProtocolType] = kusciaJob.Labels[common.LabelInterConnProtocolType]
			if kusciaJob.Labels[common.LabelInterConnProtocolType] == string(kusciaapisv1alpha1.InterConnBFIA) {
				taskObject.Labels[common.LabelTaskUnschedulable] = common.True
			}
		}

		if selfClusterAsInitiator {
			taskObject.Labels[common.LabelSelfClusterAsInitiator] = common.True
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
func jobTaskSelector(jobName string) (labels.Selector, error) {
	controllerEquals, err :=
		labels.NewRequirement(common.LabelController, selection.Equals, []string{LabelControllerValueKusciaJob})
	if err != nil {
		return nil, err
	}
	ownerEquals, err :=
		labels.NewRequirement(common.LabelJobID, selection.Equals, []string{jobName})
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

func setKusciaTaskStatus(now metav1.Time, status *kusciaapisv1alpha1.KusciaTaskStatus, phase kusciaapisv1alpha1.KusciaTaskPhase, reason, message string) {
	status.Phase = phase
	status.LastReconcileTime = &now
	status.Reason = reason
	status.Message = message
	if status.StartTime == nil {
		status.StartTime = &now
	}
}
