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

package mounter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/utils/assist"
	"github.com/secretflow/kuscia/pkg/utils/lock"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	fsMetaFile = ".meta"
)

type FsMeta struct {
	MountType kii.MountType
	MergedDir string
}

type Mounter struct {
	localLayout *layout.LocalImage
	keyLocker   *lock.KeyLocker
}

func New(layout *layout.LocalImage) *Mounter {
	return &Mounter{
		localLayout: layout,
		keyLocker:   lock.NewKeyLocker(),
	}
}

func (m *Mounter) Mount(manifest *kii.Manifest, mountType kii.MountType,
	workingDir, mergedDir string) error {
	var err error
	if mergedDir, err = filepath.Abs(mergedDir); err != nil {
		return err
	}

	if err = paths.EnsureDirectory(mergedDir, true); err != nil {
		return err
	}

	var meta FsMeta
	metaFile := filepath.Join(workingDir, fsMetaFile)
	if paths.ReadJSON(metaFile, &meta) == nil { // expected read fail
		return fmt.Errorf("duplicate mounting, please unmount first, previous mount path is %v", meta.MergedDir)
	}

	meta = FsMeta{
		MountType: mountType,
		MergedDir: mergedDir,
	}

	if err := paths.WriteJSON(metaFile, meta); err != nil {
		return err
	}

	switch mountType {
	case kii.Plain:
		return m.mountPlain(manifest, mergedDir, false)
	default:
		return fmt.Errorf("unsupported mount type %q", mountType)
	}
}

func (m *Mounter) Umount(workingDir string) error {
	var meta FsMeta
	metaFile := filepath.Join(workingDir, fsMetaFile)
	if err := paths.ReadJSON(metaFile, &meta); err != nil {
		return fmt.Errorf("%q is not a valid mount working dir", workingDir)
	}

	var err error
	switch meta.MountType {
	case kii.Plain:
		err = m.unmountPlain(meta.MergedDir)
	default:
		err = fmt.Errorf("unsupported unmount type %q", meta.MountType)
	}

	if err != nil {
		return err
	}

	return os.RemoveAll(workingDir)
}

func (m *Mounter) mountPlain(manifest *kii.Manifest, targetDir string, adjustLink bool) (retErr error) {
	defer func() {
		if retErr != nil {
			if err := m.unmountPlain(targetDir); err != nil {
				retErr = fmt.Errorf("%v, and unmount also has error-> %v", retErr, err)
				return
			}
		}
	}()

	for _, layerHash := range manifest.Layers {
		tarFile := m.localLayout.GetLayerTarFile(layerHash)

		if err := assist.ExtractTarFile(targetDir, tarFile, adjustLink, true); err != nil {
			return fmt.Errorf("mount image fail, layer=%v, err-> %v", layerHash, err)
		}
	}
	return nil
}

func (m *Mounter) unmountPlain(targetDir string) error {
	return os.RemoveAll(targetDir)
}

func (m *Mounter) unpackAndGetLayer(layerHash string) (string, error) {
	m.keyLocker.Lock(layerHash)
	defer m.keyLocker.Unlock(layerHash)

	unpack := m.localLayout.GetLayerUnpackDir(layerHash)
	if paths.CheckDirExist(unpack) {
		nlog.Debugf("Layer %v is already unpacked", layerHash)
		return unpack, nil
	}

	// do unpack
	if err := assist.ExtractTarFile(unpack, m.localLayout.GetLayerTarFile(layerHash), false, true); err != nil {
		_ = os.RemoveAll(unpack)
		return "", fmt.Errorf("extract image fail, layer=%v, err-> %v", layerHash, err)
	}

	return unpack, nil
}
