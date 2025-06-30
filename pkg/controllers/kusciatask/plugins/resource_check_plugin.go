package plugins

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/controllers/domain"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ResourceCheckPlugin struct{}

func NewResourceCheckPlugin() *ResourceCheckPlugin {
	return &ResourceCheckPlugin{}
}

func (p *ResourceCheckPlugin) Permit(ctx context.Context, params interface{}) (bool, error) {
	var compositeRequest CompositeRequest
	var ok bool
	compositeRequest, ok = params.(CompositeRequest)
	if !ok {
		nlog.Errorf("Could not convert params %v to compositeRequest", params)
		return false, nil
	}

	domainName := compositeRequest.ResourceReq.DomainName
	domain.NodeResourceStore.Lock.RLock()
	defer domain.NodeResourceStore.Lock.RUnlock()

	localNodeStatuses, exists := domain.NodeResourceStore.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("domain %s not have node", domainName)
		return false, fmt.Errorf("no node status available for domain %s", domainName)
	}

	for _, nodeStatus := range localNodeStatuses {
		if nodeStatus.Status != domain.NodeStateReady {
			continue
		}

		nlog.Infof("nodeStatus is %v", nodeStatus)
		nodeCPUValue := nodeStatus.Allocatable.Cpu().MilliValue()
		nodeMEMValue := nodeStatus.Allocatable.Memory().Value()
		nlog.Infof("Node %s ncv is %d nmv is %d tcr is %d tmr is %d", nodeStatus.Name, nodeCPUValue, nodeMEMValue,
			nodeStatus.TotalCPURequest, nodeStatus.TotalMemRequest)
		if (nodeCPUValue-nodeStatus.TotalCPURequest) > compositeRequest.ResourceReq.CpuReq &&
			(nodeMEMValue-nodeStatus.TotalMemRequest) > compositeRequest.ResourceReq.MemReq {
			nlog.Infof("Domain %s node %s available resource", domainName, nodeStatus.Name)
			return true, nil
		}
	}
	return false, nil
}

func (p *ResourceCheckPlugin) Type() PluginType {
	return PluginTypeResourceCheck
}
