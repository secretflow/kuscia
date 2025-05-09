package cmd

import (
	"testing"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/stretchr/testify/assert"
)

func TestPullCommand(t *testing.T) {
	cmdCtx := &utils.ImageContext{}
	cmd := pullCommand(cmdCtx)

	assert.Equal(t, "pull image [OPTIONS]", cmd.Use)
	assert.Equal(t, "Pull an image from remote registry", cmd.Short)
	assert.NotNil(t, cmd.Flags().Lookup("creds"))

	// Test example
	example := cmd.Example
	assert.Contains(t, example, "kuscia image pull secretflow/secretflow:latest")
	assert.Contains(t, example, "--creds \"name:pass\"")
}
