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
	"errors"
	"fmt"
	"regexp"
	"strconv"

	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

const k3sRegex = `^[a-z0-9]([a-z0-9.-]{0,61}[a-z0-9])?$`

// GetMasterDomain is used to get master domain id.
func GetMasterDomain(domainLister kuscialistersv1alpha1.DomainLister, domainID string) (string, error) {
	domain, err := domainLister.Get(domainID)
	if err != nil {
		return "", err
	}

	masterDomainID := domainID
	if domain.Spec.MasterDomain != "" {
		masterDomainID = domain.Spec.MasterDomain
	}

	return masterDomainID, nil
}

// CompareResourceVersion is used to compare resource version.
func CompareResourceVersion(rv1, rv2 string) bool {
	irv1, _ := strconv.Atoi(rv1)
	irv2, _ := strconv.Atoi(rv2)
	return irv1 > irv2
}

// SelfClusterAsInitiator checks if self cluster domain is scheduling party.
func SelfClusterAsInitiator(nsLister corelisters.NamespaceLister, domainID string, annotations map[string]string) bool {
	if annotations != nil {
		switch annotations[common.SelfClusterAsInitiatorAnnotationKey] {
		case common.True:
			return true
		case common.False:
			return false
		default:

		}
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

// ValidateK8sName checks dns subdomain names
func ValidateK8sName(val string, feildName string) error {

	match, _ := regexp.MatchString(k3sRegex, val)
	if !match {
		errorMsg := fmt.Sprintf("Field '%s' is invalid, Invalid value: '%s': regex used for validation is '%s' ", feildName, val, k3sRegex)
		return errors.New(errorMsg)
	}

	return nil
}

// IsPartnerDomain check if is partner domain.
func IsPartnerDomain(nsLister corelisters.NamespaceLister, domainID string) bool {
	ns, err := nsLister.Get(domainID)
	if err != nil {
		return false
	}

	if ns.Labels != nil &&
		ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) {
		return true
	}

	return false
}
