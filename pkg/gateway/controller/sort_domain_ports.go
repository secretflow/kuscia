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

package controller

import (
	"sort"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

var (
	domainPortProtocols = map[string]int{
		"GRPC": 0,
		"HTTP": 1,
	}
)

type DomainPorts struct {
	domainPorts []kusciaapisv1alpha1.DomainPort
	dic         map[string]int
}

func (dps DomainPorts) Len() int {
	return len(dps.domainPorts)
}

func (dps DomainPorts) Less(i, j int) bool {
	return dps.dic[string(dps.domainPorts[i].Protocol)] < dps.dic[string(dps.domainPorts[j].Protocol)]
}

func (dps DomainPorts) Swap(i, j int) {
	dps.domainPorts[i], dps.domainPorts[j] = dps.domainPorts[j], dps.domainPorts[i]
}

func sortDomainPorts(dps []kusciaapisv1alpha1.DomainPort) []kusciaapisv1alpha1.DomainPort {
	ports := DomainPorts{
		dic: domainPortProtocols,
	}
	ports.domainPorts = append(ports.domainPorts, dps...)

	sort.Sort(ports)
	return ports.domainPorts
}
