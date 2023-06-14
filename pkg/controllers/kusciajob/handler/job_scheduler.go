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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// JobScheduler will compute current kuscia job and do scheduling until the kuscia job finished.
type JobScheduler struct {
	kusciaClient     versioned.Interface
	kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister
}

// NewJobScheduler return kuscia job scheduler.
func NewJobScheduler(kusciaClient versioned.Interface, kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister) *JobScheduler {
	return &JobScheduler{
		kusciaClient:     kusciaClient,
		kusciaTaskLister: kusciaTaskLister,
	}
}

// push will do scheduling for current kuscia job.
func (h *JobScheduler) push(kusciaJob *kusciaapisv1alpha1.KusciaJob) (bool, error) {
	needUpdate := false
	selector, err := jobTaskSelector(kusciaJob.Name)
	if err != nil {
		nlog.Errorf("Create job sub-tasks selector: %s", err)
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
	currentSubTasksStatus := buildJobSubtaskStatus(subTasks)
	currentJobStatus := jobStatusPhaseFrom(kusciaJob, currentSubTasksStatus)

	// compute ready task and push job when needed.
	readyTask := readyTasksOf(kusciaJob, currentSubTasksStatus)
	if currentJobStatus != kusciaapisv1alpha1.KusciaJobFailed && currentJobStatus != kusciaapisv1alpha1.KusciaJobSucceeded {
		willStartTask := willStartTasksOf(kusciaJob, readyTask, currentSubTasksStatus)
		willStartKusciaTasks := buildWillStartKusciaTask(kusciaJob, willStartTask)
		// then we will start KusciaTask
		for _, t := range willStartKusciaTasks {
			nlog.Infof("Create kuscia tasks: %s", t.ObjectMeta.Name)
			_, err := h.kusciaClient.KusciaV1alpha1().KusciaTasks().Create(context.Background(), t, metav1.CreateOptions{})
			if err != nil {
				// when create task failed, if existed, we failed fast. Otherwise, we will retry until maxBackoffLimit.
				if errors.IsAlreadyExists(err) {
					nlog.Errorf("Create kuscia task: %s, set kuscia job failed", err)
					kusciaJob.Status = buildJobStatus(kusciaJob, kusciaapisv1alpha1.KusciaJobFailed, currentSubTasksStatus)
					kusciaJob.Status.Reason = "KusciaJobCreateTaskFailed"
					kusciaJob.Status.Message = fmt.Sprintf("create kuscia task failed: %s", err.Error())
					return true, nil
				}
				return false, err
			}
		}
		// we need update lastReconcileTime
		if len(willStartKusciaTasks) != 0 {
			needUpdate = true
		}
	}

	// update current status always, because Pending status
	previousStatus := kusciaJob.Status
	currentStatus := buildJobStatus(kusciaJob, currentJobStatus, currentSubTasksStatus)
	if statusNeedUpdate(currentStatus, previousStatus) {
		kusciaJob.Status = currentStatus
		needUpdate = true
		nlog.Infof("Status Update: %+v -> %+v", previousStatus, currentStatus)
	}

	return needUpdate, nil
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
					removeSubtasks[t.TaskID] = true
				}
				return !noDependencies
			})

		// 2. then we remove all removed-task dependencies from other subtasks.
		for i, t := range copyKusciaJob.Spec.Tasks {
			copyKusciaJob.Spec.Tasks[i].Dependencies = stringFilter(t.Dependencies,
				func(taskId string, i int) bool {
					_, exist := removeSubtasks[taskId]
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
		taskIDSet[t.TaskID] = true
	}

	for _, t := range copyKusciaJob.Spec.Tasks {
		for _, d := range t.Dependencies {
			if _, exits := taskIDSet[d]; !exits {
				return fmt.Errorf("validate failed: task %s has not exist dependency task %s", t.TaskID, d)
			}
		}
	}

	return nil
}

// kusciaJobValidate check whether kusciaJob is valid.
func kusciaJobValidate(kusciaJob *kusciaapisv1alpha1.KusciaJob) error {
	if err := kusciaJobDependenciesExits(kusciaJob); err != nil {
		return err
	}

	return kusciaJobHasTaskCycle(kusciaJob)
}

// buildJobSubtaskStatus returns current subtask status.
func buildJobSubtaskStatus(currentSubTasks []*kusciaapisv1alpha1.KusciaTask) map[string]kusciaapisv1alpha1.KusciaTaskPhase {
	subTaskStatus := make(map[string]kusciaapisv1alpha1.KusciaTaskPhase, 0)
	for _, t := range currentSubTasks {
		subTaskStatus[t.Name] = t.Status.Phase
	}
	return subTaskStatus
}

// buildJobStatus make KusciaJobStatus from currentJobStatusPhase.
func buildJobStatus(job *kusciaapisv1alpha1.KusciaJob, currentJobStatusPhase kusciaapisv1alpha1.KusciaJobPhase,
	currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase) kusciaapisv1alpha1.KusciaJobStatus {
	now := metav1.Now()
	startTime := job.Status.StartTime
	if startTime == nil {
		startTime = &now
	}
	status := kusciaapisv1alpha1.KusciaJobStatus{
		Phase:             currentJobStatusPhase,
		TaskStatus:        currentSubTasksStatus,
		StartTime:         startTime,
		LastReconcileTime: &now,
	}

	return status
}

// statusNeedUpdate compute previous status and now status
func statusNeedUpdate(current kusciaapisv1alpha1.KusciaJobStatus, previous kusciaapisv1alpha1.KusciaJobStatus) bool {
	return DiffStatus(current, previous)
}

func DiffStatus(current kusciaapisv1alpha1.KusciaJobStatus, previous kusciaapisv1alpha1.KusciaJobStatus) bool {
	return !(rfc3339TimeEqual(current.StartTime, previous.StartTime) &&
		rfc3339TimeEqual(current.CompletionTime, previous.CompletionTime) &&
		current.Reason == previous.Reason && current.Message == previous.Message &&
		current.Phase == previous.Phase && reflect.DeepEqual(previous.TaskStatus, current.TaskStatus))
}

// jobStatusPhaseFrom will computer this job status from currentSubTasksStatus.
// Finished task means the task is succeeded or failed.
// Schedulable task means it's dependencies tasks has no failed one.
// Pending: this job has been submitted, but has no subtasks.
// Running : least one subtask is running.
// Succeeded : all subtasks are finished and all critical subtasks are succeeded.
// Failed:
//   - BestEffort: least one critical subtasks is failed. But all schedulable subtasks are scheduled and finished.
//   - Strict: least one critical subtasks is failed. But some schedulable subtasks may be not scheduled.
func jobStatusPhaseFrom(job *kusciaapisv1alpha1.KusciaJob, currentSubTasksStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase) (phase kusciaapisv1alpha1.KusciaJobPhase) {
	tasks := currentTaskMapFrom(job, currentSubTasksStatus)

	// no subtasks mean the job is pending.
	if tasks.AllMatch(taskUncreated) {
		return kusciaapisv1alpha1.KusciaJobPending
	}

	// Critical task means the task is not tolerable.
	// Schedulable task means the task's dependencies tasks has no failed one.
	criticalTasks := tasks.criticalTaskMap()
	schedulableTasks := tasks.schedulableTaskMap()

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
		// in BestEffort mode, all schedulable subtasks finished and any critical subtasks failed means the job is failed.
		if schedulableTasks.AllMatch(taskFinished) &&
			criticalTasks.AnyMatch(taskFailed) {
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
			return len(t.Dependencies) == 0
		})

	// ready task are sub-tasks that they are uncreated.
	readyTasks := kusciaTaskTemplateFilter(noDependenciesTasks,
		func(t kusciaapisv1alpha1.KusciaTaskTemplate, i int) bool {
			_, exist := currentTasks[t.TaskID]
			return !exist
		})

	// task with higher priority should run early.
	sort.Slice(readyTasks, func(i, j int) bool {
		return readyTasks[i].Priority > readyTasks[i].Priority
	})

	// return origin tasks
	kusciaJobTaskMap := make(map[string]kusciaapisv1alpha1.KusciaTaskTemplate)
	for _, t := range kusciaJob.Spec.Tasks {
		kusciaJobTaskMap[t.TaskID] = t
	}
	if len(readyTasks) == 0 {
		return nil
	}
	originReadyTasks := make([]kusciaapisv1alpha1.KusciaTaskTemplate, len(readyTasks))
	for i, t := range readyTasks {
		originReadyTasks[i] = kusciaJobTaskMap[t.TaskID]
	}
	return originReadyTasks
}

