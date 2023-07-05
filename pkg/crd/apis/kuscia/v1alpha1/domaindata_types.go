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
// +kubebuilder:resource:shortName=kdd

// DomainData include feature table,model,rule,report .etc
type DomainData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              DomainDataSpec `json:"spec"`
	// +optional
	Status DataStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DomainDataList contains a list of domain data.
type DomainDataList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DomainData `json:"items"`
}

// DomainDataSpec defines the spec of data object.
type DomainDataSpec struct {
	// the URI of domaindata, the relative URI to the datasource
	RelativeURI string `json:"relativeURI"`
	Name        string `json:"name"`
	//
	Type       string `json:"type"`
	DataSource string `json:"dataSource"`
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

type Partition struct {
	// enum path, odps
	Type string `json:"type"`
	// +optional
	Fields []DataColumn `json:"fields"`
}

// DataColumn defines the column of data.
type DataColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// +optional
	Comment string `json:"comment"`
}
