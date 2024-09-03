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

package utils

import (
	"io"
	"os"
)

// ReadLastNLinesAsString read the last N lines of a file as string
func ReadLastNLinesAsString(filePath string, n int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	fileSize := stat.Size()

	buf := make([]byte, 0)
	newlineChar := byte('\n')

	for offset := int64(1); offset <= fileSize; offset++ {
		_, err = file.Seek(-offset, io.SeekEnd)
		if err != nil {
			return "", err
		}

		b := make([]byte, 1)
		_, err = file.Read(b)
		if err != nil {
			return "", err
		}

		if b[0] == newlineChar {
			if len(buf) > 0 { // Avoid appending empty lines at the end of file
				n--
			}
			if n == 0 {
				break
			}
		}
		buf = append([]byte{b[0]}, buf...)
	}

	return string(buf), nil
}
