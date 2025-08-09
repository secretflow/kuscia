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
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	Variant      string   `json:"variant,omitempty"`
	DiffIDs      []string `json:"diff_ids,omitempty"`
}

/**
 * TarReader provides methods to extract files from tar/gzip streams, with cache for non-layer blobs.
 * The file handle is passed in from the caller and its lifecycle is managed externally.
 */
type TarReader struct {
	filePath          string
	isGzip            bool
	nonLayerBlobCache map[string][]byte
	file              io.ReadSeeker // Use io.ReadSeeker for compatibility with various stream types
	closer            io.Closer     // Optional closer for external resource management
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
	MediaTypeDockerConfigV1       = "application/vnd.docker.container.image.v1+json"
	MediaTypeDockerLayerGzip      = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	MediaTypeDockerLayerTar       = "application/vnd.docker.image.rootfs.diff.tar"
	indexJSONFile                 = "index.json"
	manifestJSONFile              = "manifest.json"
	maxOCIRecursiveDepth          = 8 // Prevent stack overflow in recursive index parsing
)

func (e *TarfileOsArchUnsupportedError) Error() string {
	return fmt.Sprintf("the image in tarfile %s: %+v, is not accepted by the current OS/Arch: %s, cause: %v", e.Source, e.ArchSummary, e.CurrentPlatform, e.Cause)
}

func (e *TarfileOsArchUnsupportedError) Unwrap() error {
	return e.Cause
}

/**
 * createTarReader creates a TarReader from an external stream.
 * The caller is responsible for opening and closing the file/stream.
 */
func createTarReader(filePath string, reader io.ReadSeeker, closer io.Closer) (*TarReader, error) {
	isGzip := strings.HasSuffix(strings.ToLower(filePath), ".gz")
	isTarGzip := strings.HasSuffix(strings.ToLower(filePath), ".tgz")
	return &TarReader{
		filePath:          filePath,
		isGzip:            isGzip || isTarGzip,
		nonLayerBlobCache: make(map[string][]byte),
		file:              reader,
		closer:            closer,
	}, nil
}

/**
 * Close the underlying resource if a closer is provided.
 */
func (tr *TarReader) Close() error {
	if tr.closer != nil {
		return tr.closer.Close()
	}
	return nil
}

/**
 * streamCloser only closes the gzip/tar reader, does not close the shared file handle.
 */
type streamCloser struct {
	io.Reader
	closer io.Closer
}

func (sc *streamCloser) Close() error {
	if sc.closer != nil {
		return sc.closer.Close()
	}
	return nil
}

func (tr *TarReader) ExtractFileAsStream(filePath string) (io.ReadCloser, error) {
	filePath = filepath.ToSlash(filePath)
	filePath = strings.TrimPrefix(filePath, "./")

	// Reuse the file handle held by TarReader
	file := tr.file
	if file == nil {
		return nil, fmt.Errorf("ExtractFileAsStream: tar file not opened")
	}

	createTarFileReader := func() (*tar.Reader, io.Closer, error) {
		if _, err := file.Seek(0, 0); err != nil {
			return nil, nil, fmt.Errorf("ExtractFileAsStream: failed to seek tar file '%s': %v", tr.filePath, err)
		}
		if tr.isGzip {
			gzReader, err := gzip.NewReader(file)
			if err != nil {
				if err == gzip.ErrHeader || strings.Contains(err.Error(), "invalid header") {
					if _, seekErr := file.Seek(0, 0); seekErr != nil {
						return nil, nil, fmt.Errorf("ExtractFileAsStream: failed to seek tar file '%s' for tar fallback: %v", tr.filePath, seekErr)
					}
					return tar.NewReader(file), nil, nil
				}
				return nil, nil, fmt.Errorf("ExtractFileAsStream: failed to create gzip reader '%s': %v", tr.filePath, err)
			}
			return tar.NewReader(gzReader), gzReader, nil
		}
		return tar.NewReader(file), nil, nil
	}

	tarfileReader, closer, err := createTarFileReader()
	if err != nil {
		return nil, fmt.Errorf("ExtractFileAsStream: failed to create tar reader '%s': %v", tr.filePath, err)
	}

	for {
		header, err := tarfileReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if closer != nil {
				closer.Close()
			}
			return nil, fmt.Errorf("ExtractFileAsStream: failed to read tar entry '%s': %v", tr.filePath, err)
		}
		entryPath := filepath.ToSlash(header.Name)
		entryPath = strings.TrimPrefix(entryPath, "./")
		if entryPath == filePath {
			if header.Typeflag != tar.TypeReg {
				if closer != nil {
					closer.Close()
				}
				return nil, fmt.Errorf("ExtractFileAsStream: file '%s' is not a regular file", entryPath)
			}
			// Limit read length to prevent tar.Reader from reading beyond the file
			return &streamCloser{
				Reader: io.LimitReader(tarfileReader, header.Size),
				closer: closer,
			}, nil
		}
	}
	if closer != nil {
		closer.Close()
	}
	return nil, fmt.Errorf("ExtractFileAsStream: file '%s' not found in tar '%s'", filePath, tr.filePath)
}

