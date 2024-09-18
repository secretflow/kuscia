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

package common

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompress(t *testing.T) {
	originalString := "Hello, World! This is a test string for compression and decompression."
	fmt.Println("Original String: ", originalString)
	// compress
	compressedData, err := CompressString(originalString)
	assert.NoError(t, err)
	fmt.Println("Compressed Data: ", compressedData)
	// decompress
	decompressedString, err := DecompressString(compressedData)
	assert.NoError(t, err)
	fmt.Println("Decompressed String: ", decompressedString)
	assert.Equal(t, originalString, decompressedString)
}

func TestSliceToAnnotation(t *testing.T) {
	slice := []string{"env-test1", "env-test2"}
	annotationVal := SliceToAnnotationString(slice)
	dstSlice := AnnotationStringToSlice(annotationVal)
	assert.True(t, reflect.DeepEqual(slice, dstSlice))
}
