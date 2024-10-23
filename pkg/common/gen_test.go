// Copyright 2024 Ant Group Co., Ltd.
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
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestGenDomainDataID(t *testing.T) {

	testCases := []struct { //nolint:typecheck
		name  string
		match bool
	}{
		{
			name:  "example.com",
			match: true,
		},
		{
			name:  "-example-com",
			match: true,
		},
		{
			name:  "TestExample.DomainDataSource",
			match: true,
		},
		{
			name:  "",
			match: true,
		}, {
			name:  "Test$%^&*(data_source-",
			match: true,
		},
		{
			name:  "中文测试",
			match: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			genDomainDataID := GenDomainDataID(tc.name)
			match, _ := regexp.MatchString(K3sRegex, genDomainDataID)
			assert.True(t, match, "domain data id should be valid: %s", genDomainDataID)
		})
	}
}

func TestGenDomainDataSourceID(t *testing.T) {

	testCases := []struct { //nolint:typecheck
		name  string
		match bool
	}{
		{
			name:  DomainDataSourceTypeOSS,
			match: true,
		},
		{
			name:  DomainDataSourceTypeMysql,
			match: true,
		},
		{
			name:  DomainDataSourceTypeLocalFS,
			match: true,
		},
		{
			name:  "",
			match: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			genDomainDataID := GenDomainDataSourceID(tc.name)
			match, _ := regexp.MatchString(K3sRegex, genDomainDataID)
			assert.True(t, match, "domain data source id should be valid: %s", genDomainDataID)
		})
	}
}
