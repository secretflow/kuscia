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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/utils/assist"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

func TestMounter_New(t *testing.T) {
	t.Parallel()
	m := NewMounter(Plain)

	assert.NotNil(t, m)
	assert.Equal(t, Plain, m.GetType())

	raw, ok := m.(*RawMounter)
	assert.True(t, ok)
	assert.NotNil(t, raw)
}

func TestRawMounter_MountSuccess(t *testing.T) {
	t.Parallel()
	m := NewMounter(Plain)
	assert.NotNil(t, m)
	assert.Equal(t, Plain, m.GetType())

	tempDir := t.TempDir()

	assert.NoError(t, paths.EnsureDirectory(filepath.Join(tempDir, "filess"), true))

	// create test tar file
	f, err := os.Create(filepath.Join(tempDir, "filess", "temp.txt"))
	assert.NoError(t, err)

	f.Write([]byte("hello world!"))
	assert.NoError(t, f.Close())

	tempTar := filepath.Join(tempDir, "tmp.tar.gz")
	tarFile, err := os.OpenFile(tempTar, os.O_WRONLY|os.O_CREATE, 0644)
	assert.NoError(t, err)
	assert.NotNil(t, tarFile)
	assert.NoError(t, assist.Tar(filepath.Join(tempDir, "filess"), true, tarFile))
	assert.NoError(t, err)
	assert.NoError(t, tarFile.Close())

	workingDir := t.TempDir()
	destDir := t.TempDir()

	assert.NoError(t, m.Mount([]string{tempTar}, workingDir, destDir))
	assert.NoError(t, paths.CheckAllFileExist(filepath.Join(destDir, "temp.txt")))
	assert.NoError(t, paths.CheckAllFileExist(filepath.Join(workingDir, ".meta")))

	unzipFile, err := os.Open(filepath.Join(destDir, "temp.txt"))
	assert.NoError(t, err)
	assert.NotNil(t, unzipFile)
	buf := make([]byte, 1024)
	n, err := unzipFile.Read(buf)
	assert.NoError(t, err)
	assert.Greater(t, n, 0)
	assert.Equal(t, "hello world!", string(buf[:n]))

	assert.NoError(t, m.Umount(workingDir))
}

func TestRawMounter_Mount_ReMount(t *testing.T) {
	t.Parallel()
	m := NewMounter(Plain)
	assert.NotNil(t, m)
	assert.Equal(t, Plain, m.GetType())

	workingDir := t.TempDir()
	destDir := t.TempDir()

	assert.NoError(t, WriteMountMetaFile(m, workingDir, destDir))

	assert.Error(t, m.Mount([]string{}, workingDir, destDir))
}

func TestRawMounter_Mount_InvalidatePath(t *testing.T) {
	t.Parallel()
	m := NewMounter(Plain)
	assert.NotNil(t, m)
	assert.Equal(t, Plain, m.GetType())

	workingDir := t.TempDir()

	assert.Error(t, m.Mount([]string{}, workingDir, "/dev/null"))
}

func TestRawMounter_Mount_Failed(t *testing.T) {
	t.Parallel()
	m := NewMounter(Plain)
	assert.NotNil(t, m)
	assert.Equal(t, Plain, m.GetType())

	testDir := t.TempDir()
	workingDir := t.TempDir()
	destDir := t.TempDir()

	assert.Error(t, m.Mount([]string{"/path/to/nonexistent"}, workingDir, destDir))

	assert.Error(t, m.Mount([]string{testDir}, workingDir, destDir))

	// not tar file
	f, err := os.Create(filepath.Join(testDir, "temp.txt"))
	assert.NoError(t, err)
	f.Write([]byte("hello world!"))
	assert.NoError(t, f.Close())

	assert.Error(t, m.Mount([]string{filepath.Join(testDir, "temp.txt")}, workingDir, destDir))
}

func TestRawMounter_UMount_NoMounted(t *testing.T) {
	t.Parallel()
	m := NewMounter(Plain)
	assert.NotNil(t, m)
	assert.Equal(t, Plain, m.GetType())

	workingDir := t.TempDir()

	assert.Error(t, m.Umount(workingDir))
}
