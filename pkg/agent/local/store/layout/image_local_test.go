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

package layout

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
)

func setupLocalImageTest(t *testing.T) (*LocalImage, []*kii.ImageName) {
	rootDir := t.TempDir()

	localLayout, err := NewLocalImage(rootDir)
	assert.NilError(t, err)

	newImage := func(image string) *kii.ImageName {
		in, _ := kii.NewImageName(image)
		return in
	}

	images := []*kii.ImageName{
		newImage("docker.io/000/aa:0.1"),
		newImage("docker.io/000/aa:latest"),
		newImage("111/bb:0.2"),
		newImage("111/bb:latest"),
	}

	for _, image := range images {
		assert.NilError(t, os.MkdirAll(localLayout.GetManifestsDir(image), 0755))
		assert.NilError(t, os.WriteFile(localLayout.GetManifestFilePath(image), []byte("hello world"), 0755))
	}

	return localLayout, images
}

func TestLocalImage_List(t *testing.T) {
	localLayout, _ := setupLocalImageTest(t)

	{
		retImages, err := localLayout.List(map[string]string{})
		expectedCount := 4
		expectedImages := []string{
			"docker.io/000/aa:0.1",
			"docker.io/000/aa:latest",
			"docker.io/111/bb:0.2",
			"docker.io/111/bb:latest",
		}
		assert.NilError(t, err)
		assert.Equal(t, len(retImages), expectedCount)

		for i := 0; i < expectedCount; i++ {
			assert.Equal(t, expectedImages[i], retImages[i].Image, "No.%d compare image full name", i)
		}
	}
}
