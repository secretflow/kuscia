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
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func TestRanderObject(t *testing.T) {
	workDir := GetWorkDir()
	tmpPath := filepath.Join(workDir, "etc/conf/domain-namespace-res.yaml")
	role := &rbacv1.Role{}
	input := struct {
		DomainID string
	}{
		DomainID: "alice",
	}
	err := RenderRuntimeObject(tmpPath, role, input)
	assert.NilError(t, err)
}

func GetWorkDir() string {
	_, filename, _, _ := runtime.Caller(0)
	nlog.Infof("path is %s", filename)
	return strings.SplitN(filename, "pkg", 2)[0]
}