// willStartTasksOf returns will-start subtasks according to job schedule config.
// KusciaJob.Spec.MaxParallelism defines the maximum number of tasks in the Running state.
func willStartTasksOf(kusciaJob *kusciaapisv1alpha1.KusciaJob, readyTasks []kusciaapisv1alpha1.KusciaTaskTemplate, status map[string]kusciaapisv1alpha1.KusciaTaskPhase) []kusciaapisv1alpha1.KusciaTaskTemplate {
	count := 0
	for _, phase := range status {
		if phase == kusciaapisv1alpha1.TaskRunning || phase == kusciaapisv1alpha1.TaskCreating || phase == kusciaapisv1alpha1.TaskPending {
			count++
		}
	}
	if *kusciaJob.Spec.MaxParallelism <= count {
		return nil
	}
	willStartTasks := readyTasks
	if len(readyTasks) > (*kusciaJob.Spec.MaxParallelism - count) {
		willStartTasks = readyTasks[:*kusciaJob.Spec.MaxParallelism]
	}
	return willStartTasks
}

// buildWillStartKusciaTask build KusciaTask CR from job's will-start sub-tasks.
func buildWillStartKusciaTask(kusciaJob *kusciaapisv1alpha1.KusciaJob, willStartTask []kusciaapisv1alpha1.KusciaTaskTemplate) []*kusciaapisv1alpha1.KusciaTask {
	createdTasks := make([]*kusciaapisv1alpha1.KusciaTask, 0)
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
					LabelKusciaJobOwner:    kusciaJob.Name,
				},
			},
			Spec: createTaskSpec(kusciaJob.Spec.Initiator, t),
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
		labels.NewRequirement(LabelKusciaJobOwner, selection.Equals, []string{jobName})
	if err != nil {
		return nil, err
	}

	return labels.NewSelector().Add(*controllerEquals, *ownerEquals), nil
}

