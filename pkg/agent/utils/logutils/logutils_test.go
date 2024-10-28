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

package logutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func openLogger(t *testing.T, fileName string) *ReopenableLogger {
	logger := NewReopenableLogger(fileName)
	assert.NotNil(t, logger)
	return logger
}

func writeSomething(t *testing.T, logger *ReopenableLogger) {
	testLogContent := []byte("Test log content")
	logger.Write(testLogContent)
	assert.FileExists(t, logger.fileName)
	content, err := os.ReadFile(logger.fileName)
	assert.NoError(t, err)
	assert.Equal(t, "Test log content", string(content))
}

func Test_NewLogger(t *testing.T) {
	t.Parallel()
	openLogger(t, "test.log")
}

func Test_Write(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.log")
	logger := openLogger(t, fileName)
	writeSomething(t, logger)
}

func Test_Close(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.log")
	logger := openLogger(t, fileName)
	assert.Nil(t, logger.Close())
	writeSomething(t, logger)
	assert.Nil(t, logger.Close())
}

func Test_ReopenFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test.log")
	logger := openLogger(t, fileName)
	writeSomething(t, logger)
	assert.Nil(t, logger.ReopenFile())
	// log file should be cleared after reopen
	assert.FileExists(t, logger.fileName)
	content, err := os.ReadFile(logger.fileName)
	assert.NoError(t, err)
	assert.Equal(t, "", string(content))
	writeSomething(t, logger)
}
