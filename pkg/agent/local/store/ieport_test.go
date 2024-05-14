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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
)

func TestStore_RegisterImage(t *testing.T) {
	rootDir := t.TempDir()
	s, err := NewStore(filepath.Join(rootDir, "store"))
	assert.NoError(t, err)

	testManifest := kii.Manifest{
		ID: "12345",
	}
	testManifestContent, err := json.Marshal(testManifest)
	assert.NoError(t, err)
	testManifestFile := filepath.Join(rootDir, "manifest.json")
	assert.NoError(t, os.WriteFile(testManifestFile, testManifestContent, 0644))

	tests := []struct {
		image    string
		manifest string
		imageID  string
	}{
		{
			image:    "secretflow/secretflow:0.1",
			manifest: "",
			imageID:  "secretflow/secretflow:0.1",
		},
		{
			image:    "secretflow/secretflow:0.2",
			manifest: testManifestFile,
			imageID:  "sha256:12345",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			assert.NoError(t, s.RegisterImage(tt.image, tt.manifest))
			image, err := kii.NewImageName(tt.image)
			assert.NoError(t, err)
			assert.True(t, s.CheckImageExist(image))
			manifest, err := s.GetImageManifest(image)
			assert.NoError(t, err)
			assert.Equal(t, tt.imageID, manifest.ID)
		})
	}
}

func TestStore_LoadImage(t *testing.T) {
	arch := runtime.GOARCH
	_, filename, _, ok := runtime.Caller(0)
	assert.True(t, ok)
	dir := filepath.Dir(filename)
	for i := 0; i < 4; i++ {
		dir = filepath.Dir(dir)
	}
	var pauseTarFile string
	switch arch {
	case "aarch64", "arm64":
		pauseTarFile = filepath.Join(dir, "build/pause/pause-arm64.tar")
	case "amd64", "x86_64":
		pauseTarFile = filepath.Join(dir, "build/pause/pause-amd64.tar")
	default:
		pauseTarFile = filepath.Join(dir, "build/pause/pause-amd64.tar")
	}
	file, err := os.Open(pauseTarFile)
	assert.NoError(t, err)
	defer file.Close()
	rootDir := t.TempDir()

	s, err := NewStore(rootDir)
	assert.NoError(t, err)

	assert.NoError(t, s.LoadImage(file))
	image, err := kii.NewImageName("secretflow/pause:3.6")
	assert.NoError(t, err)
	assert.True(t, s.CheckImageExist(image))

	image, err = kii.NewImageName("docker.io/secretflow/pause:3.6")
	assert.NoError(t, err)
	assert.True(t, s.CheckImageExist(image))
}
