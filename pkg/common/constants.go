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

//nolint:dupl
package common

import "time"

// labels
const (
	// LabelPortScope represents port usage scope. Its values may be Local, Domain, Cluster. Refer to PortScope for more details.
	LabelPortScope = "kuscia.secretflow/port-scope"
	// LabelPortName represents port name which defined in AppImage container port.
	LabelPortName = "kuscia.secretflow/port-name"

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
	LabelTaskUnschedulable               = "kuscia.secretflow/task-unschedulable"
	LabelHasSynced                       = "kuscia.secretflow/has-synced"
	LabelDomainDataType                  = "kuscia.secretflow/domaindata-type"
	LabelDomainDataID                    = "kuscia.secretflow/domaindataid"
	LabelDomainDataVendor                = "kuscia.secretflow/domaindata-vendor"
	LabelDomainDataSourceType            = "kuscia.secretflow/domaindatasource-type"
	LabelDomainDataGrantVendor           = "kuscia.secretflow/domaindatagrant-vendor"
	LabelDomainDataGrantDomain           = "kuscia.secretflow/domaindatagrant-domain"

	// LabelInterConnProtocolType is a label to specify the interconn protocol type of job
	// For KusciaJob, it's only used for partner job
	LabelInterConnProtocolType = "kuscia.secretflow/interconn-protocol-type"
	LabelJobUID                = "kuscia.secretflow/job-uid"
	LabelTaskUID               = "kuscia.secretflow/task-uid"
	LabelTaskResourceGroupUID  = "kuscia.secretflow/task-resource-group-uid"

	LabelJobAutoApproval = "kuscia.secretflow/job-auto-approval"
	// LabelJobStage is a label to specify the current stage of job.
	LabelJobStage = "kuscia.secretflow/job-stage"
	// LabelJobStageTrigger is a label to specify who trigger the current stage of job.
	LabelJobStageTrigger = "kuscia.secretflow/job-stage-trigger"
	LabelJobStageVersion = "kuscia.secretflow/job-stage-version"

	LabelKusciaDeploymentAppType  = "kuscia.secretflow/app-type"
	LabelKusciaDeploymentUID      = "kuscia.secretflow/kd-uid"
	LabelKusciaDeploymentName     = "kuscia.secretflow/kd-name"
	LabelKubernetesDeploymentName = "kuscia.secretflow/deployment-name"
	LabelKusciaOwnerNamespace     = "kuscia.secretflow/owner_namespace"

	LabelNodeName        = "kuscia.secretflow/node"
	LabelPodUID          = "kuscia.secretflow/pod-uid"
	LabelOwnerReferences = "kuscia.secretflow/owner-references"

	LabelDomainRoutePartner = "kuscia.secertflow/domainroute-partner"
)

const (
	PluginNameCertIssuance  = "cert-issuance"
	PluginNameConfigRender  = "config-render"
	PluginNameImageSecurity = "image-security"
	PluginNameEnvImport     = "env-import"
)

type LoadBalancerType string

const (
	DomainRouteLoadBalancer LoadBalancerType = "domainroute"
)

type KusciaDeploymentAppType string

const (
	ServingApp KusciaDeploymentAppType = "serving"
	SCQLApp    KusciaDeploymentAppType = "scql"
)

const (
	KusciaSchedulerName        = "kuscia-scheduler"
	KusciaCrossDomain          = "cross-domain"
	JobCustomFieldsLabelPrefix = "kuscia.job.custom-fields/"
	ReceiverServiceName        = "receiver"
)

