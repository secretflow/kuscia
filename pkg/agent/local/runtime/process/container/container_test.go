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

package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	storetesting "github.com/secretflow/kuscia/pkg/agent/local/store/testing"
)

func createTestContainer(t *testing.T) *Container {
	rootDir := t.TempDir()
	sandboxDir := filepath.Join(rootDir, "sandbox")
	assert.NoError(t, os.MkdirAll(sandboxDir, 0755))
	logDirectory := filepath.Join(rootDir, "logs")
	assert.NoError(t, os.MkdirAll(logDirectory, 0755))

	imageStore := storetesting.NewFakeStore()
	sandboxBundle, err := layout.NewBundle(sandboxDir)
	assert.NoError(t, err)

	c := &runtime.ContainerConfig{
		Metadata: &runtime.ContainerMetadata{
			Name: "test-name",
		},
		Image: &runtime.ImageSpec{
			Image: "test-image:0.1",
		},
		Command: []string{"echo"},
		Args:    []string{"hello"},
		LogPath: "0.log",
	}

	container, err := NewContainer(c, logDirectory, "test-sandbox-id", sandboxBundle, imageStore)
	assert.NoError(t, err)
	return container
}

func TestContainerStart(t *testing.T) {
	t.Run("Container normal exited ", func(t *testing.T) {
		container := createTestContainer(t)
		assert.NoError(t, container.Create(kii.Plain))
		assert.NoError(t, container.Start())

		time.Sleep(time.Second)

		status := container.GetStatus()
		assert.Equal(t, runtime.ContainerState_CONTAINER_EXITED, status.State())
		assert.NoError(t, container.Release())
	})

	t.Run("Container killed by signal", func(t *testing.T) {
		container := createTestContainer(t)
		container.Config.Command = []string{"sleep"}
		container.Config.Args = []string{"60"}
		assert.NoError(t, container.Create(kii.Plain))
		assert.NoError(t, container.Start())
		time.Sleep(100 * time.Millisecond)
		assert.NoError(t, container.Stop())
		assert.NoError(t, wait.PollImmediate(100*time.Millisecond, 1*time.Second, func() (done bool, err error) {
			return container.GetCRIStatus().State == runtime.ContainerState_CONTAINER_EXITED, nil
		}))
		assert.NoError(t, container.Release())
	})
}

func TestContainerGenerateCmdLine(t *testing.T) {
	tests := []struct {
		ImageEntrypoint []string
		ImageCommand    []string
		Command         []string
		Args            []string
		ExpectedCmd     string
	}{
		{
			ImageEntrypoint: []string{"aa"},
			ImageCommand:    []string{"bb"},
			Command:         []string{"cc"},
			Args:            []string{"dd"},
			ExpectedCmd:     "cc dd",
		},
		{
			ImageEntrypoint: []string{"aa"},
			ImageCommand:    []string{"bb"},
			Command:         []string{},
			Args:            []string{"dd"},
			ExpectedCmd:     "aa dd",
		},
		{
			ImageEntrypoint: []string{"aa"},
			ImageCommand:    []string{"bb"},
			Command:         []string{"cc"},
			Args:            []string{},
			ExpectedCmd:     "cc",
		},
		{
			ImageEntrypoint: []string{"aa"},
			ImageCommand:    []string{"bb"},
			Command:         []string{},
			Args:            []string{},
			ExpectedCmd:     "aa bb",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("No.%d", i), func(t *testing.T) {
			c := &Container{
				Metadata: Metadata{
					Config: &runtime.ContainerConfig{
						Command: tt.Command,
						Args:    tt.Args,
					},
					ImageManifest: &kii.Manifest{
						Config: oci.ImageConfig{
							Entrypoint: tt.ImageEntrypoint,
							Cmd:        tt.ImageCommand,
						},
					},
				},
			}

			cmdLine := c.generateOriginalCmdLine()
			assert.Equal(t, tt.ExpectedCmd, strings.Join(cmdLine, " "))
		})
	}
}

func TestAddCgroup(t *testing.T) {
	container := createTestContainer(t)
	container.addCgroup(0)
}

func TestDeleteCgroup(t *testing.T) {
	container := createTestContainer(t)
	container.deleteCgroup(0)
}
