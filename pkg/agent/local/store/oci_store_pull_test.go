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
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/utils/assist"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

func TestGetRemoteOpts(t *testing.T) {
	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)
	assert.NoError(t, err)
	options := store.(*ociStore).getRemoteOpts(context.Background(), nil)
	assert.NotNil(t, options)
	assert.Len(t, options, 3)

	options = store.(*ociStore).getRemoteOpts(context.Background(), &runtimeapi.AuthConfig{
		Username: "test",
		Password: "test",
	})
	assert.NotNil(t, options)
	assert.Len(t, options, 4)
}

func tarToFile(t *testing.T, dst string, prefix string, srcFiles ...string) {
	tarfile, err := os.Create(dst)
	assert.NoError(t, err)
	defer tarfile.Close()
	gzw := gzip.NewWriter(tarfile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, fpath := range srcFiles {
		file, err := os.Open(path.Join(prefix, fpath))
		assert.NoError(t, err)
		defer file.Close()
		info, err := file.Stat()
		assert.NoError(t, err)

		header := &tar.Header{
			Name: fpath,
			Size: info.Size(),
		}

		assert.NoError(t, tw.WriteHeader(header))
		_, err = io.Copy(tw, file)
		assert.NoError(t, err)
	}
}

func TestFlatImage(t *testing.T) {
	img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
		Created: v1.Time{Time: time.Now()},
	})
	assert.NoError(t, err)
	assert.NotNil(t, img)

	cachePath := t.TempDir()
	file1 := path.Join(cachePath, "file1.txt")
	file2 := path.Join(cachePath, "file2.txt")
	assert.NoError(t, os.WriteFile(file1, []byte("file1"), 0644))
	assert.NoError(t, os.WriteFile(file2, []byte("file2"), 0644))
	tarToFile(t, path.Join(cachePath, "layer1.tar.gz"), cachePath, "file1.txt")
	tarToFile(t, path.Join(cachePath, "layer2.tar.gz"), cachePath, "file2.txt")

	layer1, err := tarball.LayerFromFile(path.Join(cachePath, "layer1.tar.gz"), tarball.WithCompressionLevel(gzip.BestSpeed))
	assert.NoError(t, err)
	layer2, err := tarball.LayerFromFile(path.Join(cachePath, "layer2.tar.gz"), tarball.WithCompressionLevel(gzip.BestSpeed))
	assert.NoError(t, err)
	img, err = mutate.AppendLayers(img, layer1, layer2)
	assert.NoError(t, err)
	assert.NotNil(t, img)

	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)
	assert.NoError(t, err)
	nimg, _, err := store.(*ociStore).flattenImage(img)
	assert.NoError(t, err)
	assert.NotNil(t, nimg)

	layers, err := nimg.Layers()
	assert.NoError(t, err)
	assert.Len(t, layers, 1)

	layfile := path.Join(cachePath, "test")
	reader, err := layers[0].Uncompressed()
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	assert.NoError(t, assist.Untar(layfile, false, true, reader, true))
	assert.NoError(t, paths.CheckAllFileExist(path.Join(layfile, "file1.txt"), path.Join(layfile, "file2.txt")))
	content1, err := os.ReadFile(path.Join(layfile, "file1.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "file1", string(content1))

	content2, err := os.ReadFile(path.Join(layfile, "file2.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "file2", string(content2))
}
