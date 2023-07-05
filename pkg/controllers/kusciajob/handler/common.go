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

package handler

import (
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func stringFilter(collection []string, predicate func(item string, index int) bool) []string {
	var result []string
	for i, item := range collection {
		if predicate(item, i) {
			result = append(result, item)
		}
	}
	return result
}

func kusciaTaskTemplateFilter(collection []kusciaapisv1alpha1.KusciaTaskTemplate,
	predicate func(item kusciaapisv1alpha1.KusciaTaskTemplate, index int) bool) []kusciaapisv1alpha1.KusciaTaskTemplate {
	var result []kusciaapisv1alpha1.KusciaTaskTemplate
	for i, item := range collection {
		if predicate(item, i) {
			result = append(result, item)
		}
	}
	return result
}

func stringAllMatch(collection []string, predicate func(item string) bool) bool {
	for _, v := range collection {
		if !predicate(v) {
			return false
		}
	}

	return true
}
