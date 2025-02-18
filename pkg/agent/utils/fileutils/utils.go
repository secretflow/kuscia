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
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func ListDir(path string) ([]string, error) {
	var subdirs []string
	entries, err := os.ReadDir(path)
	if err != nil {
		return subdirs, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdirs = append(subdirs, filepath.Join(path, entry.Name()))
	}
	return subdirs, err
}

func ListTree(path string) ([]string, error) {
	var subdirs []string
	info, statErr := os.Stat(path)
	if statErr != nil {
		return subdirs, statErr
	}
	if !info.IsDir() {
		return subdirs, nil
	}
	walkErr := filepath.Walk(path, func(curPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path != curPath {
			subdirs = append(subdirs, curPath)
		}
		return nil
	})
	return subdirs, walkErr
}

func IsModifyBefore(path string, timepoint time.Time) bool {
	file, openErr := os.OpenFile(path, os.O_RDONLY, 0600)
	if openErr != nil || file == nil {
		return false
	}
	fileInfo, statErr := file.Stat()
	if statErr != nil {
		return false
	}
	return fileInfo.ModTime().Before(timepoint)
}
