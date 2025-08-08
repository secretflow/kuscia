// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	types "github.com/google/go-containerregistry/pkg/v1/types"
	v1spec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ImageInfo struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
	Ref          string `json:"ref"`
	Source       string `json:"source"`
}

type ImageSummary struct {
	Ref       string   `json:"ref"`
	Platforms []string `json:"platforms"`
}

type OCIIndex struct {
	SchemaVersion int                `json:"schemaVersion"`
	MediaType     string             `json:"mediaType,omitempty"`
	Manifests     []OCIIndexManifest `json:"manifests"`
	Annotations   map[string]string  `json:"annotations,omitempty"`
}

type OCIIndexManifest struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int               `json:"size,omitempty"`
	Platform    *OCIPlatform      `json:"platform,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type OCIPlatform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

type OCIManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType,omitempty"`
	Config        OCIDescriptor     `json:"config"`
	Layers        []OCIDescriptor   `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type OCIDescriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int               `json:"size"`
	URLs        []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type DockerManifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

type ImageConfig struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

type TarReader struct {
	filePath string
	isGzip   bool
}

type TarfileOsArchUnsupportedError struct {
	Source          string
	CurrentPlatform string
	ArchSummary     []ImageSummary
	Cause           error
}

const (
	sha256Prefix                  = "sha256:"
	blobPathPrefix                = "blobs/sha256"
	MediaTypeDockerManifestListV2 = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaTypeDockerManifestV2     = "application/vnd.docker.distribution.manifest.v2+json"
)

func (e *TarfileOsArchUnsupportedError) Error() string {
	return fmt.Sprintf("the image in tarfile %s: %+v, is not accepted by the current OS/Arch: %s, cause: %v", e.Source, e.ArchSummary, e.CurrentPlatform, e.Cause)
}

func (e *TarfileOsArchUnsupportedError) Unwrap() error {
	return e.Cause
}

func createTarReader(filePath string) *TarReader {
	isGzip := strings.HasSuffix(strings.ToLower(filePath), ".gz")
	isTarGzip := strings.HasSuffix(strings.ToLower(filePath), ".tgz")
	return &TarReader{
		filePath: filePath,
		isGzip:   isGzip || isTarGzip,
	}
}

func (tr *TarReader) ExtractFileToMemory(filePath string) ([]byte, error) {
	file, ioError := os.Open(tr.filePath)
	if ioError != nil {
		return nil, fmt.Errorf("failed to open file: %v", ioError)
	}
	defer file.Close()

	filePath = filepath.ToSlash(filePath)
	filePath = strings.TrimPrefix(filePath, "./")

	createTarFileReader := func() (*tar.Reader, io.Closer, error) {
		if _, err := file.Seek(0, 0); err != nil {
			return nil, nil, fmt.Errorf("failed to seek file: %v", err)
		}

		if tr.isGzip {
			// Try gzip first
			gzReader, err := gzip.NewReader(file)
			if err != nil {
				// If gzip header invalid, fallback to tar
				if err == gzip.ErrHeader || strings.Contains(err.Error(), "invalid header") {
					// Fallback: treat as tar
					if _, err := file.Seek(0, 0); err != nil {
						return nil, nil, fmt.Errorf("failed to seek file for tar fallback: %v", err)
					}
					return tar.NewReader(file), nil, nil
				}
				return nil, nil, fmt.Errorf("failed to create gzip reader: %v", err)
			}
			return tar.NewReader(gzReader), gzReader, nil
		}
		return tar.NewReader(file), nil, nil
	}

	tarfileReader, closer, err := createTarFileReader()
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer.Close()
	}

	for {
		header, err := tarfileReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %v", err)
		}

		entryPath := filepath.ToSlash(header.Name)
		entryPath = strings.TrimPrefix(entryPath, "./")

		if entryPath == filePath {
			if header.Typeflag != tar.TypeReg {
				return nil, fmt.Errorf("file %s is not a regular file", entryPath)
			}

			data := make([]byte, header.Size)
			n, err := io.ReadFull(tarfileReader, data)
			if err != nil {
				return nil, fmt.Errorf("failed to read file content: %v", err)
			}
			if int64(n) != header.Size {
				return nil, fmt.Errorf("read %d bytes, expected %d", n, header.Size)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("file %s not found in tar archive", filePath)
}

func (tr *TarReader) ReadJSONFile(filePath string, v interface{}) error {
	data, err := tr.ExtractFileToMemory(filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

type ArchReader struct {
	tarReader *TarReader
}

func (ar *ArchReader) parseOCIIndex() ([]ImageInfo, error) {
	var archInfos []ImageInfo

	var parseOCIIndexRecursive func(indexPath string, parentRef string) error
	parseOCIIndexRecursive = func(indexPath string, parentRef string) error {
		var index OCIIndex
		err := ar.tarReader.ReadJSONFile(indexPath, &index)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", indexPath, err)
		}
		for _, manifest := range index.Manifests {
			refName := parentRef
			if manifest.Annotations != nil {
				if v := manifest.Annotations["org.opencontainers.image.ref.name"]; v != "" {
					refName = v
				} else if v := manifest.Annotations["io.containerd.image.name"]; v != "" {
					refName = v
				}
			}
			if manifest.Platform != nil {
				// Directly read platform field of manifest
				archInfos = append(archInfos, ImageInfo{
					Architecture: manifest.Platform.Architecture,
					OS:           manifest.Platform.OS,
					Variant:      manifest.Platform.Variant,
					Ref:          refName,
					Source:       fmt.Sprintf("%s manifest platform", indexPath),
				})
				continue
			}
			if manifest.Digest == "" || !strings.HasPrefix(manifest.Digest, sha256Prefix) {
				continue
			}
			digestID := strings.TrimPrefix(manifest.Digest, sha256Prefix)
			blobPath := fmt.Sprintf("%s/%s", blobPathPrefix, digestID)
			switch manifest.MediaType {
			case v1spec.MediaTypeImageIndex, MediaTypeDockerManifestListV2:
				// Nested index, recursively process
				if err := parseOCIIndexRecursive(blobPath, refName); err != nil {
					nlog.Warnf("Failed to recursively parse nested index: %v", err)
					continue
				}
			case v1spec.MediaTypeImageManifest, MediaTypeDockerManifestV2:
				// Manifest type, read config
				var ociManifest OCIManifest
				if err := ar.tarReader.ReadJSONFile(blobPath, &ociManifest); err != nil {
					nlog.Warnf("Failed to read manifest: %v", err)
					continue
				}
				configDigestID := strings.TrimPrefix(ociManifest.Config.Digest, sha256Prefix)
				configBlobPath := fmt.Sprintf("%s/%s", blobPathPrefix, configDigestID)
				var config ImageConfig
				if err := ar.tarReader.ReadJSONFile(configBlobPath, &config); err != nil {
					nlog.Warnf("Failed to read config: %v", err)
					continue
				}
				archInfos = append(archInfos, ImageInfo{
					Architecture: config.Architecture,
					OS:           config.OS,
					Variant:      config.Variant,
					Ref:          refName,
					Source:       fmt.Sprintf("%s manifest config", indexPath),
				})
			default:
				nlog.Warnf("Unsupported mediaType: %s", manifest.MediaType)
			}
		}
		return nil
	}

	// 从index.json递归解析
	if err := parseOCIIndexRecursive("index.json", ""); err != nil {
		return nil, err
	}
	return archInfos, nil
}

func (ar *ArchReader) parseDockerManifest() ([]ImageInfo, error) {
	var archInfos []ImageInfo
	var manifests []DockerManifest

	err := ar.tarReader.ReadJSONFile("manifest.json", &manifests)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %v", err)
	}

	for _, manifest := range manifests {
		if manifest.Config == "" {
			continue
		}
		var refName string
		if len(manifest.RepoTags) > 0 {
			refName = manifest.RepoTags[0]
		}
		var configFile string
		if strings.HasPrefix(manifest.Config, "sha256:") {
			// Compatible with config file named as "sha256:xxxx"
			configFile = manifest.Config
		} else {
			configFile = manifest.Config
		}
		var config ImageConfig
		err = ar.tarReader.ReadJSONFile(configFile, &config)
		if err != nil {
			nlog.Warnf("Failed to read or parse docker image config '%s' for repo tags %v: %v", configFile, manifest.RepoTags, err)
			continue
		}
		archInfos = append(archInfos, ImageInfo{
			Architecture: config.Architecture,
			OS:           config.OS,
			Variant:      config.Variant,
			Ref:          refName,
			Source:       "docker manifest.json config",
		})
	}

	return archInfos, nil
}

type ImageMetaData struct {
	OCIIndex        *OCIIndex
	DockerManifests []DockerManifest
	OCIManifests    []OCIManifest
	Configs         []ImageConfig
	RawConfigs      [][]byte
}

type imageMetaAdapter struct {
	meta *ImageMetaData
}

func (a *imageMetaAdapter) RawConfigFile() ([]byte, error) {
	if len(a.meta.RawConfigs) > 0 {
		return a.meta.RawConfigs[0], nil
	}
	return nil, errors.New("no config")
}
func (a *imageMetaAdapter) ConfigFile() (*v1.ConfigFile, error) {
	// 优先使用 OCI config
	if len(a.meta.RawConfigs) > 0 {
		var cfg v1.ConfigFile
		if err := json.Unmarshal(a.meta.RawConfigs[0], &cfg); err == nil {
			return &cfg, nil
		}
	}
	// 兼容 Docker config
	if len(a.meta.Configs) > 0 {
		return imageConfigToV1ConfigFile(&a.meta.Configs[0]), nil
	}
	return nil, errors.New("no config found")
}

// 公共辅助函数：ImageConfig 转 v1.ConfigFile
func imageConfigToV1ConfigFile(cfg *ImageConfig) *v1.ConfigFile {
	return &v1.ConfigFile{
		Architecture: cfg.Architecture,
		OS:           cfg.OS,
	}
}
func (a *imageMetaAdapter) Manifest() (*v1.Manifest, error) {
	// 优先使用 OCI manifest
	if len(a.meta.OCIManifests) > 0 {
		oci := a.meta.OCIManifests[0]
		m := &v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.MediaType(oci.MediaType),
			Config: v1.Descriptor{
				MediaType: types.MediaType(oci.Config.MediaType),
				Digest:    v1.Hash{Algorithm: "sha256", Hex: strings.TrimPrefix(oci.Config.Digest, "sha256:")},
				Size:      int64(oci.Config.Size),
			},
		}
		// layers
		for _, l := range oci.Layers {
			m.Layers = append(m.Layers, v1.Descriptor{
				MediaType: types.MediaType(l.MediaType),
				Digest:    v1.Hash{Algorithm: "sha256", Hex: strings.TrimPrefix(l.Digest, "sha256:")},
				Size:      int64(l.Size),
			})
		}
		return m, nil
	}
	// 兼容 Docker manifest
	if len(a.meta.DockerManifests) > 0 {
		docker := a.meta.DockerManifests[0]
		m := &v1.Manifest{
			SchemaVersion: 2,
			MediaType:     MediaTypeDockerManifestV2,
			Config: v1.Descriptor{
				MediaType: MediaTypeDockerManifestV2,
				Digest:    v1.Hash{}, // Docker manifest没有digest字段
				Size:      0,
			},
		}
		for range docker.Layers {
			m.Layers = append(m.Layers, v1.Descriptor{
				MediaType: MediaTypeDockerManifestV2,
				Digest:    v1.Hash{}, // Docker manifest没有digest字段
				Size:      0,
			})
		}
		return m, nil
	}
	return nil, errors.New("no manifest found")
}
func (a *imageMetaAdapter) RawManifest() ([]byte, error) { return nil, errors.New("not implemented") }

type minimalLayer struct {
	v1.Descriptor
}

func (l *minimalLayer) Digest() (v1.Hash, error)            { return l.Descriptor.Digest, nil }
func (l *minimalLayer) MediaType() (types.MediaType, error) { return l.Descriptor.MediaType, nil }
func (l *minimalLayer) Size() (int64, error)                { return l.Descriptor.Size, nil }
func (l *minimalLayer) DiffID() (v1.Hash, error)            { return v1.Hash{}, errors.New("not implemented") }
func (l *minimalLayer) Compressed() (io.ReadCloser, error)  { return nil, errors.New("not implemented") }
func (l *minimalLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (a *imageMetaAdapter) Layers() ([]v1.Layer, error) {
	var layers []v1.Layer
	// 优先 OCI
	if len(a.meta.OCIManifests) > 0 {
		oci := a.meta.OCIManifests[0]
		for _, l := range oci.Layers {
			desc := v1.Descriptor{
				MediaType: types.MediaType(l.MediaType),
				Digest:    v1.Hash{Algorithm: "sha256", Hex: strings.TrimPrefix(l.Digest, "sha256:")},
				Size:      int64(l.Size),
			}
			layers = append(layers, &minimalLayer{Descriptor: desc})
		}
		return layers, nil
	}
	// 兼容 Docker
	if len(a.meta.DockerManifests) > 0 {
		docker := a.meta.DockerManifests[0]
		for range docker.Layers {
			desc := v1.Descriptor{
				MediaType: MediaTypeDockerManifestV2,
				Digest:    v1.Hash{}, // 无法获取digest
				Size:      0,
			}
			layers = append(layers, &minimalLayer{Descriptor: desc})
		}
		return layers, nil
	}
	return nil, errors.New("no layers found")
}
func (a *imageMetaAdapter) MediaType() (types.MediaType, error) {
	m, err := a.Manifest()
	if err != nil {
		return "", err
	}
	return m.MediaType, nil
}
func (a *imageMetaAdapter) Digest() (v1.Hash, error) { return v1.Hash{}, errors.New("not implemented") }
func (a *imageMetaAdapter) ConfigName() (v1.Hash, error) {
	m, err := a.Manifest()
	if err != nil {
		return v1.Hash{}, err
	}
	return m.Config.Digest, nil
}
func (a *imageMetaAdapter) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	layers, err := a.Layers()
	if err != nil {
		return nil, err
	}
	for _, l := range layers {
		d, err := l.Digest()
		if err == nil && d == h {
			return l, nil
		}
	}
	return nil, errors.New("layer not found")
}
func (a *imageMetaAdapter) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	return nil, errors.New("not implemented")
}
func (a *imageMetaAdapter) Size() (int64, error) {
	layers, err := a.Layers()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, l := range layers {
		sz, err := l.Size()
		if err == nil {
			total += sz
		}
	}
	return total, nil
}

func ImageInFile(tarFile string) ([]ImageSummary, v1.Image, error) {
	tr := createTarReader(tarFile)
	meta := &ImageMetaData{}

	archReader := &ArchReader{tarReader: tr}
	ociArchInfos, _ := archReader.parseOCIIndex()

	var dockerManifests []DockerManifest
	if err := tr.ReadJSONFile("manifest.json", &dockerManifests); err == nil {
		meta.DockerManifests = dockerManifests
	}

	for _, m := range meta.DockerManifests {
		if m.Config != "" {
			var config ImageConfig
			var raw []byte
			if b, err := tr.ExtractFileToMemory(m.Config); err == nil {
				raw = b
				_ = json.Unmarshal(b, &config)
				meta.Configs = append(meta.Configs, config)
				meta.RawConfigs = append(meta.RawConfigs, raw)
			}
		}
	}

	img := &imageMetaAdapter{meta: meta}

	result := make([]ImageSummary, 0)
	refMap := make(map[string][]string)

	for _, info := range ociArchInfos {
		osArch := fmt.Sprintf("%s/%s", info.OS, info.Architecture)
		refMap[info.Ref] = append(refMap[info.Ref], osArch)
	}
	for _, m := range meta.DockerManifests {
		var refName string
		if len(m.RepoTags) > 0 {
			refName = m.RepoTags[0]
		}
		for _, config := range meta.Configs {
			osArch := fmt.Sprintf("%s/%s", config.OS, config.Architecture)
			refMap[refName] = append(refMap[refName], osArch)
		}
	}
	for ref, osArchs := range refMap {
		unique := make(map[string]struct{})
		platforms := make([]string, 0)
		for _, osArch := range osArchs {
			if _, ok := unique[osArch]; !ok {
				unique[osArch] = struct{}{}
				platforms = append(platforms, osArch)
			}
		}
		result = append(result, ImageSummary{
			Ref:       ref,
			Platforms: platforms,
		})
	}

	return result, img, nil
}

func CheckOsArchCompliance(tarFile string) error {
	currentPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	currentPlatform = strings.ToLower(currentPlatform)
	return CheckOsArchComplianceWithPlatform(tarFile, currentPlatform)
}

func CheckArchSummaryComplianceWithPlatform(tarFile string, expectedOsArch string, archSummary []ImageSummary) error {
	if len(archSummary) == 0 {
		return &TarfileOsArchUnsupportedError{
			Source:          tarFile,
			CurrentPlatform: expectedOsArch,
			ArchSummary:     archSummary,
			Cause:           errors.New("no architecture information found in the image tar file"),
		}
	}
	if len(archSummary) > 1 {
		return &TarfileOsArchUnsupportedError{
			Source:          tarFile,
			CurrentPlatform: expectedOsArch,
			ArchSummary:     archSummary,
			Cause:           errors.New("only one image tag is allowed per load command"),
		}
	}
	isCompatible := false
	for _, osArch := range archSummary[0].Platforms {
		if osArch == expectedOsArch {
			isCompatible = true
			break
		}
	}

	if !isCompatible {
		return &TarfileOsArchUnsupportedError{
			Source:          tarFile,
			CurrentPlatform: expectedOsArch,
			ArchSummary:     archSummary,
			Cause:           errors.New("unsupported platform"),
		}
	}

	return nil
}

func CheckOsArchComplianceWithPlatform(tarFile string, expectedOsArch string) error {
	archSummary, _, err := ImageInFile(tarFile)
	if err != nil {
		nlog.Warnf("Error reading image tar file: %s, error: %v", tarFile, err)
		return err
	}
	return CheckArchSummaryComplianceWithPlatform(tarFile, expectedOsArch, archSummary)
}
