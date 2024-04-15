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

package kii

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewImageName(t *testing.T) {
	tests := []struct {
		Image string
		Repo  string
		Tag   string
	}{
		{
			Image: "ssecretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.2.0b0",
			Repo:  "ssecretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8",
			Tag:   "1.2.0b0",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			imageName, _ := NewImageName(tt.Image)
			assert.Equal(t, tt.Repo, imageName.Repo)
			assert.Equal(t, tt.Tag, imageName.Tag)
		})
	}
}

func TestParseImageNameFromPath(t *testing.T) {
	tests := []struct {
		Path  string
		Image string
		Repo  string
		Tag   string
	}{
		{
			Path:  "ssecretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8/1.2.0b0",
			Image: "ssecretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8:1.2.0b0",
			Repo:  "ssecretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/secretflow-lite-anolis8",
			Tag:   "1.2.0b0",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			imageName, err := ParseImageNameFromPath(tt.Path)
			assert.NoError(t, err)
			assert.Equal(t, tt.Image, imageName.Image)
			assert.Equal(t, tt.Repo, imageName.Repo)
			assert.Equal(t, tt.Tag, imageName.Tag)
		})
	}
}

func TestFormatImageID(t *testing.T) {
	tests := []struct {
		srcID string
		dstID string
	}{
		{
			"abc",
			"sha256:abc",
		},
		{
			"sha256:abc",
			"sha256:abc",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			assert.Equal(t, tt.dstID, FormatImageID(tt.srcID))
		})
	}
}
