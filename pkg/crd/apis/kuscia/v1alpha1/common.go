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

type InterConnProtocolType string

const (
	InterConnKuscia InterConnProtocolType = "kuscia"
	InterConnBFIA   InterConnProtocolType = "bfia" // Beijing FinTech Industry Alliance
)

// DataStatus defines current data status.
type DataStatus struct {
	// +kubebuilder:validation:Enum=Available;Unavailable;Unknown
	Phase      DataPhase       `json:"phase"`
	Conditions []DataCondition `json:"conditions"`
}

// DataPhase is a label for the condition of data at the current time.
type DataPhase string

const (
	AvailablePhase   DataPhase = "Available"
	UnavailablePhase DataPhase = "Unavailable"
	UnknownPhase     DataPhase = "Unknown"
)

// DataCondition defines current state of data.
type DataCondition struct {
	Status corev1.ConditionStatus `json:"status"`
	// +optional
	Message        string      `json:"message,omitempty"`
	Reason         string      `json:"reason"`
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
}

// NetworkPolicy defines the network policy.
type NetworkPolicy struct {
	Ingresses []Ingress `json:"ingresses"`
}

// Ingress defines the ingress information.
type Ingress struct {
	From  IngressFrom   `json:"from"`
	Ports []IngressPort `json:"ports"`
}

// IngressFrom defines the requester info.
type IngressFrom struct {
	Roles []string `json:"roles"`
}

// IngressPort defines the port that is allowed to access.
type IngressPort struct {
	Port string `json:"port"`
}

// 定义MetricProbe字段
type MetricProbe struct {
	Path string `json:"path,omitempty"`
	//!!!!!!!!!这里的unit16进行了更改
	Port uint16 `json:"port,omitempty"`
}

// PodSpec defines the spec info of pod.
type PodSpec struct {
	// Restart policy for all containers within the pod.
	// One of Always, OnFailure, Never.
	// Default to Never.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#restart-policy
	// +optional
	RestartPolicy corev1.RestartPolicy `json:"restartPolicy,omitempty"`
	// +optional
	Containers []Container `json:"containers,omitempty"`
	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// Container defines the container info.
type Container struct {
	Name string `json:"name"`
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args       []string `json:"args,omitempty"`
	WorkingDir string   `json:"workingDir"`
	// +optional
	ConfigVolumeMounts []ConfigVolumeMount `json:"configVolumeMounts,omitempty"`
	// +optional
	Ports []ContainerPort `json:"ports"`
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// +optional
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`
	// +optional
	MetricProbe *MetricProbe `json:"metricProbe,omitempty"` //这里将类型从string修改为了*MetricProbe
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// SecurityContext only privileged works now.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// ConfigVolumeMount defines config volume mount info.
type ConfigVolumeMount struct {
	MountPath string `json:"mountPath"`
	SubPath   string `json:"subPath"`
}

// PortProtocol defines the network protocols.
type PortProtocol string

const (
	// ProtocolHTTP is the HTTP protocol.
	ProtocolHTTP PortProtocol = "HTTP"
	// ProtocolGRPC is the GRPC protocol.
	ProtocolGRPC PortProtocol = "GRPC"
)

// PortScope defines the port usage scope.
type PortScope string

const (
	// ScopeCluster allows to access inside and outside the cluster.
	ScopeCluster PortScope = "Cluster"
	// ScopeDomain allows to access inside the cluster.
	ScopeDomain PortScope = "Domain"
	// ScopeLocal allows to access inside the pod containers.
	ScopeLocal PortScope = "Local"
)

// ContainerPort describes container port info.
type ContainerPort struct {
	Name string `json:"name"`
	// +optional
	Port int32 `json:"port,omitempty"`
	// +kubebuilder:validation:Enum=HTTP;GRPC
	// +kubebuilder:default:=HTTP
	// +optional
	Protocol PortProtocol `json:"protocol,omitempty"`
	// +kubebuilder:validation:Enum=Cluster;Domain;Local
	// +kubebuilder:default:=Local
	// +optional
	Scope PortScope `json:"scope,omitempty"`
}

// PartyAllocatedPorts defines the ports allocated to the party.
type PartyAllocatedPorts struct {
	DomainID string `json:"domainID"`
	// +optional
	Role string `json:"role,omitempty"`
	// +optional
	NamedPort map[string]int32 `json:"namedPort,omitempty"`
}
