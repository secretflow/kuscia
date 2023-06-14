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
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source`
// +kubebuilder:printcolumn:name="Destination",type=string,JSONPath=`.spec.destination`
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.endpoint.host`
// +kubebuilder:printcolumn:name="Authentication",type=string,JSONPath=`.spec.authenticationType`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:resource:scope=Cluster,shortName=cdr

// ClusterDomainRoute defines the routing rules between domains on the center side.
type ClusterDomainRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ClusterDomainRouteSpec `json:"spec,omitempty"`

	// +optional
	Status ClusterDomainRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterDomainRouteList is a list of ClusterDomainRoutes.
type ClusterDomainRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterDomainRoute `json:"items"`
}

// ClusterDomainRouteSpec is a description of ClusterDomainRoute.
type ClusterDomainRouteSpec struct {
	DomainRouteSpec `json:",inline"`
}

// ClusterDomainRouteStatus defines the observed state of ClusterDomainRoute
type ClusterDomainRouteStatus struct {
	// +optional
	TokenStatus ClusterDomainRouteTokenStatus `json:"tokenStatus"`
	// Conditions is an array of current observed ClusterDomainRoute conditions.
	// +optional
	Conditions []ClusterDomainRouteCondition `json:"conditions,omitempty"`
	// EndpointStatuses shows the health status from all gateway instance of the source domain to the endpoint.
	// +optional
	EndpointStatuses map[string]ClusterDomainRouteEndpointStatus `json:"endpointStatuses,omitempty"`
}

// ClusterDomainRouteTokenStatus represents the status information related to token authentication.
type ClusterDomainRouteTokenStatus struct {
	// A sequence number representing a specific generation.
	Revision int64 `json:"revision"`
	// Timestamp representing the time when this revision created.
	RevisionTime metav1.Time `json:"revisionTime"`
	// SourceTokens keeps the most recently two generated tokens.
	SourceTokens []DomainRouteToken `json:"sourceTokens,omitempty"`
	// DestinationTokens keeps the most recently two generated tokens.
	DestinationTokens []DomainRouteToken `json:"destinationTokens,omitempty"`
}

// DomainRouteToken represents a generated token.
type DomainRouteToken struct {
	// Generated token.
	Token string `json:"token"`
	// A sequence number representing a specific generation.
	Revision int64 `json:"revision"`
	// Timestamp representing the time when this revision created.
	RevisionTime metav1.Time `json:"revisionTime"`
	// Record effective instances
	EffectiveInstances []string `json:"effectiveInstances,omitempty"`
}

// ClusterDomainRouteConditionType defines condition types for ClusterDomainRoute.
type ClusterDomainRouteConditionType string

// These are valid conditions of a ClusterDomainRoute.
const (
	// ClusterDomainRouteRunning means source and destination gateway are negotiating next token, RSA-GEN only.
	ClusterDomainRouteRunning ClusterDomainRouteConditionType = "Running"
	// ClusterDomainRoutePending means token has generated and waiting for next rolling update.
	ClusterDomainRoutePending ClusterDomainRouteConditionType = "Pending"
	// ClusterDomainRouteFailure means for some reason token can't be generated, typically due to network error
	// that no gateway available in source or destination namespace.
	ClusterDomainRouteFailure ClusterDomainRouteConditionType = "Failure"
	// ClusterDomainRouteReady means at least one token has been generated.
	ClusterDomainRouteReady ClusterDomainRouteConditionType = "Ready"
)

