package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestTagCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := tagCommand(cmdCtx)

	assert.Equal(t, "tag SOURCE_IMAGE[:TAG] TARGET_IMAGE[:TAG]", cmd.Use)
	assert.Equal(t, "Create a tag TARGET_IMAGE that refers to SOURCE_IMAGE", cmd.Short)
	assert.Equal(t, cobra.ExactArgs(2), cmd.Args)

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image tag secretflow/secretflow:v1 registry.mycompany.com/secretflow/secretflow:v1")
}
