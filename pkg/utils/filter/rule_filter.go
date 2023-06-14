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

package filter

import (
	"regexp"

	"github.com/tidwall/match"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	permissionAllow = "allow"
	permissionDeny  = "deny"
)

type RuleConfig struct {
	Permission string   `yaml:"permission,omitempty"`
	Regex      bool     `yaml:"regex,omitempty"`
	Patterns   []string `yaml:"patterns,omitempty"`
}

// RuleFilter filter string according to rules. Return true if the 'allow' rule matches, false otherwise.
func RuleFilter(rules []RuleConfig, s string) (bool, error) {
	var matched bool
	var err error

	for i, rule := range rules {
		for _, pattern := range rule.Patterns {
			if rule.Regex {
				matched, err = regexp.MatchString(pattern, s)
				if err != nil {
					return false, err
				}
			} else {
				matched = match.Match(s, pattern)
			}

			if matched {
				nlog.Debugf("String %v Matched rule %v pattern %v", s, i, pattern)

				// if permission is deny then return false, else return true
				return rule.Permission != permissionDeny, nil
			}
		}

	}

	// default deny
	return false, nil
}
