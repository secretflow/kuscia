/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Modified by Ant Group in 2023.

package kuberuntime

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	containertest "github.com/secretflow/kuscia/pkg/agent/container/testing"
)

// TestRemoveContainer tests removing the container and its corresponding container logs.
func TestRemoveContainer(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "bar",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "foo",
					Image:           "busybox",
					ImagePullPolicy: v1.PullIfNotPresent,
				},
			},
		},
	}

	// Create fake sandbox and container
	_, fakeContainers := makeAndSetFakePod(t, m, fakeRuntime, pod)
	assert.Equal(t, len(fakeContainers), 1)

	containerID := fakeContainers[0].Id
	fakeOS := m.osInterface.(*containertest.FakeOS)
	fakeOS.GlobFn = func(pattern, path string) bool {
		pattern = strings.Replace(pattern, "*", ".*", -1)
		return regexp.MustCompile(pattern).MatchString(path)
	}
	expectedContainerLogPath := filepath.Join(m.podStdoutRootDirectory, "new_bar_12345678", "foo", "0.log")
	expectedContainerLogPathRotated := filepath.Join(m.podStdoutRootDirectory, "new_bar_12345678", "foo", "0.log.20060102-150405")

	fakeOS.Create(expectedContainerLogPath)
	fakeOS.Create(expectedContainerLogPathRotated)

	ctx := context.Background()
	err = m.removeContainer(ctx, containerID)
	assert.NoError(t, err)

	// Verify container is removed
	assert.Contains(t, fakeRuntime.Called, "RemoveContainer")
	containers, err := fakeRuntime.ListContainers(ctx, &runtimeapi.ContainerFilter{Id: containerID})
	assert.NoError(t, err)
	assert.Empty(t, containers)
}

// TestKillContainer tests killing the container in a Pod.
func TestKillContainer(t *testing.T) {
	_, _, m, _ := createTestRuntimeManager()

	tests := []struct {
		caseName            string
		pod                 *v1.Pod
		containerID         pkgcontainer.CtrID
		containerName       string
		reason              string
		gracePeriodOverride int64
		succeed             bool
	}{
		{
			caseName: "Failed to find container in pods, expect to return error",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{UID: "pod1_id", Name: "pod1", Namespace: "default"},
				Spec:       v1.PodSpec{Containers: []v1.Container{{Name: "empty_container"}}},
			},
			containerID:         pkgcontainer.CtrID{Type: "docker", ID: "not_exist_container_id"},
			containerName:       "not_exist_container",
			reason:              "unknown reason",
			gracePeriodOverride: 0,
			succeed:             false,
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		err := m.killContainer(ctx, test.pod, test.containerID, test.containerName, test.reason, "", &test.gracePeriodOverride)
		if test.succeed != (err == nil) {
			t.Errorf("%s: expected %v, got %v (%v)", test.caseName, test.succeed, err == nil, err)
		}
	}
}

