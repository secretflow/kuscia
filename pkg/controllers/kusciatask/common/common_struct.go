package common

import (
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	proto "github.com/secretflow/kuscia/proto/api/v1alpha1/appconfig"
)

type NamedPorts map[string]kusciaapisv1alpha1.ContainerPort
type PortService map[string]string

type PodKitInfo struct {
	Index          int
	PodName        string
	PodIdentity    string
	Ports          NamedPorts
	PortService    PortService
	ClusterDef     *proto.ClusterDefine
	AllocatedPorts *proto.AllocatedPorts
}

type PartyKitInfo struct {
	KusciaTask            *kusciaapisv1alpha1.KusciaTask
	DomainID              string
	Role                  string
	Image                 string
	ImageID               string
	DeployTemplate        *kusciaapisv1alpha1.DeployTemplate
	ConfigTemplatesCMName string
	ConfigTemplates       map[string]string
	ServicedPorts         []string
	PortAccessDomains     map[string]string
	MinReservedPods       int
	Pods                  []*PodKitInfo
	BandwidthLimit        []kusciaapisv1alpha1.BandwidthLimit
}
