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

package assist

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

// Tar takes a source and variable writers and walks 'source' writing each file
// found to the tar writer; the purpose for accepting multiple writers is to allow
// for multiple outputs (for example a file, or md5 hash)
func Tar(src string, isGzip bool, writers ...io.Writer) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("unable to tar files - %v", err.Error())
	}

	mw := io.MultiWriter(writers...)

	var tw *tar.Writer
	if isGzip {
		gzw := gzip.NewWriter(mw)
		defer gzw.Close()

		tw = tar.NewWriter(gzw)
	} else {
		tw = tar.NewWriter(mw)
	}
	defer tw.Close()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		return tarFile(src, file, fi, tw)
	})
}

func tarFile(base, file string, fi os.FileInfo, tw *tar.Writer) error {
	var err error

	link := fi.Name()
	if fi.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(file)
		if err != nil {
			return err
		}
	}

	header, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(base, file)
	if err != nil {
		return err
	}
	header.Name = rel

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if !fi.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(file)
	if err != nil {
		return err
	}

	if _, err := io.Copy(tw, f); err != nil {
		return err
	}

	f.Close()
	return nil
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func Untar(dst string, isGzip bool, adjustLink bool, r io.Reader, skipUnNormalFile bool) error {
	var tr *tar.Reader
	if isGzip {
		gzr, err := gzip.NewReader(r)
		if err != nil {
			return err
		}
		defer gzr.Close()

		tr = tar.NewReader(gzr)
	} else {
		tr = tar.NewReader(r)
	}

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return fmt.Errorf("read saved tar file error: %v", err)
		case header == nil:
			nlog.Debugf("Tar file header is nil, skip. dst=%v", dst)
			continue
		}

		target := filepath.Join(dst, header.Name)
		if err := writeOneFile(header, tr, dst, target, adjustLink, skipUnNormalFile); err != nil {
			return err
		}
	}
}

// ExtractTarFile untars a single file from a given src to a destination
//
// adjustLink: Whether to automatically correct the symbolic link,
// only the absolute path is affected, and the symbolic link based on relative path is not affected
//
// For example, suppose we have a tar file:
//
//	tar
//	├── file_1
//	└── file_2 -> /file_1
//
// and want to extract to dir '/target',
//
// If adjustLink set to false, then the target dir looks like:
//
//	target
//	├── file_1
//	└── file_2 -> /file_1   # symlink broken, it not really point to file_1, since it's an abs path
//
// yes, it is same with files in tarball
//
// And if adjustLink set to true, the target dir will become:
//
//	target
//	├── file_1
//	└── file_2 -> /target/file_1
//
// as you can see, file_2's symlink was auto adjusted to /target/file_1
func ExtractTarFile(dst string, tarFile string, adjustLink bool, skipUnNormalFile bool) error {
	file, err := os.Open(tarFile)
	if err != nil {
		return fmt.Errorf("open tar file error, file=%v, err-> %v", tarFile, err)
	}
	defer file.Close()

	isGzip := false
	header := make([]byte, 2)
	if _, err = file.Read(header); err != nil {
		return err
	}

	if header[0] == 31 && header[1] == 139 {
		isGzip = true
	}

	file.Seek(0, io.SeekStart)
	return Untar(dst, isGzip, adjustLink, file, skipUnNormalFile)
}

func writeOneFile(header *tar.Header, tr *tar.Reader, targetWd, target string, adjustLink bool, skipUnNormalFile bool) error {
	switch header.Typeflag {
	case tar.TypeDir:
		if err := paths.EnsureDirectory(target, true); err != nil {
			return err
		}
	case tar.TypeReg:
		if err := paths.EnsureDirectory(filepath.Dir(target), true); err != nil {
			return err
		}

		f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.FileMode(header.Mode))
		if err != nil {
			// try remove and re-open
			if err := os.Remove(target); err == nil {
				// remove success, open it
				f, err = os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.FileMode(header.Mode))
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if _, err := io.Copy(f, tr); err != nil {
			return err
		}

		f.Close()
	case tar.TypeLink:
		srcPath := header.Linkname
		if filepath.IsAbs(srcPath) {
			if adjustLink {
				absTargetWd, err := filepath.Abs(targetWd)
				if err != nil {
					return fmt.Errorf("extract hard link file %v -> %v fail: %v",
						header.Name, header.Linkname, err)
				}
				srcPath = filepath.Join(absTargetWd, srcPath)
			}
		} else {
			srcPath = filepath.Join(targetWd, srcPath)
		}

		if err := paths.Link(srcPath, target, false); err != nil {
			return err
		}
	case tar.TypeSymlink:
		srcPath := header.Linkname
		if filepath.IsAbs(srcPath) && adjustLink {
			absTargetWd, err := filepath.Abs(targetWd)
			if err != nil {
				return fmt.Errorf("extract symlink file %v -> %v fail: %v",
					header.Name, header.Linkname, err)
			}
			srcPath = filepath.Join(absTargetWd, srcPath)
		}

		if err := paths.Link(srcPath, target, true); err != nil {
			return err
		}
	default:
		msg := fmt.Sprintf("unsupported file type, file-> %+v", header)
		nlog.Warn(msg)
		if !skipUnNormalFile {
			return errors.New(msg)
		}

	}
	return nil
}
