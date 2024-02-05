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
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="TotalParties",type=integer,JSONPath=`.status.totalParties`
// +kubebuilder:printcolumn:name="AvailableParties",type=integer,JSONPath=`.status.availableParties`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kd

// KusciaDeployment is the Schema for the kuscia deployment API.
type KusciaDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KusciaDeploymentSpec `json:"spec"`
	// +optional
	Status KusciaDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KusciaDeploymentList contains a list of kuscia deployments.
type KusciaDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KusciaDeployment `json:"items"`
}

// KusciaDeploymentSpec defines the information of kuscia deployment spec.
type KusciaDeploymentSpec struct {
	Initiator   string                  `json:"initiator"`
	InputConfig string                  `json:"inputConfig"`
	Parties     []KusciaDeploymentParty `json:"parties"`
}

// KusciaDeploymentParty defines the kuscia deployment party info.
type KusciaDeploymentParty struct {
	DomainID    string `json:"domainID"`
	AppImageRef string `json:"appImageRef"`
	// +optional
	Role string `json:"role,omitempty"`
	// +optional
	Template KusciaDeploymentPartyTemplate `json:"template,omitempty"`
}

// KusciaDeploymentPartyTemplate defines the template info for party.
type KusciaDeploymentPartyTemplate struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// The deployment strategy to use to replace existing pods with new ones.
	// +optional
	Strategy *v1.DeploymentStrategy `json:"strategy,omitempty"`
	// +optional
	Spec PodSpec `json:"spec,omitempty"`
}

// KusciaDeploymentPartyStatus defines party status of kuscia deployment.
type KusciaDeploymentPartyStatus struct {
	// The party deployment phase.
	// +optional
	Phase KusciaDeploymentPhase `json:"phase,omitempty"`
	// +optional
	Role string `json:"role,omitempty"`
	// Total number of non-terminated pods targeted by this deployment (their labels match the selector).
	Replicas int32 `json:"replicas"`
	// Total number of non-terminated pods targeted by this deployment that have the desired template spec.
	UpdatedReplicas int32 `json:"updatedReplicas"`
	// Total number of available pods (ready for at least minReadySeconds) targeted by this deployment.
	AvailableReplicas int32 `json:"availableReplicas"`
	// Total number of unavailable pods targeted by this deployment. This is the total number of
	// pods that are still required for the deployment to have 100% available capacity. They may
	// either be pods that are running but not yet available or pods that still have not been created.
	UnavailableReplicas int32 `json:"unavailableReplicas"`
	// Represents the latest available observations of a deployment's current state.
	// +optional
	Conditions []v1.DeploymentCondition `json:"conditions,omitempty"`
	// +optional
	CreationTimestamp *metav1.Time `json:"creationTimestamp,omitempty"`
}

// KusciaDeploymentStatus defines the observed state of kuscia deployment.
type KusciaDeploymentStatus struct {
	// The phase of a KusciaDeployment is a simple, high-level summary of
	// where the deployment is in its lifecycle.
	// +optional
	Phase KusciaDeploymentPhase `json:"phase,omitempty"`
	// A brief CamelCase message indicating details about why it is in this state.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A readable message indicating details about why it is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// Total number of parties.
	TotalParties int `json:"totalParties"`
	// Total number of available parties.
	AvailableParties int `json:"availableParties"`
	// PartyDeploymentStatuses defines deployment status for all party.
	// +optional
	PartyDeploymentStatuses map[string]map[string]*KusciaDeploymentPartyStatus `json:"partyDeploymentStatuses,omitempty"`
	// Represents last time when the deployment was reconciled. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

// KusciaDeploymentPhase defines the phase for deployment.
type KusciaDeploymentPhase string

const (
	// KusciaDeploymentPhaseProgressing means the deployments are progressing.
	KusciaDeploymentPhaseProgressing KusciaDeploymentPhase = "Progressing"

	// KusciaDeploymentPhasePartialAvailable means the deployments are partial available.
	KusciaDeploymentPhasePartialAvailable KusciaDeploymentPhase = "PartialAvailable"

	// KusciaDeploymentPhaseAvailable means the deployments are available.
	KusciaDeploymentPhaseAvailable KusciaDeploymentPhase = "Available"

	// KusciaDeploymentPhaseFailed means failed to parse and create deployment.
	KusciaDeploymentPhaseFailed KusciaDeploymentPhase = "Failed"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=kds

// KusciaDeploymentSummary is used to sync deployment status between clusters
type KusciaDeploymentSummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// +optional
	Status KusciaDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KusciaDeploymentSummaryList contains a list of kuscia deployments.
type KusciaDeploymentSummaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KusciaDeploymentSummary `json:"items"`
}
