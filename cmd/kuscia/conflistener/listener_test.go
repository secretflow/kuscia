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

