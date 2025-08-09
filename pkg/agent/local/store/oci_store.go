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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/docker/go-units"

	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	defaultRegistry   = "docker.io"
	defaultRepository = "library"
)

type ociStore struct {
	ctx     context.Context
	options []name.Option

	cachePath string

	imagePath layout.Path

	mounter mounter.Mounter

	mutex sync.Mutex

	// pull worker count
	pullWorkers   uint32
	pullingImages map[string]*imageAction
}

func NewOCIStore(rootDir string, mountType mounter.MountType) (Store, error) {
	imagePath := path.Join(rootDir, "repositories")

	lp, err := layout.FromPath(imagePath)
	if err != nil { // reinit 'index.json'
		nlog.Infof("Path not exists, so create and init index.json")
		lp, err = layout.Write(imagePath, empty.Index)
		if err != nil {
			return nil, err
		}
	}

	// used to cache combined images
	cachePath := path.Join(rootDir, "cache")
	if err := paths.EnsureDirectory(cachePath, true); err != nil {
		nlog.Warnf("Create OCIStore cache folder(%s) failed with error: %s", cachePath, err.Error())
		return nil, err
	}

	return &ociStore{
		cachePath:     cachePath,
		imagePath:     lp,
		mutex:         sync.Mutex{},
		mounter:       mounter.NewMounter(mountType),
		pullWorkers:   4,
		pullingImages: make(map[string]*imageAction),
	}, nil
}

// interface [Store]
func (s *ociStore) CheckImageExist(image *kii.ImageName) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	desc, _ := s.findImageFromLocal(image.Image)

	return desc != nil
}

// interface [Store]
func (s *ociStore) ListImage() ([]*Image, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	ii, err := s.imagePath.ImageIndex()
	if err != nil {
		return nil, err
	}

	imf, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}

	images := []*Image{}
	for _, img := range imf.Manifests {
		if name, ok := s.getImageName(img); ok {
			if i, err := kii.NewImageName(name); err == nil {
				size, err := s.getImageSize(img)
				if err != nil {
					nlog.Warnf("Get image [%s] size failed -> %v", name, err)
				}
				images = append(images, &Image{
					Repository: i.Repo,
					Tag:        i.Tag,
					ImageID:    img.Digest.String()[7:19],
					Size:       size,
				})
			}
		}
	}

	return images, nil
}

func (s *ociStore) getImageName(img v1.Descriptor) (string, bool) {
	// {
	//	 "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
	//	 "size": 428,
	//	 "digest": "sha256:529df535fbe5d76b34e5dee5d4dba372362b1a3dae5337888d019f3bbd5c3351",
	//	 "annotations": {
	//	   "org.opencontainers.image.ref.name": "docker.io/secretflow/app:v1.7.0"
	//	 }
	// }
	if img.MediaType == types.DockerManifestSchema2 {
		if name, ok := img.Annotations[oci.AnnotationRefName]; ok {
			return name, true
		}
	}
	return "", false
}

func (s *ociStore) getImageSize(img v1.Descriptor) (string, error) {
	ii, err := s.imagePath.ImageIndex()
	if err != nil {
		return "", err
	}
	image, err := ii.Image(img.Digest)
	if err != nil {
		return "", err
	}
	mf, err := image.Manifest()
	if err != nil {
		return "", err
	}
	var size int64
	size += mf.Config.Size
	for _, layer := range mf.Layers {
		size += layer.Size
	}
	return units.CustomSize("%02.1f %s", float64(size), 1024.0, []string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}), nil
}

// interface [Store]
func (s *ociStore) MountImage(image *kii.ImageName, workingDir, targetDir string) error {
	img, err := s.findImageFromLocal(image.Image)
	if err != nil || img == nil {
		nlog.Warnf("Not found image(%s) or some error(%v) happened", image.Image, err)
		return fmt.Errorf("not found image: %s", image.Image)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("query image(%s) layer failed", image.Image)
	}

	if len(layers) == 0 {
		return s.mounter.Mount([]string{}, workingDir, targetDir)
	}

	if len(layers) != 1 {
		nlog.Warnf("Image(%s) is not 1 layer, can't mount", image.Image)
		return fmt.Errorf("image(%s) is not combined", image.Image)
	}

	digest, err := layers[0].Digest()
	if err != nil {
		return fmt.Errorf("query image(%s) layer[0] digest failed", image.Image)
	}

	tarFile := path.Join(string(s.imagePath), "blobs", digest.Algorithm, digest.Hex)
	nlog.Infof("Image(%s) layer tar file path=%s", image.Image, tarFile)

	return s.mounter.Mount([]string{tarFile}, workingDir, targetDir)
}

