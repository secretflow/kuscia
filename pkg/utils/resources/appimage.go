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
	"strings"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

// SelectDeployTemplate selects a matching template according to the role.
// value of templates[i].Role may have the following values: 'client', 'server', 'client,server', ...
// Matching process:
//  1. if role [in] template role list, matched.
//  2. if template role list is empty，template is universal.
//  3. if role is empty，select the first template.
func SelectDeployTemplate(templates []kusciaapisv1alpha1.DeployTemplate, role string) (*kusciaapisv1alpha1.DeployTemplate, error) {
	if len(templates) == 0 {
		return nil, errors.New("deploy templates are empty")
	}

	var defaultTemplate *kusciaapisv1alpha1.DeployTemplate
	for _, template := range templates {
		templateRoles := strings.Split(strings.Trim(template.Role, ","), ",")
		for _, tRole := range templateRoles {
			if tRole == role {
				return template.DeepCopy(), nil
			}
		}

		if template.Role == "" {
			defaultTemplate = template.DeepCopy()
		}
	}

	if defaultTemplate != nil {
		return defaultTemplate, nil
	}

	if role == "" {
		return templates[0].DeepCopy(), nil
	}

	return nil, fmt.Errorf("not found deploy template for role %q", role)
}
