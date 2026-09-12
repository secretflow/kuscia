// Copyright 2026 Ant Group Co., Ltd.
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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUntarDotFilenames(t *testing.T) {
	for _, name := range []string{"file.txt", "model..json", "..data", ".../config", "nested/model..json"} {
		t.Run(name, func(t *testing.T) {
			var archive bytes.Buffer
			writer := tar.NewWriter(&archive)
			require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: 4}))
			_, err := writer.Write([]byte("data"))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			destination := t.TempDir()
			require.NoError(t, Untar(destination, false, false, &archive, false))
			content, err := os.ReadFile(filepath.Join(destination, name))
			require.NoError(t, err)
			require.Equal(t, "data", string(content))
		})
	}
}

func TestUntarRejectsParentDirectory(t *testing.T) {
	for _, name := range []string{"../outside", "nested/../../outside"} {
		t.Run(name, func(t *testing.T) {
			var archive bytes.Buffer
			writer := tar.NewWriter(&archive)
			require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: 4}))
			_, err := writer.Write([]byte("data"))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			root := t.TempDir()
			require.Error(t, Untar(filepath.Join(root, "destination"), false, false, &archive, false))
			require.NoFileExists(t, filepath.Join(root, "outside"))
		})
	}
}
