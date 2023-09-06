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

package common

// labels
const (
	// LabelPortScope represents port usage scope. Its values may be Local, Domain, Cluster. Refer to PortScope for more details.
	LabelPortScope = "kuscia.secretflow/port-scope"

	LabelController                      = "kuscia.secretflow/controller"
	LabelGatewayProxy                    = "kuscia.secretflow/gateway-proxy"
	LabelLoadBalancer                    = "kuscia.secretflow/loadbalancer"
	LabelCommunicationRoleServer         = "kuscia.secretflow/communication-role-server"
	LabelCommunicationRoleClient         = "kuscia.secretflow/communication-role-client"
	LabelDomainName                      = "kuscia.secretflow/domain-name"
	LabelDomainAuth                      = "kuscia.secretflow/domain-auth"
	LabelNodeNamespace                   = "kuscia.secretflow/namespace"
	LabelDomainDeleted                   = "kuscia.secretflow/deleted"
	LabelDomainRole                      = "kuscia.secretflow/role"
	LabelInterConnProtocols              = "kuscia.secretflow/interconn-protocols"
	LabelResourceVersionUnderHostCluster = "kuscia.secretflow/resource-version-under-host-cluster"
	LabelTaskResourceGroup               = "kuscia.secretflow/task-resource-group"
	LabelTaskInitiator                   = "kuscia.secretflow/initiator"
	LabelTaskUnschedulable               = "kuscia.secretflow/task-unschedulable"
	LabelPodHasSynced                    = "kuscia.secretflow/has-synced"
	LabelDomainDataType                  = "kuscia.secretflow/domaindata-type"
	LabelDomainDataVendor                = "kuscia.secretflow/domaindata-vendor"
	LabelDomainDataSourceType            = "kuscia.secretflow/domaindatasource-type"

	LabelSelfClusterAsInitiator = "kuscia.secretflow/self-cluster-as-initiator"
	LabelInterConnProtocolType  = "kuscia.secretflow/interconn-protocol-type"
	LabelJobID                  = "kuscia.secretflow/job-id"
	LabelTaskID                 = "kuscia.secretflow/task-id"
	LabelTaskAlias              = "kuscia.secretflow/task-alias"
)

type LoadBalancerType string

const (
	DomainRouteLoadBalancer LoadBalancerType = "domainroute"
)

const (
	KusciaSchedulerName = "kuscia-scheduler"
)

// annotations
const (
	AccessDomainAnnotationKey = "kuscia.secretflow/access-domain"
	ProtocolAnnotationKey     = "kuscia.secretflow/protocol"
	ReadyTimeAnnotationKey    = "kuscia.secretflow/ready-time"

	ConfigTemplateVolumesAnnotationKey = "kuscia.secretflow/config-template-volumes"

	TaskResourceReservingTimestamp = "kuscia.secretflow/task-resource-reserving-timestamp"

	ComponentSpecAnnotationKey = "kuscia.secretflow/component-spec"
)

// Environment variables issued to the task pod.
const (
	EnvTaskID            = "TASK_ID"
	EnvTaskInputConfig   = "TASK_INPUT_CONFIG"
	EnvTaskClusterDefine = "TASK_CLUSTER_DEFINE"
	EnvAllocatedPorts    = "ALLOCATED_PORTS"
	EnvServerCertFile    = "SERVER_CERT_FILE"
	EnvServerKeyFile     = "SERVER_PRIVATE_KEY_FILE"
	EnvClientCertFile    = "CLIENT_CERT_FILE"
	EnvClientKeyFile     = "CLIENT_PRIVATE_KEY_FILE"
	EnvTrustedCAFile     = "TRUSTED_CA_FILE"
)

const (
	KusciaTaintTolerationKey = "kuscia.secretflow/agent"
)

const (
	KusciaSourceKey      = "kuscia.secretflow/clusterdomainroute-source"
	KusciaDestinationKey = "kuscia.secretflow/clusterdomainroute-destination"
)

const (
	// PodIdentityGroupInternal means the pod is created locally. The pod may be a static pod.
	PodIdentityGroupInternal = "internal"
	// PodIdentityGroupExternal means the pod is created remotely, usually apiserver.
	PodIdentityGroupExternal = "external"

	True  = "true"
	False = "false"
)

const (
	DefaultDataSourceID     = "default-data-source"
	DefaultDomainDataVendor = "manual"
)
