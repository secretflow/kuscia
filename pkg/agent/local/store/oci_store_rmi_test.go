// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
)

// TestRemoveImage tests the RemoveImage function
func TestOciStore_RemoveImage(t *testing.T) {

	root := t.TempDir()
	store, err := NewOCIStore(root, mounter.Plain)

	assert.NoError(t, err)
	assert.NotNil(t, store)

	img, _ := kii.NewImageName("docker.io/secretflow/test:v1")
	assert.False(t, store.CheckImageExist(img))
	imgNames := []string{img.Tag}
	assert.NoError(t, store.RemoveImage(imgNames))

}
