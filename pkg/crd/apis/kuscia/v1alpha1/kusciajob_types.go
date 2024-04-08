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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="StartTime",type=date,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="CompletionTime",type=date,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="LastReconcileTime",type=date,JSONPath=`.status.lastReconcileTime`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kj

// KusciaJob is the Schema for the kuscia job API.
type KusciaJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KusciaJobSpec `json:"spec"`
	// +optional
	Status KusciaJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KusciaJobList contains a list of kuscia jobs.
type KusciaJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KusciaJob `json:"items"`
}

type JobStage string

const (
	JobCreateStage  JobStage = "Create"
	JobStartStage   JobStage = "Start"
	JobStopStage    JobStage = "Stop"
	JobCancelStage  JobStage = "Cancel"
	JobRestartStage JobStage = "Restart"
	JobSuspendStage JobStage = "Suspend"
)

// KusciaJobSpec defines the information of kuscia job spec.
type KusciaJobSpec struct {
	// FlowID defines the id of flow
	FlowID string `json:"flowID,omitempty"`
	// Initiator who schedule this KusciaJob.
	Initiator string `json:"initiator"`
	// ScheduleMode defines how this job will be scheduled.
	// In Strict, if any non-tolerable subtasks failed, Scheduling for this task stops immediately, and it immediately enters the final Failed state.
	// In BestEffort, if any non-tolerable subtasks failed, Scheduling for this job will continue.
	// But the successor subtask of the failed subtask stops scheduling, and the current state will be running.
	// When all subtasks succeed or fail, the job will enter the Failed state.
	// +optional
	// +kubebuilder:validation:Enum=Strict;BestEffort
	// +kubebuilder:default=Strict
	ScheduleMode KusciaJobScheduleMode `json:"scheduleMode,omitempty"`
	// MaxParallelism max parallelism of tasks, default 1.
	// At a certain moment, there may be multiple subtasks that can be scheduled.
	// this field defines the maximum number of tasks in the Running state.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=128
	MaxParallelism *int `json:"maxParallelism,omitempty"`
	// Tasks defines the subtasks participating in scheduling and their dependencies,
	// and the subtasks and dependencies should constitute a directed acyclic graph.
	// During runtime, each subtask will be created as a KusciaTask.
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:MinItems=1
	Tasks []KusciaTaskTemplate `json:"tasks"`
}

type KusciaTaskTemplate struct {
	// Alias represents KusciaTask alias.
	Alias string `json:"alias"`
	// TaskID represents KusciaTask id, it should match rfc1123 DNS_LABEL pattern.
	// It will be used in Dependencies.
	// +optional
	// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?
	TaskID string `json:"taskID,omitempty"`
	// Dependencies defines the dependencies of this subtask.
	// Only when the dependencies of this subtask are all in the Succeeded state, this subtask can be scheduled.
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:MinItems=1
	// +optional
	Dependencies []string `json:"dependencies,omitempty"`
	// Tolerable default false. If this sub-task failed, job will not be failed.
	// tolerable sub-task can not be other sub-tasks dependencies.
	// +kubebuilder:default=false
	// +optional
	Tolerable *bool `json:"tolerable,omitempty"`
	// AppImage defines image be used in KusciaTask
	AppImage string `json:"appImage"`
	// TaskInputConfig defines input config for KusciaTask.
	TaskInputConfig string `json:"taskInputConfig"`
	// ScheduleConfig defines the schedule config for KusciaTask.
	// +optional
	ScheduleConfig *ScheduleConfig `json:"scheduleConfig,omitempty"`
	// Priority defines priority of ready subtask.
	// When multiple subtasks are ready, which one is scheduled first.
	// The larger the value of this field, the higher the priority.
	// +optional
	Priority int `json:"priority,omitempty"`
	// Parties defines participants and role in this KusciaTask
	Parties []Party `json:"parties"`
}

type Party struct {
	DomainID string `json:"domainID"`
	// +optional
	Role string `json:"role,omitempty"`
}

