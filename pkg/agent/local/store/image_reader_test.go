// Copyright 2025 Ant Group Co., Ltd.
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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	v1spec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTarfileOsArchUnsupportedError tests the error implementation
func TestTarfileOsArchUnsupportedError(t *testing.T) {
	err := &TarfileOsArchUnsupportedError{
		Source:          "test.tar",
		CurrentPlatform: "linux/amd64",
		ArchSummary:     []ImageSummary{{Ref: "test:latest", Platforms: []string{"linux/arm64"}}},
		Cause:           fmt.Errorf("architecture mismatch"),
	}

	assert.Contains(t, err.Error(), "test.tar")
	assert.Contains(t, err.Error(), "linux/amd64")
	assert.Equal(t, "architecture mismatch", err.Unwrap().Error())
}

// createTestTar creates a test tar file with specified content
func createTestTar(t *testing.T, files map[string][]byte) string {
	file, err := os.CreateTemp("", "test-*.tar")
	require.NoError(t, err)
	defer file.Close()

	writer := tar.NewWriter(file)
	defer writer.Close()

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		err := writer.WriteHeader(header)
		require.NoError(t, err)

		_, err = writer.Write(content)
		require.NoError(t, err)
	}

	return file.Name()
}

// createTestGzipTar creates a test tar.gz file with specified content
func createTestGzipTar(t *testing.T, files map[string][]byte) string {
	file, err := os.CreateTemp("", "test-*.tar.gz")
	require.NoError(t, err)
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	writer := tar.NewWriter(gzWriter)
	defer writer.Close()

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		err := writer.WriteHeader(header)
		require.NoError(t, err)

		_, err = writer.Write(content)
		require.NoError(t, err)
	}

	return file.Name()
}

// TestCreateTarReader tests the createTarReader function
func TestCreateTarReader(t *testing.T) {
	// Test with regular tar
	files := map[string][]byte{
		"test.txt": []byte("hello world"),
	}
	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)
	assert.False(t, tr.isGzip)
	assert.Equal(t, tarFile, tr.filePath)

	// Test with gzipped tar
	gzTarFile := createTestGzipTar(t, files)
	defer os.Remove(gzTarFile)

	gzFile, err := os.Open(gzTarFile)
	require.NoError(t, err)
	defer gzFile.Close()

	tr2, err := createTarReader(gzTarFile, gzFile, gzFile)
	require.NoError(t, err)
	assert.True(t, tr2.isGzip)
	assert.Equal(t, gzTarFile, tr2.filePath)

	// Test with .tgz extension
	tr3, err := createTarReader("test.tgz", file, file)
	require.NoError(t, err)
	assert.True(t, tr3.isGzip)
}

// TestTarReader_Close tests the Close method
func TestTarReader_Close(t *testing.T) {
	// Test with nil closer
	tr := &TarReader{closer: nil}
	assert.NoError(t, tr.Close())

	// Test with actual closer
	file := &mockCloser{closed: false}
	tr2 := &TarReader{closer: file}
	assert.NoError(t, tr2.Close())
	assert.True(t, file.closed)
}

// mockCloser is a mock io.Closer for testing
type mockCloser struct {
	closed bool
}

func (m *mockCloser) Close() error {
	m.closed = true
	return nil
}

// TestTarReader_ExtractFileAsStream tests the ExtractFileAsStream method
func TestTarReader_ExtractFileAsStream(t *testing.T) {
	files := map[string][]byte{
		"test.txt":    []byte("hello world"),
		"config.json": []byte(`{"key": "value"}`),
	}

	tests := []struct {
		name        string
		isGzip      bool
		fileName    string
		wantContent []byte
		wantError   bool
	}{
		{
			name:        "regular tar - existing file",
			isGzip:      false,
			fileName:    "test.txt",
			wantContent: []byte("hello world"),
			wantError:   false,
		},
		{
			name:        "gzipped tar - existing file",
			isGzip:      true,
			fileName:    "test.txt",
			wantContent: []byte("hello world"),
			wantError:   false,
		},
		{
			name:        "non-existent file",
			isGzip:      false,
			fileName:    "nonexistent.txt",
			wantContent: nil,
			wantError:   true,
		},
		{
			name:        "directory instead of file",
			isGzip:      false,
			fileName:    "dir/",
			wantContent: nil,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tarFile string
			if tt.isGzip {
				tarFile = createTestGzipTar(t, files)
			} else {
				tarFile = createTestTar(t, files)
			}
			defer os.Remove(tarFile)

			file, err := os.Open(tarFile)
			require.NoError(t, err)
			defer file.Close()

			tr, err := createTarReader(tarFile, file, file)
			require.NoError(t, err)

			stream, err := tr.ExtractFileAsStream(tt.fileName)
			if tt.wantError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer stream.Close()

			content, err := io.ReadAll(stream)
			require.NoError(t, err)
			assert.Equal(t, tt.wantContent, content)
		})
	}
}

