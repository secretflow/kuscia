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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type LegacyImageInput struct {
	RepoTags     []string
	Config       string
	OS           string
	Architecture string
}

type legacyImageInternal struct {
	LegacyImageInput
	Layers  []string
	DiffIDs []string
}

type ociManifest struct {
	SchemaVersion int                     `json:"schemaVersion"`
	MediaType     string                  `json:"mediaType,omitempty"`
	Manifests     []ociManifestDescriptor `json:"manifests"`
}

type ociManifestDescriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int               `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform    *PlatformInfo     `json:"platform,omitempty"`
}

type PlatformInfo struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}
type archImage struct {
	cfgHex   string
	layerHex string
	manHex   string
	cfgBytes []byte
	layerBuf []byte
	manBytes []byte
	arch     string
	os       string
}

const tempImageTestDir = "kuscia-temp-image-test"

func writeTarFile(w *tar.Writer, name string, content []byte) error {
	hdr := &tar.Header{
		Name:     name,
		Mode:     0644,
		Size:     int64(len(content)),
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}
	if err := w.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := w.Write(content)
	return err
}

func randomDigest(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func buildLayerTarGz(content string) (layerBytes []byte, diffID string, err error) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	hdr := &tar.Header{
		Name:    "message.txt",
		Mode:    0644,
		Size:    int64(len(content)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, "", err
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return nil, "", err
	}
	if err := tw.Close(); err != nil {
		return nil, "", err
	}

	uncompressedData := tarBuf.Bytes()
	hash := sha256.Sum256(uncompressedData)
	diffID = "sha256:" + hex.EncodeToString(hash[:])

	var gzBuf bytes.Buffer
	gzw := gzip.NewWriter(&gzBuf)
	if _, err := gzw.Write(uncompressedData); err != nil {
		return nil, "", err
	}
	if err := gzw.Close(); err != nil {
		return nil, "", err
	}

	return gzBuf.Bytes(), diffID, nil
}
func buildArchImage(osArch string) (*archImage, error) {
	parts := strings.Split(osArch, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid os/arch: %s", osArch)
	}
	osStr, archStr := parts[0], parts[1]

	layerBytes, diffID, err := buildLayerTarGz(fmt.Sprintf("Hello from %s/%s", osStr, archStr))
	if err != nil {
		return nil, err
	}
	layerDigest := sha256.Sum256(layerBytes)

	cfg := map[string]interface{}{
		"architecture": archStr,
		"os":           osStr,
		"rootfs": map[string]interface{}{
			"type":     "layers",
			"diff_ids": []string{diffID},
		},
		"config": map[string]interface{}{},
	}
	cfgContent, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	cfgDigest := sha256.Sum256(cfgContent)

	manifest := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config": map[string]interface{}{
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest":    "sha256:" + hex.EncodeToString(cfgDigest[:]),
			"size":      len(cfgContent),
		},
		"layers": []map[string]interface{}{
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"digest":    "sha256:" + hex.EncodeToString(layerDigest[:]),
				"size":      len(layerBytes),
			},
		},
	}
	manBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	manDigest := sha256.Sum256(manBytes)

	return &archImage{
		cfgHex:   hex.EncodeToString(cfgDigest[:]),
		layerHex: hex.EncodeToString(layerDigest[:]),
		manHex:   hex.EncodeToString(manDigest[:]),
		cfgBytes: cfgContent,
		layerBuf: layerBytes,
		manBytes: manBytes,
		arch:     archStr,
		os:       osStr,
	}, nil
}

func createFakeLayerTar(fileName, content string) ([]byte, string, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	data := []byte(content)
	hdr := &tar.Header{
		Name: fileName,
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, "", err
	}
	if _, err := tw.Write(data); err != nil {
		return nil, "", err
	}
	tw.Close()
	diffID := sha256.Sum256(buf.Bytes())
	diffIDStr := fmt.Sprintf("sha256:%x", diffID[:])
	return buf.Bytes(), diffIDStr, nil
}

func createDockerLegacyTarball(outputPath string, images []LegacyImageInput) error {
	tempDir, err := os.MkdirTemp("", "legacy-docker")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	manifest := []map[string]interface{}{}
	repoMap := map[string]map[string]string{}

	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)

	for imgIdx, imgInput := range images {
		image := legacyImageInternal{LegacyImageInput: imgInput}
		image.Layers = []string{}
		image.DiffIDs = []string{}

		for i := 0; i < 2; i++ {
			layerName := fmt.Sprintf("%s-layer-%d.tar", randomDigest(fmt.Sprintf("img-%d-%s-%d", imgIdx, strings.Join(imgInput.RepoTags, ","), i)), i)
			tarContent, diffID, err := createFakeLayerTar("hello.txt", fmt.Sprintf("hello world %d\n", i))
			if err != nil {
				return err
			}

			image.Layers = append(image.Layers, layerName)
			image.DiffIDs = append(image.DiffIDs, diffID)

			if err := writeTarFile(tarWriter, layerName, tarContent); err != nil {
				return err
			}
		}

		configName := imgInput.Config
		if configName == "" {
			configName = randomDigest(fmt.Sprintf("img-%d-config", imgIdx)) + ".json"
		}

		configContent := map[string]interface{}{
			"architecture": image.Architecture,
			"os":           image.OS,
			"config": map[string]interface{}{
				"Env":        []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
				"Entrypoint": []string{"docker-entrypoint.sh"},
			},
			"rootfs": map[string]interface{}{
				"type":     "layers",
				"diff_ids": image.DiffIDs,
			},
		}
		configBytes, jsonReadErr := json.Marshal(configContent)
		if jsonReadErr != nil {
			return jsonReadErr
		}
		if err := writeTarFile(tarWriter, configName, configBytes); err != nil {
			return err
		}

		manifest = append(manifest, map[string]interface{}{
			"Config":   configName,
			"RepoTags": imgInput.RepoTags,
			"Layers":   image.Layers,
		})

		for _, tag := range imgInput.RepoTags {
			parts := strings.SplitN(tag, ":", 2)
			if len(parts) != 2 {
				continue
			}
			repo := parts[0]
			tagName := parts[1]
			if _, ok := repoMap[repo]; !ok {
				repoMap[repo] = map[string]string{}
			}
			repoMap[repo][tagName] = configName
		}
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := writeTarFile(tarWriter, "manifest.json", manifestBytes); err != nil {
		return err
	}

	repoBytes, _ := json.MarshalIndent(repoMap, "", "  ")
	if err := writeTarFile(tarWriter, "repositories", repoBytes); err != nil {
		return err
	}

	tarWriter.Close()
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

func createMultiArchOCIImageFile(tarFile string, archTags [][]string, emitPlatformForIndexManifest bool) error {
	tempRoot, err := os.MkdirTemp("", "oci-tmp-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	if err := os.MkdirAll(filepath.Join(tempRoot, "blobs", "sha256"), 0755); err != nil {
		return err
	}

	tagToImages := map[string][]*archImage{}

	for _, pair := range archTags {
		osArch := pair[0]
		tag := pair[1]

		img, err := buildArchImage(osArch)
		if err != nil {
			return fmt.Errorf("buildArchImage failed: %w", err)
		}

		for _, file := range []struct {
			name string
			data []byte
		}{
			{img.cfgHex, img.cfgBytes},
			{img.layerHex, img.layerBuf},
			{img.manHex, img.manBytes},
		} {
			if err := os.WriteFile(filepath.Join(tempRoot, "blobs", "sha256", file.name), file.data, 0644); err != nil {
				return err
			}
		}

		tagToImages[tag] = append(tagToImages[tag], img)
	}

	var topIndex ociManifest
	topIndex.SchemaVersion = 2

	for tag, imgs := range tagToImages {
		if len(imgs) == 1 {
			img := imgs[0]
			desc := ociManifestDescriptor{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:" + img.manHex,
				Size:      len(img.manBytes),
				Annotations: map[string]string{
					"org.opencontainers.image.ref.name": tag,
					"io.containerd.image.name":          tag,
				},
			}
			if emitPlatformForIndexManifest {
				desc.Platform = &PlatformInfo{
					Architecture: img.arch,
					OS:           img.os,
				}
			}
			topIndex.Manifests = append(topIndex.Manifests, desc)
		} else {
			var subIndex ociManifest
			subIndex.SchemaVersion = 2
			for _, img := range imgs {
				subIndex.Manifests = append(subIndex.Manifests, ociManifestDescriptor{
					MediaType: "application/vnd.oci.image.manifest.v1+json",
					Digest:    "sha256:" + img.manHex,
					Size:      len(img.manBytes),
					Platform: &PlatformInfo{
						Architecture: img.arch,
						OS:           img.os,
					},
				})
			}
			subBytes, _ := json.Marshal(subIndex)
			subDigest := sha256.Sum256(subBytes)
			subHex := hex.EncodeToString(subDigest[:])
			subPath := filepath.Join(tempRoot, "blobs", "sha256", subHex)
			if err := os.WriteFile(subPath, subBytes, 0644); err != nil {
				return err
			}
			topIndex.Manifests = append(topIndex.Manifests, ociManifestDescriptor{
				MediaType: "application/vnd.oci.image.index.v1+json",
				Digest:    "sha256:" + subHex,
				Size:      len(subBytes),
				Annotations: map[string]string{
					"org.opencontainers.image.ref.name": tag,
					"io.containerd.image.name":          tag,
				},
			})
		}
	}

	idxBytes, _ := json.MarshalIndent(topIndex, "", "  ")
	if err := os.WriteFile(filepath.Join(tempRoot, "index.json"), idxBytes, 0644); err != nil {
		return err
	}
	layoutJSON := []byte(`{"imageLayoutVersion":"1.0.0"}`)
	if err := os.WriteFile(filepath.Join(tempRoot, "oci-layout"), layoutJSON, 0644); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(tarFile), 0755); err != nil {
		return err
	}
	f, err := os.Create(tarFile)
	if err != nil {
		return err
	}
	defer f.Close()
	tarw := tar.NewWriter(f)
	defer tarw.Close()

	return filepath.Walk(tempRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(tempRoot, path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = relPath
		if err := tarw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = io.Copy(tarw, file)
		return err
	})
}

func createSingleArchOCIImageFile(tarFile string, osArch string, tag string) error {
	tempRoot, err := os.MkdirTemp("", "oci-tmp-single-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	if err := os.MkdirAll(filepath.Join(tempRoot, "blobs", "sha256"), 0755); err != nil {
		return err
	}

	img, err := buildArchImage(osArch)
	if err != nil {
		return fmt.Errorf("buildArchImage failed: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tempRoot, "blobs", "sha256", img.cfgHex), img.cfgBytes, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tempRoot, "blobs", "sha256", img.layerHex), img.layerBuf, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tempRoot, "blobs", "sha256", img.manHex), img.manBytes, 0644); err != nil {
		return err
	}

	index := ociManifest{
		SchemaVersion: 2,
		Manifests: []ociManifestDescriptor{
			{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:" + img.manHex,
				Size:      len(img.manBytes),
				Annotations: map[string]string{
					"org.opencontainers.image.ref.name": tag,
					"io.containerd.image.name":          tag,
				},
			},
		},
	}
	idxBytes, _ := json.Marshal(index)
	if err := os.WriteFile(filepath.Join(tempRoot, "index.json"), idxBytes, 0644); err != nil {
		return err
	}
	layoutJSON := []byte(`{"imageLayoutVersion":"1.0.0"}`)
	if err := os.WriteFile(filepath.Join(tempRoot, "oci-layout"), layoutJSON, 0644); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(tarFile), 0755); err != nil {
		return err
	}
	f, err := os.Create(tarFile)
	if err != nil {
		return err
	}
	defer f.Close()
	tarw := tar.NewWriter(f)
	defer tarw.Close()

	return filepath.Walk(tempRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(tempRoot, path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = relPath
		if err := tarw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = io.Copy(tarw, file)
		return err
	})
}

func TestReadOsArchFromImageTarFile_DockerLegacySingleArmImage(t *testing.T) {
	t.Parallel()
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "docker-legacy-single-arm64.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	img := LegacyImageInput{
		RepoTags:     []string{"example.com/docker-legacy-single:1.0.0_arm64"},
		Config:       randomDigest("config") + ".json",
		Architecture: "arm64",
		OS:           "linux",
	}
	err := createDockerLegacyTarball(tarPath, []LegacyImageInput{img})
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
	assert.True(t, slices.Contains(archSummary[0].Platforms, "linux/arm64"))
}

func TestReadOsArchFromImageTarFile_DockerLegacySingleAmdImage(t *testing.T) {
	t.Parallel()
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "docker-legacy-single-amd64.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	img := LegacyImageInput{
		RepoTags:     []string{"example.com/docker-legacy-single:1.0.0_amd64"},
		Config:       randomDigest("config") + ".json",
		Architecture: "amd64",
		OS:           "linux",
	}
	err := createDockerLegacyTarball(tarPath, []LegacyImageInput{img})
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
	assert.True(t, slices.Contains(archSummary[0].Platforms, "linux/amd64"))
}

func TestReadOsArchFromImageTarFile_OCISingleAmdImage(t *testing.T) {
	t.Parallel()
	archs := "linux/amd64"
	tag := "example.com/oci-single:1.0.0_amd64"
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "oci-single-amd64.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)

	err := createSingleArchOCIImageFile(tarPath, archs, tag)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
	assert.True(t, slices.Contains(archSummary[0].Platforms, "linux/amd64"))
}

func TestReadOsArchFromImageTarFile_OCISingleArmImage(t *testing.T) {
	t.Parallel()
	archs := "linux/arm64"
	tag := "example.com/oci-single:1.0.0_arm64"
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "oci-single-arm64.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	err := createSingleArchOCIImageFile(tarPath, archs, tag)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
	assert.True(t, slices.Contains(archSummary[0].Platforms, "linux/arm64"))
}

func TestReadOsArchFromImageTarFile_OCISinglePpcImage(t *testing.T) {
	t.Parallel()
	archs := "linux/ppc64le"
	tag := "example.com/oci-single:1.0.0_ppc64le"
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "oci-single-ppc64le.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	err := createSingleArchOCIImageFile(tarPath, archs, tag)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
	assert.True(t, slices.Contains(archSummary[0].Platforms, "linux/ppc64le"))
}

func TestReadOsArchFromImageTarFile_OCISingleTarGzipImage(t *testing.T) {
	t.Parallel()
	archs := "linux/arm64"
	tag := "example.com/oci-single-tar-gz:1.0.0_arm64"
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "oci-single-tar-gz-arm64.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	err := createSingleArchOCIImageFile(tarPath, archs, tag)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	tarFile, _ := os.Open(tarPath)
	defer tarFile.Close()

	gzfileName := "oci-single-tar-gz-arm64.tar.gz"
	tarGzPath := filepath.Join(tempImageOutputDir, gzfileName)
	gzFile, err := os.Create(tarGzPath)
	assert.NoError(t, err, fmt.Sprintf("create gzip file %s failed", gzfileName))
	defer gzFile.Close()

	gzWriter := gzip.NewWriter(gzFile)
	defer gzWriter.Close()

	_, err = io.Copy(gzWriter, tarFile)
	assert.NoError(t, err, fmt.Sprintf("copy tar file into %s failed", gzfileName))
	err = gzWriter.Close()
	assert.NoError(t, err, fmt.Sprintf("close gzip writer for %s failed", gzfileName))
	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", gzfileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", gzfileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
	assert.True(t, slices.Contains(archSummary[0].Platforms, "linux/arm64"))
}

func TestReadOsArchFromImageTarFile_OCIMultiArch(t *testing.T) {
	t.Parallel()
	archs := [][]string{
		{"linux/amd64", "example.com/oci-multi-arch:1.0.0"},
		{"linux/arm64", "example.com/oci-multi-arch:1.0.0"},
	}
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "oci-multi-arch.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	err := createMultiArchOCIImageFile(tarPath, archs, false)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
	assert.True(t, slices.Contains(archSummary[0].Platforms, "linux/amd64") && slices.Contains(archSummary[0].Platforms, "linux/arm64"))

}

func TestReadOsArchFromImageTarFile_OCIMultiImage(t *testing.T) {
	t.Parallel()
	archs := [][]string{
		{"linux/amd64", "example.com/oci-multi-image:1.0.0"},
		{"linux/arm64", "example.com/oci-multi-image:1.0.0"},
		{"linux/amd64", "example.com/oci-multi-image:1.0.1"},
	}
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "oci-muti-image.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	err := createMultiArchOCIImageFile(tarPath, archs, false)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.True(t, len(archSummary) == 2, fmt.Sprintf("tarball %s should contains two image repo tags", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
}

func TestReadOsArchFromImageTarFile_DockerLegacyMultiImage(t *testing.T) {
	t.Parallel()
	firstImage := LegacyImageInput{
		RepoTags:     []string{"example.com/legacy-multi-image:1.0"},
		Config:       randomDigest("firstImage-config") + ".json",
		OS:           "linux",
		Architecture: "amd64",
	}
	secondImage := LegacyImageInput{
		RepoTags:     []string{"example.com/legacy-multi-image:2.0"},
		Config:       randomDigest("secondImage-config") + ".json",
		OS:           "linux",
		Architecture: "amd64",
	}
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName := "legacy-muti-image.tar"
	tarPath := filepath.Join(tempImageOutputDir, fileName)
	err := createDockerLegacyTarball(tarPath, []LegacyImageInput{firstImage, secondImage})
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	archSummary, err := ReadOsArchFromImageTarFile(tarPath)
	assert.NoError(t, err, fmt.Sprintf("read os and arch info from tarball %s failed", fileName))

	assert.True(t, len(archSummary) > 0, fmt.Sprintf("tarball %s does not found any valid manifest", fileName))
	assert.True(t, len(archSummary) == 2, fmt.Sprintf("tarball %s should contains two image repo tags", fileName))
	assert.NotEmpty(t, archSummary[0].Platforms, "read platform failed")
}

func TestValidateArch_SingleArchSupported(t *testing.T) {
	t.Parallel()
	var arch, fileName, tarPath, tag, currentPlatform string
	currentPlatform = "linux/arm64"

	arch = "linux/arm64"
	tag = "example.com/oci-single:1.0.0_arm64"
	tempArmImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempArmImageOutputDir)
	fileName = "oci-single-arm64.tar"
	tarPath = filepath.Join(tempArmImageOutputDir, fileName)
	createArmImageErr := createSingleArchOCIImageFile(tarPath, arch, tag)
	assert.NoError(t, createArmImageErr, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	unsupportedErr := ValidateArch(tarPath, currentPlatform)
	assert.NoError(t, unsupportedErr, fmt.Sprintf("validate os/arch from tarfile %s failed", fileName))
}

func TestValidateArch_SingleArchUnsupported(t *testing.T) {
	t.Parallel()
	var arch, fileName, tarPath, tag, currentPlatform string
	currentPlatform = "linux/arm64"

	arch = "linux/amd64"
	tag = "example.com/oci-single:1.0.0_amd64"
	tempAmdImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempAmdImageOutputDir)
	fileName = "oci-single-amd64.tar"
	tarPath = filepath.Join(tempAmdImageOutputDir, fileName)
	createAmdImageErr := createSingleArchOCIImageFile(tarPath, arch, tag)
	assert.NoError(t, createAmdImageErr, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	unsupportedErr := ValidateArch(tarPath, currentPlatform)
	assert.Error(t, unsupportedErr, "expected an error but got none")
	if _, ok := unsupportedErr.(*TarfileOsArchUnsupportedError); !ok {
		assert.Fail(t, "validate os/arch from tarfile %s failed, error should be of type *TarfileOsArchUnsupportedError")
	}
}

func TestValidateArch_MultiArchSupported(t *testing.T) {
	t.Parallel()
	var fileName, tarPath, currentPlatform string
	currentPlatform = "linux/amd64"

	archs := [][]string{
		{"linux/amd64", "example.com/oci-multi-arch:1.0.0"},
		{"linux/arm64", "example.com/oci-multi-arch:1.0.0"},
	}
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName = "oci-multi-arch.tar"
	tarPath = filepath.Join(tempImageOutputDir, fileName)
	err := createMultiArchOCIImageFile(tarPath, archs, false)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	unsupportedErr := ValidateArch(tarPath, currentPlatform)
	assert.NoError(t, unsupportedErr, fmt.Sprintf("validate os/arch from tarfile %s failed", fileName))

}

func TestValidateArch_MultiArchUnSupported(t *testing.T) {
	t.Parallel()
	var fileName, tarPath, currentPlatform string
	currentPlatform = "linux/amd64"

	archs := [][]string{
		{"linux/ppc64le", "example.com/oci-multi-arch:1.0.0"},
		{"linux/arm64", "example.com/oci-multi-arch:1.0.0"},
	}
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName = "oci-multi-arch.tar"
	tarPath = filepath.Join(tempImageOutputDir, fileName)
	err := createMultiArchOCIImageFile(tarPath, archs, false)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	unsupportedErr := ValidateArch(tarPath, currentPlatform)
	assert.Error(t, unsupportedErr, "expected an error but got none")
	t.Logf("unsupportedErr: %v", unsupportedErr)
	if _, ok := unsupportedErr.(*TarfileOsArchUnsupportedError); !ok {
		assert.Fail(t, "validate os/arch from tarfile %s failed, error should be of type *TarfileOsArchUnsupportedError")
	}
}
func TestValidateArch_MultiImage(t *testing.T) {
	t.Parallel()
	var fileName, tarPath, currentPlatform string
	currentPlatform = "linux/amd64"

	archs := [][]string{
		{"linux/amd64", "example.com/oci-multi-arch:1.0.0"},
		{"linux/amd64", "example.com/oci-multi-arch:2.0.0"},
	}
	tempImageOutputDir, tempErr := os.MkdirTemp("", tempImageTestDir)
	assert.NoError(t, tempErr)
	defer os.RemoveAll(tempImageOutputDir)
	fileName = "oci-multi-arch.tar"
	tarPath = filepath.Join(tempImageOutputDir, fileName)
	err := createMultiArchOCIImageFile(tarPath, archs, false)
	assert.NoError(t, err, fmt.Sprintf("create tarball %s failed", fileName))
	assert.FileExistsf(t, tarPath, fmt.Sprintf("expected tarball %s not found", fileName))

	unsupportedErr := ValidateArch(tarPath, currentPlatform)
	assert.Error(t, unsupportedErr, "expected an error but got none")
	t.Logf("unsupportedErr: %v", unsupportedErr)
	if _, ok := unsupportedErr.(*TarfileOsArchUnsupportedError); !ok {
		assert.Fail(t, "validate os/arch from tarfile %s failed, error should be of type *TarfileOsArchUnsupportedError")
	}
}
