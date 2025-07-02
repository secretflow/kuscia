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