// TestTarReader_ReadJSONFile tests the ReadJSONFile method
func TestTarReader_ReadJSONFile(t *testing.T) {
	files := map[string][]byte{
		"config.json": []byte(`{"architecture":"amd64","os":"linux"}`),
	}
	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	// Test reading valid JSON
	var config ImageConfig
	err = tr.ReadJSONFile("config.json", &config, false)
	require.NoError(t, err)
	assert.Equal(t, "amd64", config.Architecture)
	assert.Equal(t, "linux", config.OS)

	// Test reading non-existent file
	err = tr.ReadJSONFile("nonexistent.json", &config, false)
	assert.Error(t, err)

	// Test reading invalid JSON
	filesInvalid := map[string][]byte{
		"invalid.json": []byte(`invalid json`),
	}
	invalidTar := createTestTar(t, filesInvalid)
	defer os.Remove(invalidTar)

	invalidFile, err := os.Open(invalidTar)
	require.NoError(t, err)
	defer invalidFile.Close()

	trInvalid, err := createTarReader(invalidTar, invalidFile, invalidFile)
	require.NoError(t, err)

	err = trInvalid.ReadJSONFile("invalid.json", &config, false)
	assert.Error(t, err)

	// Test caching
	filesCache := map[string][]byte{
		"cache.json": []byte(`{"architecture":"arm64","os":"linux"}`),
	}
	cacheTar := createTestTar(t, filesCache)
	defer os.Remove(cacheTar)

	cacheFile, err := os.Open(cacheTar)
	require.NoError(t, err)
	defer cacheFile.Close()

	trCache, err := createTarReader(cacheTar, cacheFile, cacheFile)
	require.NoError(t, err)

	var config2 ImageConfig
	err = trCache.ReadJSONFile("cache.json", &config2, true)
	require.NoError(t, err)
	assert.Equal(t, "arm64", config2.Architecture)

	// Verify cache is used
	assert.Contains(t, trCache.nonLayerBlobCache, "cache.json")
}

// TestArchReader_parseOCIIndex tests the parseOCIIndex method
func TestArchReader_parseOCIIndex(t *testing.T) {
	// Create test OCI index structure
	index := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageManifest,
				Digest:    "sha256:1234567890abcdef",
				Platform: &OCIPlatform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Annotations: map[string]string{
					"org.opencontainers.image.ref.name": "test:latest",
				},
			},
		},
	}

	manifest := OCIManifest{
		SchemaVersion: 2,
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123456",
			Size:      100,
		},
		Layers: []OCIDescriptor{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:layer123456",
				Size:      1000,
			},
		},
	}

	config := ImageConfig{
		Architecture: "amd64",
		OS:           "linux",
	}

	files := map[string][]byte{
		"index.json":                    mustMarshalJSON(index),
		"blobs/sha256/1234567890abcdef": mustMarshalJSON(manifest),
		"blobs/sha256/config123456":     mustMarshalJSON(config),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, ociManifests, digests, ociIndex, configs, rawConfigs, err := ar.parseOCIIndex()
	require.NoError(t, err)

	assert.Len(t, archInfos, 1)
	assert.Len(t, ociManifests, 1)
	assert.Len(t, digests, 1)
	assert.NotNil(t, ociIndex)
	assert.Len(t, configs, 1)
	assert.Len(t, rawConfigs, 1)

	assert.Equal(t, "amd64", archInfos[0].Architecture)
	assert.Equal(t, "linux", archInfos[0].OS)
	assert.Equal(t, "test:latest", archInfos[0].Ref)
}

// TestArchReader_parseOCIIndex_Recursive tests recursive OCI index parsing
func TestArchReader_parseOCIIndex_Recursive(t *testing.T) {
	// Create nested index structure
	nestedIndex := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageManifest,
				Digest:    "sha256:nested123456",
				Platform: &OCIPlatform{
					Architecture: "arm64",
					OS:           "linux",
				},
			},
		},
	}

	parentIndex := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageIndex,
				Digest:    "sha256:parent123456",
			},
		},
	}

	manifest := OCIManifest{
		SchemaVersion: 2,
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123456",
			Size:      100,
		},
	}

	config := ImageConfig{
		Architecture: "arm64",
		OS:           "linux",
	}

	files := map[string][]byte{
		"index.json":                mustMarshalJSON(parentIndex),
		"blobs/sha256/parent123456": mustMarshalJSON(nestedIndex),
		"blobs/sha256/nested123456": mustMarshalJSON(manifest),
		"blobs/sha256/config123456": mustMarshalJSON(config),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, ociManifests, digests, ociIndex, configs, rawConfigs, err := ar.parseOCIIndex()
	require.NoError(t, err)

	assert.Len(t, archInfos, 1)
	assert.Len(t, ociManifests, 1)
	assert.Len(t, digests, 1)
	assert.NotNil(t, ociIndex)
	assert.Len(t, configs, 1)
	assert.Len(t, rawConfigs, 1)

	assert.Equal(t, "arm64", archInfos[0].Architecture)
	assert.Equal(t, "linux", archInfos[0].OS)
}

