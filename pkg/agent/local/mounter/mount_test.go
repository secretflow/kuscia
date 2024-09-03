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
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/utils/paths"
)

func TestMount_WriteMetaFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	assert.NoError(t, os.WriteFile(path.Join(tmpDir, fsMetaFile), []byte("xyz"), 0644))

	assert.Error(t, WriteMountMetaFile(NewMounter(Plain), tmpDir, tmpDir))

	tmpDir2 := t.TempDir()
	assert.NoError(t, WriteMountMetaFile(NewMounter(Plain), tmpDir2, tmpDir))

	_, err := os.ReadFile(path.Join(tmpDir2, fsMetaFile))
	assert.NoError(t, err)

	target, err := ReadMountMetaFile(NewMounter(Plain), tmpDir2)
	assert.NoError(t, err)
	assert.Equal(t, tmpDir, target)
}

func TestMount_ReadMountMetaFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	_, err := ReadMountMetaFile(NewMounter(Plain), tmpDir)
	assert.Error(t, err)

	meta := &FsMeta{}
	assert.NoError(t, paths.WriteJSON(path.Join(tmpDir, fsMetaFile), meta))

	_, err = ReadMountMetaFile(NewMounter(Plain), tmpDir)
	assert.Error(t, err)

	_, err = ReadMountMetaFile(NewMounter(Plain), tmpDir)
	assert.Error(t, err)

	meta = &FsMeta{
		MountType:  Plain,
		WorkingDir: tmpDir,
		MergedDir:  "xyz",
	}
	assert.NoError(t, paths.WriteJSON(path.Join(tmpDir, fsMetaFile), meta))
	target, err := ReadMountMetaFile(NewMounter(Plain), tmpDir)
	assert.NoError(t, err)
	assert.Equal(t, "xyz", target)
}

func TestMount_ReadMountMetaFile_TypeError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	meta := &FsMeta{
		MountType:  "xyz",
		WorkingDir: tmpDir,
		MergedDir:  "xyz",
	}
	assert.NoError(t, paths.WriteJSON(path.Join(tmpDir, fsMetaFile), meta))
	_, err := ReadMountMetaFile(NewMounter(Plain), tmpDir)
	assert.Error(t, err)
}

func TestMount_ReadMountMetaFile_WorkingDirError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	meta := &FsMeta{
		MountType:  Plain,
		WorkingDir: "xyz",
		MergedDir:  "xyz",
	}
	assert.NoError(t, paths.WriteJSON(path.Join(tmpDir, fsMetaFile), meta))
	_, err := ReadMountMetaFile(NewMounter(Plain), tmpDir)
	assert.Error(t, err)
}