// interface [Store]
func (s *ociStore) UmountImage(workingDir string) error {
	return s.mounter.Umount(workingDir)
}

// interface [Store]
func (s *ociStore) GetImageManifest(image *kii.ImageName, auth *runtimeapi.AuthConfig) (*kii.Manifest, error) {
	img, err := s.findImageFromLocal(image.Image)
	if err != nil {
		return nil, fmt.Errorf("not found image: %s with some error(%s)", image.Image, err.Error())
	}

	if img == nil {
		nlog.Infof("Not found image(%s) in local store", image.Image)
		return nil, nil
	}

	var digest v1.Hash
	if digest, err = img.Digest(); err != nil {
		return nil, err
	}

	var layers []v1.Layer
	if layers, err = img.Layers(); err != nil {
		return nil, err
	}

	imgConfig, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("query image(%s) coonfig file failed", image.Image)
	}

	// convert manifest.Config to v1.Config
	config := oci.ImageConfig{}
	c, _ := json.Marshal(imgConfig.Config)
	if err := json.Unmarshal(c, &config); err != nil {
		// almost never reach here
		nlog.Warnf("Parse v1.Config to manifest.Config failed. err=%s", err.Error())
	}

	m := &kii.Manifest{
		Architecture: imgConfig.Architecture,
		Created:      &imgConfig.Created.Time,
		Author:       imgConfig.Author,
		ID:           digest.String(),
		Type:         kii.ImageTypeStandard,
		Config:       config,
	}

	if len(layers) == 0 {
		// layer is empty, it is builtin image
		nlog.Infof("Image(%s) is builtin image", image.Image)
		m.Type = kii.ImageTypeBuiltIn
	}

	return m, nil
}

// interface [Store]
func (s *ociStore) TagImage(sourceImage, targetImage *kii.ImageName) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	org, err := s.findImageFromLocal(sourceImage.Image)
	if err != nil {
		return err
	}
	if org == nil {
		return fmt.Errorf("not found input image: %s", sourceImage.Image)
	}

	err = s.imagePath.ReplaceImage(org, match.Name(targetImage.Image),
		layout.WithAnnotations(map[string]string{
			oci.AnnotationRefName: targetImage.Image,
		}))

	return err
}

// interface [Store]
func (s *ociStore) LoadImage(tarFile string) error {
	if _, err := os.Stat(tarFile); err != nil {
		return fmt.Errorf("file not exists: %s", tarFile)
	}

	file, err := os.Open(tarFile)
	if err != nil {
		return fmt.Errorf("open tar file failed: %w", err)
	}
	defer file.Close()
	imgSummary, compatibleImages, err := ImageInFile(tarFile, file, file)
	if err != nil {
		return fmt.Errorf("load image failed with: %w", err)
	}
	currentPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	currentPlatform = strings.ToLower(currentPlatform)
	if err = CheckArchSummaryComplianceWithPlatform(tarFile, currentPlatform, imgSummary); err != nil {
		return err
	}

	// Iterate all compatible images and process them, improving readability and error reporting
	for _, compatibleImage := range compatibleImages {
		tag := compatibleImage.Info.Ref
		img := compatibleImage.Image
		tag = CheckTagCompliance(tag)
		// If the image does not have a valid tag, use the tarball filename as a fallback
		if tag == "" || tag == "docker.io/library/" {
			// Get the tarFile's filename (remove path and extension)
			baseName := path.Base(tarFile)
			tag = fmt.Sprintf("%s/%s/%s", defaultRegistry, defaultRepository, strings.TrimSuffix(baseName, path.Ext(baseName)))
			nlog.Warnf("No valid image tag found, using tarball filename as fallback: %s", tag)
		}
		nlog.Infof("[OCI] Start processing image(%s) ...", tag)
		if err := processCompatibleImage(s, tag, img); err != nil {
			return fmt.Errorf("Failed to process image(%s): %w", tag, err)
		}
	}
	return nil
}

