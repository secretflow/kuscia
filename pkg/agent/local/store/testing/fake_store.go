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

package testing

import (
	"io"
	"time"

	oci "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
)

type fakeStore struct {
}

func NewFakeStore() store.Store {
	return &fakeStore{}
}

func (f *fakeStore) LoadImage(imageReader io.Reader) error {
	return nil
}

func (f *fakeStore) CheckImageExist(image *kii.ImageName) bool {
	return true
}

func (f *fakeStore) MountImage(image *kii.ImageName, mountType kii.MountType, workingDir, targetDir string) error {
	return nil
}

func (f *fakeStore) UmountImage(workingDir string) error {
	return nil
}

func (f *fakeStore) GetImageManifest(image *kii.ImageName) (*kii.Manifest, error) {
	now := time.Now()
	return &kii.Manifest{
		Created: &now,
		Config:  oci.ImageConfig{},
		ID:      "abc",
		Type:    kii.ImageTypeBuiltIn,
	}, nil
}

func (f *fakeStore) TagImage(sourceImage, targetImage *kii.ImageName) error {
	return nil
}

func (f *fakeStore) RegisterImage(image, manifest string) error {
	return nil
}
