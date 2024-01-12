//go:build linux
// +build linux

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

package mount

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/sys/mountinfo"
	"github.com/stretchr/testify/assert"
)

func TestSysMount(t *testing.T) {
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "aa")
	mountRootDir := filepath.Join(rootDir, "bb")
	destDir1 := filepath.Join(mountRootDir, "1")
	destDir2 := filepath.Join(mountRootDir, "2")
	assert.NoError(t, os.MkdirAll(sourceDir, 0755))
	assert.NoError(t, os.MkdirAll(destDir1, 0755))
	assert.NoError(t, os.MkdirAll(destDir2, 0755))

	m, err := NewSysMounter(mountRootDir)
	assert.NoError(t, err)
	if err := m.Mount(sourceDir, "1", "", "bind,rw"); err != nil {
		t.Logf("Mount error: %v. Maybe there is no SYS_ADMIN permission, skip", err)
		return
	}
	assert.NoError(t, m.MountIfNotMounted(sourceDir, "2", "", "bind,rw"))
	assert.NoError(t, m.MountIfNotMounted(sourceDir, "2", "", "bind,rw"))

	mounts, err := m.(*sysMounter).GetMountsRoot()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(mounts))
	assert.NoError(t, m.UmountRoot())

	mounted, err := mountinfo.Mounted(destDir1)
	assert.NoError(t, err)
	assert.False(t, mounted)
}