// TestTopkgcontainerStatus tests the converting the CRI container status to
// the internal type (i.e., topkgcontainerStatus()) for containers in
// different states.
func TestTopkgcontainerStatus(t *testing.T) {
	cid := &pkgcontainer.CtrID{Type: "testRuntime", ID: "dummyid"}
	meta := &runtimeapi.ContainerMetadata{Name: "cname", Attempt: 3}
	imageSpec := &runtimeapi.ImageSpec{Image: "fimage"}
	var (
		createdAt  int64 = 327
		startedAt  int64 = 999
		finishedAt int64 = 1278
	)

	for desc, test := range map[string]struct {
		input    *runtimeapi.ContainerStatus
		expected *pkgcontainer.Status
	}{
		"created container": {
			input: &runtimeapi.ContainerStatus{
				Id:        cid.ID,
				Metadata:  meta,
				Image:     imageSpec,
				State:     runtimeapi.ContainerState_CONTAINER_CREATED,
				CreatedAt: createdAt,
			},
			expected: &pkgcontainer.Status{
				ID:        *cid,
				Image:     imageSpec.Image,
				State:     pkgcontainer.ContainerStateCreated,
				CreatedAt: time.Unix(0, createdAt),
			},
		},
		"running container": {
			input: &runtimeapi.ContainerStatus{
				Id:        cid.ID,
				Metadata:  meta,
				Image:     imageSpec,
				State:     runtimeapi.ContainerState_CONTAINER_RUNNING,
				CreatedAt: createdAt,
				StartedAt: startedAt,
			},
			expected: &pkgcontainer.Status{
				ID:        *cid,
				Image:     imageSpec.Image,
				State:     pkgcontainer.ContainerStateRunning,
				CreatedAt: time.Unix(0, createdAt),
				StartedAt: time.Unix(0, startedAt),
			},
		},
		"exited container": {
			input: &runtimeapi.ContainerStatus{
				Id:         cid.ID,
				Metadata:   meta,
				Image:      imageSpec,
				State:      runtimeapi.ContainerState_CONTAINER_EXITED,
				CreatedAt:  createdAt,
				StartedAt:  startedAt,
				FinishedAt: finishedAt,
				ExitCode:   int32(121),
				Reason:     "GotKilled",
				Message:    "The container was killed",
			},
			expected: &pkgcontainer.Status{
				ID:         *cid,
				Image:      imageSpec.Image,
				State:      pkgcontainer.ContainerStateExited,
				CreatedAt:  time.Unix(0, createdAt),
				StartedAt:  time.Unix(0, startedAt),
				FinishedAt: time.Unix(0, finishedAt),
				ExitCode:   121,
				Reason:     "GotKilled",
				Message:    "The container was killed",
			},
		},
		"unknown container": {
			input: &runtimeapi.ContainerStatus{
				Id:        cid.ID,
				Metadata:  meta,
				Image:     imageSpec,
				State:     runtimeapi.ContainerState_CONTAINER_UNKNOWN,
				CreatedAt: createdAt,
				StartedAt: startedAt,
			},
			expected: &pkgcontainer.Status{
				ID:        *cid,
				Image:     imageSpec.Image,
				State:     pkgcontainer.ContainerStateUnknown,
				CreatedAt: time.Unix(0, createdAt),
				StartedAt: time.Unix(0, startedAt),
			},
		},
	} {
		actual := topkgcontainerStatus(test.input, cid.Type)
		assert.Equal(t, test.expected, actual, desc)
	}
}

func TestRestartCountByLogDir(t *testing.T) {
	for _, tc := range []struct {
		filenames    []string
		restartCount int
	}{
		{
			filenames:    []string{"0.log.rotated-log"},
			restartCount: 1,
		},
		{
			filenames:    []string{"0.log"},
			restartCount: 1,
		},
		{
			filenames:    []string{"0.log", "1.log", "2.log"},
			restartCount: 3,
		},
		{
			filenames:    []string{"0.log.rotated", "1.log", "2.log"},
			restartCount: 3,
		},
		{
			filenames:    []string{"5.log.rotated", "6.log.rotated"},
			restartCount: 7,
		},
		{
			filenames:    []string{"5.log.rotated", "6.log", "7.log"},
			restartCount: 8,
		},
	} {
		tempDirPath, err := os.MkdirTemp("", "test-restart-count-")
		assert.NoError(t, err, "create tempdir error")
		defer os.RemoveAll(tempDirPath)
		for _, filename := range tc.filenames {
			err = os.WriteFile(filepath.Join(tempDirPath, filename), []byte("a log line"), 0600)
			assert.NoError(t, err, "could not write log file")
		}
		count, _ := CalcRestartCountByLogDir(tempDirPath)
		assert.Equal(t, count, tc.restartCount, "count %v should equal restartCount %v", count, tc.restartCount)
	}
}

func makeExpectedConfig(m *kubeGenericRuntimeManager, pod *v1.Pod, containerIndex int, enforceMemoryQoS bool) *runtimeapi.ContainerConfig {
	container := &pod.Spec.Containers[containerIndex]
	podIP := ""
	restartCount := 0
	opts, _, _ := m.runtimeHelper.GenerateRunContainerOptions(pod, container, podIP, []string{podIP})
	containerLogsPath := BuildContainerLogsPath(container.Name, restartCount)
	restartCountUint32 := uint32(restartCount)
	envs := make([]*runtimeapi.KeyValue, len(opts.Envs))

	expectedConfig := &runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{
			Name:    container.Name,
			Attempt: restartCountUint32,
		},
		Image:       &runtimeapi.ImageSpec{Image: container.Image},
		Command:     container.Command,
		Args:        []string(nil),
		WorkingDir:  container.WorkingDir,
		Labels:      newContainerLabels(container, pod),
		Annotations: newContainerAnnotations(container, pod, restartCount, opts),
		Devices:     []*runtimeapi.Device(nil),
		Mounts:      m.makeMounts(opts, container),
		LogPath:     containerLogsPath,
		Stdin:       container.Stdin,
		StdinOnce:   container.StdinOnce,
		Tty:         container.TTY,
		Linux:       m.generateLinuxContainerConfig(container, pod),
		Envs:        envs,
	}
	return expectedConfig
}

