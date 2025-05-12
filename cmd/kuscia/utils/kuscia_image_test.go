// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/stretchr/testify/assert"
	"path"
	"testing"
)

func TestOciImage_RemoveImage(t *testing.T) {
	root := t.TempDir()
	store2, err := store.NewOCIStore(root, mounter.Plain)
	assert.NoError(t, err)
	assert.NotNil(t, store2)
	image := OciImage{
		Store: store2,
	}

	img, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	image.args = []string{img.Tag}
	assert.NoError(t, image.RemoveImage())
}

func TestOciImage_LoadImage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store2, err := store.NewOCIStore(root, mounter.Plain)
	assert.Nil(t, err, "init oci store failed")
	image := OciImage{
		Store: store2,
	}

	err = image.LoadImage("/path/to/not-exists")
	assert.Error(t, err)
}

func TestOciImage_PullImage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store2, err := store.NewOCIStore(root, mounter.Plain)
	assert.Nil(t, err, "init oci store failed")
	image := OciImage{
		Store: store2,
		args: []string{
			"docker.io/secretflow/test:v1",
		},
	}

	assert.Error(t, image.PullImage(""))
}

func TestOciImage_ListImage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store2, err := store.NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store2)

	image := OciImage{
		Store: store2,
	}

	err = image.ListImage()
	assert.NoError(t, err)
}

func TestOciImage_Mount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store2, err := store.NewOCIStore(root, mounter.Plain)
	assert.NoError(t, err)
	assert.NotNil(t, store2)

	image := OciImage{
		Store:      store2,
		StorageDir: path.Join(root, "storage"),
		args: []string{
			"test1",
			"docker.io/secretflow/test:v1",
			"test1",
		},
	}

	assert.Error(t, image.MountImage())
}

func TestOciImage_ImageRun(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store2, err := store.NewOCIStore(root, mounter.Plain)
	assert.NoError(t, err)
	assert.NotNil(t, store2)

	image := OciImage{
		Store: store2,
	}

	assert.Error(t, image.ImageRun("docker.io/secretflow/test:v1"))
}
