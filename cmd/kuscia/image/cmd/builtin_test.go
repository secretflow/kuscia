package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/stretchr/testify/assert"
)

func TestBuiltinCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := builtinCommand(cmdCtx)

	assert.Equal(t, "builtin [OPTIONS]", cmd.Use)
	assert.Equal(t, "Load a built-in image", cmd.Short)
	assert.NotNil(t, cmd.Flags().Lookup("manifest"))

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image builtin secretflow:latest")
}