func (tr *TarReader) ReadJSONFile(filePath string, v interface{}, shouldCache bool) error {
	filePath = filepath.ToSlash(filePath)
	filePath = strings.TrimPrefix(filePath, "./")
	if shouldCache {
		if data, ok := tr.nonLayerBlobCache[filePath]; ok {
			return json.Unmarshal(data, v)
		}
	}
	stream, err := tr.ExtractFileAsStream(filePath)
	if err != nil {
		return fmt.Errorf("ReadJSONFile: failed to stream read file '%s': %v", filePath, err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return fmt.Errorf("ReadJSONFile: failed to read file content '%s': %v", filePath, err)
	}
	if shouldCache {
		tr.nonLayerBlobCache[filePath] = data
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("ReadJSONFile: failed to parse JSON '%s': %v", filePath, err)
	}
	return nil
}

type ArchReader struct {
	tarReader *TarReader
}

func (ar *ArchReader) parseOCIIndex() ([]ImageInfo, []OCIManifest, *OCIIndex, []ImageConfig, [][]byte, error) {
	var archInfos []ImageInfo
	var ociManifests []OCIManifest
	var ociIndex *OCIIndex
	var configs []ImageConfig
	var rawConfigs [][]byte

	err := ar.parseOCIIndexRecursive(indexJSONFile, "", 0, &archInfos, &ociManifests, &ociIndex, &configs, &rawConfigs)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return archInfos, ociManifests, ociIndex, configs, rawConfigs, nil
}

func (ar *ArchReader) parseOCIIndexRecursive(indexPath string, parentRef string, depth int, archInfos *[]ImageInfo, ociManifests *[]OCIManifest, ociIndex **OCIIndex, configs *[]ImageConfig, rawConfigs *[][]byte) error {
	// Recursively parse OCI index, ensure archInfos, ociManifests, configs, rawConfigs are in corresponding order
	if depth > maxOCIRecursiveDepth {
		return fmt.Errorf("parseOCIIndexRecursive: max recursive depth exceeded: %d for indexPath '%s'", depth, indexPath)
	}
	var index OCIIndex
	err := ar.tarReader.ReadJSONFile(indexPath, &index, true)
	if err != nil {
		return fmt.Errorf("parseOCIIndexRecursive: failed to read index file '%s': %v", indexPath, err)
	}

	if *ociIndex == nil && indexPath == indexJSONFile {
		*ociIndex = &index
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
		if manifest.Digest == "" || !strings.HasPrefix(manifest.Digest, sha256Prefix) {
			continue
		}
		digestID := strings.TrimPrefix(manifest.Digest, sha256Prefix)
		blobPath := fmt.Sprintf("%s/%s", blobPathPrefix, digestID)
		switch manifest.MediaType {
		case v1spec.MediaTypeImageIndex, MediaTypeDockerManifestListV2:
			// Recursively parse nested index
			if err := ar.parseOCIIndexRecursive(blobPath, refName, depth+1, archInfos, ociManifests, ociIndex, configs, rawConfigs); err != nil {
				nlog.Warnf("Failed to recursively parse nested index: %v", err)
				continue
			}
		case v1spec.MediaTypeImageManifest, MediaTypeDockerManifestV2:
			var ociManifest OCIManifest
			if err := ar.tarReader.ReadJSONFile(blobPath, &ociManifest, true); err != nil {
				nlog.Warnf("Failed to read manifest: %v", err)
				continue
			}
			configDigestID := strings.TrimPrefix(ociManifest.Config.Digest, sha256Prefix)
			configBlobPath := fmt.Sprintf("%s/%s", blobPathPrefix, configDigestID)
			var b []byte
			if data, ok := ar.tarReader.nonLayerBlobCache[configBlobPath]; ok {
				b = data
			} else {
				stream, streamErr := ar.tarReader.ExtractFileAsStream(configBlobPath)
				if streamErr != nil {
					nlog.Warnf("Failed to extract OCI image config '%s': %v", configBlobPath, streamErr)
					continue
				}
				b, err = io.ReadAll(stream)
				stream.Close()
				if err != nil {
					nlog.Warnf("Failed to read OCI image config '%s': %v", configBlobPath, err)
					continue
				}
				ar.tarReader.nonLayerBlobCache[configBlobPath] = b
			}
			var config ImageConfig
			if err := json.Unmarshal(b, &config); err != nil {
				nlog.Warnf("Failed to unmarshal OCI image config '%s': %v", configBlobPath, err)
				continue
			}
			*archInfos = append(*archInfos, ImageInfo{
				Architecture: config.Architecture,
				OS:           config.OS,
				Variant:      config.Variant,
				Ref:          refName,
				Source:       fmt.Sprintf("%s manifest config", indexPath),
			})
			*ociManifests = append(*ociManifests, ociManifest)
			*configs = append(*configs, config)
			*rawConfigs = append(*rawConfigs, b)
		default:
			nlog.Warnf("Unsupported mediaType: %s", manifest.MediaType)
		}
	}
	return nil
}

func (ar *ArchReader) parseDockerManifest() ([]ImageInfo, []DockerManifest, []ImageConfig, [][]byte, []string, error) {
	var archInfos []ImageInfo
	var manifests []DockerManifest
	var configs []ImageConfig
	var rawConfigs [][]byte
	var diffIDs []string

	err := ar.tarReader.ReadJSONFile(manifestJSONFile, &manifests, true)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to read %s: %v", manifestJSONFile, err)
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
		configFile = manifest.Config
		var b []byte
		if data, ok := ar.tarReader.nonLayerBlobCache[configFile]; ok {
			b = data
		} else {
			stream, err := ar.tarReader.ExtractFileAsStream(configFile)
			if err != nil {
				nlog.Warnf("Failed to extract docker image config '%s' for repo tags %v: %v", configFile, manifest.RepoTags, err)
				continue
			}
			b, err = io.ReadAll(stream)
			stream.Close()
			if err != nil {
				nlog.Warnf("Failed to read docker image config '%s' for repo tags %v: %v", configFile, manifest.RepoTags, err)
				continue
			}
			ar.tarReader.nonLayerBlobCache[configFile] = b
		}
		var config ImageConfig
		if err := json.Unmarshal(b, &config); err != nil {
			nlog.Warnf("Failed to unmarshal docker image config '%s' for repo tags %v: %v", configFile, manifest.RepoTags, err)
			continue
		}
		var cfgFile struct {
			RootFS struct {
				DiffIDs []string `json:"diff_ids"`
			} `json:"rootfs"`
		}
		if err := json.Unmarshal(b, &cfgFile); err != nil {
			nlog.Warnf("Failed to unmarshal diff_ids from docker image config '%s' for repo tags %v: %v", configFile, manifest.RepoTags, err)
		}
		config.DiffIDs = cfgFile.RootFS.DiffIDs
		configs = append(configs, config)
		rawConfigs = append(rawConfigs, b)
		diffIDs = append(diffIDs, cfgFile.RootFS.DiffIDs...)

		archInfos = append(archInfos, ImageInfo{
			Architecture: config.Architecture,
			OS:           config.OS,
			Variant:      config.Variant,
			Ref:          refName,
			Source:       "docker manifest.json config",
		})
	}
	return archInfos, manifests, configs, rawConfigs, diffIDs, nil
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
}

