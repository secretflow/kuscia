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
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="StartTime",type=date,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="CompletionTime",type=date,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="LastReconcileTime",type=date,JSONPath=`.status.lastReconcileTime`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kt

// KusciaTask is the Schema for the kuscia task API.
type KusciaTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KusciaTaskSpec `json:"spec"`
	// +optional
	Status KusciaTaskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KusciaTaskList contains a list of kuscia tasks.
type KusciaTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KusciaTask `json:"items"`
}

// KusciaTaskSpec defines the information of kuscia task spec.
type KusciaTaskSpec struct {
	Initiator       string `json:"initiator"`
	TaskInputConfig string `json:"taskInputConfig"`
	// +optional
	ScheduleConfig ScheduleConfig `json:"scheduleConfig,omitempty"`
	Parties        []PartyInfo    `json:"parties"`
}

// ScheduleConfig defines the config for scheduling.
type ScheduleConfig struct {
	// +kubebuilder:validation:Minimum:=1
	MinReservedMembers int `json:"minReservedMembers,omitempty"`
	// +optional
	ResourceReservedSeconds int `json:"resourceReservedSeconds,omitempty"`
	// +optional
	LifecycleSeconds int `json:"lifecycleSeconds,omitempty"`
	// +optional
	RetryIntervalSeconds int `json:"retryIntervalSeconds,omitempty"`
}

// PartyInfo defines the basic party info.
type PartyInfo struct {
	DomainID    string `json:"domainID"`
	AppImageRef string `json:"appImageRef"`
	// +optional
	Role string `json:"role,omitempty"`
	// +optional
	MinReservedPods int           `json:"minReservedPods,omitempty"`
	Template        PartyTemplate `json:"template"`
}

// PartyTemplate defines the specific info for party.
type PartyTemplate struct {
	// +optional
	Replicas *int32  `json:"replicas,omitempty"`
	Spec     PodSpec `json:"spec"`
}

// PartyTaskStatus defines party task status.
type PartyTaskStatus struct {
	DomainID string `json:"domainID"`
	// +optional
	Role string `json:"role,omitempty"`
	// +optional
	Phase KusciaTaskPhase `json:"phase,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

// KusciaTaskStatus defines the observed state of kuscia task.
type KusciaTaskStatus struct {
	// The phase of a KusciaTask is a simple, high-level summary of
	// where the task is in its lifecycle.
	// +optional
	Phase KusciaTaskPhase `json:"phase,omitempty"`

	// PartyTaskStatus defines task status for all party.
	// +optional
	PartyTaskStatus []PartyTaskStatus `json:"partyTaskStatus,omitempty"`

	// A brief CamelCase message indicating details about why the task is in this state.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human-readable message indicating details about why the task is in this condition.
	// +optional
	Message string `json:"message,omitempty"`

	// The latest available observations of an object's current state.
	// +optional
	Conditions []KusciaTaskCondition `json:"conditions,omitempty"`

	// PodStatuses is map of ns/name and PodStatus,
	// specifies the status of each pod.
	// +optional
	PodStatuses map[string]*PodStatus `json:"podStatuses,omitempty"`

	// Represents time when the task was acknowledged by the task controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Represents time when the task was completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Represents last time when the task was reconciled. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

// KusciaTaskPhase is a label for the condition of a kuscia task at the current time.
type KusciaTaskPhase string

// These are valid statuses of kuscia task.
const (
	// TaskPending means the task has been accepted by the controller,
	// but some initialization work has not yet been completed.
	TaskPending KusciaTaskPhase = "Pending"

	// TaskRunning means all sub-resources (e.g. services/pods) of this task
	// have been successfully scheduled and launched.
	TaskRunning KusciaTaskPhase = "Running"

	// TaskSucceeded means all sub-resources (e.g. services/pods) of this task
	// reached phase have terminated in success.
	TaskSucceeded KusciaTaskPhase = "Succeeded"

	// TaskFailed means one or more sub-resources (e.g. services/pods) of this task
	// reached phase failed with no restarting.
	TaskFailed KusciaTaskPhase = "Failed"
)

// KusciaTaskConditionType is a valid value for a kuscia task condition type.
type KusciaTaskConditionType string

// These are built-in conditions of kuscia task.
const (
	// KusciaTaskCondResourceCreated means all sub-resources (e.g. services/pods) of the task has been created.
	KusciaTaskCondResourceCreated KusciaTaskConditionType = "ResourceCreated"
	// KusciaTaskCondRunning means task is running.
	KusciaTaskCondRunning KusciaTaskConditionType = "Running"
	// KusciaTaskCondSuccess means task run success.
	KusciaTaskCondSuccess KusciaTaskConditionType = "Success"
	// KusciaTaskCondStatusSynced represents condition of syncing task status.
	KusciaTaskCondStatusSynced KusciaTaskConditionType = "StatusSynced"
)

// KusciaTaskCondition describes current state of a kuscia task.
type KusciaTaskCondition struct {
	// Type of task condition.
	Type KusciaTaskConditionType `json:"type"`
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

// PodStatus describes pod status.
type PodStatus struct {
	// Pod name.
	PodName string `json:"podName"`
	// The phase of a Pod is a simple, high-level summary of where the Pod is in its lifecycle.
	PodPhase corev1.PodPhase `json:"podPhase"`
	// Pod's namespace.
	Namespace string `json:"namespace"`
	// Pod's node name.
	// +optional
	NodeName string `json:"nodeName,omitempty"`
	// A human-readable message indicating details about why the pod is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// The latest stdout/stderr message if app exit fail.
	TerminationLog string `json:"terminationLog,omitempty"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// e.g. 'Evicted'
	// +optional
	Reason string `json:"reason,omitempty"`
}
