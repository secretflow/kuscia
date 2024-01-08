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

package paths

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureDirectory(t *testing.T) {
	wd := t.TempDir()

	path := filepath.Join(wd, "/dir")
	assert.ErrorContains(t, EnsureDirectory(path, false), "no such file or directory")
	assert.NoError(t, EnsureDirectory(path, true)) // first: auto create
	assert.NoError(t, EnsureDirectory(path, true)) // second: no create

	file := path + "/file"
	_, err := os.Create(file)
	assert.NoError(t, err)

	assert.Error(t, EnsureDirectory(file, false), fmt.Sprintf("'%s' already exist as a file", file))
	assert.Error(t, EnsureDirectory(file, true), fmt.Sprintf("'%s' already exist as a file", file))
}

func TestEnsureFile(t *testing.T) {
	path := t.TempDir()

	// dir already exist
	assert.Error(t, EnsureFile(path, false), fmt.Sprintf("'%s' already exist as a directory", path))
	assert.Error(t, EnsureFile(path, true), fmt.Sprintf("'%s' already exist as a directory", path))

	file := path + "/file"
	assert.ErrorContains(t, EnsureDirectory(file, false), "no such file or directory")
	assert.NoError(t, EnsureFile(file, true)) // create
	assert.NoError(t, EnsureFile(file, true)) // no create

	file = path + "/d1/d2/file"
	assert.ErrorContains(t, EnsureDirectory(file, false), "no such file or directory")
	assert.NoError(t, EnsureFile(file, true)) // create
	assert.NoError(t, EnsureFile(file, true)) // no create
}

func TestLinkUnlink(t *testing.T) {
	wd := t.TempDir()

	// file not exist, pass
	assert.NoError(t, Unlink(wd+"/dir/non-exist"))

	// symlink file2 -> file1
	// symlink a_dir/file2 -> ../file1
	content := []byte("a secret")
	originalFile := filepath.Join(wd, "file1")
	assert.NoError(t, WriteFile(originalFile, content))
	assert.NoError(t, Link("./file1", wd+"/file2", true))
	assert.NoError(t, Link("../file1", wd+"/a_dir/file2", true))

	assert.NoError(t, Unlink(wd+"/file2"))
	assert.NoError(t, Unlink(wd+"/a_dir/file2"))
	c, err := ioutil.ReadFile(wd + "/file1")
	assert.NoError(t, err)
	assert.Equal(t, c, content)

	// unlink dir
	assert.NoError(t, os.Mkdir(wd+"a_dir", 0755))
	assert.NoError(t, Link(wd+"a_dir", wd+"/dir2", true))
	assert.NoError(t, EnsureDirectory(wd+"/dir2", false))
	assert.NoError(t, Unlink(wd+"/dir2"))
	assert.NoError(t, EnsureDirectory(wd+"a_dir", false))
	assert.False(t, CheckFileOrDirExist(wd+"/dir2"))

	// unlink normal file, fail
	assert.ErrorContains(t, Unlink(originalFile), "not a hard link file")

	// hardlink file2 -> file1
	assert.NoError(t, Link("./file1", wd+"/file2", true))
	assert.NoError(t, Unlink(wd+"/file2"))
	assert.False(t, CheckFileOrDirExist(wd+"/file2"))
	c, err = os.ReadFile(wd + "/file1")
	assert.NoError(t, err)
	assert.Equal(t, c, content)
}

func TestReadWriteJson(t *testing.T) {
	rootDir := t.TempDir()
	file := filepath.Join(rootDir, "a.json")
	WData := map[string]int{"a": 1}

	assert.NoError(t, WriteJSON(file, WData))

	rData := map[string]int{}
	assert.NoError(t, ReadJSON(file, &rData))
	assert.Equal(t, 1, rData["a"])
}

func TestMove(t *testing.T) {
	rootDir := t.TempDir()
	oldPath := filepath.Join(rootDir, "src.txt")
	assert.NoError(t, os.WriteFile(oldPath, []byte("hello world"), 0644))
	newPath := filepath.Join(rootDir, "dst.txt")
	assert.NoError(t, Move(oldPath, newPath))
}
