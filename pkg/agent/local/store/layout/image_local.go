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

package layout

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

// storage structure

// baseDir
//    ├── layers
//    │   ├── 2f
//    │   │   └── 2f89e79891a6ba6a7bee841785b87a3873d7e74bf26bd01239ddce5d183a1fb2
//    │   │       ├── json
//    │   │       ├── layer.tar             <-- layer content
//    |   |       ├── unpack                            <-- unpack of layer.tar
//    │   │       └── VERSION
//    │   ├── 63
//    │   │   └── 637d1c46b718e7f2b39f811f898b013b6fc12a48356e07231e2521bf833f27df
//    │   │       ├── json
//    │   │       ├── layer.tar
//    │   │       └── VERSION
//    │   ├── 76
//    │   │   └── 761a3a1c2d4c390f099909c4841f8f44794982cdba9fd67390c1bde18a374fed
//    │   │       ├── json
//    │   │       ├── layer.tar
//    │   │       └── VERSION
//    │   ├── 7a
//    │   │   └── 7a07cd391204e3a6f1c463d04d6f1fc60e03eec5a6765f2194d466297730f8d1
//    │   │       ├── json
//    │   │       ├── layer.tar
//    │   │       └── VERSION
//    │   ├── 7c
//    │   │   └── 7c7c54ff1b37d79b9b1af3e96720d956a6758199b12c1d3902c8db4a1f4d7f30
//    │   │       ├── json
//    │   │       ├── layer.tar
//    │   │       └── VERSION
//    │   └── 8a
//    │       └── 8a3a82b19fea87e0a08cdf4f350fa08b7bd187d3a8604d6d92e910b5f1ce292f
//    │           ├── json
//    │           ├── layer.tar
//    │           └── VERSION
//    └── repositories                                    <-- dir to store image manifests
//        └── docker.io
//            └── secretflow
//                └── secretflow
//                    └── latest                          <-- tag
//                        └── _manifests
//                            ├── config.json             <-- OCI image-spec config file (optional)
//                            └── manifest.json

type LocalImage struct {
	baseDir string

	repositoriesEntry string
	buildEntry        string
}

func NewLocalImage(rootDir string) (*LocalImage, error) {
	rl := &LocalImage{
		baseDir: rootDir,
	}

	rl.repositoriesEntry = filepath.Join(rl.baseDir, repositoriesEntry)
	if err := paths.EnsureDirectory(rl.repositoriesEntry, true); err != nil {
		return nil, err
	}

	rl.buildEntry = filepath.Join(rl.baseDir, buildEntry)
	if err := paths.EnsureDirectory(rl.buildEntry, true); err != nil {
		return nil, err
	}

	return rl, nil
}

func (li *LocalImage) GetRepositoriesEntry() string {
	return li.repositoriesEntry
}

func (li *LocalImage) GetManifestsDir(image *kii.ImageName) string {
	return filepath.Join(li.repositoriesEntry, image.Repo, image.Tag, manifests)
}

func (li *LocalImage) GetManifestFilePath(image *kii.ImageName) string {
	return filepath.Join(li.GetManifestsDir(image), manifestFilename)
}

func (li *LocalImage) GetConfigFilePath(image *kii.ImageName) string {
	return filepath.Join(li.GetManifestsDir(image), configFilename)
}

func (li *LocalImage) GetLayerDir(layerHash string) string {
	var hashPrefix string
	if len(layerHash) >= 2 {
		hashPrefix = layerHash[:2]
	}

	return filepath.Join(li.baseDir, layersEntry, hashPrefix, layerHash)
}

func (li *LocalImage) GetLayerTarFile(layerHash string) string {
	return filepath.Join(li.GetLayerDir(layerHash), layerFilename)
}

func (li *LocalImage) GetLayerUnpackDir(layerHash string) string {
	return filepath.Join(li.GetLayerDir(layerHash), layerUnpackDir)
}

// GetTemporaryFile Multiple programs calling GetTemporaryFile simultaneously
// will not choose the same file. The caller can use f.Name()
// to find the pathname of the file. It is the caller's responsibility
// to remove the file when no longer needed.
func (li *LocalImage) GetTemporaryFile(pattern string) (*os.File, error) {
	return os.CreateTemp(li.buildEntry, pattern)
}

// GetTemporaryDir It is the caller's responsibility to remove the directory when no longer needed.
func (li *LocalImage) GetTemporaryDir(pattern string) (string, error) {
	return os.MkdirTemp(li.buildEntry, pattern)
}

// List When some images list success and some fail, the return value "ImageName" and "error" can have
// values at the same time, and "error" variable stores the last failed message.
// If all images list fail, "ImageName" is an empty slice and will never be nil
func (li *LocalImage) List(filter map[string]string) ([]*kii.ImageName, error) {
	var result []*kii.ImageName

	var imgErr error
	root := li.GetRepositoriesEntry()
	listDir := root

	// get filter item
	filterEndpointValue, ContainFilterEndpoint := filter["endpoint"]
	if ContainFilterEndpoint {
		listDir = filepath.Join(root, filterEndpointValue)
	}

	err := filepath.Walk(listDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				imgErr = err
				return filepath.SkipDir
			}
			if info.Name() != manifests {
				return nil
			}

			if exist, _ := paths.CheckFileNotEmpty(filepath.Join(path, manifestFilename)); !exist {
				return nil
			}

			namePath, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				imgErr = err
				return nil
			}

			img, err := kii.ParseImageNameFromPath(namePath)
			if err != nil {
				imgErr = err
				return nil
			}

			result = append(result, img)
			return nil
		},
	)

	if imgErr != nil {
		return result, imgErr
	}

	return result, err
}

func (li *LocalImage) ListLayers() ([]string, error) {
	var ret []string
	// The layers folder prohibits walking because there are a large number of files in unpack/, resulting in very low performance.
	layerDir := filepath.Join(li.baseDir, layersEntry)
	prefixes, lastErr := os.ReadDir(layerDir)
	for _, prefix := range prefixes {
		if !prefix.IsDir() {
			continue
		}

		hashes, err := os.ReadDir(filepath.Join(layerDir, prefix.Name()))
		for _, hash := range hashes {
			if !hash.IsDir() {
				continue
			}

			ret = append(ret, hash.Name())
			if err != nil {
				if lastErr == nil {
					lastErr = err
				} else {
					lastErr = fmt.Errorf("%v | %v", lastErr, err)
				}
			}
		}
	}

	return ret, lastErr
}
