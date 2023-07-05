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
// +kubebuilder:printcolumn:name="MinReservedPods",type=integer,JSONPath=`.spec.minReservedPods`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="StartTime",type=date,JSONPath=`.status.startTime`
// +kubebuilder:printcolumn:name="CompletionTime",type=date,JSONPath=`.status.completionTime`
// +kubebuilder:printcolumn:name="LastTransitionTime",type=date,JSONPath=`.status.lastTransitionTime`
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=tr

// TaskResource is the Schema for the task resource API.
type TaskResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              TaskResourceSpec `json:"spec"`
	// +optional
	Status TaskResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskResourceList contains a list of TaskResource.
type TaskResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TaskResource `json:"items"`
}

// TaskResourceSpec defines the details of TaskResource.
type TaskResourceSpec struct {
	// +optional
	Role string `json:"role,omitempty"`
	// +kubebuilder:validation:Minimum:=1
	MinReservedPods int `json:"minReservedPods"`
	// +optional
	ResourceReservedSeconds int `json:"resourceReservedSeconds,omitempty"`
	// +optional
	Initiator string `json:"initiator,omitempty"`
	// +optional
	Pods []TaskResourcePod `json:"pods,omitempty"`
}

// TaskResourcePod defines the details of task resource pod info.
type TaskResourcePod struct {
	Name string `json:"name"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

const LabelTaskResource = GroupName + "/task-resource"

// TaskResourcePhase is a label for the condition of a task resource at the current time.
type TaskResourcePhase string

const (
	// TaskResourcePhasePending means task resource is created.
	TaskResourcePhasePending TaskResourcePhase = "Pending"
	// TaskResourcePhaseReserving means resources are reserving.
	TaskResourcePhaseReserving TaskResourcePhase = "Reserving"
	// TaskResourcePhaseReserved means resources are reserved.
	TaskResourcePhaseReserved TaskResourcePhase = "Reserved"
	// TaskResourcePhaseSchedulable means pod related to the task resource can continue the subsequent scheduling process.
	TaskResourcePhaseSchedulable TaskResourcePhase = "Schedulable"
	// TaskResourcePhaseFailed means resources are reserved failed or task failed.
	TaskResourcePhaseFailed TaskResourcePhase = "Failed"
)

// TaskResourceStatus defines the details of task resource status.
type TaskResourceStatus struct {
	Phase TaskResourcePhase `json:"phase"`
	// +optional
	Conditions []TaskResourceCondition `json:"conditions,omitempty"`
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// TaskResourceConditionType is a valid value for a task resource condition type.
type TaskResourceConditionType string

const (
	TaskResourceCondPending     TaskResourceConditionType = "Pending"
	TaskResourceCondReserving   TaskResourceConditionType = "Reserving"
	TaskResourceCondFailed      TaskResourceConditionType = "Failed"
	TaskResourceCondReserved    TaskResourceConditionType = "Reserved"
	TaskResourceCondSchedulable TaskResourceConditionType = "Schedulable"
)

// TaskResourceCondition defines the details of task resource condition.
type TaskResourceCondition struct {
	LastTransitionTime *metav1.Time           `json:"lastTransitionTime,omitempty"`
	Status             corev1.ConditionStatus `json:"status"`
	// +optional
	Reason string                    `json:"reason,omitempty"`
	Type   TaskResourceConditionType `json:"type"`
}
