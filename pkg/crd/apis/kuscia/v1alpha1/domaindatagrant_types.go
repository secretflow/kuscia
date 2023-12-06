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
	Status DomainDataGrantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DomainDataGrantList contains a list of domain data grant.
type DomainDataGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DomainDataGrant `json:"items"`
}

type GrantLimit struct {
	// +optional
	ExpirationTime *metav1.Time `json:"expirationTime,omitempty"`
	// +optional
	UseCount  int         `json:"useCount,omitempty"`
	GrantMode []GrantType `json:"grantMode"`
	// +optional
	FlowID string `json:"flowID,omitempty"`
	// +optional
	Components []string `json:"components,omitempty"`
	// +optional
	Initiator string `json:"initiator,omitempty"`
	// +optional
	InputConfig string `json:"inputConfig,omitempty"`
}

// DomainDataGrantSpec defines the spec of data grant info.
type DomainDataGrantSpec struct {
	Author       string `json:"author"`
	DomainDataID string `json:"domainDataID"`
	// +optional
	Signature   string `json:"signature,omitempty"`
	GrantDomain string `json:"grantDomain"`
	// +optional
	Limit *GrantLimit `json:"limit,omitempty"`
	// +optional
	Description map[string]string `json:"description,omitempty"`
}

type DataSchema struct {
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`
	// +optional
	Partition *Partition `json:"partitions,omitempty"`
	// +optional
	Columns []DataColumn `json:"columns,omitempty"`
	// +optional ,The vendor is the one who outputs the domain data, which may be done by the secretFlow engine,
	// another vendor's engine, or manually registered.
	Vendor string `json:"vendor,omitempty"`
}

// DomainDataGrantStatus defines current data status.
type DomainDataGrantStatus struct {
	Phase   GrantPhase `json:"phase"`
	Message string     `json:"message"`
	// +optional
	UseRecords []UseRecord `json:"use_records"`
}

// GrantPhase is phase of data grant at the current time.
// +kubebuilder:validation:Enum=Ready;Unavailable;Unknown
type GrantPhase string

const (
	GrantReady       GrantPhase = "Ready"
	GrantUnavailable GrantPhase = "Unavailable"
	GrantUnknown     GrantPhase = "Unknown"
)

type UseRecord struct {
	UseTime     metav1.Time `json:"use_time"`
	GrantDomain string      `json:"grant_domain"`
	// +optional
	Component string `json:"componet,omitempty"`
	// +optional
	Output string `json:"output,omitempty"`
}

// GrantLevel
// +kubebuilder:validation:Enum=normal;metadata;file
type GrantType string