type CompatibleImage struct {
	Image v1.Image
	Info  ImageInfo
}

func (c CompatibleImage) RawConfigFile() ([]byte, error) {
	return c.Image.RawConfigFile()
}

func (c CompatibleImage) ConfigFile() (*v1.ConfigFile, error) {
	return c.Image.ConfigFile()
}

func (c CompatibleImage) Manifest() (*v1.Manifest, error) {
	return c.Image.Manifest()
}

func (c CompatibleImage) RawManifest() ([]byte, error) {
	return c.Image.RawManifest()
}

func (c CompatibleImage) Digest() (v1.Hash, error) {
	return c.Image.Digest()
}

func (c CompatibleImage) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	return c.Image.LayerByDiffID(h)
}

func (c CompatibleImage) Compressed() (io.ReadCloser, error) {
	if img, ok := c.Image.(interface{ Compressed() (io.ReadCloser, error) }); ok {
		return img.Compressed()
	}
	// fallback: compatible with go-containerregistry v1.Image default implementation
	layers, err := c.Image.Layers()
	if err != nil {
		return nil, err
	}
	if len(layers) == 0 {
		return nil, errors.New("no layers found for compressed")
	}
	return layers[0].Compressed()
}

func (c CompatibleImage) Uncompressed() (io.ReadCloser, error) {
	if img, ok := c.Image.(interface{ Uncompressed() (io.ReadCloser, error) }); ok {
		return img.Uncompressed()
	}
	// fallback: compatible with go-containerregistry v1.Image default implementation
	layers, err := c.Image.Layers()
	if err != nil {
		return nil, err
	}
	if len(layers) == 0 {
		return nil, errors.New("no layers found for uncompressed")
	}
	return layers[0].Uncompressed()
}

