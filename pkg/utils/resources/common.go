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

package resources

import (
	"strconv"

	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

// CompareResourceVersion is used to compare resource version.
func CompareResourceVersion(rv1, rv2 string) bool {
	irv1, _ := strconv.Atoi(rv1)
	irv2, _ := strconv.Atoi(rv2)
	return irv1 > irv2
}

// SelfClusterAsInitiator checks if self cluster domain is scheduling party.
func SelfClusterAsInitiator(nsLister corelisters.NamespaceLister, domainID string, labels map[string]string) bool {
	if labels != nil && labels[common.LabelSelfClusterAsInitiator] == "true" {
		return true
	}

	ns, err := nsLister.Get(domainID)
	if err != nil {
		return false
	}

	if ns.Labels == nil {
		return true
	}

	if ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) {
		return false
	}
	return true
}

// IsOuterBFIAInterConnDomain checks if outer domain with BFIA protocol.
func IsOuterBFIAInterConnDomain(nsLister corelisters.NamespaceLister, domainID string) bool {
	ns, err := nsLister.Get(domainID)
	if err != nil {
		return false
	}

	if ns.Labels != nil &&
		ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) &&
		ns.Labels[common.LabelInterConnProtocols] == string(kusciaapisv1alpha1.InterConnBFIA) {
		return true
	}

	return false
}
