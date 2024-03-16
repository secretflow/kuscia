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
	"errors"
	"fmt"
	"os"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/local/runtime/empty"
	ctr "github.com/secretflow/kuscia/pkg/agent/local/runtime/process/container"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/errdefs"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/sandbox"
	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type RuntimeDependence struct {
	HostIP         string
	SandboxRootDir string
	ImageRootDir   string
}

type Runtime struct {
	empty.Runtime

	imageStore     store.Store
	sandboxStore   *sandbox.Store
	containerStore *ctr.Store
	hostIP         string

	sandboxRootDir string
	mountType      kii.MountType
}

func NewRuntime(dep *RuntimeDependence) (*Runtime, error) {
	imageStore, err := store.NewStore(dep.ImageRootDir)
	if err != nil {
		return nil, errors.New("failed to create image store")
	}

	if err := paths.EnsureDirectory(dep.SandboxRootDir, true); err != nil {
		return nil, err
	}
	sandboxStore := sandbox.NewStore(dep.SandboxRootDir)
	sandboxStore.Monitor()
	containerStore := ctr.NewStore()

	nlog.Info("Process runtime initialized")

	return &Runtime{
		imageStore:     imageStore,
		sandboxStore:   sandboxStore,
		containerStore: containerStore,
		hostIP:         dep.HostIP,
		sandboxRootDir: dep.SandboxRootDir,
		mountType:      kii.Plain,
	}, nil
}

func (r *Runtime) Version(ctx context.Context, apiVersion string) (*runtimeapi.VersionResponse, error) {
	return &runtimeapi.VersionResponse{
		Version:           "0.1.0",
		RuntimeName:       "runp",
		RuntimeVersion:    meta.AgentVersionString(),
		RuntimeApiVersion: meta.AgentVersionString(),
	}, nil
}