func TestGenerateContainerConfig(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "bar",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "foo",
					Image:           "busybox",
					ImagePullPolicy: v1.PullIfNotPresent,
					Command:         []string{"testCommand"},
					WorkingDir:      "testWorkingDir",
				},
			},
		},
	}

	expectedConfig := makeExpectedConfig(m, pod, 0, false)
	containerConfig, _, err := m.generateContainerConfig(&pod.Spec.Containers[0], pod, 0, "", pod.Spec.Containers[0].Image, []string{})
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, containerConfig, "generate container config for kubelet runtime v1.")
}

func TestGenerateLinuxContainerConfigResources(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	m.cpuCFSQuota = true

	assert.NoError(t, err)

	tests := []struct {
		name         string
		podResources v1.ResourceRequirements
		expected     *runtimeapi.LinuxContainerResources
	}{
		{
			name: "Request 128M/1C, Limit 256M/3C",
			podResources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("128Mi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
				Limits: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("256Mi"),
					v1.ResourceCPU:    resource.MustParse("3"),
				},
			},
			expected: &runtimeapi.LinuxContainerResources{
				CpuPeriod:          100000,
				CpuQuota:           300000,
				CpuShares:          1024,
				MemoryLimitInBytes: 256 * 1024 * 1024,
			},
		},
		{
			name: "Request 128M/2C, No Limit",
			podResources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("128Mi"),
					v1.ResourceCPU:    resource.MustParse("2"),
				},
			},
			expected: &runtimeapi.LinuxContainerResources{
				CpuPeriod:          100000,
				CpuQuota:           0,
				CpuShares:          2048,
				MemoryLimitInBytes: 0,
			},
		},
	}

	for _, test := range tests {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "12345678",
				Name:      "bar",
				Namespace: "new",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:            "foo",
						Image:           "busybox",
						ImagePullPolicy: v1.PullIfNotPresent,
						Command:         []string{"testCommand"},
						WorkingDir:      "testWorkingDir",
						Resources:       test.podResources,
					},
				},
			},
		}

		linuxConfig := m.generateLinuxContainerConfig(&pod.Spec.Containers[0], pod)
		assert.Equal(t, test.expected.CpuPeriod, linuxConfig.GetResources().CpuPeriod, test.name)
		assert.Equal(t, test.expected.CpuQuota, linuxConfig.GetResources().CpuQuota, test.name)
		assert.Equal(t, test.expected.CpuShares, linuxConfig.GetResources().CpuShares, test.name)
		assert.Equal(t, test.expected.MemoryLimitInBytes, linuxConfig.GetResources().MemoryLimitInBytes, test.name)
	}
}

func TestCalculateLinuxResources(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	m.cpuCFSQuota = true

	assert.NoError(t, err)

	tests := []struct {
		name     string
		cpuReq   resource.Quantity
		cpuLim   resource.Quantity
		memLim   resource.Quantity
		expected *runtimeapi.LinuxContainerResources
	}{
		{
			name:   "Request128MBLimit256MB",
			cpuReq: resource.MustParse("1"),
			cpuLim: resource.MustParse("2"),
			memLim: resource.MustParse("128Mi"),
			expected: &runtimeapi.LinuxContainerResources{
				CpuPeriod:          100000,
				CpuQuota:           200000,
				CpuShares:          1024,
				MemoryLimitInBytes: 134217728,
			},
		},
		{
			name:   "RequestNoMemory",
			cpuReq: resource.MustParse("2"),
			cpuLim: resource.MustParse("8"),
			memLim: resource.MustParse("0"),
			expected: &runtimeapi.LinuxContainerResources{
				CpuPeriod:          100000,
				CpuQuota:           800000,
				CpuShares:          2048,
				MemoryLimitInBytes: 0,
			},
		},
	}
	for _, test := range tests {
		linuxContainerResources := m.calculateLinuxResources(&test.cpuReq, &test.cpuLim, &test.memLim)
		assert.Equal(t, test.expected, linuxContainerResources)
	}
}
