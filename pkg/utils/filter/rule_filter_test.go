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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleFilter(t *testing.T) {
	tests := []struct {
		rules []RuleConfig
		ss    []string
		ok    []bool
	}{
		{
			rules: []RuleConfig{
				{
					Permission: permissionAllow,
					Patterns: []string{
						"ab?de*",
						"*jk?mn",
						"o?q*t",
					},
				},
			},
			ss: []string{
				"abcdefg",
				"hijklmn",
				"opq rst",
				"uvw xyz",
			},
			ok: []bool{true, true, true, false},
		},
		{
			rules: []RuleConfig{
				{
					Permission: permissionDeny,
					Patterns: []string{
						"ab?de*",
						"*jk?mn",
						"o?q*t",
					},
				},
				{
					Permission: permissionAllow,
					Patterns: []string{
						"*",
					},
				},
			},
			ss: []string{
				"abcdefg",
				"hijklmn",
				"opq rst",
				"uvw xyz",
			},
			ok: []bool{false, false, false, true},
		},
		{
			rules: []RuleConfig{
				{
					Permission: permissionAllow,
					Regex:      true,
					Patterns: []string{
						"^dog$",
						"flo*r",
						"colou?r",
					},
				},
			},
			ss: []string{
				"dog",
				"floor",
				"color",
				"dogs",
				"flo=r",
				"colouur",
			},
			ok: []bool{true, true, true, false, false, false},
		},
	}

	for i, tt := range tests {
		assert.Equal(t, len(tt.ss), len(tt.ok))
		for j := 0; j < len(tt.ss); j++ {
			t.Logf("====Test No.%d.%d", i, j)

			ok, err := RuleFilter(tt.rules, tt.ss[j])
			assert.NoError(t, err)
			assert.Equal(t, tt.ok[j], ok)
		}
	}
}
