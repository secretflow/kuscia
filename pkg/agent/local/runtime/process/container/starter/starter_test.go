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

package starter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/secretflow/kuscia/pkg/agent/utils/logutils"
	"github.com/stretchr/testify/assert"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func createTestInitConfig(t *testing.T) *InitConfig {
	rootDir := t.TempDir()
	rootfs := filepath.Join(rootDir, "rootfs")
	assert.NoError(t, os.MkdirAll(rootfs, 0755))
	srcDir := filepath.Join(rootDir, "src")
	assert.NoError(t, os.MkdirAll(srcDir, 0755))
	dstDir := "/dst"
	srcFile := filepath.Join(rootDir, "src.txt")
	_, err := os.Create(srcFile)
	assert.NoError(t, err)
	dstFile := "/dst.txt"

	c := &InitConfig{
		CmdLine: []string{"ls"},
		Env:     []string{"HOST_NAME=test-host"},
		ContainerConfig: &runtime.ContainerConfig{
			WorkingDir: "/",
			Mounts: []*runtime.Mount{
				{
					ContainerPath: dstDir,
					HostPath:      srcDir,
				},
				{
					ContainerPath: dstFile,
					HostPath:      srcFile,
				},
			},
		},
		Rootfs:  rootDir,
		LogFile: logutils.NewReopenableLogger(filepath.Join(rootDir, "test.log")),
	}

	return c
}
