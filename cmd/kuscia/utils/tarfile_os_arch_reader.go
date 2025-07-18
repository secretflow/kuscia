package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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

func (e *TarfileOsArchUnsupportedError) Error() string {
	return fmt.Errorf("the image in tarfile %s: %+v, is not accepted by the current OS/Arch: %s, cause: %v", e.Source, e.ArchSummary, e.CurrentPlatform, e.Cause).Error()
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
	file, err := os.Open(tr.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var tarReader *tar.Reader
	if tr.isGzip {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	filePath = filepath.ToSlash(filePath)
	filePath = strings.TrimPrefix(filePath, "./")

	for {
		header, err := tarReader.Next()
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
				return nil, fmt.Errorf("file %s is not a regular file", filePath)
			}

			data, err := ioutil.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read file content: %v", err)
			}
			return data, nil
		}
	}

	file.Seek(0, 0)
	if tr.isGzip {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	lowerFilePath := strings.ToLower(filePath)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %v", err)
		}

		entryPath := filepath.ToSlash(header.Name)
		entryPath = strings.TrimPrefix(entryPath, "./")

		if strings.ToLower(entryPath) == lowerFilePath {
			if header.Typeflag != tar.TypeReg {
				return nil, fmt.Errorf("file %s is not a regular file", entryPath)
			}

			data, err := ioutil.ReadAll(tarReader)
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
	}

	if len(archInfos) == 0 {
		dockerArch, err := ar.parseDockerManifest()
		if err == nil {
			archInfos = append(archInfos, dockerArch...)
		}
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
		// 如果索引中直接包含平台信息
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

		if manifest.Digest == "" || !strings.HasPrefix(manifest.Digest, "sha256:") {
			continue
		}

		digestID := strings.TrimPrefix(manifest.Digest, "sha256:")
		blobPath := fmt.Sprintf("blobs/sha256/%s", digestID)

		switch manifest.MediaType {
		case "application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json":
			var nestedIndex OCIIndex
			err := ar.tarReader.ReadJSONFile(blobPath, &nestedIndex)
			if err == nil {
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
			}
		case "application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json":
			var ociManifest OCIManifest
			err := ar.tarReader.ReadJSONFile(blobPath, &ociManifest)
			if err == nil {
				configDigestID := strings.TrimPrefix(ociManifest.Config.Digest, "sha256:")
				configBlobPath := fmt.Sprintf("blobs/sha256/%s", configDigestID)

				var config ImageConfig
				err := ar.tarReader.ReadJSONFile(configBlobPath, &config)
				if err == nil {
					archInfos = append(archInfos, ArchInfo{
						Architecture: config.Architecture,
						OS:           config.OS,
						Variant:      config.Variant,
						Ref:          refName,
						Source:       fmt.Sprintf("manifest config (%s)", manifest.MediaType),
					})
				}
			}
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

		var config ImageConfig
		err := ar.tarReader.ReadJSONFile(manifest.Config, &config)
		if err == nil {
			archInfos = append(archInfos, ArchInfo{
				Architecture: config.Architecture,
				OS:           config.OS,
				Variant:      config.Variant,
				Ref:          manifest.RepoTags[0],
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
		nlog.Fatalf("Error detecting architecture: %v", err)
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

func ValidateArch(tarFile string, currentPlatform string) error {
	archSummary, err := ReadOsArchFromImageTarFile(tarFile)
	if err != nil {
		nlog.Warnf("Error reading image tar file: %s, error: %v", tarFile, err)
	}
	if len(archSummary) == 0 {
		return &TarfileOsArchUnsupportedError{
			Source:          tarFile,
			CurrentPlatform: currentPlatform,
			ArchSummary:     archSummary,
			Cause:           errors.New("no architecture information found in the image tar file"),
		}
	}
	if len(archSummary) > 1 {
		return &TarfileOsArchUnsupportedError{
			Source:          tarFile,
			CurrentPlatform: currentPlatform,
			ArchSummary:     archSummary,
			Cause:           errors.New("only one image tag is allowed per load command"),
		}
	}
	isCompatible := false
	for _, osArch := range archSummary[0].Platforms {
		normalized := osArch
		if parts := strings.Split(osArch, "/"); len(parts) > 2 {
			normalized = parts[0] + "/" + parts[1]
		}
		if normalized == currentPlatform {
			isCompatible = true
			break
		}
	}

	if !isCompatible {
		return &TarfileOsArchUnsupportedError{
			Source:          tarFile,
			CurrentPlatform: currentPlatform,
			ArchSummary:     archSummary,
			Cause:           errors.New("unsupported platform"),
		}
	}

	return nil
}
