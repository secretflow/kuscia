package main

import (
	"testing"

	"github.com/secretflow/kuscia/pkg/transport/commands"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestMainCommand(t *testing.T) {
	opts := &commands.Opts{}
	rootCmd := commands.NewCommand(opts)
	opts.AddFlags(rootCmd.Flags())

	assert.NotNil(t, rootCmd)
	assert.IsType(t, &cobra.Command{}, rootCmd)
}
