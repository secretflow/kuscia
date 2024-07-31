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
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
}

func GetWorkDir() string {
	_, filename, _, _ := runtime.Caller(0)
	nlog.Infof("path is %s", filename)
	return strings.SplitN(filename, "pkg", 2)[0]
}

func TestQueryByFields(t *testing.T) {
	t.Parallel()

	v1str := `{"v1":"xyz","v2":10,"v3":{"s1":"abc","s2":["x","y","z"]},"v4":[{"t1":"10"},{"t1":"20"}]}`

	var val interface{}
	assert.NoError(t, json.Unmarshal([]byte(v1str), &val))

	assert.Equal(t, "xyz", QueryByFields(val, ".v1"))
	assert.Equal(t, float64(10), QueryByFields(val, ".v2"))
	assert.Equal(t, "abc", QueryByFields(val, ".v3.s1"))
	assert.Equal(t, "x", QueryByFields(val, ".v3.s2[0]"))
	assert.Equal(t, "10", QueryByFields(val, ".v4[0].t1"))
	assert.Equal(t, "20", QueryByFields(val, ".v4[t1=20].t1"))

	assert.Nil(t, QueryByFields(val, ".v3.v4[t1=20].t1"))
}
