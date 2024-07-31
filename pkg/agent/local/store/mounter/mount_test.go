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

package mounter

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/utils/assist"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

func TestMounter_unpackAndGetLayer(t *testing.T) {
	localLayout, err := layout.NewLocalImage(t.TempDir())
	assert.NilError(t, err)
	m := New(localLayout)

	layerHash := "abc"

	tempDir := t.TempDir()
	{
		f, err := os.Create(filepath.Join(tempDir, "temp.txt"))
		assert.NilError(t, err)
		defer f.Close()

		assert.NilError(t, f.Truncate(1000000))
	}
	tempTar, err := localLayout.GetTemporaryFile("pack-layer-")
	assert.NilError(t, err)
	defer os.Remove(tempTar.Name())

	assert.NilError(t, assist.Tar(tempDir, false, tempTar))

	assert.NilError(t, paths.Move(tempTar.Name(), localLayout.GetLayerTarFile(layerHash)))

	concurrency := 2
	wg := sync.WaitGroup{}
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(index int) {
			defer wg.Done()
			t.Logf("G%d start unpackAndGetLayer", index)
			layer, err := m.unpackAndGetLayer(layerHash)
			assert.NilError(t, err)
			t.Logf("G%d finish unpackAndGetLayer, layer is %v", index, layer)
		}(i)
	}
	wg.Wait()
}