// Optimized helper function for readability and error reporting
func processCompatibleImage(s *ociStore, tag string, img v1.Image) error {
	flat, cacheFile, err := s.flattenImage(img)
	// Only when flattenImage creates a temporary file, cacheFile is non-empty; empty string means no cleanup needed
	if cacheFile != "" {
		defer func() {
			if osErr := os.Remove(cacheFile); osErr != nil {
				nlog.Warnf("Failed to clean up temporary cache file %s: %v", cacheFile, osErr)
			}
		}()
	}
	if err != nil {
		nlog.Warnf("flattenImage failed to process image(%s): %s", tag, err.Error())
		return fmt.Errorf("flattenImage failed: %w", err)
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := s.imagePath.ReplaceImage(flat, match.Name(tag), s.kusciaImageAnnotation(tag, "")); err != nil {
		nlog.Warnf("ReplaceImage failed to process image(%s): %s", tag, err.Error())
		return fmt.Errorf("ReplaceImage failed: %w", err)
	}
	nlog.Infof("Image loaded: %s", tag)
	return nil
}

// interface [Store]
func (s *ociStore) RegisterImage(tag, manifestFile string) error {
	var manifest kii.Manifest
	var config v1.Config

	if manifestFile != "" {
		if err := paths.ReadJSON(manifestFile, &manifest); err != nil {
			return err
		}

		// convert manifest.Config to v1.Config
		c, _ := json.Marshal(manifest.Config)
		if err := json.Unmarshal(c, &config); err != nil {
			// almost never reach here
			nlog.Warnf("Parse manifest.Config to v1.Config failed. err=%s", err.Error())
		}
	}

	arch := manifest.Architecture
	osName := manifest.OS
	if arch == "" {
		arch = runtime.GOARCH
	}
	if osName == "" {
		osName = runtime.GOOS
	}
	cf := &v1.ConfigFile{
		Architecture: arch,
		OS:           osName,
		Created:      v1.Time{Time: time.Now()},
		Author:       manifest.Author,
		Config:       config,
	}

	img, err := mutate.ConfigFile(empty.Image, cf)
	if err != nil {
		return fmt.Errorf("create empty image failed: %w", err)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err = s.imagePath.ReplaceImage(img, match.Name(tag), s.kusciaImageAnnotation(tag, "")); err != nil {
		return err
	}

	nlog.Infof("Load image: %s", tag)
	return nil
}

func (s *ociStore) findImageFromLocal(imageNameOrID string) (v1.Image, error) {
	var err error
	ii, err := s.imagePath.ImageIndex()
	if err != nil {
		return nil, err
	}

	imf, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}

	var imgRes v1.Image
	for _, img := range imf.Manifests {
		if name, ok := s.getImageName(img); ok && (name == imageNameOrID) {
			nlog.Debugf("Found the image(%s) in local store, digest is %s", imageNameOrID, img.Digest.String())
			imgRes, err = s.imagePath.Image(img.Digest)
			return imgRes, err
		}
		if strings.HasPrefix(img.Digest.String(), fmt.Sprintf("%s%s", common.ImageIDPrefix, imageNameOrID)) {
			nlog.Debugf("Found the image(%s) in local store, digest is %s", imageNameOrID, img.Digest.String())
			imgRes, err = s.imagePath.Image(img.Digest)
			return imgRes, err
		}
	}
	return nil, nil
}

func (s *ociStore) findMatchImage(idx v1.ImageIndex) (v1.Image, error) {
	manifests, err := partial.Manifests(idx)
	if err != nil {
		return nil, err
	}

	var matched partial.Describable
	for _, m := range manifests {
		// Keep the old descriptor (annotations and whatnot).
		desc, err := partial.Descriptor(m)
		if err != nil {
			return nil, err
		}

		// High-priority: platform/arch are matched
		// Middle-priority: arch matched
		if p := desc.Platform; p != nil {
			nlog.Infof("Image=%s, Platform: OS=%s, Architecture=%s", desc.Digest.String(), p.OS, p.Architecture)

			if p.Architecture == runtime.GOARCH && p.OS == runtime.GOOS {
				matched = m
				break
			} else if matched == nil && p.Architecture == runtime.GOARCH {
				matched = m
			}
		}
	}

	if matched != nil {
		if img, ok := matched.(v1.Image); ok {
			desc, _ := partial.Descriptor(matched)
			nlog.Infof("Selected best matched image=%s", desc.Digest.String())
			return img, nil
		}
		return nil, fmt.Errorf("found a matched index, but is not an image")
	}

	return nil, errors.New("not found matched image")
}

func (s *ociStore) kusciaImageAnnotation(tag string, _ string) layout.Option {
	return layout.WithAnnotations(map[string]string{
		oci.AnnotationRefName: tag,
	})
}

// CheckTagCompliance ensures that the given image name is in a valid format by
// adding default values for registry and repository if they are missing.
func CheckTagCompliance(targetImage string) string {
	parts := strings.Split(targetImage, "/")
	image := targetImage

	if len(parts) == 1 {
		image = fmt.Sprintf("%s/%s/%s", defaultRegistry, defaultRepository, image)
	} else if len(parts) > 1 && !strings.Contains(parts[0], ".") {
		image = fmt.Sprintf("%s/%s", defaultRegistry, image)
	}

	return image
}
