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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	defaultDirMode  = 0755
	defaultFileMode = 0644
)

// LinkTreatment is the base type for constants used by Exists that indicate
// how symlinks are treated for existence checks.
type LinkTreatment int

const (
	// CheckFollowSymlink follows the symlink and verifies that the target of
	// the symlink exists.
	CheckFollowSymlink LinkTreatment = iota

	// CheckSymlinkOnly does not follow the symlink and verifies only that they
	// symlink itself exists.
	CheckSymlinkOnly
)

// ErrInvalidLinkTreatment indicates that the link treatment behavior requested
// is not a valid behavior.
var ErrInvalidLinkTreatment = errors.New("unknown link behavior")

func CheckFileOrDirExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func CheckFileExist(path string) bool {
	if src, err := os.Stat(path); err == nil {
		// exist
		return !src.IsDir()
	}
	return false
}

// CheckExists checks if specified file, directory, or symlink exists. The behavior
// of the test depends on the linkBehaviour argument. See LinkTreatment for
// more details.
func CheckExists(linkBehavior LinkTreatment, filename string) (bool, error) {
	var err error

	if linkBehavior == CheckFollowSymlink {
		_, err = os.Stat(filename)
	} else if linkBehavior == CheckSymlinkOnly {
		_, err = os.Lstat(filename)
	} else {
		return false, ErrInvalidLinkTreatment
	}

	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// return value: (isNotEmpty, fileSize)
func CheckFileNotEmpty(path string) (bool, int64) {
	if src, err := os.Stat(path); err == nil {
		return !src.IsDir() && src.Size() > 0, src.Size()
	}

	return false, 0
}

func CheckDirExist(path string) bool {
	if src, err := os.Stat(path); err == nil {
		// exist
		return src.Mode().IsDir()
	}
	return false
}

func EnsurePath(path string, autoCreate bool) error {
	_, err := os.Stat(path)

	if autoCreate && os.IsNotExist(err) {
		return os.MkdirAll(path, defaultDirMode)
	}

	return err
}

func EnsureDirectory(dirName string, autoCreate bool) error {
	return EnsureDirectoryPerm(dirName, autoCreate, defaultDirMode)
}

func EnsureDirectoryPerm(dirName string, autoCreate bool, perm fs.FileMode) error {
	src, err := os.Stat(dirName)

	if err != nil {
		if autoCreate && os.IsNotExist(err) {
			return os.MkdirAll(dirName, perm)
		}
		return err
	}

	if !src.Mode().IsDir() {
		return fmt.Errorf("'%s' already exist as a file", dirName)
	}

	return nil
}

func EnsureFile(fileName string, autoCreate bool) error {
	src, err := os.Stat(fileName)

	if err != nil {
		if autoCreate && os.IsNotExist(err) {
			// create file's parent directory
			if err := os.MkdirAll(filepath.Dir(fileName), defaultDirMode); err != nil {
				return err
			}

			f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultFileMode)
			if err == nil {
				_ = f.Close()
			}
			return err
		}
		return err
	}

	if src.Mode().IsDir() {
		return fmt.Errorf("'%s' already exist as a directory", fileName)
	}

	return nil
}

// CopyDirectory copy contents from source directory to dest directory.
func CopyDirectory(source, dest string) error {
	if err := CreateIfNotExists(dest, 0755); err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(source, entry.Name())
		dstPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}

		switch mode := fileInfo.Mode(); {
		case mode.IsRegular():
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		case mode.IsDir():
			if err := CreateIfNotExists(dstPath, 0755); err != nil {
				return err
			}
			if err := CopyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			if err := CopySymLink(srcPath, dstPath); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unsupported file type, file: %s", srcPath)
		}
	}
	return nil
}

// CopySymLink copy symbolic link.
func CopySymLink(srcFile, dstFile string) error {
	link, err := os.Readlink(srcFile)
	if err != nil {
		return err
	}
	return os.Symlink(link, dstFile)
}

// CopyFile copy regular file.
func CopyFile(srcFile, dstFile string) error {
	if err := CreateIfNotExists(filepath.Dir(dstFile), 0755); err != nil {
		return err
	}

	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer out.Close()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	// preserve permissions
	si, err := os.Stat(srcFile)
	if err != nil {
		return err
	}

	err = os.Chmod(dstFile, si.Mode())
	if err != nil {
		return err
	}

	return nil
}

// CreateIfNotExists creates directory if not exists.
func CreateIfNotExists(dir string, perm os.FileMode) error {
	if _, err := os.Stat(dir); err == nil || !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}
