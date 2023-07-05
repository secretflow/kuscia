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
// +kubebuilder:printcolumn:name="MinReservedMembers",type=integer,JSONPath=`.spec.minReservedMembers`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="StartTime",type=date,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="CompletionTime",type=date,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="LastTransitionTime",type=date,JSONPath=`.status.lastTransitionTime`
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=trg

// TaskResourceGroup is the Schema for the task resource group API.
type TaskResourceGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              TaskResourceGroupSpec `json:"spec"`
	// +optional
	Status TaskResourceGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskResourceGroupList contains a list of TaskResourceGroup.
type TaskResourceGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TaskResourceGroup `json:"items"`
}

// TaskResourceGroupSpec defines the details of TaskResourceGroup.
type TaskResourceGroupSpec struct {
	// MinReservedMembers represents the number of minimum reserved resource parties.
	// This field is used for parties level co-scheduling.
	// +kubebuilder:validation:Minimum:=1
	MinReservedMembers int `json:"minReservedMembers"`
	// ResourceReservedSeconds represents resource reserved time.
	// This field is used for the waiting time after the scheduler has reserved resources.
	// +optional
	ResourceReservedSeconds int `json:"resourceReservedSeconds,omitempty"`
	// RetryIntervalSeconds represents retry waiting time for next scheduling.
	// +optional
	RetryIntervalSeconds int `json:"retryIntervalSeconds,omitempty"`
	// LifecycleSeconds represents task resource group lifecycle.
	// If the task has not been scheduled successfully in the lifecycle, the task resource group is set to failed.
	// +optional
	LifecycleSeconds int `json:"lifecycleSeconds,omitempty"`
	// Initiator represents who initiated the task.
	Initiator string `json:"initiator"`
	// Parties represents parties info of task.
	Parties []TaskResourceGroupParty `json:"parties"`
}

// TaskResourceGroupParty defines the details of task resource group party.
type TaskResourceGroupParty struct {
	// +optional
	Role     string `json:"role,omitempty"`
	DomainID string `json:"domainID"`
	// +optional
	MinReservedPods int                         `json:"minReservedPods,omitempty"`
	Pods            []TaskResourceGroupPartyPod `json:"pods"`
	// +optional
	TaskResourceName string `json:"taskResourceName,omitempty"`
}

// TaskResourceGroupPartyPod defines the details of task resource group party pod.
type TaskResourceGroupPartyPod struct {
	Name string `json:"name"`
}

// TaskResourceGroupPhase is a label for the condition of a task resource group at the current time.
type TaskResourceGroupPhase string

const (
	// TaskResourceGroupPhasePending means TaskResourceGroup resource is created.
	TaskResourceGroupPhasePending TaskResourceGroupPhase = "Pending"
	// TaskResourceGroupPhaseCreating means create TaskResource.
	TaskResourceGroupPhaseCreating TaskResourceGroupPhase = "Creating"
	// TaskResourceGroupPhaseReserving means resources are reserving by parties.
	TaskResourceGroupPhaseReserving TaskResourceGroupPhase = "Reserving"
	// TaskResourceGroupPhaseReserved means resources are reserved by parties.
	TaskResourceGroupPhaseReserved TaskResourceGroupPhase = "Reserved"
	// TaskResourceGroupPhaseReserveFailed means parties which successfully reserved resources do not meet the minimum reserved count.
	TaskResourceGroupPhaseReserveFailed TaskResourceGroupPhase = "ReserveFailed"
	// TaskResourceGroupPhaseFailed means no resources are reserved in the whole task resource group life cycle
	TaskResourceGroupPhaseFailed TaskResourceGroupPhase = "Failed"
)

// TaskResourceGroupStatus defines the details of task group resource status.
type TaskResourceGroupStatus struct {
	Phase TaskResourceGroupPhase `json:"phase"`
	// +optional
	RetryCount int `json:"retryCount,omitempty"`
	// +optional
	Conditions []TaskResourceGroupCondition `json:"conditions,omitempty"`
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// TaskResourceGroupConditionType is a valid value for a task resource group condition type.
type TaskResourceGroupConditionType string

const (
	TaskResourceGroupValidated TaskResourceGroupConditionType = "TaskResourceGroupValidated"
	TaskResourceNameGenerated  TaskResourceGroupConditionType = "TaskResourceNameGenerated"
	TaskResourcesCreated       TaskResourceGroupConditionType = "TaskResourcesCreated"
	PodAnnotationUpdated       TaskResourceGroupConditionType = "PodAnnotationUpdated"
	TaskResourcesListed        TaskResourceGroupConditionType = "TaskResourcesListed"
	TaskResourcesReserved      TaskResourceGroupConditionType = "TaskResourcesReserved"
	TaskResourceGroupExpired   TaskResourceGroupConditionType = "TaskResourceGroupExpired"
	TaskResourcesScheduled     TaskResourceGroupConditionType = "TaskResourcesScheduled"
	TaskResourceGroupFailed    TaskResourceGroupConditionType = "TaskResourceGroupFailed"
	DependentTaskFailed        TaskResourceGroupConditionType = "DependentTaskFailed"
)

// TaskResourceGroupCondition defines the details of task resource group condition.
type TaskResourceGroupCondition struct {
	LastTransitionTime *metav1.Time           `json:"lastTransitionTime,omitempty"`
	Status             corev1.ConditionStatus `json:"status"`
	// +optional
	Reason string                         `json:"reason,omitempty"`
	Type   TaskResourceGroupConditionType `json:"type"`
}

// SchedulerPluginArgs defines the parameters for scheduler plugin.
type SchedulerPluginArgs struct {
	// ResourceReservedSeconds is the waiting timeout in seconds.
	// +optional
	ResourceReservedSeconds int `json:"resourceReservedSeconds,omitempty"`
}
