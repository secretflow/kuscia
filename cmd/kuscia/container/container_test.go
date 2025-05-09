package container

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewContainerCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewContainerCommand(ctx)

	assert.Equal(t, "container", cmd.Use)
	assert.Equal(t, "Manage containers", cmd.Short)

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
