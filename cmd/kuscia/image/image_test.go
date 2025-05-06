package image

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewImageCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewImageCommand(ctx)

	assert.Equal(t, "image", cmd.Use)
	assert.Equal(t, "Manage images", cmd.Short)
	assert.Contains(t, cmd.Aliases, "images")

	// Test flags
	storeFlag := cmd.PersistentFlags().Lookup("store")
	assert.NotNil(t, storeFlag)
	assert.Equal(t, "kuscia image storage directory", storeFlag.Usage)

	runtimeFlag := cmd.PersistentFlags().Lookup("runtime")
	assert.NotNil(t, runtimeFlag)
	assert.Equal(t, "kuscia runtime type: runp/runc", runtimeFlag.Usage)

	// Test command execution (no-op for now)
	cmd.PersistentPreRun(&cobra.Command{}, []string{})
}
