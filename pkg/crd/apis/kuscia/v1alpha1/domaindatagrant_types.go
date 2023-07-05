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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ddg

// DomainDataGrant is the Schema for the data object API.
type DomainDataGrant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              DomainDataGrantSpec `json:"spec"`
	// +optional
	Status GrantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DomainDataList contains a list of domain data.
type DomainDataGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DomainDataGrant `json:"items"`
}

// DomainDataSpec defines the spec of data object.
type DomainDataGrantSpec struct {
	DomainData  string      `json:"domainData"`
	Signature   string      `json:"signature"`
	GrantDomain string      `json:"grantDomain"`
	GrantMode   []GrantType `json:"grantMode"`
	// +optional
	Description map[string]string `json:"description"`
}

// GrantLevel
// +kubebuilder:validation:Enum=normal;metadata;file
type GrantType string

// GrantStatus defines current data status.
type GrantStatus struct {
	// +kubebuilder:validation:Enum=Init;Granted;Unknown
	Phase   GrantPhase `json:"phase"`
	Message string     `json:"message"`
}

// GrantPhase is phase of data grant at the current time.
type GrantPhase string
