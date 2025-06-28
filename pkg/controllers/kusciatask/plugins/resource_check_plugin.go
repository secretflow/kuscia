package plugins

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/controllers/domain"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ResourceRequest struct {
	DomainName string
	CpuReq int64
	MemReq int64
}

type ResourceCheckPlugin struct{}

func NewResourceCheckPlugin() *ResourceCheckPlugin {
	return &ResourceCheckPlugin{}
}

func (p *ResourceCheckPlugin) Permit(ctx context.Context, params interface{}) (bool, error) {
	var resourceRequest ResourceRequest
	var ok bool
	resourceRequest, ok = params.(ResourceRequest)
	if !ok {
		nlog.Errorf("Could not convert params %v to resourceRequest", params)
		return false, nil
	}

	domainName := resourceRequest.DomainName
	domain.NodeResourceManager.Lock.RLock()
	defer domain.NodeResourceManager.Lock.RUnlock()

	localNodeStatuses, exists := domain.NodeResourceManager.LocalNodeStatuses[domainName]
	if !exists {
		nlog.Warnf("domain %s not have node", domainName)
		return false, fmt.Errorf("no node status available for domain %s", domainName)
	}

	for _, nodeStatus := range localNodeStatuses {
		if nodeStatus.Status != domain.NodeStateReady {
			continue
		}

		nodeCPUValue := nodeStatus.Allocatable.Cpu().MilliValue()
		nodeMEMValue := nodeStatus.Allocatable.Memory().Value()
		nlog.Infof("Node %s ncv is %d nmv is %d tcr is %d tmr is %d", nodeStatus.Name, nodeCPUValue, nodeMEMValue,
			nodeStatus.TotalCPURequest, nodeStatus.TotalMemRequest)
		if (nodeCPUValue-nodeStatus.TotalCPURequest) > resourceRequest.CpuReq &&
			(nodeMEMValue-nodeStatus.TotalMemRequest) > resourceRequest.MemReq {
			nlog.Infof("Domain %s node %s available resource", domainName, nodeStatus.Name)
			return true, nil
		}
	}
	return false, nil
}

func (p *ResourceCheckPlugin) Type() PluginType {
	return PluginTypeResourceCheck
}
