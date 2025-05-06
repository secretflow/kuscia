package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/stretchr/testify/assert"
)

func TestLoadCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := loadCommand(cmdCtx)

	assert.Equal(t, "load [OPTIONS]", cmd.Use)
	assert.Equal(t, "Load an image from a tar archive or STDIN", cmd.Short)
	assert.NotNil(t, cmd.Flags().Lookup("input"))

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image load < app.tar")
	assert.Contains(t, example, "--input app.tar")
}
