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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func GetInterConnParties(annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	res := make(map[string]string, 0)
	getInterConnPartiesByAnnotations(annotations, common.InterConnBFIAPartyAnnotationKey, res)
	getInterConnPartiesByAnnotations(annotations, common.InterConnKusciaPartyAnnotationKey, res)
	return res
}

func GetInterConnProtocolTypeByPartyAnnotation(key string) kusciaapisv1alpha1.InterConnProtocolType {
	if key == common.InterConnBFIAPartyAnnotationKey {
		return kusciaapisv1alpha1.InterConnBFIA
	}
	return kusciaapisv1alpha1.InterConnKuscia
}

func getInterConnPartiesByAnnotations(annotations map[string]string, key string, res map[string]string) {
	if v, ok := annotations[key]; ok && len(v) != 0 {
		domains := strings.Split(v, "_")
		for _, domain := range domains {
			res[domain] = key
		}
	}
}
func IsBFIAResource(obj metav1.Object) bool {
	if obj.GetAnnotations() != nil {
		if val, ok := obj.GetAnnotations()[common.InterConnBFIAPartyAnnotationKey]; ok && val != "" {
			return true
		}
	}
	if obj.GetLabels() != nil {
		if interConnType, ok := obj.GetLabels()[common.LabelInterConnProtocolType]; ok && interConnType == string(kusciaapisv1alpha1.InterConnBFIA) {
			return true
		}
	}
	return false
}
