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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/secretflow/kuscia/pkg/utils/assist"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type RawMounter struct {
}

func (m *RawMounter) Mount(layers []string, workingDir, mergedDir string) error {
	var err error
	if mergedDir, err = filepath.Abs(mergedDir); err != nil {
		return err
	}

	if err := paths.EnsureDirectory(mergedDir, true); err != nil {
		return err
	}

	if err := WriteMountMetaFile(m, workingDir, mergedDir); err != nil {
		return err
	}

	return m.mountPlain(layers, mergedDir, false)

}

func (m *RawMounter) Umount(workingDir string) error {
	mergedDir, err := ReadMountMetaFile(m, workingDir)
	if err != nil {
		return err
	}

	if err := m.unmountPlain(mergedDir); err != nil {
		return err
	}

	return os.RemoveAll(workingDir)
}

func (m *RawMounter) mountPlain(layers []string, targetDir string, adjustLink bool) (retErr error) {
	defer func() {
		if retErr != nil {
			if err := m.unmountPlain(targetDir); err != nil {
				retErr = fmt.Errorf("%v, and unmount also has error-> %v", retErr, err)
				return
			}
		}
	}()

	for _, layer := range layers {
		fileInfo, err := os.Stat(layer)
		if err != nil {
			return err
		}
		if !fileInfo.IsDir() {
			// TODO: check file is tar file
			if err := assist.ExtractTarFile(targetDir, layer, adjustLink, true); err != nil {
				return fmt.Errorf("untar layer(%s) to folder(%s) fail, err-> %v", layer, targetDir, err)
			}
		} else {
			// TODO: else copy <layer> folder to dest folder
			return errors.New("not support folder now")
		}
	}
	return nil
}

func (m *RawMounter) unmountPlain(targetDir string) error {
	return os.RemoveAll(targetDir)
}

func (m *RawMounter) GetType() MountType {
	return Plain
}
