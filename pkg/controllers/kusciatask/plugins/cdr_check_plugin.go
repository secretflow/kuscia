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
package plugins

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/secretflow/kuscia/pkg/controllers/kusciatask/common"
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
	var partyKitInfo common.PartyKitInfo
	var ok bool
	partyKitInfo, ok = params.(common.PartyKitInfo)
	if !ok {
		return false, fmt.Errorf("cdr-check could not convert params %v to PartyKitInfo", params)
	}

	cdrResourceRequest := p.cdrResourceRequest(partyKitInfo)
	for _, cdr := range cdrResourceRequest {
		cdrObj, err := p.cdrLister.Get(cdr)
		if err != nil {
			return false, fmt.Errorf("cdr-check get cdr %s failed with %v", cdr, err)
		}

		parts := strings.Split(cdr, "-")
		for _, condition := range cdrObj.Status.Conditions {
			if condition.Type == kusciaapisv1alpha1.ClusterDomainRouteReady && condition.Status != v1.ConditionTrue {
				return false, fmt.Errorf("cdr-check initiator %s to collaborator %s failed with %v", parts[0], parts[1], condition.Reason)
			}
		}
	}
	return true, nil
}

func (p *CDRCheckPlugin) cdrResourceRequest(partyKitInfo common.PartyKitInfo) []string {
	var cdrs []string
	initiator := partyKitInfo.KusciaTask.Spec.Initiator

	for _, party := range partyKitInfo.KusciaTask.Spec.Parties {
		if party.DomainID == initiator {
			continue
		}

		cdrName := fmt.Sprintf("%s-%s", initiator, party.DomainID)
		cdrs = append(cdrs, cdrName)
	}
	return cdrs
}
