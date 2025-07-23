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

package utils

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

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ArchInfo struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
	Ref          string `json:"ref"`
	Source       string `json:"source"`
}

type ArchSummary struct {
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
	ArchSummary     []ArchSummary
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

	createTarFileReader := func() (*tar.Reader, error) {
		if _, err := file.Seek(0, 0); err != nil {
			return nil, fmt.Errorf("failed to seek file: %v", err)
		}

		if tr.isGzip {
			gzReader, err := gzip.NewReader(file)
			if err != nil {
				return nil, fmt.Errorf("failed to create gzip reader: %v", err)
			}
			return tar.NewReader(gzReader), nil
		}
		return tar.NewReader(file), nil
	}

	tarFileReader, err := createTarFileReader()
	if err != nil {
		return nil, err
	}

	for {
		header, err := tarFileReader.Next()
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

			data, err := io.ReadAll(tarFileReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read file content: %v", err)
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

func createArchReader(tarFilePath string) *ArchReader {
	return &ArchReader{
		tarReader: createTarReader(tarFilePath),
	}
}

func (ar *ArchReader) detectArchitecture() ([]ArchInfo, error) {
	var archInfos []ArchInfo

	ociArch, err := ar.parseOCIIndex()
	if err == nil && len(ociArch) > 0 {
		archInfos = append(archInfos, ociArch...)
	} else if err != nil {
		if len(archInfos) == 0 {
			dockerArch, dockerErr := ar.parseDockerManifest()
			if dockerErr != nil {
				return nil, fmt.Errorf("failed to parse OCI index: %v; and failed to parse Docker manifest: %v", err, dockerErr)
			}
			archInfos = append(archInfos, dockerArch...)
		}
	} else if len(archInfos) == 0 {
		dockerArch, dockerErr := ar.parseDockerManifest()
		if dockerErr != nil {
			return nil, dockerErr
		}
		archInfos = append(archInfos, dockerArch...)
	}
	return archInfos, nil
}

func (ar *ArchReader) parseOCIIndex() ([]ArchInfo, error) {
	var archInfos []ArchInfo
	var index OCIIndex

	err := ar.tarReader.ReadJSONFile("index.json", &index)
	if err != nil {
		return nil, fmt.Errorf("failed to read index.json: %v", err)
	}

	for _, manifest := range index.Manifests {
		var imageRefName, imageName, refName string
		if manifest.Annotations != nil {
			imageRefName = manifest.Annotations["org.opencontainers.image.ref.name"]
			imageName = manifest.Annotations["io.containerd.image.name"]
		}
		if imageRefName != "" {
			refName = imageRefName
		}
		if imageName != "" {
			refName = imageName
		}
		if manifest.Platform != nil {
			archInfos = append(archInfos, ArchInfo{
				Architecture: manifest.Platform.Architecture,
				OS:           manifest.Platform.OS,
				Variant:      manifest.Platform.Variant,
				Ref:          refName,
				Source:       "index.json manifest platform",
			})
			continue
		}

		if manifest.Digest == "" || !strings.HasPrefix(manifest.Digest, sha256Prefix) {
			continue
		}

		digestID := strings.TrimPrefix(manifest.Digest, sha256Prefix)
		blobPath := fmt.Sprintf("%s/%s", blobPathPrefix, digestID)

		switch manifest.MediaType {
		case v1.MediaTypeImageIndex, MediaTypeDockerManifestListV2:
			var nestedIndex OCIIndex
			err := ar.tarReader.ReadJSONFile(blobPath, &nestedIndex)
			if err != nil {
				fmt.Printf("Failed to read nested index: %v\n", err)
				continue
			}
			for _, nestedManifest := range nestedIndex.Manifests {
				if nestedManifest.Platform != nil {
					archInfos = append(archInfos, ArchInfo{
						Architecture: nestedManifest.Platform.Architecture,
						OS:           nestedManifest.Platform.OS,
						Variant:      nestedManifest.Platform.Variant,
						Ref:          refName,
						Source:       fmt.Sprintf("nested manifest index (%s)", manifest.MediaType),
					})
				}
			}
		case v1.MediaTypeImageManifest, MediaTypeDockerManifestV2:
			var ociManifest OCIManifest
			err := ar.tarReader.ReadJSONFile(blobPath, &ociManifest)
			if err != nil {
				fmt.Printf("Failed to read OCI manifest: %v\n", err)
				continue
			}
			configDigestID := strings.TrimPrefix(ociManifest.Config.Digest, sha256Prefix)
			configBlobPath := fmt.Sprintf("%s/%s", blobPathPrefix, configDigestID)

			var config ImageConfig
			err = ar.tarReader.ReadJSONFile(configBlobPath, &config)
			if err != nil {
				fmt.Printf("Failed to read image config: %v\n", err)
				continue
			}
			archInfos = append(archInfos, ArchInfo{
				Architecture: config.Architecture,
				OS:           config.OS,
				Variant:      config.Variant,
				Ref:          refName,
				Source:       fmt.Sprintf("manifest config (%s)", manifest.MediaType),
			})
		default:
			fmt.Printf("Unsupported media type: %s\n", manifest.MediaType)
		}
	}

	return archInfos, nil
}

func (ar *ArchReader) parseDockerManifest() ([]ArchInfo, error) {
	var archInfos []ArchInfo
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
		var config ImageConfig
		err = ar.tarReader.ReadJSONFile(manifest.Config, &config)
		if err == nil {
			archInfos = append(archInfos, ArchInfo{
				Architecture: config.Architecture,
				OS:           config.OS,
				Variant:      config.Variant,
				Ref:          refName,
				Source:       "docker manifest.json config",
			})
		}
	}

	return archInfos, nil
}

func ReadOsArchFromImageTarFile(tarFile string) ([]ArchSummary, error) {
	reader := createArchReader(tarFile)
	archInfos, err := reader.detectArchitecture()
	if err != nil {
		nlog.Errorf("Error detecting architecture: %v", err)
		return nil, err
	}
	result := make([]ArchSummary, 0)

	refMap := make(map[string][]string)
	for _, arch := range archInfos {
		osArch := fmt.Sprintf("%s/%s", arch.OS, arch.Architecture)
		refMap[arch.Ref] = append(refMap[arch.Ref], osArch)
	}

	for ref, osArchs := range refMap {
		result = append(result, ArchSummary{
			Ref:       ref,
			Platforms: osArchs,
		})
	}

	return result, nil
}

func validateImageWithCurrentOsArch(tarFile string) error {
	currentPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	currentPlatform = strings.ToLower(currentPlatform)
	return ValidateArch(tarFile, currentPlatform)
}

func ValidateArch(tarFile string, expectedOsArch string) error {
	archSummary, err := ReadOsArchFromImageTarFile(tarFile)
	if err != nil {
		nlog.Warnf("Error reading image tar file: %s, error: %v", tarFile, err)
		return err
	}
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
