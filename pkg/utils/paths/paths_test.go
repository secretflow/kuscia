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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureDirectory(t *testing.T) {
	wd, err := os.MkdirTemp("", t.Name())
	assert.NoError(t, err)
	defer os.RemoveAll(wd)

	path := filepath.Join(wd, "/dir")
	assert.ErrorContains(t, EnsureDirectory(path, false), "no such file or directory")
	assert.NoError(t, EnsureDirectory(path, true)) // first: auto create
	assert.NoError(t, EnsureDirectory(path, true)) // second: no create

	file := path + "/file"
	_, err = os.Create(file)
	assert.NoError(t, err)

	assert.Error(t, EnsureDirectory(file, false), fmt.Sprintf("'%s' already exist as a file", file))
	assert.Error(t, EnsureDirectory(file, true), fmt.Sprintf("'%s' already exist as a file", file))
}

func TestEnsureFile(t *testing.T) {
	path, err := os.MkdirTemp("", t.Name())
	assert.NoError(t, err)
	defer os.RemoveAll(path)

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
