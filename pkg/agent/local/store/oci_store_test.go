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
	"compress/gzip"
	"encoding/json"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/utils/assist"
)

func TestNewOCIStore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	content, err := os.ReadFile(path.Join(root, "repositories", "index.json"))
	assert.NoError(t, err)
	assert.NotEmpty(t, content)

	_, err = NewOCIStore("/dev/null/no-exists", mounter.Plain)
	assert.Error(t, err)
}

func TestOCIStore_Regist_CheckImageExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	assert.False(t, store.CheckImageExist(&kii.ImageName{
		Image: "docker.io/secretflow/test:v1",
	}))

	assert.NoError(t, store.RegisterImage("docker.io/secretflow/test:v1", ""))
	assert.True(t, store.CheckImageExist(&kii.ImageName{
		Image: "docker.io/secretflow/test:v1",
	}))

	img, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	m, err := store.GetImageManifest(img, nil)
	assert.NoError(t, err)
	assert.Equal(t, runtime.GOARCH, m.Architecture)

	// re-regist
	manifest, _ := json.Marshal(&kii.Manifest{
		Architecture: "x86",
	})
	assert.NoError(t, os.WriteFile(path.Join(root, "test.json"), manifest, 0644))
	assert.NoError(t, store.RegisterImage("docker.io/secretflow/test:v1", path.Join(root, "test.json")))
	assert.True(t, store.CheckImageExist(&kii.ImageName{
		Image: "docker.io/secretflow/test:v1",
	}))
	m, err = store.GetImageManifest(img, nil)
	assert.NoError(t, err)
	assert.Equal(t, "x86", m.Architecture)
}

func TestOCIStore_ListImages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	imgs, err := store.ListImage()
	assert.NoError(t, err)
	assert.Len(t, imgs, 0)

	assert.NoError(t, store.RegisterImage("docker.io/secretflow/test:v1", ""))
	imgs, err = store.ListImage()
	assert.NoError(t, err)
	assert.Len(t, imgs, 1)

}

func TestOCIStore_Mount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	// image not exists
	tmp := t.TempDir()
	img, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	assert.Error(t, store.MountImage(img, tmp, tmp))

	// layer count is not 0
	assert.NoError(t, store.RegisterImage("docker.io/secretflow/test:v1", ""))
	assert.NoError(t, store.MountImage(img, tmp, tmp))

	//////////////////////////////////
	// append 1layer to image
	//////////////////////////////////
	// create layer file
	tmp = t.TempDir()
	tmp2 := t.TempDir()
	assert.NoError(t, os.WriteFile(path.Join(tmp2, "test.txt"), []byte("xyz"), 0644))
	file, err := os.OpenFile(path.Join(root, "cache", "layer1.tar.gz"), os.O_WRONLY|os.O_CREATE, 0644)
	assert.NoError(t, err)
	assert.NotNil(t, file)
	assert.NoError(t, assist.Tar(tmp2, true, file))
	assert.NoError(t, file.Close())

	m, err := store.(*ociStore).findImageFromLocal("docker.io/secretflow/test:v1")
	assert.NoError(t, err)
	assert.NotNil(t, m)

	// append to image
	layer, err := tarball.LayerFromFile(path.Join(root, "cache", "layer1.tar.gz"), tarball.WithCompressionLevel(gzip.BestSpeed))
	assert.NoError(t, err)
	newImg, err := mutate.AppendLayers(m, layer)
	assert.NoError(t, err)
	assert.NotNil(t, newImg)

	assert.NoError(t, store.(*ociStore).imagePath.ReplaceImage(
		newImg,
		match.Name("docker.io/secretflow/test:v1"),
		store.(*ociStore).kusciaImageAnnotation("docker.io/secretflow/test:v1", "")))

	assert.NoError(t, store.MountImage(img, tmp, tmp))
	content, err := os.ReadFile(path.Join(tmp, "test.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "xyz", string(content))

	assert.NoError(t, store.UmountImage(tmp))
	_, err = os.Stat(tmp)
	assert.True(t, os.IsNotExist(err))
}

func TestOCIStore_GetImageManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	manifest, _ := json.Marshal(&kii.Manifest{
		Architecture: "x86",
		Config: v1.ImageConfig{
			Cmd:        []string{"echo"},
			WorkingDir: "/root",
		},
	})
	assert.NoError(t, os.WriteFile(path.Join(root, "test.json"), manifest, 0644))
	assert.NoError(t, store.RegisterImage("docker.io/secretflow/test:v1", path.Join(root, "test.json")))

	img, _ := kii.NewImageName("docker.io/secretflow/test:v1")

	assert.True(t, store.CheckImageExist(img))

	m, err := store.GetImageManifest(img, nil)
	assert.NoError(t, err)
	assert.Equal(t, "x86", m.Architecture)
	assert.NotEmpty(t, m.Created)
	assert.NotEmpty(t, m.ID)
	assert.Equal(t, kii.ImageTypeBuiltIn, m.Type)
	assert.NotEmpty(t, m.Config.WorkingDir)
	assert.Len(t, m.Config.Cmd, 1)
	assert.Equal(t, "echo", m.Config.Cmd[0])
}

