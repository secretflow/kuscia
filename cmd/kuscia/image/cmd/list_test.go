package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/stretchr/testify/assert"
)

func TestListCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := listCommand(cmdCtx)

	assert.Equal(t, "ls [OPTIONS]", cmd.Use)
	assert.Equal(t, "List local images", cmd.Short)
	assert.Contains(t, cmd.Aliases, "list")

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image list")
}
