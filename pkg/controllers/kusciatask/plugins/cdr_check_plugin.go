package plugins

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

type CDRCheckPlugin struct {
	cdrLister kuscialistersv1alpha1.ClusterDomainRouteLister
}

func NewCDRCheckPlugin(cdrLister kuscialistersv1alpha1.ClusterDomainRouteLister) *CDRCheckPlugin {
	return &CDRCheckPlugin{
		cdrLister: cdrLister,
	}
}

func (p *CDRCheckPlugin) Permit(ctx context.Context, params interface{}) (bool, error) {
	cdrs, _ := params.([]string)

	for _, cdr := range cdrs {
		cdrObj, err := p.cdrLister.Get(cdr)
		if err != nil {
			return false, fmt.Errorf("get cdr %s failed with %v", cdr, err)
		}

		parts := strings.Split(cdr, "-")
		for _, condition := range cdrObj.Status.Conditions {
			if condition.Type == kusciaapisv1alpha1.ClusterDomainRouteReady && condition.Status != v1.ConditionTrue {
				return false, fmt.Errorf("initiator %s to collaborator %s failed with %v", parts[0], parts[1], condition.Reason)
			}
		}
	}
	return true, nil
}

func (p *CDRCheckPlugin) Type() PluginType {
	return PluginTypeCDRCheck
}