// KusciaJobStatus defines the observed state of kuscia job.
type KusciaJobStatus struct {
	// The phase of a KusciaJob is a simple, high-level summary of
	// where the job is in its lifecycle.
	// +optional
	Phase KusciaJobPhase `json:"phase,omitempty"`

	// job approve status of each party, if job controller is configured with "AutoApproved",
	// the party's approved status will be initiated with "JobAccepted"
	// +optional
	ApproveStatus map[string]JobApprovePhase `json:"approveStatus,omitempty"`

	// job stage status of each party,
	// +optional
	StageStatus map[string]JobStagePhase `json:"stageStatus,omitempty"`

	// The latest available observations of an object's current state.
	// +optional
	Conditions []KusciaJobCondition `json:"conditions,omitempty"`

	// A brief CamelCase message indicating details about why the job is in this state.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human-readable message indicating details about why the job is in this condition.
	// +optional
	Message string `json:"message,omitempty"`

	// PartyTaskCreateStatus describes the created status of party task.
	// +optional
	PartyTaskCreateStatus map[string][]PartyTaskCreateStatus `json:"partyTaskCreateStatus,omitempty"`

	// TaskStatus describes subtasks state. The key is taskId.
	// Uncreated subtasks will not appear here.
	// +optional
	TaskStatus map[string]KusciaTaskPhase `json:"taskStatus,omitempty"`

	// Represents time when the job was acknowledged by the job controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Represents time when the job was completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Represents last time when the job was reconciled. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

// PartyTaskCreateStatus defines party task create status.
type PartyTaskCreateStatus struct {
	DomainID string `json:"domainID"`
	// +optional
	Role string `json:"role,omitempty"`
	// +optional
	Phase KusciaTaskCreatePhase `json:"phase,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

type KusciaTaskCreatePhase string

const (
	KusciaTaskCreateSucceeded KusciaTaskCreatePhase = "Succeeded"
	KusciaTaskCreateFailed    KusciaTaskCreatePhase = "Failed"
)

// KusciaJobScheduleMode defines how this job will be scheduled.
type KusciaJobScheduleMode string

const (
	// KusciaJobScheduleModeStrict means If any non-tolerable subtasks failed, Scheduling for this task stops immediately,
	// and it immediately enters the final Failed state.
	KusciaJobScheduleModeStrict KusciaJobScheduleMode = "Strict"
	// KusciaJobScheduleModeBestEffort means that If nay non-tolerable subtasks failed, Scheduling for this job will continue.
	// But the successor subtask of the failed subtask stops scheduling, and the current state will be running.
	// When all subtasks succeed or fail, the job will enter the Failed state.
	KusciaJobScheduleModeBestEffort KusciaJobScheduleMode = "BestEffort"
)

// KusciaJobConditionType is a valid value for a kuscia job condition type.
type KusciaJobConditionType string

// These are built-in conditions of kuscia job.
const (
	// JobValidated represents job is validated.
	JobValidated KusciaJobConditionType = "JobValidated"
	// JobCreateInitialized represents job-create stage is initialized.
	JobCreateInitialized KusciaJobConditionType = "JobCreateInitialized"
	// JobCreateSucceeded represents job-create stage is succeeded.
	JobCreateSucceeded KusciaJobConditionType = "JobCreateSucceeded"
	// JobStartInitialized represents job-start stage is initialized.
	JobStartInitialized KusciaJobConditionType = "JobStartInitialized"
	// JobStartSucceeded represents job-start stage is succeeded.
	JobStartSucceeded KusciaJobConditionType = "JobStartSucceeded"
	// JobStopInitialized represents job-stop stage is initialized.
	JobStopInitialized KusciaJobConditionType = "JobStopInitialized"
	// JobStopSucceeded represents job-stop stage is succeeded.
	JobStopSucceeded KusciaJobConditionType = "JobStopSucceeded"
	// TaskStopped represents condition of stopping task.
	TaskStopped KusciaJobConditionType = "TaskStopped"
	// JobStatusSynced represents condition of syncing job status.
	JobStatusSynced KusciaJobConditionType = "JobStatusSynced"
)

// KusciaJobCondition describes current state of a kuscia job.
type KusciaJobCondition struct {
	// Type of job condition.
	Type KusciaJobConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human-readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// KusciaJobPhase defines current status of this kuscia job.
type KusciaJobPhase string

// These are valid statuses of kuscia job.
const (

	// KusciaJobInitialized means the job is initialized, just be created soon.
	KusciaJobInitialized KusciaJobPhase = "initialized"

	// KusciaJobAwaitingApproval means the job is waiting for approval by some parties
	KusciaJobAwaitingApproval KusciaJobPhase = "AwaitingApproval"

	// KusciaJobPending means the job has been accepted by the controller,
	// but no kuscia task has not been created.
	KusciaJobPending KusciaJobPhase = "Pending"

	// KusciaJobRunning means least one tasks has created, and some kuscia task are running.
	KusciaJobRunning KusciaJobPhase = "Running"

	// KusciaJobSucceeded means all tasks are finished and all non-tolerable tasks are succeeded.
	KusciaJobSucceeded KusciaJobPhase = "Succeeded"

	// KusciaJobFailed means least one non-tolerable tasks are failed and kuscia job scheduling is stopped.
	KusciaJobFailed KusciaJobPhase = "Failed"

	// KusciaJobCancelled means the job is cancelled and cannot to be restarted.
	KusciaJobCancelled KusciaJobPhase = "Cancelled"

	// KusciaJobApprovalReject means the job is rejected by some parties.
	KusciaJobApprovalReject KusciaJobPhase = "ApprovalReject"

	// KusciaJobSuspended means the job has been suspended by some parties.
	KusciaJobSuspended KusciaJobPhase = "Suspended"
)

type JobStagePhase string

const (
	JobCreateStageSucceeded JobStagePhase = "JobCreateStageSucceeded"
	JobCreateStageFailed    JobStagePhase = "JobCreateStageFailed"

	JobStartStageSucceeded JobStagePhase = "JobStartStageSucceeded"
	JobStartStageFailed    JobStagePhase = "JobStartStageFailed"

	JobRestartStageSucceeded JobStagePhase = "JobRestartStageSucceeded"
	JobRestartStageFailed    JobStagePhase = "JobRestartStageFailed"

	JobStopStageSucceeded JobStagePhase = "JobStopStageSucceeded"
	JobStopStageFailed    JobStagePhase = "JobStopStageFailed"

	JobCancelStageSucceeded JobStagePhase = "JobCancelStageSucceeded"
	JobCancelStageFailed    JobStagePhase = "JobCancelStageFailed"

	JobSuspendStageSucceeded JobStagePhase = "JobSuspendStageSucceeded"
	JobSuspendStageFailed    JobStagePhase = "JobSuspendStageFailed"
)

type JobApprovePhase string

const (
	JobRejected JobApprovePhase = "JobRejected"
	JobAccepted JobApprovePhase = "JobAccepted"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="StartTime",type=date,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="CompletionTime",type=date,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="LastReconcileTime",type=date,JSONPath=`.status.lastReconcileTime`
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=kjs

// KusciaJobSummary is used to sync job status between clusters
type KusciaJobSummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec KusciaJobSummarySpec `json:"spec"`
	// +optional
	Status KusciaJobStatus `json:"status,omitempty"`
}

type KusciaJobSummarySpec struct {
	// Stage defines the current situation of a job.
	// +optional
	// +kubebuilder:default=Create
	Stage JobStage `json:"stage,omitempty"`

	// StageTrigger refers to the party who trigger current stage
	// +optional
	StageTrigger string `json:"stageTrigger,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KusciaJobSummaryList contains a list of kuscia tasks.
type KusciaJobSummaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KusciaJobSummary `json:"items"`
}
