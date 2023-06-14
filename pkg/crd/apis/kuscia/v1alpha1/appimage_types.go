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
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=aimg

// AppImage is the Schema for the app image API.
type AppImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              AppImageSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppImageList contains a list of app images.
type AppImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppImage `json:"items"`
}

// AppImageSpec defines the details of app image.
type AppImageSpec struct {
	Image AppImageInfo `json:"image"`
	// +optional
	ConfigTemplates map[string]string `json:"configTemplates,omitempty"`
	DeployTemplates []DeployTemplate  `json:"deployTemplates"`
}

// AppImageInfo defines the basic app image info.
type AppImageInfo struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
	ID   string `json:"id"`
	Sign string `json:"sign"`
}

// DeployTemplate defines the app deploy template.
type DeployTemplate struct {
	Name string `json:"name"`
	// +optional
	Role string `json:"role,omitempty"`
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// +optional
	NetworkPolicy *NetworkPolicy `json:"networkPolicy,omitempty"`
	Spec          PodSpec        `json:"spec"`
}
