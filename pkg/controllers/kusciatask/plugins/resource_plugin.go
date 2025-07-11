// Copyright 2025 Ant Group Co., Ltd.
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
package plugins

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/common"
	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/resource"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ResourceCheckPlugin struct {
	nrm *resource.NodeResourceManager
}

func NewResourceCheckPlugin(manager *resource.NodeResourceManager) *ResourceCheckPlugin {
	return &ResourceCheckPlugin{
		nrm: manager,
	}
}

func (p *ResourceCheckPlugin) Permit(ctx context.Context, params interface{}) (bool, error) {
	var partyKitInfo common.PartyKitInfo
	var ok bool
	partyKitInfo, ok = params.(common.PartyKitInfo)
	if !ok {
		return false, fmt.Errorf("resource-check could not convert params %v to PartyKitInfo", params)
	}

	var allContainerCPURequest, allContainerMEMRequest int64
	for _, container := range partyKitInfo.DeployTemplate.Spec.Containers {
		if container.Resources.Requests == nil {
			nlog.Warnf("Kt %s container %s have no requests settings", partyKitInfo.KusciaTask.Name, container.Name)
			continue
		}
		cpuValue := container.Resources.Requests.Cpu().MilliValue()
		memValue := container.Resources.Requests.Memory().Value()
		allContainerCPURequest += cpuValue
		allContainerMEMRequest += memValue
	}

	_, err := p.nrm.ResourceCheck(partyKitInfo.DomainID, allContainerCPURequest, allContainerMEMRequest)
	if err != nil {
		return false, err
	}
	return true, nil
}
