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

import "path/filepath"

const (
	bundleWorkingDir    = "working"
	bundleOciConfigFile = "config.json"
	bundleOciRootfsDir  = "rootfs"
)

type Bundle struct {
	root string
}

type ContainerBundle struct {
	containerRoot string
}

func NewBundle(root string) (*Bundle, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	b := &Bundle{
		root: absRoot,
	}
	return b, nil
}

// GetRootDirectory returns the root directory of the bundle.
// For example, if we have a directory structure like this:
//
// root                   <--- return by GetRootDirectory
// └── ${container_id}    <--- return by ContainerBundle's root
//
//	├── working        <--- return by GetFsWorkingDirPath, subdirectory managed by FS itself
//	│   ├── .meta
//	│   ├── upperDir
//	│   └── workDir
//	├── config.json    <--- return by GetOciConfigPath
//	└── rootfs         <--- return by GetOciRootfsPath
func (bl *Bundle) GetRootDirectory() string {
	return bl.root
}

func (bl *Bundle) GetContainerBundle(containerID string) *ContainerBundle {
	return &ContainerBundle{
		containerRoot: filepath.Join(bl.root, containerID),
	}
}

func (cb *ContainerBundle) GetOciRootfsName() string {
	return bundleOciRootfsDir
}

func (cb *ContainerBundle) GetRootDirectory() string {
	return cb.containerRoot
}

func (cb *ContainerBundle) GetFsWorkingDirPath() string {
	return filepath.Join(cb.containerRoot, bundleWorkingDir)
}

func (cb *ContainerBundle) GetOciConfigPath() string {
	return filepath.Join(cb.containerRoot, bundleOciConfigFile)
}

func (cb *ContainerBundle) GetOciRootfsPath() string {
	return filepath.Join(cb.containerRoot, bundleOciRootfsDir)
}