// annotations
const (
	InitiatorAnnotationKey              = "kuscia.secretflow/initiator"
	SelfClusterAsInitiatorAnnotationKey = "kuscia.secretflow/self-cluster-as-initiator"

	// InterConnKusciaPartyAnnotationKey is a annotation which has parties interconnected with kuscia protocol,
	// the value is a series of domain id join with '_', such as alice_bob_carol .
	InterConnKusciaPartyAnnotationKey = "kuscia.secretflow/interconn-kuscia-parties"

	// InterConnBFIAPartyAnnotationKey is a annotation which has parties interconnected with bfia protocol,
	// the value is a series of domain id join with '_', such as alice_bob_carol .
	InterConnBFIAPartyAnnotationKey = "kuscia.secretflow/interconn-bfia-parties"

	InitiatorMasterDomainAnnotationKey   = "kuscia.secretflow/initiator-master-domain"
	InterConnSelfPartyAnnotationKey      = "kuscia.secretflow/interconn-self-parties"
	KusciaPartyMasterDomainAnnotationKey = "kuscia.secretflow/party-master-domain"

	TaskSummaryResourceVersionAnnotationKey = "kuscia.secretflow/tasksummary-resource-version"

	JobIDAnnotationKey             = "kuscia.secretflow/job-id"
	TaskIDAnnotationKey            = "kuscia.secretflow/task-id"
	TaskAliasAnnotationKey         = "kuscia.secretflow/task-alias"
	TaskResourceGroupAnnotationKey = "kuscia.secretflow/task-resource-group"

	AccessDomainAnnotationKey = "kuscia.secretflow/access-domain"
	ProtocolAnnotationKey     = "kuscia.secretflow/protocol"
	ReadyTimeAnnotationKey    = "kuscia.secretflow/ready-time"

	ConfigTemplateVolumesAnnotationKey = "kuscia.secretflow/config-template-volumes"

	TaskResourceReservingTimestampAnnotationKey = "kuscia.secretflow/taskresource-reserving-timestamp"

	ComponentSpecAnnotationKey  = "kuscia.secretflow/component-spec"
	AllocatedPortsAnnotationKey = "kuscia.secretflow/allocated-ports"
	ImageIDAnnotationKey        = "kuscia.secretflow/image-id"
)

// Environment variables issued to the pod.
const (
	EnvTaskID              = "TASK_ID"
	EnvServingID           = "SERVING_ID"
	EnvInputConfig         = "INPUT_CONFIG"
	EnvClusterDefine       = "CLUSTER_DEFINE"
	EnvTaskInputConfig     = "TASK_INPUT_CONFIG"
	EnvTaskClusterDefine   = "TASK_CLUSTER_DEFINE"
	EnvAllocatedPorts      = "ALLOCATED_PORTS"
	EnvServerCertFile      = "SERVER_CERT_FILE"
	EnvServerKeyFile       = "SERVER_PRIVATE_KEY_FILE"
	EnvClientCertFile      = "CLIENT_CERT_FILE"
	EnvClientKeyFile       = "CLIENT_PRIVATE_KEY_FILE"
	EnvTrustedCAFile       = "TRUSTED_CA_FILE"
	EnvDomainID            = "KUSCIA_DOMAIN_ID"
	EnvPortNumber          = "KUSCIA_PORT_%s_NUMBER"
	EnvKusciaAPIProtocol   = "KUSCIA_API_PROTOCOL"
	EnvKusciaAPIToken      = "KUSCIA_API_TOKEN"
	EnvKusciaDomainKeyData = "KUSCIA_DOMAIN_KEY_DATA"
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
	DefaultDataSourceID          = "default-data-source"
	DefaultDataProxyDataSourceID = "default-dp-data-source"

	DefaultDomainDataVendor = "manual"
	DomainDataVendorGrant   = "grant"
)

const (
	DomainDataSourceTypeLocalFS        = "localfs"
	DomainDataSourceTypeOSS            = "oss"
	DomainDataSourceTypeMysql          = "mysql"
	DefaultDomainDataSourceLocalFSPath = "var/storage/data"
)

type RunModeType = string

const (
	RunModeMaster   = "master"
	RunModeAutonomy = "autonomy"
	RunModeLite     = "lite"
)

const (
	DefaultSecretBackendName = "default"
	DefaultSecretBackendType = "mem"
)

type CommunicationProtocol string

type Protocol string

const (
	NOTLS Protocol = "NOTLS"
	TLS   Protocol = "TLS"
	MTLS  Protocol = "MTLS"
)

const DomainCsrExtensionID = "1.2.3.4"

const (
	CertPrefix   = "var/certs/"
	LogPrefix    = "var/logs/"
	StdoutPrefix = "var/stdout/"
	TmpPrefix    = "var/tmp/"
	ConfPrefix   = "etc/conf/"
)

const (
	GatewayLiveTimeout = 3 * time.Minute
)

const (
	DeployTokenUsedState   = "used"
	DeployTokenUnusedState = "unused"
)
