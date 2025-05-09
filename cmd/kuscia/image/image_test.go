package image

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

func TestNewImageCommand(t *testing.T) {
	t.Run("CommandStructure", func(t *testing.T) {
		ctx := context.Background()
		cmd := NewImageCommand(ctx)
		
		assert.Equal(t, "image", cmd.Use)
		assert.Equal(t, "Manage images", cmd.Short)
		assert.Equal(t, []string{"images"}, cmd.Aliases)
		assert.True(t, cmd.SilenceUsage)
	})

	t.Run("PersistentFlags", func(t *testing.T) {
		ctx := context.Background()
		cmd := NewImageCommand(ctx)
		
		
		storeFlag := cmd.PersistentFlags().Lookup("store")
		require.NotNil(t, storeFlag)
		assert.Equal(t, config.DefaultImageStoreDir(), storeFlag.DefValue)
		
		
		runtimeFlag := cmd.PersistentFlags().Lookup("runtime")
		require.NotNil(t, runtimeFlag)
		assert.Empty(t, runtimeFlag.DefValue)
	})
}