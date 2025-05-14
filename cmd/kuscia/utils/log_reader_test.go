// Copyright 2025 Ant Group Co., Ltd.
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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadLastNLinesAsString_Normal(t *testing.T) {
	// Create a temporary file with test content
	tempFile, err := os.CreateTemp("", "test_log_*.txt")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	content := "line1\nline2\nline3\nline4\nline5\n"
	_, err = tempFile.WriteString(content)
	assert.NoError(t, err)
	tempFile.Close()

	// Read the last 3 lines
	result, err := ReadLastNLinesAsString(tempFile.Name(), 3)
	assert.NoError(t, err)
	assert.Equal(t, "line3\nline4\nline5\n", result, "The last 3 lines should match")
}

func TestReadLastNLinesAsString_FileNotFound(t *testing.T) {
	// Attempt to read a non-existent file
	_, err := ReadLastNLinesAsString("non_existent_file.txt", 3)
	assert.Error(t, err, "Reading a non-existent file should return an error")
}

func TestReadLastNLinesAsString_EmptyFile(t *testing.T) {
	// Create an empty temporary file
	tempFile, err := os.CreateTemp("", "empty_log_*.txt")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Read the last 3 lines from the empty file
	result, err := ReadLastNLinesAsString(tempFile.Name(), 3)
	assert.NoError(t, err)
	assert.Equal(t, "", result, "Reading an empty file should return an empty string")
}