func (r *Runtime) CreateContainer(ctx context.Context, podSandboxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (id string, retErr error) {
	podSandbox, ok := r.sandboxStore.Get(podSandboxID)
	if !ok {
		return "", fmt.Errorf("not found sandbox id %q", podSandboxID)
	}

	container, err := ctr.NewContainer(config, podSandbox.Config.LogDirectory, podSandboxID, podSandbox.Bundle, r.imageStore)
	if err != nil {
		return "", fmt.Errorf("failed to create container in sandbox %q: %v", podSandboxID, err)
	}

	// Add container into container store.
	if err := r.containerStore.Add(container); err != nil {
		return "", fmt.Errorf("failed to add container %q into store: %w", id, err)
	}

	if err := container.Create(r.mountType); err != nil {
		return "", fmt.Errorf("failed to prepare container %q environment, detail-> %v", container.ID, err)
	}

	nlog.Infof("Create container %q in sandbox %q", container.SandboxID, podSandboxID)

	return container.ID, nil
}

func (r *Runtime) StartContainer(ctx context.Context, containerID string) error {
	nlog.Infof("Start container %q", containerID)

	container, err := r.containerStore.Get(containerID)
	if err != nil {
		return fmt.Errorf("failed to find container %q, detail-> %v", containerID, err)
	}

	if err := container.Start(); err != nil {
		return fmt.Errorf("failed to start container %q, detail-> %v", containerID, err)
	}

	return nil
}

func (r *Runtime) StopContainer(ctx context.Context, containerID string, timeout int64) error {
	nlog.Infof("Stop container %q", containerID)

	container, err := r.containerStore.Get(containerID)
	if err != nil {
		return fmt.Errorf("failed to find container %q, detail-> %v", containerID, err)
	}

	if err := container.Stop(); err != nil {
		return fmt.Errorf("failed to stop container %q, detail-> %v", containerID, err)
	}

	return nil
}

func (r *Runtime) RemoveContainer(ctx context.Context, containerID string) error {
	nlog.Infof("Remove container %q", containerID)

	container, err := r.containerStore.Get(containerID)
	if err != nil && !errors.Is(err, errdefs.ErrNotFound) {
		return fmt.Errorf("failed to find container %q, detail-> %v", containerID, err)
	}

	if err := container.Release(); err != nil {
		return fmt.Errorf("failed to release container %q, detail-> %v", containerID, err)
	}

	r.containerStore.Delete(containerID)

	return nil
}

func (r *Runtime) ListContainers(ctx context.Context, filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	// List all containers from store.
	containersInStore := r.containerStore.List()

	var containers []*runtimeapi.Container
	for _, container := range containersInStore {
		containers = append(containers, toCRIContainer(container))
	}

	containers = r.filterCRIContainers(containers, filter)

	return containers, nil
}

// toCRIContainer converts internal container object into CRI container.
func toCRIContainer(container *ctr.Container) *runtimeapi.Container {
	status := container.GetStatus()
	return &runtimeapi.Container{
		Id:           container.ID,
		PodSandboxId: container.SandboxID,
		Metadata:     container.Config.GetMetadata(),
		Image:        container.Config.GetImage(),
		ImageRef:     container.ImageManifest.ID,
		State:        status.State(),
		CreatedAt:    status.CreatedAt,
		Labels:       container.Config.GetLabels(),
		Annotations:  container.Config.GetAnnotations(),
	}
}

// filterCRIContainers filters CRIContainers.
func (r *Runtime) filterCRIContainers(containers []*runtimeapi.Container, filter *runtimeapi.ContainerFilter) []*runtimeapi.Container {
	if filter == nil {
		return containers
	}

	sb := filter.GetPodSandboxId()

	var filtered []*runtimeapi.Container
	for _, cntr := range containers {
		if filter.GetId() != "" && filter.GetId() != cntr.Id {
			continue
		}
		if sb != "" && sb != cntr.PodSandboxId {
			continue
		}
		if filter.GetState() != nil && filter.GetState().GetState() != cntr.State {
			continue
		}
		if filter.GetLabelSelector() != nil {
			match := true
			for k, v := range filter.GetLabelSelector() {
				got, ok := cntr.Labels[k]
				if !ok || got != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		filtered = append(filtered, cntr)
	}

	return filtered
}

func (r *Runtime) ContainerStatus(ctx context.Context, containerID string, verbose bool) (*runtimeapi.ContainerStatusResponse, error) {
	container, err := r.containerStore.Get(containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find container %q, detail-> %v", containerID, err)
	}

	status := container.GetCRIStatus()

	return &runtimeapi.ContainerStatusResponse{Status: status}, nil
}

func (r *Runtime) RunPodSandbox(ctx context.Context, config *runtimeapi.PodSandboxConfig, runtimeHandler string) (id string, retErr error) {
	podSandbox, err := sandbox.NewSandbox(config, r.hostIP, r.sandboxRootDir)
	if err != nil {
		return "", err
	}

	nlog.Infof("Run pod sandbox %v(%v)", podSandbox.ID, podSandbox.Name)

	if err := r.sandboxStore.Add(podSandbox); err != nil {
		return "", fmt.Errorf("failed to add sandbox %q to store, detail-> %v", podSandbox.ID, err)
	}

	return podSandbox.ID, nil
}

func (r *Runtime) StopPodSandbox(ctx context.Context, podSandboxID string) error {
	nlog.Infof("Stop pod sandbox %q", podSandboxID)

	containers := r.containerStore.List()
	for _, container := range containers {
		if container.SandboxID != podSandboxID {
			continue
		}

		if err := r.StopContainer(ctx, container.ID, 0); err != nil {
			return err
		}
	}

	podSandbox, ok := r.sandboxStore.Get(podSandboxID)
	if !ok {
		nlog.Warnf("Pod sandbox %q has been deleted", podSandboxID)
		return nil
	}
	podSandbox.Stop()

	return nil
}

func (r *Runtime) RemovePodSandbox(ctx context.Context, podSandboxID string) error {
	nlog.Infof("Remove pod sandbox %q", podSandboxID)

	if err := r.StopPodSandbox(ctx, podSandboxID); err != nil {
		return fmt.Errorf("failed to forcibly stop sandbox %q: %v", podSandboxID, err)
	}

	cntrs := r.containerStore.List()
	for _, cntr := range cntrs {
		if cntr.SandboxID != podSandboxID {
			continue
		}
		if err := r.RemoveContainer(ctx, cntr.ID); err != nil {
			return fmt.Errorf("failed to remove container %q: %w", cntr.ID, err)
		}
	}

	podSandbox, ok := r.sandboxStore.Get(podSandboxID)
	if !ok {
		return nil
	}

	if err := os.RemoveAll(podSandbox.Bundle.GetRootDirectory()); err != nil {
		return err
	}

	r.sandboxStore.Delete(podSandboxID)

	return nil
}

func (r *Runtime) PodSandboxStatus(ctx context.Context, podSandboxID string, verbose bool) (*runtimeapi.PodSandboxStatusResponse, error) {
	nlog.Infof("Get pod sandboxes %q status", podSandboxID)

	podSandbox, ok := r.sandboxStore.Get(podSandboxID)
	if !ok {
		return nil, fmt.Errorf("not found sandbox %q", podSandboxID)
	}

	return &runtimeapi.PodSandboxStatusResponse{
		Status: &runtimeapi.PodSandboxStatus{
			Id:       podSandbox.ID,
			Metadata: podSandbox.Config.Metadata,
			State:    podSandbox.State(),
			Network: &runtimeapi.PodSandboxNetworkStatus{
				Ip: podSandbox.IP,
			},
			CreatedAt:   podSandbox.CreatedAt.UnixMicro(),
			Labels:      podSandbox.Config.Labels,
			Annotations: podSandbox.Config.Annotations,
		},
	}, nil
}

func (r *Runtime) ListPodSandbox(ctx context.Context, filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	var retPodSandboxes []*runtimeapi.PodSandbox
	podSandboxes := r.sandboxStore.List()

	for _, ps := range podSandboxes {
		retPodSandboxes = append(retPodSandboxes, &runtimeapi.PodSandbox{
			Id:          ps.ID,
			Metadata:    ps.Config.Metadata,
			State:       ps.State(),
			CreatedAt:   ps.CreatedAt.UnixMicro(),
			Labels:      ps.Config.Labels,
			Annotations: ps.Config.Annotations,
		})
	}
	retPodSandboxes = r.filterCRISandboxes(retPodSandboxes, filter)

	return retPodSandboxes, nil
}

// filterCRISandboxes filters CRISandboxes.
func (r *Runtime) filterCRISandboxes(sandboxes []*runtimeapi.PodSandbox, filter *runtimeapi.PodSandboxFilter) []*runtimeapi.PodSandbox {
	if filter == nil {
		return sandboxes
	}

	var filtered []*runtimeapi.PodSandbox
	for _, s := range sandboxes {
		// Filter by id
		if filter.GetId() != "" && filter.GetId() != s.Id {
			continue
		}
		// Filter by state
		if filter.GetState() != nil && filter.GetState().GetState() != s.State {
			continue
		}
		// Filter by label
		if filter.GetLabelSelector() != nil {
			match := true
			for k, v := range filter.GetLabelSelector() {
				got, ok := s.Labels[k]
				if !ok || got != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	return filtered
}

func (r *Runtime) ImageStatus(ctx context.Context, image *runtimeapi.ImageSpec, verbose bool) (*runtimeapi.ImageStatusResponse, error) {
	nlog.Infof("Get image %q status", image.Image)

	imageManifest, err := r.getImageManifest(image.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to get image %q manifest, detail-> %v", image.Image, err)
	}

	resp := &runtimeapi.ImageStatusResponse{
		Image: &runtimeapi.Image{
			Id:       image.Image,
			RepoTags: []string{imageManifest.ID},
		},
	}
	return resp, nil
}

func (r *Runtime) PullImage(ctx context.Context, image *runtimeapi.ImageSpec, auth *runtimeapi.AuthConfig, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	return "", errors.New("pulling images is not supported currently")
}

func (r *Runtime) getImageManifest(image string) (*kii.Manifest, error) {
	nlog.Infof("Get image %q manifest", image)
	imageName, err := kii.NewImageName(image)
	if err != nil {
		return nil, err
	}
	manifest, err := r.imageStore.GetImageManifest(imageName)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}
