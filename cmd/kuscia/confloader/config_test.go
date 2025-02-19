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

package confloader

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/web/asserts"
)

const (
	overwriteRootDir      = "/home/test-kuscia"
	overwriteMasterConfig = "rootDir: " + overwriteRootDir
)

func Test_defaultMasterOverwrite(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantRootDir string
	}{
		{
			name:        "default kuscia config overwrite",
			content:     overwriteMasterConfig,
			wantRootDir: overwriteRootDir,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultMaster(common.DefaultKusciaHomePath())
			err := yaml.Unmarshal([]byte(tt.content), &got)
			_ = asserts.IsNil(err, "unmarshal yaml should success")
			if got.RootDir != tt.wantRootDir {
				t.Errorf("rootDir should be overwrite")
			}
		})
	}
}
