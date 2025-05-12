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

package conflistener

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"github.com/stretchr/testify/require"
)

func TestConcurrentUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("logLevel: info"), 0644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)


	start := make(chan struct{})

	go func() {
		<-start
		defer wg.Done()
		updateLogLevel(ctx, configPath)
	}()

	go func() {
		<-start
		defer wg.Done()
		updateLogLevel(ctx, configPath)
	}()

	close(start) 
	wg.Wait()    
}
