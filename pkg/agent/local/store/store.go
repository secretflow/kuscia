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
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
)

type Store interface {
	CheckImageExist(image *kii.ImageName) bool
	MountImage(image *kii.ImageName, workingDir, targetDir string) error
	UmountImage(workingDir string) error
	GetImageManifest(image *kii.ImageName, auth *runtimeapi.AuthConfig) (*kii.Manifest, error)
	TagImage(sourceImage, targetImage *kii.ImageName) error
	LoadImage(tarFile string) error
	RegisterImage(image, manifest string) error
	PullImage(image string, auth *runtimeapi.AuthConfig) error
	ListImage() ([]*Image, error)
	RemoveImage(imageNameOrIDs []string) error
}
