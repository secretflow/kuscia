package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewKernelCheckCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewKernelCheckCommand(ctx)

	assert.Equal(t, "kernel-check", cmd.Use)
	assert.Equal(t, "Check Kernel Params", cmd.Short)
	assert.Equal(t, `Check Kernel Params, make sure satisfy the requirements`, cmd.Long)

	// Test command execution
	err := cmd.RunE(cmd, []string{})
	assert.NoError(t, err)
}