type currentTask struct {
	kusciaapisv1alpha1.KusciaTaskTemplate
	Phase kusciaapisv1alpha1.KusciaTaskPhase
}

type currentTaskMap map[string]currentTask

// schedulableTaskMap return tolerable task.
func (c currentTaskMap) tolerableTaskMap() currentTaskMap {
	tolerableMap := currentTaskMap{}
	for k, t := range c {
		if t.Tolerable != nil || *t.Tolerable == true {
			tolerableMap[k] = t
		}
	}
	return tolerableMap
}

// schedulableTaskMap return critical task.
func (c currentTaskMap) criticalTaskMap() currentTaskMap {
	criticalMap := currentTaskMap{}
	for k, t := range c {
		if t.Tolerable == nil || *t.Tolerable == false {
			criticalMap[k] = t
		}
	}
	return criticalMap
}

// schedulableTaskMap return schedulable task.
func (c currentTaskMap) schedulableTaskMap() currentTaskMap {
	schedulableMap := currentTaskMap{}
	for k, t := range c {
		// for subtask, it is schedulable if it has no-failed dependencies.
		if stringAllMatch(t.Dependencies, func(taskId string) bool {
			return c[taskId].Phase != kusciaapisv1alpha1.TaskFailed
		}) {
			schedulableMap[k] = t
		}
	}
	return schedulableMap
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
	return v.Phase == kusciaapisv1alpha1.TaskSucceeded || v.Phase == kusciaapisv1alpha1.TaskFailed
}

func taskUncreated(v currentTask) bool {
	return v.Phase == ""
}

func taskSucceeded(v currentTask) bool {
	return v.Phase == kusciaapisv1alpha1.TaskSucceeded
}

func taskFailed(v currentTask) bool {
	return v.Phase == kusciaapisv1alpha1.TaskFailed
}

// currentTaskMapFrom make currentTaskMap from the kuscia job and current task status.
func currentTaskMapFrom(kusciaJob *kusciaapisv1alpha1.KusciaJob, currentTaskStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase) currentTaskMap {
	currentTasks := make(map[string]currentTask, 0)
	for _, t := range kusciaJob.Spec.Tasks {
		currentTasks[t.TaskID] = currentTask{
			KusciaTaskTemplate: t,
			Phase:              currentTaskStatus[t.TaskID], // not exist will be ""
		}
	}
	return currentTasks
}
