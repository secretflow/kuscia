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
	"fmt"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// PullImage pulls an image from the network to local storage using the supplied
// secrets if necessary.
func (m *kubeGenericRuntimeManager) PullImage(ctx context.Context, image pkgcontainer.ImageSpec, auth *credentialprovider.AuthConfig, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	imgSpec := toRuntimeAPIImageSpec(image)

	withCredential := false
	if auth != nil && ((auth.Username != "" && auth.Password != "") || auth.Auth != "" || auth.IdentityToken != "" || auth.RegistryToken != "") {
		withCredential = true
	}

	if !withCredential {
		nlog.Infof("Pulling image %q without credential ...", image.Image)

		imageRef, err := m.imageService.PullImage(ctx, imgSpec, nil, podSandboxConfig)
		if err != nil {
			return "", fmt.Errorf("failed to pull image %q, detail-> %v", image.Image, err)
		}

		return imageRef, nil
	}

	apiAuth := &runtimeapi.AuthConfig{
		Username:      auth.Username,
		Password:      auth.Password,
		Auth:          auth.Auth,
		ServerAddress: auth.ServerAddress,
		IdentityToken: auth.IdentityToken,
		RegistryToken: auth.RegistryToken,
	}

	nlog.Infof("Pulling image %q ...", image.Image)
	imageRef, err := m.imageService.PullImage(ctx, imgSpec, apiAuth, podSandboxConfig)
	if err != nil {
		return "", fmt.Errorf("failed to pull image %q with credentials, detail-> %v", image.Image, err)
	}

	return imageRef, nil

}

// GetImageRef gets the ID of the image which has already been in
// the local storage. It returns ("", nil) if the image isn't in the local storage.
func (m *kubeGenericRuntimeManager) GetImageRef(ctx context.Context, image pkgcontainer.ImageSpec) (string, error) {
	resp, err := m.imageService.ImageStatus(ctx, toRuntimeAPIImageSpec(image), false)
	if err != nil {
		nlog.Errorf("Failed to get image status %q: %v", image.Image, err)
		return "", err
	}
	if resp.Image == nil {
		return "", nil
	}
	return resp.Image.Id, nil
}

// ListImages gets all images currently on the machine.
func (m *kubeGenericRuntimeManager) ListImages(ctx context.Context) ([]pkgcontainer.Image, error) {
	var images []pkgcontainer.Image

	allImages, err := m.imageService.ListImages(ctx, nil)
	if err != nil {
		nlog.Errorf("Failed to list images: %v", err)
		return nil, err
	}

	for _, img := range allImages {
		images = append(images, pkgcontainer.Image{
			ID:          img.Id,
			Size:        int64(img.Size_),
			RepoTags:    img.RepoTags,
			RepoDigests: img.RepoDigests,
			Spec:        topkgcontainerImageSpec(img),
		})
	}

	return images, nil
}

// RemoveImage removes the specified image.
func (m *kubeGenericRuntimeManager) RemoveImage(ctx context.Context, image pkgcontainer.ImageSpec) error {
	err := m.imageService.RemoveImage(ctx, &runtimeapi.ImageSpec{Image: image.Image})
	if err != nil {
		nlog.Errorf("Failed to remove image %q: %v", image.Image, err)
		return err
	}

	return nil
}

// ImageStats returns the statistics of the image.
// Notice that current logic doesn't really work for images which share layers (e.g. docker image),
// this is a known issue, and we'll address this by getting imagefs stats directly from CRI.
// TODO: Get imagefs stats directly from CRI.
func (m *kubeGenericRuntimeManager) ImageStats(ctx context.Context) (*pkgcontainer.ImageStats, error) {
	allImages, err := m.imageService.ListImages(ctx, nil)
	if err != nil {
		nlog.Errorf("Failed to list images: %v", err)
		return nil, err
	}
	stats := &pkgcontainer.ImageStats{}
	for _, img := range allImages {
		stats.TotalStorageBytes += img.Size_
	}
	return stats, nil
}
