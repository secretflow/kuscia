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

package mounter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type MountType string

const (
	// Plain unpacks all layers to same directory, override files with same filename
	// layers[0] ---> untar ---> dest
	// layers[1] ---> untar ---> dest
	// ....
	Plain MountType = "plain"
)

const (
	fsMetaFile = ".meta"
)

type FsMeta struct {
	MountType  MountType
	WorkingDir string
	MergedDir  string
}

type Mounter interface {
	Mount(layers []string, workingDir, mergedDir string) error
	Umount(workingDir string) error
	GetType() MountType
}

func NewMounter(mountType MountType) Mounter {
	switch mountType {
	case Plain:
		return &RawMounter{}
	}
	return nil
}

func WriteMountMetaFile(mounter Mounter, workingDir, mergedDir string) error {
	var meta FsMeta
	metaFile := filepath.Join(workingDir, fsMetaFile)
	_, err := os.Stat(metaFile)
	if !os.IsNotExist(err) {
		return fmt.Errorf("duplicate mounting, please unmount first, previous mount path is %v", meta.MergedDir)
	}

	meta = FsMeta{
		MountType:  mounter.GetType(),
		MergedDir:  mergedDir,
		WorkingDir: workingDir,
	}

	return paths.WriteJSON(metaFile, meta)
}

func ReadMountMetaFile(mounter Mounter, workingDir string) (string, error) {
	var meta FsMeta
	metaFile := filepath.Join(workingDir, fsMetaFile)
	if err := paths.ReadJSON(metaFile, &meta); err != nil {
		return "", fmt.Errorf("%q is not a valid mount working dir", workingDir)
	}

	if meta.MountType != mounter.GetType() {
		return "", fmt.Errorf("mount type is not equal")
	}

	if meta.WorkingDir != workingDir {
		return "", fmt.Errorf("working dir is not equal")
	}

	return meta.MergedDir, nil
}
