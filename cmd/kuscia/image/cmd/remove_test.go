package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/stretchr/testify/assert"
)

func TestRemoveCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := rmCommand(cmdCtx)

	assert.Equal(t, "rm image [OPTIONS]", cmd.Use)
	assert.Equal(t, "Remove local one image by imageName or imageID", cmd.Short)

	// Check if cmd.Args is of type cobra.MinimumNArgs(1)
	assert.NotNil(t, cmd.Args)

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image rm 1111111")
	assert.Contains(t, example, "kuscia image rm my-image:latest")
}
