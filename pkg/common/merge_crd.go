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

package common

import (
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func MergeDomainData(originalDomainData, modifiedDomainData *v1alpha1.DomainData) (patchBytes, originalJSON, modifiedJSON []byte, err error) {
	originalJSON, err = json.Marshal(originalDomainData)
	if err != nil {
		return
	}
	modifiedJSON, err = json.Marshal(modifiedDomainData)
	if err != nil {
		return
	}
	patchBytes, err = strategicpatch.CreateTwoWayMergePatch(originalJSON, modifiedJSON, &v1alpha1.DomainData{})
	return
}
