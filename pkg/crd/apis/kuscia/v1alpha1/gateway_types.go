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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CreationTime",type=string,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="UpTime",type=string,JSONPath=`.status.uptime`
// +kubebuilder:printcolumn:name="HeartbeatTime",type=string,JSONPath=`.status.heartbeatTime`
// +kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.status.address`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`
// +kubebuilder:resource:shortName=gw

// Gateway is a proxy for communication between domains.
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            GatewayStatus `json:"status,omitempty"`
}

// GatewayStatus represents the current state of a Gateway.
type GatewayStatus struct {
	Address       string      `json:"address"`
	UpTime        metav1.Time `json:"uptime"`
	HeartbeatTime metav1.Time `json:"heartbeatTime"`
	// PublicKey is RSA public key used by domain, base64 encoded.
	PublicKey     string                  `json:"publicKey"`
	Version       string                  `json:"version"`
	NetworkStatus []GatewayEndpointStatus `json:"networkStatus,omitempty"`
}

type GatewayEndpointStatus struct {
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	TotalEndpointsCount   int    `json:"totalEndpointsCount"`
	HealthyEndpointsCount int    `json:"healthyEndpointsCount"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GatewayList is a list of Gateway resources.
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Gateway `json:"items"`
}
