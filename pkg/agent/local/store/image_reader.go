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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	types "github.com/google/go-containerregistry/pkg/v1/types"
	v1spec "github.com/opencontainers/image-spec/specs-go/v1"
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
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	Variant      string   `json:"variant,omitempty"`
	DiffIDs      []string `json:"diff_ids,omitempty"`
}

type TarReader struct {
	filePath          string
	isGzip            bool
	nonLayerBlobCache map[string][]byte
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
		filePath:          filePath,
		isGzip:            isGzip || isTarGzip,
		nonLayerBlobCache: make(map[string][]byte),
	}
}

func (tr *TarReader) ExtractFileToMemory(filePath string, shouldCache bool) ([]byte, error) {
	filePath = filepath.ToSlash(filePath)
	filePath = strings.TrimPrefix(filePath, "./")

	isNonLayerBlob := func(path string) bool {
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".json") ||
			strings.Contains(lower, "manifest") ||
			strings.Contains(lower, "index") ||
			strings.Contains(lower, "config") {
			return true
		}
		return false
	}

	if shouldCache || isNonLayerBlob(filePath) {
		if data, ok := tr.nonLayerBlobCache[filePath]; ok {
			return data, nil
		}
	}

	file, ioError := os.Open(tr.filePath)
	if ioError != nil {
		return nil, fmt.Errorf("failed to open file: %v", ioError)
	}
	defer file.Close()

	createTarFileReader := func() (*tar.Reader, io.Closer, error) {
		if _, err := file.Seek(0, 0); err != nil {
			return nil, nil, fmt.Errorf("failed to seek file: %v", err)
		}

		if tr.isGzip {
			gzReader, err := gzip.NewReader(file)
			if err != nil {
				if err == gzip.ErrHeader || strings.Contains(err.Error(), "invalid header") {
					if _, seekErr := file.Seek(0, 0); seekErr != nil {
						return nil, nil, fmt.Errorf("failed to seek file for tar fallback: %v", seekErr)
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
			if shouldCache || isNonLayerBlob(filePath) {
				tr.nonLayerBlobCache[filePath] = data
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("file %s not found in tar archive", filePath)
}

func (tr *TarReader) ReadJSONFile(filePath string, v interface{}, shouldCache bool) error {
	data, err := tr.ExtractFileToMemory(filePath, shouldCache)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

type ArchReader struct {
	tarReader *TarReader
}

func (ar *ArchReader) parseOCIIndex() ([]ImageInfo, []OCIManifest, *OCIIndex, error) {
	var archInfos []ImageInfo
	var ociManifests []OCIManifest
	var ociIndex *OCIIndex

	var parseOCIIndexRecursive func(indexPath string, parentRef string) error
	parseOCIIndexRecursive = func(indexPath string, parentRef string) error {
		var index OCIIndex
		err := ar.tarReader.ReadJSONFile(indexPath, &index, true)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", indexPath, err)
		}

		if ociIndex == nil && indexPath == "index.json" {
			ociIndex = &index
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

				if err := parseOCIIndexRecursive(blobPath, refName); err != nil {
					continue
				}
			case v1spec.MediaTypeImageManifest, MediaTypeDockerManifestV2:

				var ociManifest OCIManifest
				if err := ar.tarReader.ReadJSONFile(blobPath, &ociManifest, true); err != nil {
					continue
				}
				ociManifests = append(ociManifests, ociManifest)
				configDigestID := strings.TrimPrefix(ociManifest.Config.Digest, sha256Prefix)
				configBlobPath := fmt.Sprintf("%s/%s", blobPathPrefix, configDigestID)
				var config ImageConfig
				if err := ar.tarReader.ReadJSONFile(configBlobPath, &config, true); err != nil {
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
			}
		}
		return nil
	}

	if err := parseOCIIndexRecursive("index.json", ""); err != nil {
		return nil, nil, nil, err
	}
	return archInfos, ociManifests, ociIndex, nil
}

func (ar *ArchReader) parseDockerManifest() ([]ImageInfo, error) {
	var archInfos []ImageInfo
	var manifests []DockerManifest

	err := ar.tarReader.ReadJSONFile("manifest.json", &manifests, true)
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
			configFile = manifest.Config
		} else {
			configFile = manifest.Config
		}
		var config ImageConfig
		err = ar.tarReader.ReadJSONFile(configFile, &config, true)
		if err != nil {
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
	Layers          []OCIDescriptor
	DiffIDs         []string
	RawManifest     []byte
	tarReader       *TarReader
}

type imageMetaAdapter struct {
	meta          *ImageMetaData
	metaTarReader *TarReader

	cachedLayers []v1.Layer
	cachedDigest *v1.Hash
}

func (a *imageMetaAdapter) RawConfigFile() ([]byte, error) {
	if len(a.meta.RawConfigs) > 0 {
		return a.meta.RawConfigs[0], nil
	}
	return nil, errors.New("no config")
}
func (a *imageMetaAdapter) ConfigFile() (*v1.ConfigFile, error) {

	if len(a.meta.RawConfigs) > 0 {
		var cfg v1.ConfigFile
		if err := json.Unmarshal(a.meta.RawConfigs[0], &cfg); err == nil {
			return &cfg, nil
		}
	}

	if len(a.meta.Configs) > 0 {
		return imageConfigToV1ConfigFile(&a.meta.Configs[0]), nil
	}
	return nil, errors.New("no config found")
}

func imageConfigToV1ConfigFile(cfg *ImageConfig) *v1.ConfigFile {
	return &v1.ConfigFile{
		Architecture: cfg.Architecture,
		OS:           cfg.OS,
	}
}
func (a *imageMetaAdapter) Manifest() (*v1.Manifest, error) {

	if len(a.meta.DockerManifests) > 0 {
		docker := a.meta.DockerManifests[0]

		var configDigest v1.Hash
		var configSize int64
		var configMediaType types.MediaType = "application/vnd.docker.container.image.v1+json"
		if len(docker.Config) > 0 {
			b, err := a.metaTarReader.ExtractFileToMemory(docker.Config, true)
			if err == nil {
				configSize = int64(len(b))
				configDigest, _, _ = v1.SHA256(bytes.NewReader(b))
			}
		}
		m := &v1.Manifest{
			SchemaVersion: 2,
			MediaType:     MediaTypeDockerManifestV2,
			Config: v1.Descriptor{
				MediaType: configMediaType,
				Digest:    configDigest,
				Size:      configSize,
			},
		}

		for _, layerPath := range docker.Layers {
			var layerDigest v1.Hash
			var layerSize int64
			var layerMediaType types.MediaType = "application/vnd.docker.image.rootfs.diff.tar.gzip"
			if len(layerPath) > 0 {
				b, err := a.metaTarReader.ExtractFileToMemory(layerPath, false)
				if err == nil {
					layerSize = int64(len(b))
					layerDigest, _, _ = v1.SHA256(bytes.NewReader(b))
				}
			}
			m.Layers = append(m.Layers, v1.Descriptor{
				MediaType: layerMediaType,
				Digest:    layerDigest,
				Size:      layerSize,
			})
		}
		return m, nil
	}

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
		for _, l := range oci.Layers {
			m.Layers = append(m.Layers, v1.Descriptor{
				MediaType: types.MediaType(l.MediaType),
				Digest:    v1.Hash{Algorithm: "sha256", Hex: strings.TrimPrefix(l.Digest, "sha256:")},
				Size:      int64(l.Size),
			})
		}
		return m, nil
	}
	return nil, errors.New("no manifest found")
}
func (a *imageMetaAdapter) RawManifest() ([]byte, error) {
	if len(a.meta.RawManifest) > 0 {
		return a.meta.RawManifest, nil
	}
	return nil, errors.New("no manifest found")
}

func (a *imageMetaAdapter) Digest() (v1.Hash, error) {
	// 直接代理到 go-containerregistry 的 Layer 实现
	layers, err := a.Layers()
	if err != nil || len(layers) == 0 {
		return v1.Hash{}, errors.New("no layers found for digest")
	}
	// 返回第一个 Layer 的 Digest（如需整体 Digest，可根据 Manifest 计算）
	return layers[0].Digest()
}

func (a *imageMetaAdapter) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	// 直接在 go-containerregistry 的 Layer 列表中查找
	layers, err := a.Layers()
	if err != nil {
		return nil, err
	}
	for _, l := range layers {
		diffID, err := l.DiffID()
		if err == nil && diffID == h {
			return l, nil
		}
	}
	return nil, errors.New("layer not found for diffid")
}

/*
Compressed 返回 layer 的 gzip 压缩内容，兼容 go-containerregistry v1.Layer.Compressed 方法：

- 返回 io.ReadCloser，可流式读取完整 gzip 压缩内容
- 错误处理逻辑与 go-containerregistry 保持一致：找不到 tarReader 或 digest 时直接报错

兼容性说明：该实现与 go-containerregistry/pkg/v1/layer.go 的 Layer.Compressed 方法完全兼容，可安全替换。
*/
func (a *imageMetaAdapter) Compressed() (io.ReadCloser, error) {
	// 直接代理到 go-containerregistry 的 Layer 实现
	layers, err := a.Layers()
	if err != nil || len(layers) == 0 {
		return nil, errors.New("no layers found for compressed")
	}
	return layers[0].Compressed()
}

/*
Uncompressed 返回 layer 的解压（tar）数据流，兼容 go-containerregistry v1.Layer.Uncompressed 方法：

- 输入必须为标准 gzip 压缩的 tar 层，否则会报错（与 go-containerregistry 行为一致）
- 返回 io.ReadCloser，可流式读取完整 tar 内容
- 错误处理与 go-containerregistry 保持一致

兼容性说明：该实现与 go-containerregistry/pkg/v1/layer.go 的 Layer.Uncompressed 方法完全兼容，可安全替换。
*/
func (a *imageMetaAdapter) Uncompressed() (io.ReadCloser, error) {
	// 直接代理到 go-containerregistry 的 Layer 实现
	layers, err := a.Layers()
	if err != nil || len(layers) == 0 {
		return nil, errors.New("no layers found for uncompressed")
	}
	return layers[0].Uncompressed()
}

func (a *imageMetaAdapter) Layers() ([]v1.Layer, error) {
	// 直接用 go-containerregistry 的 tarball.LayerFromReader 构造标准 Layer
	if a.cachedLayers != nil {
		return a.cachedLayers, nil
	}
	var layers []v1.Layer
	if len(a.meta.OCIManifests) > 0 {
		oci := a.meta.OCIManifests[0]
		for _, l := range oci.Layers {
			layerData, err := a.meta.tarReader.ExtractFileToMemory(l.Digest, false)
			if err != nil {
				return nil, err
			}
			layer, err := tarball.LayerFromReader(bytes.NewReader(layerData), tarball.WithMediaType(types.MediaType(l.MediaType)))
			if err != nil {
				return nil, err
			}
			layers = append(layers, layer)
		}
		a.cachedLayers = layers
		return layers, nil
	}
	if len(a.meta.DockerManifests) > 0 {
		docker := a.meta.DockerManifests[0]
		for _, layerPath := range docker.Layers {
			layerData, err := a.meta.tarReader.ExtractFileToMemory(layerPath, false)
			if err != nil {
				return nil, err
			}
			layer, err := tarball.LayerFromReader(bytes.NewReader(layerData), tarball.WithMediaType(types.MediaType("application/vnd.docker.image.rootfs.diff.tar.gzip")))
			if err != nil {
				return nil, err
			}
			layers = append(layers, layer)
		}
		a.cachedLayers = layers
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
func (a *imageMetaAdapter) Size() (int64, error) {

	return 0, errors.New("not implemented")
}

func ImageInFile(tarFile string) ([]ImageSummary, v1.Image, error) {
	tr := createTarReader(tarFile)
	meta := &ImageMetaData{
		tarReader: tr}

	archReader := &ArchReader{tarReader: tr}
	result := make([]ImageSummary, 0)
	refMap := make(map[string][]string)

	dockerErr := tr.ReadJSONFile("manifest.json", &meta.DockerManifests, true)
	if dockerErr == nil && len(meta.DockerManifests) > 0 {
		for _, m := range meta.DockerManifests {
			if m.Config != "" {
				var config ImageConfig
				var raw []byte
				b, err := tr.ExtractFileToMemory(m.Config, true)
				if err != nil {
					continue
				}
				raw = b
				if err := json.Unmarshal(b, &config); err != nil {
					continue
				}

				var cfgFile struct {
					RootFS struct {
						DiffIDs []string `json:"diff_ids"`
					} `json:"rootfs"`
				}
				_ = json.Unmarshal(b, &cfgFile)
				config.DiffIDs = cfgFile.RootFS.DiffIDs
				meta.Configs = append(meta.Configs, config)
				meta.RawConfigs = append(meta.RawConfigs, raw)
				meta.DiffIDs = append(meta.DiffIDs, cfgFile.RootFS.DiffIDs...)
			}
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
	} else {
		ociArchInfos, ociManifests, ociIndex, ociErr := archReader.parseOCIIndex()
		if ociErr == nil {
			meta.OCIIndex = ociIndex
			meta.OCIManifests = ociManifests
			if ociErr == nil && len(ociArchInfos) > 0 {
				for _, info := range ociArchInfos {
					osArch := fmt.Sprintf("%s/%s", info.OS, info.Architecture)
					refMap[info.Ref] = append(refMap[info.Ref], osArch)
				}
				for _, info := range ociArchInfos {
					osArch := fmt.Sprintf("%s/%s", info.OS, info.Architecture)
					refMap[info.Ref] = append(refMap[info.Ref], osArch)
				}

				for _, manifest := range meta.OCIManifests {
					configDigestID := strings.TrimPrefix(manifest.Config.Digest, sha256Prefix)
					configBlobPath := fmt.Sprintf("%s/%s", blobPathPrefix, configDigestID)
					b, err := tr.ExtractFileToMemory(configBlobPath, true)
					if err == nil {
						meta.RawConfigs = append(meta.RawConfigs, b)
						var config ImageConfig
						if err := json.Unmarshal(b, &config); err == nil {
							var cfgFile struct {
								RootFS struct {
									DiffIDs []string `json:"diff_ids"`
								} `json:"rootfs"`
							}
							_ = json.Unmarshal(b, &cfgFile)
							config.DiffIDs = cfgFile.RootFS.DiffIDs
							meta.Configs = append(meta.Configs, config)
							meta.DiffIDs = append(meta.DiffIDs, cfgFile.RootFS.DiffIDs...)
						}
					}
					for _, layer := range manifest.Layers {
						meta.Layers = append(meta.Layers, layer)
					}
				}
			}
		}
	}

	img := &imageMetaAdapter{meta: meta, metaTarReader: tr}

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

func CheckOsArchComplianceWithPlatform(tarFile string, expectedOsArch string) error {
	archSummary, _, err := ImageInFile(tarFile)
	if err != nil {
		return err
	}
	return CheckArchSummaryComplianceWithPlatform(tarFile, expectedOsArch, archSummary)
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

	var incompatible []ImageSummary
	isCompatible := false
	for _, summary := range archSummary {
		for _, osArch := range summary.Platforms {
			if osArch == expectedOsArch {
				isCompatible = true
			} else {
				incompatible = append(incompatible, summary)
			}
		}
	}

	if len(incompatible) > 0 {
		for _, s := range incompatible {
			var unsupported []string
			for _, arch := range s.Platforms {
				if arch != expectedOsArch {
					unsupported = append(unsupported, arch)
				}
			}
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
