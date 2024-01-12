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

package empty

import (
	"context"
	"time"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Runtime struct {
}

func (r *Runtime) Version(ctx context.Context, apiVersion string) (*runtimeapi.VersionResponse, error) {
	return nil, nil
}

func (r *Runtime) CreateContainer(ctx context.Context, podSandboxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	return "", nil
}

func (r *Runtime) StartContainer(ctx context.Context, containerID string) error {
	return nil
}

func (r *Runtime) StopContainer(ctx context.Context, containerID string, timeout int64) error {
	return nil
}

func (r *Runtime) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}

func (r *Runtime) ListContainers(ctx context.Context, filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	return nil, nil
}

func (r *Runtime) ContainerStatus(ctx context.Context, containerID string, verbose bool) (*runtimeapi.ContainerStatusResponse, error) {
	return nil, nil
}

func (r *Runtime) UpdateContainerResources(ctx context.Context, containerID string, resources *runtimeapi.ContainerResources) error {
	return nil
}

func (r *Runtime) ExecSync(ctx context.Context, containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
	return nil, nil, nil
}

func (r *Runtime) Exec(ctx context.Context, request *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	return nil, nil
}

func (r *Runtime) Attach(ctx context.Context, req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	return nil, nil
}

func (r *Runtime) ReopenContainerLog(ctx context.Context, ContainerID string) error {
	return nil
}

func (r *Runtime) CheckpointContainer(ctx context.Context, options *runtimeapi.CheckpointContainerRequest) error {
	return nil
}

func (r *Runtime) GetContainerEvents(containerEventsCh chan *runtimeapi.ContainerEventResponse) error {
	return nil
}

func (r *Runtime) RunPodSandbox(ctx context.Context, config *runtimeapi.PodSandboxConfig, runtimeHandler string) (string, error) {
	return "", nil
}

func (r *Runtime) StopPodSandbox(pctx context.Context, odSandboxID string) error {
	return nil
}

func (r *Runtime) RemovePodSandbox(ctx context.Context, podSandboxID string) error {
	return nil
}

func (r *Runtime) PodSandboxStatus(ctx context.Context, podSandboxID string, verbose bool) (*runtimeapi.PodSandboxStatusResponse, error) {
	return nil, nil
}

func (r *Runtime) ListPodSandbox(ctx context.Context, filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	return nil, nil
}

func (r *Runtime) PortForward(ctx context.Context, request *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	return nil, nil
}

func (r *Runtime) ContainerStats(ctx context.Context, containerID string) (*runtimeapi.ContainerStats, error) {
	return nil, nil
}

func (r *Runtime) ListContainerStats(ctx context.Context, filter *runtimeapi.ContainerStatsFilter) ([]*runtimeapi.ContainerStats, error) {
	return nil, nil
}

func (r *Runtime) PodSandboxStats(ctx context.Context, podSandboxID string) (*runtimeapi.PodSandboxStats, error) {
	return nil, nil
}

func (r *Runtime) ListPodSandboxStats(ctx context.Context, filter *runtimeapi.PodSandboxStatsFilter) ([]*runtimeapi.PodSandboxStats, error) {
	return nil, nil
}

func (r *Runtime) ListMetricDescriptors(ctx context.Context) ([]*runtimeapi.MetricDescriptor, error) {
	return nil, nil
}

func (r *Runtime) ListPodSandboxMetrics(ctx context.Context) ([]*runtimeapi.PodSandboxMetrics, error) {
	return nil, nil
}

func (r *Runtime) UpdateRuntimeConfig(ctx context.Context, runtimeConfig *runtimeapi.RuntimeConfig) error {
	return nil
}

func (r *Runtime) Status(ctx context.Context, verbose bool) (*runtimeapi.StatusResponse, error) {
	return nil, nil
}

func (r *Runtime) ListImages(ctx context.Context, filter *runtimeapi.ImageFilter) ([]*runtimeapi.Image, error) {
	return nil, nil
}

func (r *Runtime) ImageStatus(ctx context.Context, image *runtimeapi.ImageSpec, verbose bool) (*runtimeapi.ImageStatusResponse, error) {
	return nil, nil
}

func (r *Runtime) PullImage(ctx context.Context, image *runtimeapi.ImageSpec, auth *runtimeapi.AuthConfig, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	return "", nil
}

func (r *Runtime) RemoveImage(ctx context.Context, image *runtimeapi.ImageSpec) error {
	return nil
}

func (r *Runtime) ImageFsInfo(ctx context.Context) ([]*runtimeapi.FilesystemUsage, error) {
	return nil, nil
}