func TestOCIStore_TagImages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	assert.NoError(t, store.RegisterImage("docker.io/secretflow/test:v1", ""))
	org, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	assert.True(t, store.CheckImageExist(org))

	dst, _ := kii.NewImageName("docker.io/secretflow/xxxxx:v1")
	assert.NoError(t, store.TagImage(org, dst))
	assert.True(t, store.CheckImageExist(org))
	assert.True(t, store.CheckImageExist(dst))
}

func TestOCIStore_LoadImage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	// tarfile not exists
	assert.Error(t, store.LoadImage("/path/to/not-exists"))

	// create a new image.tar.gz and load it
	org, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	dst, _ := kii.NewImageName("docker.io/secretflow/xxxx:v1")

	assert.NoError(t, store.RegisterImage("docker.io/secretflow/test:v1", ""))

	m, err := store.(*ociStore).findImageFromLocal("docker.io/secretflow/test:v1")
	assert.NoError(t, err)
	assert.NotNil(t, m)

	tarfile := path.Join(root, "cache", "test.tar.gz")
	ref, _ := name.ParseReference("docker.io/secretflow/xxxx:v1")
	assert.NoError(t, tarball.WriteToFile(tarfile, ref, m))

	assert.False(t, store.CheckImageExist(dst))

	assert.NoError(t, store.LoadImage(tarfile))

	assert.True(t, store.CheckImageExist(org))
	assert.True(t, store.CheckImageExist(dst))
}

func TestOCIStore_PullImageFailed(t *testing.T) {
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	img, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	assert.False(t, store.CheckImageExist(img))

	assert.Error(t, store.PullImage("invalidate:image:xxx:xyz", nil))

	// patch := gomonkeyv2.ApplyFunc(remote.Get, func(ref name.Reference, options ...remote.Option) (*remote.Descriptor, error) {
	// 	return nil, errors.New("xy")
	// })
	// defer patch.Reset()
	// assert.NoError(t, store.PullImage("docker.io/secretflow/test:v1", nil))
}

func TestOCIStore_PullImage_Image(t *testing.T) {
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	img, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	assert.False(t, store.CheckImageExist(img))

	// desc := &remote.Descriptor{}
	// desc.MediaType = types.DockerManifestSchema2
	// patch := gomonkeyv2.ApplyFunc(remote.Get, func(ref name.Reference, options ...remote.Option) (*remote.Descriptor, error) {
	// 	nlog.Info("xxxx")
	// 	return desc, nil
	// })
	// patch.ApplyMethod(desc, "Image", func(c *remote.Descriptor) (v1.Image, error) {
	// 	nlog.Info("yyyy")
	// 	return v1.Image{}, errors.New("xyzdss")
	// })
	// defer patch.Reset()
	// assert.NoError(t, store.PullImage("docker.io/secretflow/test:v1", nil))
}
