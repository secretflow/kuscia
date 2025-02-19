// Copyright 2024 Ant Group Co., Ltd.
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

package store

import (
	"errors"
	"fmt"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

// TODO: deprecated, remove it after 6 months
type dockerStore struct {
	layout  *layout.LocalImage
	mounter mounter.Mounter
}

func NewDockerStore(rootDir string, mountType mounter.MountType) (Store, error) {
	localLayout, err := layout.NewLocalImage(rootDir)
	if err != nil {
		return nil, err
	}

	return &dockerStore{
		layout:  localLayout,
		mounter: mounter.NewMounter(mountType),
	}, nil
}

func (s *dockerStore) CheckImageExist(image *kii.ImageName) bool {
	manifest, err := s.GetImageManifest(image, nil)
	if err != nil {
		return false
	}

	for _, layerHash := range manifest.Layers {
		if !paths.CheckFileExist(s.layout.GetLayerTarFile(layerHash)) {
			return false
		}
	}
	return true
}

func (s *dockerStore) MountImage(image *kii.ImageName, workingDir, targetDir string) error {
	manifest, err := s.GetImageManifest(image, nil)
	if err != nil {
		return err
	}

	tarFiles := []string{}
	for _, layerHash := range manifest.Layers {
		tarFiles = append(tarFiles, s.layout.GetLayerTarFile(layerHash))
	}

	return s.mounter.Mount(tarFiles, workingDir, targetDir)
}

func (s *dockerStore) UmountImage(workingDir string) error {
	return s.mounter.Umount(workingDir)
}

func (s *dockerStore) GetImageManifest(image *kii.ImageName, auth *runtimeapi.AuthConfig) (*kii.Manifest, error) {
	if !paths.CheckFileExist(s.layout.GetManifestFilePath(image)) {
		return nil, fmt.Errorf("image %q not exist in local repository", image.Image)
	}

	var ret kii.Manifest
	err := paths.ReadJSON(s.layout.GetManifestFilePath(image), &ret)
	return &ret, err
}

func (s *dockerStore) TagImage(sourceImage, targetImage *kii.ImageName) error {
	if !s.CheckImageExist(sourceImage) {
		return fmt.Errorf("source image %q not exist in local repository", sourceImage.Image)
	}

	if err := paths.CopyDirectory(s.layout.GetManifestsDir(sourceImage), s.layout.GetManifestsDir(targetImage)); err != nil {
		return fmt.Errorf("tag image from %v to %v fail, err-> %v", sourceImage.Image, targetImage.Image, err)
	}
	return nil
}

func (s *dockerStore) PullImage(image string, auth *runtimeapi.AuthConfig) error {
	return errors.New("pulling images is not supported currently")
}

func (s *dockerStore) ListImage() ([]*Image, error) {
	return nil, errors.New("not supported currently")
}

func (s *dockerStore) RemoveImage(imageNameOrIDs []string) error {
	return errors.New("not supported currently")
}