func (c CompatibleImage) Layers() ([]v1.Layer, error) {
	return c.Image.Layers()
}

func (c CompatibleImage) MediaType() (types.MediaType, error) {
	return c.Image.MediaType()
}

func (c CompatibleImage) ConfigName() (v1.Hash, error) {
	return c.Image.ConfigName()
}

func (c CompatibleImage) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	return c.Image.LayerByDigest(h)
}

func (c CompatibleImage) Size() (int64, error) {
	return c.Image.Size()
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
	// Only use metadata, do not read config content.
	if len(a.meta.DockerManifests) > 0 {
		docker := a.meta.DockerManifests[0]
		var configDigest v1.Hash
		var configSize int64
		var configMediaType types.MediaType = MediaTypeDockerConfigV1
		// Only use metadata, do not read config content.
		if len(docker.Config) > 0 {
			// Use file name as digest (not guaranteed to be consistent, but avoids extraction).
			configDigest = v1.Hash{Algorithm: strings.TrimSuffix(sha256Prefix, ":"), Hex: strings.TrimSuffix(docker.Config, ".json")}
			configSize = 0 // Do not read actual size.
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
			// Only use metadata, do not read layer content.
			layerDigest := v1.Hash{Algorithm: strings.TrimSuffix(sha256Prefix, ":"), Hex: strings.TrimSuffix(layerPath, ".tar.gz")}
			layerSize := int64(0)
			layerMediaType := types.MediaType(MediaTypeDockerLayerGzip)
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
				Digest:    v1.Hash{Algorithm: strings.TrimSuffix(sha256Prefix, ":"), Hex: strings.TrimPrefix(oci.Config.Digest, sha256Prefix)},
				Size:      int64(oci.Config.Size),
			},
		}
		for _, l := range oci.Layers {
			// Only use metadata, do not read layer content.
			m.Layers = append(m.Layers, v1.Descriptor{
				MediaType: types.MediaType(l.MediaType),
				Digest:    v1.Hash{Algorithm: strings.TrimSuffix(sha256Prefix, ":"), Hex: strings.TrimPrefix(l.Digest, sha256Prefix)},
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
	layers, err := a.Layers()
	if err != nil || len(layers) == 0 {
		return v1.Hash{}, errors.New("no layers found for digest")
	}
	return layers[0].Digest()
}

func (a *imageMetaAdapter) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
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

func (a *imageMetaAdapter) Compressed() (io.ReadCloser, error) {
	layers, err := a.Layers()
	if err != nil || len(layers) == 0 {
		return nil, errors.New("no layers found for compressed")
	}
	return layers[0].Compressed()
}

func (a *imageMetaAdapter) Uncompressed() (io.ReadCloser, error) {
	layers, err := a.Layers()
	if err != nil || len(layers) == 0 {
		return nil, errors.New("no layers found for uncompressed")
	}
	return layers[0].Uncompressed()
}

func (a *imageMetaAdapter) Layers() ([]v1.Layer, error) {
	if a.cachedLayers != nil {
		return a.cachedLayers, nil
	}
	var layers []v1.Layer
	if len(a.meta.OCIManifests) > 0 {
		oci := a.meta.OCIManifests[0]
		for _, l := range oci.Layers {
			stream, err := a.meta.tarReader.ExtractFileAsStream(fmt.Sprintf("%s/%s", blobPathPrefix, strings.TrimPrefix(l.Digest, sha256Prefix)))
			if err != nil {
				return nil, err
			}
			layer, err := tarball.LayerFromReader(stream, tarball.WithMediaType(types.MediaType(l.MediaType)))
			if err != nil {
				stream.Close()
				return nil, err
			}
			layers = append(layers, layer)
			stream.Close()
		}
		a.cachedLayers = layers
		return layers, nil
	}
	if len(a.meta.DockerManifests) > 0 {
		docker := a.meta.DockerManifests[0]
		for _, layerPath := range docker.Layers {
			stream, err := a.meta.tarReader.ExtractFileAsStream(layerPath)
			if err != nil {
				return nil, err
			}
			var mediaType types.MediaType
			if strings.HasSuffix(layerPath, ".tar.gz") {
				mediaType = types.MediaType("application/vnd.docker.image.rootfs.diff.tar.gzip")
			} else if strings.HasSuffix(layerPath, ".tar") {
				mediaType = types.MediaType("application/vnd.docker.image.rootfs.diff.tar")
			} else {
				mediaType = types.MediaType("application/vnd.docker.image.rootfs.diff.tar.gzip")
			}
			layer, err := tarball.LayerFromReader(stream, tarball.WithMediaType(mediaType))
			if err != nil {
				stream.Close()
				return nil, err
			}
			layers = append(layers, layer)
			stream.Close()
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
	// Proxy go-containerregistry v1.Image Size() method: sum all layer sizes.
	if a == nil {
		return 0, errors.New("imageMetaAdapter is nil")
	}
	layers, err := a.Layers()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, l := range layers {
		size, err := l.Size()
		if err != nil {
			return 0, fmt.Errorf("Size: failed to get layer size: %v", err)
		}
		total += size
	}
	return total, nil
}

/**
 * ImageInFile parses the image tarball using an external stream.
 * The caller is responsible for opening and closing the file/stream.
 */
func ImageInFile(filePath string, reader io.ReadSeeker, closer io.Closer) ([]ImageSummary, []CompatibleImage, error) {
	tr, ioErr := createTarReader(filePath, reader, closer)
	if ioErr != nil {
		return nil, nil, fmt.Errorf("Create tar reader failed: %v", ioErr)
	}
	// Do not close tr (and file) here; the caller is responsible for closing the file after all image/layer operations are done.
	meta := &ImageMetaData{tarReader: tr}
	archReader := &ArchReader{tarReader: tr}

	var allArchInfos []ImageInfo
	var allImages []v1.Image
	var compatibleImages []CompatibleImage

	dockerArchInfos, dockerManifests, dockerConfigs, dockerRawConfigs, dockerDiffIDs, dockerErr := archReader.parseDockerManifest()
	if dockerErr == nil && len(dockerArchInfos) > 0 {
		for idx, info := range dockerArchInfos {
			allArchInfos = append(allArchInfos, info)
			selectedMeta := &ImageMetaData{
				tarReader:       meta.tarReader,
				DockerManifests: []DockerManifest{dockerManifests[idx]},
				RawConfigs:      [][]byte{dockerRawConfigs[idx]},
				Configs:         []ImageConfig{dockerConfigs[idx]},
				DiffIDs:         dockerDiffIDs,
			}
			allImages = append(allImages, &imageMetaAdapter{meta: selectedMeta, metaTarReader: tr})
		}
	}

	ociArchInfos, ociManifests, ociIndex, ociConfigs, ociRawConfigs, ociErr := archReader.parseOCIIndex()
	if ociErr == nil && len(ociArchInfos) > 0 {
		meta.OCIIndex = ociIndex
		meta.OCIManifests = ociManifests
		meta.Configs = ociConfigs
		meta.RawConfigs = ociRawConfigs
		count := len(ociManifests)
		for idx := 0; idx < count; idx++ {
			var info ImageInfo
			if idx < len(ociArchInfos) {
				info = ociArchInfos[idx]
			}
			allArchInfos = append(allArchInfos, info)
			selectedMeta := &ImageMetaData{
				tarReader:    meta.tarReader,
				OCIManifests: []OCIManifest{ociManifests[idx]},
			}
			if idx < len(ociRawConfigs) {
				selectedMeta.RawConfigs = [][]byte{ociRawConfigs[idx]}
			}
			if idx < len(ociConfigs) {
				selectedMeta.Configs = []ImageConfig{ociConfigs[idx]}
			}
			allImages = append(allImages, &imageMetaAdapter{meta: selectedMeta, metaTarReader: tr})
		}
	}

	if len(allArchInfos) == 0 && dockerErr != nil && ociErr != nil {
		return nil, nil, fmt.Errorf("failed to parse image tarball: not a valid docker or oci image format. docker error: %v, oci error: %v", dockerErr, ociErr)
	}

	summaryMap := make(map[string]*ImageSummary)
	for _, info := range allArchInfos {
		osArch := fmt.Sprintf("%s/%s", strings.ToLower(info.OS), strings.ToLower(info.Architecture))
		if info.Ref == "" {
			continue
		}
		if summaryMap[info.Ref] == nil {
			summaryMap[info.Ref] = &ImageSummary{
				Ref:       info.Ref,
				Platforms: []string{osArch},
			}
		} else {
			// Merge multi-platform images
			summaryMap[info.Ref].Platforms = append(summaryMap[info.Ref].Platforms, osArch)
		}
	}
	var allSummaries []ImageSummary
	for _, v := range summaryMap {
		allSummaries = append(allSummaries, *v)
	}

	// Filter image objects by current platform and wrap as CompatibleImage struct
	currentPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	currentPlatform = strings.ToLower(currentPlatform)
	for idx, info := range allArchInfos {
		osArch := fmt.Sprintf("%s/%s", strings.ToLower(info.OS), strings.ToLower(info.Architecture))
		if osArch == currentPlatform {
			compatibleImages = append(compatibleImages, CompatibleImage{
				Image: allImages[idx],
				Info:  info,
			})
		}
	}

	return allSummaries, compatibleImages, nil
}

func CheckOsArchCompliance(tarFile string) error {
	currentPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	currentPlatform = strings.ToLower(currentPlatform)
	return CheckOsArchComplianceWithPlatform(tarFile, currentPlatform)
}

/**
 * CheckOsArchComplianceWithPlatform opens the tar file and checks platform compatibility.
 */
func CheckOsArchComplianceWithPlatform(tarFile string, expectedOsArch string) error {
	file, err := os.Open(tarFile)
	if err != nil {
		nlog.Warnf("Error opening image tar file: %s, error: %v", tarFile, err)
		return err
	}
	defer file.Close()
	archSummary, _, err := ImageInFile(tarFile, file, file)
	if err != nil {
		nlog.Warnf("Error reading image tar file: %s, error: %v", tarFile, err)
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
			Cause:           errors.New("no compatible architecture information found in the image tar file"),
		}
	}

	isCompatible := false
	for _, summary := range archSummary {
		var unsupported []string
		for _, osArch := range summary.Platforms {
			if osArch == expectedOsArch {
				isCompatible = true
			} else {
				unsupported = append(unsupported, osArch)
			}
		}
		if len(unsupported) > 0 {
			nlog.Warnf("Some incompatibilities were found. Current platform [%s], image [%s] supports platforms: %v, unsupported platforms : %v", expectedOsArch, summary.Ref, summary.Platforms, unsupported)
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
