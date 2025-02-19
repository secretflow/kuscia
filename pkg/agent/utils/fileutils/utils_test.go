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

package fileutils

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// create temp directory for test
func CreateTempDir() (string, error) {
	dir, err := os.MkdirTemp("", "gc-test-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	return dir, nil
}

func CreateTempFile(dir string, fileName string, isDir bool) (string, error) {
	path := filepath.Join(dir, fileName)
	var err error
	if isDir {
		err = os.Mkdir(path, 0644)
	} else {
		_, err = os.Create(path)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	return path, nil
}

func changePathTime(path string, duration time.Duration) error {
	oldTime := time.Now().Add(-duration)
	return os.Chtimes(path, oldTime, oldTime)
}

// return all path, the first level sub dir/files, and the dir/files will be left after gc
func CreateGCTestDir() (dir string, allPaths []string, childPaths []string, leftPaths []string, err error) {
	dir, err = CreateTempDir()

	// create old file
	path, err := CreateTempFile(dir, "alice_sf-job-01_12345678-1234-1234-1234-12345678abcd", true)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)

	path, err = CreateTempFile(path, "secretflow", true)
	allPaths = append(allPaths, path)

	path, err = CreateTempFile(path, "0.log", false)
	allPaths = append(allPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 31*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-2], 31*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-3], 31*24*time.Hour)

	path, err = CreateTempFile(dir, "alice_sf-job-02_12345678-1234-1234-1234-12345678abcd", true)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)

	path, err = CreateTempFile(path, "secretflow", true)
	allPaths = append(allPaths, path)

	path, err = CreateTempFile(path, "0.log", false)
	allPaths = append(allPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 1*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-2], 31*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-3], 31*24*time.Hour)

	path, err = CreateTempFile(dir, "temp.log", false)
	allPaths = append(allPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 31*24*time.Hour)

	sort.StringSlice(allPaths).Sort()
	sort.StringSlice(childPaths).Sort()
	sort.StringSlice(leftPaths).Sort()

	return
}

func TestListDir(t *testing.T) {
	t.Parallel()
	rootDir, _, childPaths, _, err := CreateGCTestDir()
	assert.NoError(t, err)

	listResults, err := ListDir(rootDir)
	assert.True(t, reflect.DeepEqual(listResults, childPaths))

	os.RemoveAll(rootDir)
	// after remove list dir should fail
	listResults, err = ListDir(rootDir)
	assert.Nil(t, listResults)
	assert.Error(t, err)
}

func TestListTree(t *testing.T) {
	t.Parallel()
	rootDir, allPaths, _, _, err := CreateGCTestDir()

	assert.NoError(t, err)

	treeResult, err := ListTree(rootDir)
	assert.True(t, reflect.DeepEqual(treeResult, allPaths))

	// no tree for file, return nil nil
	treeResult, err = ListTree(filepath.Join(rootDir, "temp.log"))
	assert.Nil(t, treeResult)
	assert.NoError(t, err)

	os.RemoveAll(rootDir)
	treeResult, err = ListTree(rootDir)
	assert.Nil(t, treeResult)
	assert.Error(t, err)
}

func TestModTimeCheck(t *testing.T) {
	t.Parallel()
	rootDir, _, _, _, err := CreateGCTestDir()
	assert.NoError(t, err)

	defer os.RemoveAll(rootDir)

	// test mod before
	flag := IsModifyBefore(filepath.Join(rootDir, "temp.log"), time.Now().Add(-30*24*time.Hour))
	assert.True(t, flag)

	flag = IsModifyBefore(filepath.Join(rootDir, "temp.log"), time.Now().Add(-32*24*time.Hour))
	assert.False(t, flag)

	// test file not exist
	flag = IsModifyBefore(filepath.Join(rootDir, "temp-not-exist.log"), time.Now().Add(-30*24*time.Hour))
	assert.False(t, flag)

	// same test for mod after
	flag = IsModifyBefore(filepath.Join(rootDir, "temp.log"), time.Now().Add(-30*24*time.Hour))
	assert.True(t, flag)

	flag = IsModifyBefore(filepath.Join(rootDir, "temp.log"), time.Now().Add(-32*24*time.Hour))
	assert.False(t, flag)

	flag = IsModifyBefore(filepath.Join(rootDir, "temp-not-exist.log"), time.Now().Add(-30*24*time.Hour))
	assert.False(t, flag)

}
