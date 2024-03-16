// Copyright 2023 Ant Group Co., Ltd.
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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/secretflow/kuscia/pkg/agent/local/store/assist"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type DockerPackageManifestItem struct {
	Config string `json:"Config"`

	RepoTags []string `json:"RepoTags"`

	// Layers is an indexed list of layers referenced by the manifest.
	Layers []string `json:"Layers"`

	Type kii.ImageType `json:"Type"`
}

const (
	dockerManifestFilename    = "manifest.json"
	dockerManifestLayerSuffix = "/layer.tar"
)

func (s *store) RegisterImage(image, manifestFile string) error {

	var manifest kii.Manifest
	if manifestFile != "" {
		if err := paths.ReadJSON(manifestFile, &manifest); err != nil {
			return err
		}
	}

	manifest.Type = kii.ImageTypeBuiltIn
	if manifest.Created == nil {
		now := time.Now()
		manifest.Created = &now
	}
	if manifest.ID == "" {
		manifest.ID = image
	} else {
		manifest.ID = kii.FormatImageID(manifest.ID)
	}

	imageName, err := kii.NewImageName(image)
	if err != nil {
		return err
	}

	if err := paths.EnsureDirectory(s.layout.GetManifestsDir(imageName), true); err != nil {
		return err
	}

	if err := paths.WriteJSON(s.layout.GetManifestFilePath(imageName), manifest); err != nil {
		return err
	}

	nlog.Infof("Image registered successfully")
	return nil
}

func (s *store) LoadImage(imageReader io.Reader) error {
	// step 1: create temp working dir
	tempDir, err := s.layout.GetTemporaryDir("img-load-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	nlog.Infof("Parsing external image package ...")

	if err = assist.Untar(tempDir, false, false, imageReader); err != nil {
		return err
	}

	// step 2: read and convert manifest file
	var dockerManifests []DockerPackageManifestItem
	if err := paths.ReadJSON(filepath.Join(tempDir, dockerManifestFilename), &dockerManifests); err != nil {
		return err
	}

	// step 3: preprocess layer, replace symlink file
	nlog.Infof("Preprocess layers ...")
	for _, dockerManifest := range dockerManifests {
		for _, layerTarFile := range dockerManifest.Layers {
			tempLayerTar := filepath.Join(tempDir, layerTarFile)
			fi, err := os.Lstat(tempLayerTar)
			if err != nil {
				return err
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				srcTar, err := filepath.EvalSymlinks(tempLayerTar)
				if err != nil {
					return err
				}
				if err = os.Remove(tempLayerTar); err != nil {
					return err
				}
				if err = paths.CopyFile(srcTar, tempLayerTar); err != nil {
					return err
				}
			}
		}
	}

	totalImages := 0
	for _, manifest := range dockerManifests {
		if len(manifest.RepoTags) == 0 {
			return fmt.Errorf("no image tag found")
		}

		image, err := kii.NewImageName(manifest.RepoTags[0])
		if err != nil {
			return err
		}
		nlog.Infof("Loading image %v ...", image.Image)

		// convert manifest
		kiiManifest, err := fromDockerManifest(tempDir, &manifest)
		if err != nil {
			return err
		}

		kiiManifest.Type = kii.ImageTypeStandard
		if manifest.Type != "" {
			kiiManifest.Type = manifest.Type
		}

		// move layers
		var totalSize int64
		for _, layerHash := range kiiManifest.Layers {
			tempLayerDir := filepath.Join(tempDir, layerHash)
			if paths.CheckDirExist(tempLayerDir) {
				nlog.Infof("Loading layer %v ...", layerHash)
				// use move instead copy
				if err := paths.Move(tempLayerDir, s.layout.GetLayerDir(layerHash)); err != nil {
					return err
				}
			} else if paths.CheckDirExist(s.layout.GetLayerDir(layerHash)) {
				nlog.Infof("Layer %v already exist, skip", layerHash)
			} else {
				return fmt.Errorf("missing layer %v", layerHash)
			}

			info, err := os.Stat(s.layout.GetLayerTarFile(layerHash))
			if err != nil {
				return err
			}
			totalSize += info.Size()
		}

		// move config.json
		if err := paths.Move(filepath.Join(tempDir, manifest.Config), s.layout.GetConfigFilePath(image)); err != nil {
			return err
		}
		// dump manifest
		kiiManifest.Size = totalSize
		if err := paths.WriteJSON(s.layout.GetManifestFilePath(image), kiiManifest); err != nil {
			return err
		}

		// tag other image
		for i := 1; i < len(manifest.RepoTags); i++ {
			targetImage, err := kii.NewImageName(manifest.RepoTags[i])
			if err != nil {
				return err
			}
			nlog.Infof("Tag image %v", targetImage.Image)
			if err := s.TagImage(image, targetImage); err != nil {
				return err
			}
		}
		totalImages += len(manifest.RepoTags)
	}

	nlog.Infof("Images loading completed, image count=%v", totalImages)
	return nil
}

func fromDockerManifest(tempDir string, dockerManifest *DockerPackageManifestItem) (*kii.Manifest, error) {
	configPath := filepath.Join(tempDir, dockerManifest.Config)
	manifest := &kii.Manifest{}

	// hash/layer.tar  => hash
	for _, layer := range dockerManifest.Layers {
		if strings.HasSuffix(layer, dockerManifestLayerSuffix) {
			manifest.Layers = append(manifest.Layers, layer[:len(layer)-len(dockerManifestLayerSuffix)])
		} else {
			return nil, fmt.Errorf("un-supported docker image format, layer not end with %v", dockerManifestLayerSuffix)
		}
	}

	// image_id.json => image_id
	if strings.HasSuffix(dockerManifest.Config, ".json") {
		manifest.ID = kii.FormatImageID(dockerManifest.Config[:len(dockerManifest.Config)-len(".json")])
	}

	// fill meta info
	var ociImage oci.Image
	runtime.Must(paths.ReadJSON(configPath, &ociImage))
	manifest.Created = ociImage.Created
	manifest.Author = ociImage.Author
	manifest.Architecture = ociImage.Architecture
	manifest.OS = ociImage.OS
	manifest.Config = ociImage.Config
	return manifest, nil
}