// ClusterDomainRouteCondition describes the state of a ClusterDomainRoute at a certain point.
type ClusterDomainRouteCondition struct {
	// Type of ClusterDomainRoute condition.
	Type ClusterDomainRouteConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// +optional
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human-readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// ClusterDomainRouteEndpointStatus describes the health status of the endpoint.
type ClusterDomainRouteEndpointStatus struct {
	// Whether the connection state from the gateway instance of the source node to the endpoint is healthy.
	EndpointHealthy bool `json:"endpointHealthy"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source`
// +kubebuilder:printcolumn:name="Destination",type=string,JSONPath=`.spec.destination`
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.endpoint.host`
// +kubebuilder:printcolumn:name="Authentication",type=string,JSONPath=`.spec.authenticationType`
// +kubebuilder:resource:shortName=dr

// DomainRoute defines the routing rules between domains on the center side.
type DomainRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec DomainRouteSpec `json:"spec,omitempty"`
	// +optional
	Status DomainRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DomainRouteList is a list of DomainRoutes.
type DomainRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DomainRoute `json:"items"`
}

// DomainRouteSpec is a description of DomainRoute.
type DomainRouteSpec struct {
	// Source namespace.
	Source string `json:"source"`
	// Destination namespace.
	Destination string `json:"destination"`
	// Endpoint defines address for the source to access destination.
	Endpoint DomainEndpoint `json:"endpoint"`
	// Transit entity. If it is not empty, the requests between nodes need to be transferred through a third party.
	// +optional
	Transit *Transit `json:"transit,omitempty"`
	// AuthenticationType describes how destination authenticates the source's request.
	AuthenticationType DomainAuthenticationType `json:"authenticationType"`
	// +optional
	TokenConfig *TokenConfig `json:"tokenConfig,omitempty"`
	// +optional
	BodyEncryption *BodyEncryption `json:"bodyEncryption,omitempty"`
	// +optional
	MTLSConfig *DomainRouteMTLSConfig `json:"mTLSConfig,omitempty"`
	// Whitelist of source IP address or CIDR. If it is empty, the source ip will not be checked.
	// +optional
	SourceWhiteIPList []string `json:"sourceWhiteIPList,omitempty"`
	// add specified headers to requests from source.
	// +optional
	RequestHeadersToAdd map[string]string `json:"requestHeadersToAdd,omitempty"`
}

// DomainEndpoint defines destination access address.
type DomainEndpoint struct {
	Host  string       `json:"host"`
	Ports []DomainPort `json:"ports"`
}

// DomainRouteProtocolType defines protocol type supported by the port.
type DomainRouteProtocolType string

const (
	DomainRouteProtocolHTTP DomainRouteProtocolType = "HTTP"
	DomainRouteProtocolGRPC DomainRouteProtocolType = "GRPC"
)

// DomainPort defines the port information of domain.
type DomainPort struct {
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=HTTP;GRPC
	Protocol DomainRouteProtocolType `json:"protocol"`
	// +optional
	IsTLS bool `json:"isTLS,omitempty"`
	Port  int  `json:"port"`
}

// Transit defines the information of the transit entity used to forward the request.
type Transit struct {
	// DomainTransit means to forward the request through the domain.
	// +optional
	Domain *DomainTransit `json:"domain,omitempty"`
}

// DomainTransit defines the information of the transit domain.
type DomainTransit struct {
	DomainID string `json:"domainID"`
}

// DomainAuthenticationType defines the type of authentication between domains.
type DomainAuthenticationType string

const (
	DomainAuthenticationToken DomainAuthenticationType = "Token"
	DomainAuthenticationMTLS  DomainAuthenticationType = "MTLS"
	DomainAuthenticationNone  DomainAuthenticationType = "None"
)

// TokenGenMethodType defines he method type for generating token.
type TokenGenMethodType string

const (
	// TokenGenMethodRSA means tokens are generated by negotiation like TLS handshake protocol.
	TokenGenMethodRSA = "RSA-GEN"
	// TokenGenMethodRAND means tokens are randomly generated by controller.
	TokenGenMethodRAND = "RAND-GEN"
)

// TokenConfig is used to realize authentication by negotiating token.
type TokenConfig struct {
	// Source namespace RSA public key, must be base64 encoded.
	// +optional
	SourcePublicKey string `json:"sourcePublicKey,omitempty"`
	// Destination namespace RSA public key, must be base64 encoded.
	// +optional
	DestinationPublicKey string `json:"destinationPublicKey,omitempty"`
	// Token periodic rolling update interval in seconds, 0 means no update.
	// +kubebuilder:validation:Minimum=0
	// +optional
	RollingUpdatePeriod int `json:"rollingUpdatePeriod"`
	// Token generation method.
	// +kubebuilder:validation:Enum=RSA-GEN;RAND-GEN
	TokenGenMethod TokenGenMethodType `json:"tokenGenMethod"`
}

type BodyEncryptionAlgorithmType string

const (
	BodyEncryptionAlgorithmAES BodyEncryptionAlgorithmType = "AES"
	BodyEncryptionAlgorithmSM4 BodyEncryptionAlgorithmType = "SM4"
)

// BodyEncryption defines detailed parameters for body encryption.
type BodyEncryption struct {
	// +kubebuilder:validation:Enum=AES;SM4
	Algorithm BodyEncryptionAlgorithmType `json:"algorithm"`
}

// DomainRouteMTLSConfig defines the configuration required by mTLS.
type DomainRouteMTLSConfig struct {
	// The tls certificate is used to verify the https server.
	// Must be base64 encoded.
	// +optional
	TLSCA string `json:"tlsCA,omitempty"`
	// When MTLS is only used as the communication layer, the public and private keys can be randomly generated
	// by the destination. The public key is ultimately used to generate the MTLS certificate, and the private key
	// needs to be given to the source.
	// When MTLS is used for authentication, the sourceClientKey should be the local private key of the source.
	// There is no need to specify it here.
	// Must be base64 encoded.
	// +optional
	SourceClientPrivateKey string `json:"sourceClientPrivateKey,omitempty"`
	// SourceClientCert is issued by the local self-signed CA of destination.
	// When MTLS is only used as the communication layer, it can be generated based on the randomly generated public key.
	// When MTLS is used for authentication, it needs to be generated based on the authorized public key of the source.
	// Must be base64 encoded.
	// +optional
	SourceClientCert string `json:"sourceClientCert,omitempty"`
}

// DomainRouteStatus represents information about the status of DomainRoute.
type DomainRouteStatus struct {
	// +optional
	TokenStatus DomainRouteTokenStatus `json:"tokenStatus,omitempty"`
}

// DomainRouteTokenStatus represents information about the token in DomainRoute.
type DomainRouteTokenStatus struct {
	// Initializer in source namespace that will start negotiation in this revision, RSA-GEN only.
	// +optional
	RevisionInitializer string `json:"revisionInitializer,omitempty"`
	// Token generated in specific revision, RSA-GEN only.
	// +optional
	RevisionToken DomainRouteToken `json:"revisionToken,omitempty"`
	// Tokens keeps the most recently two generated tokens.
	// +optional
	Tokens []DomainRouteToken `json:"tokens,omitempty"`
}
