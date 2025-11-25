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

package process

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubelet/pkg/types"

	ctr "github.com/secretflow/kuscia/pkg/agent/local/runtime/process/container"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/sandbox"
	storetest "github.com/secretflow/kuscia/pkg/agent/local/store/testing"
)

func Test_RuntimeVersion(t *testing.T) {
	rootDir := t.TempDir()
	sandboxDir := filepath.Join(rootDir, "sandbox")
	imageDir := filepath.Join(rootDir, "image")

	runtime, err := NewRuntime(&RuntimeDependence{
		HostIP:         "127.0.0.1",
		SandboxRootDir: sandboxDir,
		ImageRootDir:   imageDir,
	})
	assert.NoError(t, err)
	_, err = runtime.Version(context.Background(), "v1")
	assert.NoError(t, err)
}

func Test_RuntimeSandboxAndContainers(t *testing.T) {
	rootDir := t.TempDir()
	imageStore := storetest.NewFakeStore()
	ctx := context.Background()

	sandboxStore := sandbox.NewStore(rootDir)
	containerStore := ctr.NewStore()

	runtime := &Runtime{
		sandboxStore:   sandboxStore,
		containerStore: containerStore,
		imageStore:     imageStore,
		hostIP:         "127.0.0.1",
		sandboxRootDir: rootDir,
	}

	imageStatus, err := runtime.ImageStatus(ctx, &runtimeapi.ImageSpec{Image: "test:01"}, false)
	assert.NoError(t, err)
	assert.Equal(t, imageStatus.Image.Id, "test:01")

	sandboxID, err := runtime.RunPodSandbox(ctx, &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{
			Name:      "test-name",
			Namespace: "test-ns",
			Uid:       "test-uid",
		},
		Labels: map[string]string{
			types.KubernetesPodUIDLabel: "test-uid",
		},
		LogDirectory: filepath.Join(rootDir, "logs"),
	}, "")
	assert.NoError(t, err)

	sandboxStatus, err := runtime.PodSandboxStatus(ctx, sandboxID, false)
	assert.NoError(t, err)
	assert.Equal(t, runtimeapi.PodSandboxState_SANDBOX_READY, sandboxStatus.Status.State)

	filter := &runtimeapi.PodSandboxFilter{
		State: &runtimeapi.PodSandboxStateValue{State: runtimeapi.PodSandboxState_SANDBOX_READY},
	}
	sandboxList, err := runtime.ListPodSandbox(ctx, filter)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(sandboxList))
	assert.Equal(t, sandboxID, sandboxList[0].Id)

	filter = &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{types.KubernetesPodUIDLabel: "test-uid"},
	}
	sandboxList, err = runtime.ListPodSandbox(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(sandboxList))
	assert.Equal(t, sandboxID, sandboxList[0].Id)

	containerID, err := runtime.CreateContainer(context.Background(), sandboxID, &runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{
			Name:    "test-container",
			Attempt: 0,
		},
		Image: &runtimeapi.ImageSpec{
			Image: "docker.io/secretflow/secretflow:0.1",
		},
		LogPath: "0.log",
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", "sleep 10"},
	}, nil)
	assert.NoError(t, err)

	assert.NoError(t, runtime.StartContainer(ctx, containerID))

	containerList, err := runtime.ListContainers(context.Background(), &runtimeapi.ContainerFilter{
		PodSandboxId: sandboxID,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(containerList))
	assert.Equal(t, containerID, containerList[0].Id)

	containerStatus, err := runtime.ContainerStatus(ctx, containerID, false)
	assert.NoError(t, err)
	assert.Equal(t, runtimeapi.ContainerState_CONTAINER_RUNNING, containerStatus.Status.State)

	assert.NoError(t, runtime.StopContainer(ctx, containerID, 1)) // Use 1 second timeout for graceful shutdown

	// make sure process killed
	time.Sleep(100 * time.Millisecond)

	assert.NoError(t, runtime.StopPodSandbox(ctx, sandboxID))

	time.Sleep(100 * time.Millisecond)

	assert.NoError(t, runtime.RemovePodSandbox(ctx, sandboxID))

	containerList, err = runtime.ListContainers(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(containerList))

	sandboxList, err = runtime.ListPodSandbox(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(sandboxList))
}
func TestGetGracePeriod(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    int64
	}{
		{
			name:        "no annotations",
			annotations: nil,
			expected:    minimumGracePeriodInSeconds,
		},
		{
			name: "pod deletion grace period",
			annotations: map[string]string{
				podDeletionGracePeriodLabel: "30",
			},
			expected: 30,
		},
		{
			name: "pod termination grace period",
			annotations: map[string]string{
				podTerminationGracePeriodLabel: "20",
			},
			expected: 20,
		},
		{
			name: "both annotations, prefer deletion grace period",
			annotations: map[string]string{
				podDeletionGracePeriodLabel:    "30",
				podTerminationGracePeriodLabel: "20",
			},
			expected: 30,
		},
		{
			name: "invalid annotation value",
			annotations: map[string]string{
				podDeletionGracePeriodLabel: "invalid",
			},
			expected: minimumGracePeriodInSeconds,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getGracePeriod(tt.annotations)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestGetInt64PointerFromLabel(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		label    string
		expected *int64
		err      bool
	}{
		{
			name:     "label not found",
			labels:   map[string]string{},
			label:    "test",
			expected: nil,
			err:      false,
		},
		{
			name: "valid int64 value",
			labels: map[string]string{
				"test": "123",
			},
			label:    "test",
			expected: int64Ptr(123),
			err:      false,
		},
		{
			name: "invalid int64 value",
			labels: map[string]string{
				"test": "abc",
			},
			label:    "test",
			expected: nil,
			err:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getInt64PointerFromLabel(tt.labels, tt.label)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