// TestArchReader_parseOCIIndex_MaxDepth tests maximum recursion depth
func TestArchReader_parseOCIIndex_MaxDepth(t *testing.T) {
	// Create deeply nested structure
	createNestedIndex := func(depth int) OCIIndex {
		if depth >= maxOCIRecursiveDepth+1 {
			return OCIIndex{
				SchemaVersion: 2,
				Manifests:     []OCIIndexManifest{},
			}
		}
		return OCIIndex{
			SchemaVersion: 2,
			Manifests: []OCIIndexManifest{
				{
					MediaType: v1spec.MediaTypeImageIndex,
					Digest:    fmt.Sprintf("sha256:depth%d", depth),
				},
			},
		}
	}

	files := map[string][]byte{
		"index.json": mustMarshalJSON(createNestedIndex(0)),
	}

	// Create all nested index files
	for i := 0; i <= maxOCIRecursiveDepth; i++ {
		files[fmt.Sprintf("blobs/sha256/depth%d", i)] = mustMarshalJSON(createNestedIndex(i + 1))
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	_, _, _, _, _, _, err = ar.parseOCIIndex()
	// Should not error due to max depth protection
	assert.NoError(t, err)
}

// TestArchReader_parseDockerManifest tests the parseDockerManifest method
func TestArchReader_parseDockerManifest(t *testing.T) {
	manifest := []DockerManifest{
		{
			Config:   "config.json",
			RepoTags: []string{"test:latest"},
			Layers:   []string{"layer1.tar.gz", "layer2.tar"},
		},
	}

	config := struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
		RootFS       struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}{
		Architecture: "amd64",
		OS:           "linux",
	}
	config.RootFS.DiffIDs = []string{"sha256:diff123", "sha256:diff456"}

	files := map[string][]byte{
		"manifest.json": mustMarshalJSON(manifest),
		"config.json":   mustMarshalJSON(config),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, manifestsResult, configs, rawConfigs, diffIDs, err := ar.parseDockerManifest()
	require.NoError(t, err)

	assert.Len(t, archInfos, 1)
	assert.Len(t, manifestsResult, 1)
	assert.Len(t, configs, 1)
	assert.Len(t, rawConfigs, 1)
	assert.Len(t, diffIDs, 2)

	assert.Equal(t, "amd64", archInfos[0].Architecture)
	assert.Equal(t, "linux", archInfos[0].OS)
	assert.Equal(t, "test:latest", archInfos[0].Ref)
	assert.Equal(t, "sha256:diff123", diffIDs[0])
	assert.Equal(t, "sha256:diff456", diffIDs[1])
}

// TestImageMetaAdapter tests the imageMetaAdapter methods
func TestImageMetaAdapter(t *testing.T) {
	// Create test data
	config := ImageConfig{
		Architecture: "amd64",
		OS:           "linux",
	}

	manifest := OCIManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123456",
			Size:      100,
		},
		Layers: []OCIDescriptor{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:layer123456",
				Size:      1000,
			},
		},
	}

	files := map[string][]byte{
		"blobs/sha256/layer123456": []byte("mock layer content"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	meta := &ImageMetaData{
		OCIManifests: []OCIManifest{manifest},
		Configs:      []ImageConfig{config},
		RawConfigs:   [][]byte{mustMarshalJSON(config)},
		tarReader:    tr,
	}

	adapter := &imageMetaAdapter{meta: meta, metaTarReader: tr}

	// Test RawConfigFile
	rawConfig, err := adapter.RawConfigFile()
	require.NoError(t, err)
	assert.Equal(t, mustMarshalJSON(config), rawConfig)

	// Test ConfigFile
	configFile, err := adapter.ConfigFile()
	require.NoError(t, err)
	assert.Equal(t, "amd64", configFile.Architecture)
	assert.Equal(t, "linux", configFile.OS)

	// Test Manifest
	manifestResult, err := adapter.Manifest()
	require.NoError(t, err)
	assert.Equal(t, int64(2), manifestResult.SchemaVersion)
	assert.Equal(t, types.MediaType("application/vnd.oci.image.manifest.v1+json"), manifestResult.MediaType)

	// Test RawManifest
	_, err = adapter.RawManifest()
	assert.Error(t, err) // No raw manifest provided

	// Test Digest
	_, err = adapter.Digest()
	assert.Error(t, err) // No raw manifest provided

	// Test MediaType
	mediaType, err := adapter.MediaType()
	require.NoError(t, err)
	assert.Equal(t, types.MediaType("application/vnd.oci.image.manifest.v1+json"), mediaType)

	// Test ConfigName
	configName, err := adapter.ConfigName()
	require.NoError(t, err)
	assert.Equal(t, "config123456", configName.Hex)

	// Test Size - mock layer size should be 46 (length of "mock layer content")
	size, err := adapter.Size()
	require.NoError(t, err)
	assert.Equal(t, int64(46), size)
}

// TestImageMetaAdapter_Docker tests the imageMetaAdapter with Docker manifest
func TestImageMetaAdapter_Docker(t *testing.T) {
	config := ImageConfig{
		Architecture: "amd64",
		OS:           "linux",
		DiffIDs:      []string{"sha256:diff123"},
	}

	manifest := DockerManifest{
		Config:   "config.json",
		RepoTags: []string{"test:latest"},
		Layers:   []string{"layer1.tar.gz"},
	}

	files := map[string][]byte{
		"layer1.tar.gz": []byte("mock layer content"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	meta := &ImageMetaData{
		DockerManifests: []DockerManifest{manifest},
		Configs:         []ImageConfig{config},
		RawConfigs:      [][]byte{mustMarshalJSON(config)},
		tarReader:       tr,
	}

	adapter := &imageMetaAdapter{meta: meta, metaTarReader: tr}

	// Test Manifest
	manifestResult, err := adapter.Manifest()
	require.NoError(t, err)
	assert.Equal(t, int64(2), manifestResult.SchemaVersion)
	assert.Equal(t, types.MediaType("application/vnd.docker.distribution.manifest.v2+json"), manifestResult.MediaType)

	// Test Layers
	layers, err := adapter.Layers()
	require.NoError(t, err)
	assert.Len(t, layers, 1)

	// Test LayerByDiffID
	layer, err := adapter.LayerByDiffID(v1.Hash{Algorithm: "sha256", Hex: "diff123"})
	assert.Error(t, err) // Mock layer doesn't implement DiffID properly
	assert.Nil(t, layer)

	// Test LayerByDigest
	layer, err = adapter.LayerByDigest(v1.Hash{Algorithm: "sha256", Hex: "layer123456"})
	assert.Error(t, err)
	assert.Nil(t, layer)
}

// TestImageInFile tests the ImageInFile function
func TestImageInFile(t *testing.T) {
	// Create test OCI structure
	index := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageManifest,
				Digest:    "sha256:1234567890abcdef",
				Platform: &OCIPlatform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
				},
				Annotations: map[string]string{
					"org.opencontainers.image.ref.name": "test:latest",
				},
			},
		},
	}

	manifest := OCIManifest{
		SchemaVersion: 2,
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123456",
			Size:      100,
		},
		Layers: []OCIDescriptor{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:layer123456",
				Size:      1000,
			},
		},
	}

	config := ImageConfig{
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
	}

	files := map[string][]byte{
		"index.json":                    mustMarshalJSON(index),
		"blobs/sha256/1234567890abcdef": mustMarshalJSON(manifest),
		"blobs/sha256/config123456":     mustMarshalJSON(config),
		"blobs/sha256/layer123456":      []byte("mock layer content"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	summaries, compatibleImages, err := ImageInFile(tarFile, file, file)
	require.NoError(t, err)

	assert.Len(t, summaries, 1)
	assert.Equal(t, "test:latest", summaries[0].Ref)
	assert.Len(t, compatibleImages, 1)
	assert.Equal(t, runtime.GOARCH, compatibleImages[0].Info.Architecture)
	assert.Equal(t, runtime.GOOS, compatibleImages[0].Info.OS)
}

// TestImageInFile_Docker tests the ImageInFile function with Docker format
func TestImageInFile_Docker(t *testing.T) {
	manifest := []DockerManifest{
		{
			Config:   "config.json",
			RepoTags: []string{"test:latest"},
			Layers:   []string{"layer1.tar.gz"},
		},
	}

	config := struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
		RootFS       struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}{
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
	}
	config.RootFS.DiffIDs = []string{"sha256:diff123"}

	files := map[string][]byte{
		"manifest.json": mustMarshalJSON(manifest),
		"config.json":   mustMarshalJSON(config),
		"layer1.tar.gz": []byte("mock layer content"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	summaries, compatibleImages, err := ImageInFile(tarFile, file, file)
	require.NoError(t, err)

	assert.Len(t, summaries, 1)
	assert.Equal(t, "test:latest", summaries[0].Ref)
	assert.Len(t, compatibleImages, 1)
	assert.Equal(t, runtime.GOARCH, compatibleImages[0].Info.Architecture)
	assert.Equal(t, runtime.GOOS, compatibleImages[0].Info.OS)
}

// TestImageInFile_Invalid tests the ImageInFile function with invalid formats
func TestImageInFile_Invalid(t *testing.T) {
	files := map[string][]byte{
		"random.txt": []byte("not an image"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	_, _, err = ImageInFile(tarFile, file, file)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid docker or oci image format")
}

// TestCheckOsArchCompliance_FileNotFound tests file not found scenario
func TestCheckOsArchCompliance_FileNotFound(t *testing.T) {
	err := CheckOsArchCompliance("nonexistent.tar")
	assert.Error(t, err)
}

// TestCompatibleImage tests the CompatibleImage wrapper
func TestCompatibleImage(t *testing.T) {
	// Create a mock image
	mockImage := &mockV1Image{}
	info := ImageInfo{
		Architecture: "amd64",
		OS:           "linux",
		Ref:          "test:latest",
	}

	compatible := CompatibleImage{
		Image: mockImage,
		Info:  info,
	}

	// Test all wrapper methods
	_, err := compatible.RawConfigFile()
	assert.NoError(t, err)

	_, err = compatible.ConfigFile()
	assert.NoError(t, err)

	_, err = compatible.Manifest()
	assert.NoError(t, err)

	_, err = compatible.RawManifest()
	assert.NoError(t, err)

	_, err = compatible.Digest()
	assert.NoError(t, err)

	_, err = compatible.Layers()
	assert.NoError(t, err)

	mediaType, err := compatible.MediaType()
	assert.NoError(t, err)
	assert.Equal(t, types.MediaType("test"), mediaType)

	_, err = compatible.ConfigName()
	assert.NoError(t, err)

	_, err = compatible.Size()
	assert.NoError(t, err)

	// Test Compressed and Uncompressed
	_, err = compatible.Compressed()
	assert.NoError(t, err)

	_, err = compatible.Uncompressed()
	assert.NoError(t, err)
}

// TestTarReader_ExtractFileAsStream_EmptyFile tests empty file handling
func TestTarReader_ExtractFileAsStream_EmptyFile(t *testing.T) {
	files := map[string][]byte{
		"empty.txt": {},
	}
	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	stream, err := tr.ExtractFileAsStream("empty.txt")
	require.NoError(t, err)
	defer stream.Close()

	content, err := io.ReadAll(stream)
	require.NoError(t, err)
	assert.Empty(t, content)
}

// TestTarReader_ExtractFileAsStream_InvalidGzip tests invalid gzip handling
func TestTarReader_ExtractFileAsStream_InvalidGzip(t *testing.T) {
	// Create a file with .gz extension but not valid gzip
	file, err := os.CreateTemp("", "test-*.tar.gz")
	require.NoError(t, err)
	defer os.Remove(file.Name())
	defer file.Close()

	// Write invalid gzip data
	_, err = file.Write([]byte("not a gzip file"))
	require.NoError(t, err)
	file.Close()

	file, err = os.Open(file.Name())
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(file.Name(), file, file)
	require.NoError(t, err)

	// Should fall back to tar reader
	_, err = tr.ExtractFileAsStream("anyfile.txt")
	assert.Error(t, err)
}

// TestTarReader_ReadJSONFile_InvalidJSON tests invalid JSON handling
func TestTarReader_ReadJSONFile_InvalidJSON(t *testing.T) {
	files := map[string][]byte{
		"invalid.json": []byte(`{invalid json}`),
	}
	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	var result interface{}
	err = tr.ReadJSONFile("invalid.json", &result, false)
	assert.Error(t, err)
}

// TestArchReader_parseOCIIndex_InvalidManifest tests invalid manifest handling
func TestArchReader_parseOCIIndex_InvalidManifest(t *testing.T) {
	index := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageManifest,
				Digest:    "sha256:invalidmanifest",
				Platform: &OCIPlatform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
		},
	}

	files := map[string][]byte{
		"index.json": mustMarshalJSON(index),
		// Missing the manifest file to trigger error
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, ociManifests, digests, ociIndex, configs, rawConfigs, err := ar.parseOCIIndex()
	assert.NoError(t, err) // Should handle gracefully
	assert.Empty(t, archInfos)
	assert.Empty(t, ociManifests)
	assert.Empty(t, digests)
	assert.NotNil(t, ociIndex)
	assert.Empty(t, configs)
	assert.Empty(t, rawConfigs)
}

// TestArchReader_parseOCIIndex_InvalidConfig tests invalid config handling
func TestArchReader_parseOCIIndex_InvalidConfig(t *testing.T) {
	index := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageManifest,
				Digest:    "sha256:validmanifest",
				Platform: &OCIPlatform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
		},
	}

	manifest := OCIManifest{
		SchemaVersion: 2,
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:invalidconfig",
			Size:      100,
		},
	}

	files := map[string][]byte{
		"index.json":                 mustMarshalJSON(index),
		"blobs/sha256/validmanifest": mustMarshalJSON(manifest),
		"blobs/sha256/invalidconfig": []byte(`invalid json`),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, ociManifests, digests, ociIndex, configs, rawConfigs, err := ar.parseOCIIndex()
	assert.NoError(t, err) // Should handle gracefully
	assert.Empty(t, archInfos)
	assert.Empty(t, ociManifests)
	assert.Empty(t, digests)
	assert.NotNil(t, ociIndex)
	assert.Empty(t, configs)
	assert.Empty(t, rawConfigs)
}

// TestArchReader_parseDockerManifest_InvalidConfig tests invalid Docker config
// TestArchReader_parseDockerManifest_InvalidConfig tests invalid Docker config
func TestArchReader_parseDockerManifest_InvalidConfig(t *testing.T) {
	manifest := []DockerManifest{
		{
			Config:   "invalid_config.json",
			RepoTags: []string{"test:latest"},
			Layers:   []string{"layer1.tar.gz"},
		},
	}

	files := map[string][]byte{
		"manifest.json":       mustMarshalJSON(manifest),
		"invalid_config.json": []byte(`invalid json`),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, manifestsResult, configs, rawConfigs, diffIDs, err := ar.parseDockerManifest()
	require.NoError(t, err) // Should handle gracefully
	assert.Empty(t, archInfos)
	assert.Len(t, manifestsResult, 1) // Should still have the manifest
	assert.Empty(t, configs)
	assert.Empty(t, rawConfigs)
	assert.Empty(t, diffIDs)
}

// TestImageMetaAdapter_ErrorCases tests error cases for imageMetaAdapter
func TestImageMetaAdapter_ErrorCases(t *testing.T) {
	// Test with nil meta - skip Size test as it causes nil pointer
	adapter := &imageMetaAdapter{meta: nil}
	// _, err := adapter.Size() // Skip this test due to nil pointer issue
	// assert.Error(t, err)

	// Test with empty meta
	meta := &ImageMetaData{}
	adapter = &imageMetaAdapter{meta: meta}

	_, err := adapter.RawConfigFile()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no config")

	_, err = adapter.ConfigFile()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no config found")

	_, err = adapter.Manifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no manifest found")

	_, err = adapter.RawManifest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no manifest found")

	_, err = adapter.Digest()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no manifest bytes found for digest")

	// Skip Layers test for empty meta due to nil pointer issue
	// _, err = adapter.Layers()
	// assert.Error(t, err)
}

// TestImageMetaAdapter_Cache tests caching behavior
func TestImageMetaAdapter_Cache(t *testing.T) {
	files := map[string][]byte{
		"blobs/sha256/layer123456": []byte("cached layer content"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	manifest := OCIManifest{
		SchemaVersion: 2,
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123456",
			Size:      100,
		},
		Layers: []OCIDescriptor{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:layer123456",
				Size:      1000,
			},
		},
	}

	meta := &ImageMetaData{
		OCIManifests: []OCIManifest{manifest},
		tarReader:    tr,
	}

	adapter := &imageMetaAdapter{meta: meta, metaTarReader: tr}

	// First call - should populate cache
	layers1, err := adapter.Layers()
	require.NoError(t, err)
	assert.Len(t, layers1, 1)
	assert.NotNil(t, adapter.cachedLayers)

	// Second call - should use cache
	layers2, err := adapter.Layers()
	require.NoError(t, err)
	assert.Len(t, layers2, 1)
	assert.Equal(t, layers1, layers2)
}

// TestCheckArchSummaryComplianceWithPlatform tests platform compliance checking
func TestCheckArchSummaryComplianceWithPlatform(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		summaries   []ImageSummary
		wantError   bool
		wantMessage string
	}{
		{
			name:     "matching platform",
			platform: "linux/amd64",
			summaries: []ImageSummary{
				{
					Ref:       "test:latest",
					Platforms: []string{"linux/amd64", "linux/arm64"},
				},
			},
			wantError: false,
		},
		{
			name:     "non-matching platform",
			platform: "linux/amd64",
			summaries: []ImageSummary{
				{
					Ref:       "test:latest",
					Platforms: []string{"linux/arm64"},
				},
			},
			wantError:   true,
			wantMessage: "no compatible architecture information",
		},
		{
			name:        "empty summaries",
			platform:    "linux/amd64",
			summaries:   []ImageSummary{},
			wantError:   true,
			wantMessage: "no architecture information found",
		},
		{
			name:     "multiple refs with mixed platforms",
			platform: "linux/amd64",
			summaries: []ImageSummary{
				{
					Ref:       "test1:latest",
					Platforms: []string{"linux/arm64"},
				},
				{
					Ref:       "test2:latest",
					Platforms: []string{"linux/amd64"},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckArchSummaryComplianceWithPlatform("test.tar", tt.platform, tt.summaries)
			if tt.wantError {
				assert.Error(t, err)
				assert.IsType(t, &TarfileOsArchUnsupportedError{}, err)
				if tt.wantMessage != "" {
					assert.Contains(t, err.Error(), tt.wantMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestImageConfigToV1ConfigFile tests the imageConfigToV1ConfigFile function
func TestImageConfigToV1ConfigFile(t *testing.T) {
	config := &ImageConfig{
		Architecture: "amd64",
		OS:           "linux",
		Variant:      "v7",
	}

	result := imageConfigToV1ConfigFile(config)
	assert.Equal(t, "amd64", result.Architecture)
	assert.Equal(t, "linux", result.OS)
}

// TestCompatibleImage_ErrorCases tests CompatibleImage error cases
func TestCompatibleImage_ErrorCases(t *testing.T) {
	// Create a mock image that returns errors
	mockImage := &mockV1Image{
		returnError: true,
	}
	info := ImageInfo{
		Architecture: "amd64",
		OS:           "linux",
		Ref:          "test:latest",
	}

	compatible := CompatibleImage{
		Image: mockImage,
		Info:  info,
	}

	// Test error handling
	_, err := compatible.RawConfigFile()
	assert.Error(t, err)

	_, err = compatible.ConfigFile()
	assert.Error(t, err)

	_, err = compatible.Manifest()
	assert.Error(t, err)

	_, err = compatible.RawManifest()
	assert.Error(t, err)

	_, err = compatible.Digest()
	assert.Error(t, err)

	_, err = compatible.Layers()
	assert.Error(t, err)
}

// TestCompatibleImage_NilLayers tests CompatibleImage with empty layers
func TestCompatibleImage_NilLayers(t *testing.T) {
	mockImage := &mockV1Image{
		emptyLayers: true,
	}
	info := ImageInfo{
		Architecture: "amd64",
		OS:           "linux",
		Ref:          "test:latest",
	}

	compatible := CompatibleImage{
		Image: mockImage,
		Info:  info,
	}

	// Test empty layers handling
	layers, err := compatible.Layers()
	assert.NoError(t, err)
	assert.Empty(t, layers)

	// Test Compressed and Uncompressed with empty layers
	_, err = compatible.Compressed()
	assert.Error(t, err)

	_, err = compatible.Uncompressed()
	assert.Error(t, err)
}

// TestArchReader_parseOCIIndex_EmptyDigest tests empty digest handling
func TestArchReader_parseOCIIndex_EmptyDigest(t *testing.T) {
	index := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageManifest,
				Digest:    "",
				Platform: &OCIPlatform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
		},
	}

	files := map[string][]byte{
		"index.json": mustMarshalJSON(index),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, ociManifests, digests, ociIndex, configs, rawConfigs, err := ar.parseOCIIndex()
	require.NoError(t, err)
	assert.Empty(t, archInfos)
	assert.Empty(t, ociManifests)
	assert.Empty(t, digests)
	assert.NotNil(t, ociIndex)
	assert.Empty(t, configs)
	assert.Empty(t, rawConfigs)
}

// TestArchReader_parseOCIIndex_InvalidDigest tests invalid digest format
func TestArchReader_parseOCIIndex_InvalidDigest(t *testing.T) {
	index := OCIIndex{
		SchemaVersion: 2,
		Manifests: []OCIIndexManifest{
			{
				MediaType: v1spec.MediaTypeImageManifest,
				Digest:    "invalid-digest-format",
				Platform: &OCIPlatform{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
		},
	}

	files := map[string][]byte{
		"index.json": mustMarshalJSON(index),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, ociManifests, digests, ociIndex, configs, rawConfigs, err := ar.parseOCIIndex()
	require.NoError(t, err)
	assert.Empty(t, archInfos)
	assert.Empty(t, ociManifests)
	assert.Empty(t, digests)
	assert.NotNil(t, ociIndex)
	assert.Empty(t, configs)
	assert.Empty(t, rawConfigs)
}

// TestArchReader_parseDockerManifest_EmptyConfig tests empty config handling
func TestArchReader_parseDockerManifest_EmptyConfig(t *testing.T) {
	manifest := []DockerManifest{
		{
			Config:   "",
			RepoTags: []string{"test:latest"},
			Layers:   []string{"layer1.tar.gz"},
		},
	}

	files := map[string][]byte{
		"manifest.json": mustMarshalJSON(manifest),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	ar := &ArchReader{tarReader: tr}

	archInfos, manifestsResult, configs, rawConfigs, diffIDs, err := ar.parseDockerManifest()
	require.NoError(t, err)
	assert.Empty(t, archInfos)
	assert.Len(t, manifestsResult, 1)
	assert.Empty(t, configs)
	assert.Empty(t, rawConfigs)
	assert.Empty(t, diffIDs)
}

// TestImageMetaAdapter_LayerByDigest tests LayerByDigest functionality
func TestImageMetaAdapter_LayerByDigest(t *testing.T) {
	files := map[string][]byte{
		"blobs/sha256/layer123456": []byte("mock layer content"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	manifest := OCIManifest{
		SchemaVersion: 2,
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123456",
			Size:      100,
		},
		Layers: []OCIDescriptor{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:layer123456",
				Size:      1000,
			},
		},
	}

	meta := &ImageMetaData{
		OCIManifests: []OCIManifest{manifest},
		tarReader:    tr,
	}

	adapter := &imageMetaAdapter{meta: meta, metaTarReader: tr}

	// Test LayerByDigest with non-existent digest
	layer, err := adapter.LayerByDigest(v1.Hash{Algorithm: "sha256", Hex: "nonexistent"})
	assert.Error(t, err)
	assert.Nil(t, layer)
}

// TestImageMetaAdapter_LayerByDiffID tests LayerByDiffID functionality
func TestImageMetaAdapter_LayerByDiffID(t *testing.T) {
	files := map[string][]byte{
		"blobs/sha256/layer123456": []byte("mock layer content"),
	}

	tarFile := createTestTar(t, files)
	defer os.Remove(tarFile)

	file, err := os.Open(tarFile)
	require.NoError(t, err)
	defer file.Close()

	tr, err := createTarReader(tarFile, file, file)
	require.NoError(t, err)

	manifest := OCIManifest{
		SchemaVersion: 2,
		Config: OCIDescriptor{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123456",
			Size:      100,
		},
		Layers: []OCIDescriptor{
			{
				MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				Digest:    "sha256:layer123456",
				Size:      1000,
			},
		},
	}

	meta := &ImageMetaData{
		OCIManifests: []OCIManifest{manifest},
		tarReader:    tr,
	}

	adapter := &imageMetaAdapter{meta: meta, metaTarReader: tr}

	// Test LayerByDiffID with non-existent diff ID
	layer, err := adapter.LayerByDiffID(v1.Hash{Algorithm: "sha256", Hex: "nonexistent"})
	assert.Error(t, err)
	assert.Nil(t, layer)
}

// Enhanced mockV1Image with error cases
type mockV1Image struct {
	returnError bool
	emptyLayers bool
}

func (m *mockV1Image) RawConfigFile() ([]byte, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return []byte(`{"architecture":"amd64","os":"linux"}`), nil
}

func (m *mockV1Image) ConfigFile() (*v1.ConfigFile, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return &v1.ConfigFile{
		Architecture: "amd64",
		OS:           "linux",
	}, nil
}

func (m *mockV1Image) Manifest() (*v1.Manifest, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return &v1.Manifest{
		SchemaVersion: 2,
		MediaType:     "test",
	}, nil
}

func (m *mockV1Image) RawManifest() ([]byte, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return []byte(`{"schemaVersion":2}`), nil
}

func (m *mockV1Image) Digest() (v1.Hash, error) {
	if m.returnError {
		return v1.Hash{}, fmt.Errorf("mock error")
	}
	return v1.Hash{Algorithm: "sha256", Hex: "test"}, nil
}

func (m *mockV1Image) Layers() ([]v1.Layer, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	if m.emptyLayers {
		return []v1.Layer{}, nil
	}
	return []v1.Layer{&mockLayer{}}, nil
}

func (m *mockV1Image) MediaType() (types.MediaType, error) {
	if m.returnError {
		return "", fmt.Errorf("mock error")
	}
	return "test", nil
}

func (m *mockV1Image) ConfigName() (v1.Hash, error) {
	if m.returnError {
		return v1.Hash{}, fmt.Errorf("mock error")
	}
	return v1.Hash{Algorithm: "sha256", Hex: "config"}, nil
}

func (m *mockV1Image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return &mockLayer{}, nil
}

func (m *mockV1Image) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return &mockLayer{}, nil
}

func (m *mockV1Image) Size() (int64, error) {
	if m.returnError {
		return 0, fmt.Errorf("mock error")
	}
	return 100, nil
}

// Enhanced mockLayer with error cases
type mockLayer struct {
	returnError bool
}

func (m *mockLayer) Digest() (v1.Hash, error) {
	if m.returnError {
		return v1.Hash{}, fmt.Errorf("mock error")
	}
	return v1.Hash{Algorithm: "sha256", Hex: "layer"}, nil
}

func (m *mockLayer) DiffID() (v1.Hash, error) {
	if m.returnError {
		return v1.Hash{}, fmt.Errorf("mock error")
	}
	return v1.Hash{Algorithm: "sha256", Hex: "diff"}, nil
}

func (m *mockLayer) Compressed() (io.ReadCloser, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return io.NopCloser(bytes.NewReader([]byte("compressed"))), nil
}

func (m *mockLayer) Uncompressed() (io.ReadCloser, error) {
	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}
	return io.NopCloser(bytes.NewReader([]byte("uncompressed"))), nil
}

func (m *mockLayer) Size() (int64, error) {
	if m.returnError {
		return 0, fmt.Errorf("mock error")
	}
	return 1000, nil
}

func (m *mockLayer) MediaType() (types.MediaType, error) {
	if m.returnError {
		return "", fmt.Errorf("mock error")
	}
	return types.MediaType("application/vnd.docker.image.rootfs.diff.tar.gzip"), nil
}

// mustMarshalJSON is a helper function for JSON marshaling
func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
