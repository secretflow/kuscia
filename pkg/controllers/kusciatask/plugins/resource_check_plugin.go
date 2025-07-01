package plugins

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/controllers/domain"
	"github.com/secretflow/kuscia/pkg/controllers/kusciatask"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ResourceCheckPlugin struct{}

func NewResourceCheckPlugin() *ResourceCheckPlugin {
	return &ResourceCheckPlugin{}
}

func (p *ResourceCheckPlugin) Permit(ctx context.Context, params interface{}) (bool, error) {
	var partyKitInfo kusciatask.PartyKitInfo
	var ok bool
	partyKitInfo, ok = params.(kusciatask.PartyKitInfo)
	if !ok {
		return false, fmt.Errorf("resource-check could not convert params %v to PartyKitInfo", params)
	}

	requestReq := p.resourceRequest(partyKitInfo)
	domainName := requestReq.DomainName
	domain.NodeResourceStore.Lock.RLock()
	defer domain.NodeResourceStore.Lock.RUnlock()

	localNodeStatuses, exists := domain.NodeResourceStore.LocalNodeStatuses[domainName]
	if !exists {
		return false, fmt.Errorf("resource-check no node status available for domain %s", domainName)
	}

	for _, nodeStatus := range localNodeStatuses {
		if nodeStatus.Status != domain.NodeStateReady {
			continue
		}

		nodeCPUValue := nodeStatus.Allocatable.Cpu().MilliValue()
		nodeMEMValue := nodeStatus.Allocatable.Memory().Value()
		nlog.Debugf("Node %s ncv is %d nmv is %d tcr is %d tmr is %d", nodeStatus.Name, nodeCPUValue, nodeMEMValue,
			nodeStatus.TotalCPURequest, nodeStatus.TotalMemRequest)
		if (nodeCPUValue-nodeStatus.TotalCPURequest) > requestReq.CpuReq &&
			(nodeMEMValue-nodeStatus.TotalMemRequest) > requestReq.MemReq {
			nlog.Debugf("Domain %s node %s available resource", domainName, nodeStatus.Name)
			return true, nil
		}
	}
	return false, fmt.Errorf("resource-check no node status available for kusciatask %s", partyKitInfo.KusciaTask.Name)
}

func (p *ResourceCheckPlugin) resourceRequest(partyKitInfo kusciatask.PartyKitInfo) ResourceRequest {
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

	return ResourceRequest{
		DomainName: partyKitInfo.DomainID,
		CpuReq:     allContainerCPURequest,
		MemReq:     allContainerMEMRequest,
	}
}
