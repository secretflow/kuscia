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

package store

import (
	"fmt"
	"io"

	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/agent/local/store/mounter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type Store interface {
	CheckImageExist(image *kii.ImageName) bool
	MountImage(image *kii.ImageName, mountType kii.MountType, workingDir, targetDir string) error
	UmountImage(workingDir string) error
	GetImageManifest(image *kii.ImageName) (*kii.Manifest, error)
	TagImage(sourceImage, targetImage *kii.ImageName) error
	LoadImage(imageReader io.Reader) error
	RegisterImage(image, manifest string) error
}

type store struct {
	layout  *layout.LocalImage
	mounter *mounter.Mounter
}

func NewStore(rootDir string) (Store, error) {
	localLayout, err := layout.NewLocalImage(rootDir)
	if err != nil {
		return nil, err
	}

	m := mounter.New(localLayout)
	return &store{
		layout:  localLayout,
		mounter: m,
	}, nil
}

func (s *store) CheckImageExist(image *kii.ImageName) bool {
	manifest, err := s.GetImageManifest(image)
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

func (s *store) MountImage(image *kii.ImageName, mountType kii.MountType, workingDir, targetDir string) error {
	manifest, err := s.GetImageManifest(image)
	if err != nil {
		return err
	}

	return s.mounter.Mount(manifest, mountType, workingDir, targetDir)
}

func (s *store) UmountImage(workingDir string) error {
	return s.mounter.Umount(workingDir)
}

func (s *store) GetImageManifest(image *kii.ImageName) (*kii.Manifest, error) {
	if !paths.CheckFileExist(s.layout.GetManifestFilePath(image)) {
		return nil, fmt.Errorf("image %q not exist in local repository", image.Image)
	}

	var ret kii.Manifest
	err := paths.ReadJSON(s.layout.GetManifestFilePath(image), &ret)
	return &ret, err
}

func (s *store) TagImage(sourceImage, targetImage *kii.ImageName) error {
	if !s.CheckImageExist(sourceImage) {
		return fmt.Errorf("source image %q not exist in local repository", sourceImage.Image)
	}

	if err := paths.CopyDirectory(s.layout.GetManifestsDir(sourceImage), s.layout.GetManifestsDir(targetImage)); err != nil {
		return fmt.Errorf("tag image from %v to %v fail, err-> %v", sourceImage.Image, targetImage.Image, err)
	}
	return nil
}
