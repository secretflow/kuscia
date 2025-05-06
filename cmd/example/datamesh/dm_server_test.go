package main

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewCommand(t *testing.T) {
	ctx := context.Background()
	o := &opts{}
	cmd := newCommand(ctx, o)

	assert.Equal(t, "flightMetaServer", cmd.Use)
	assert.Equal(t, "Mock flightMetaServer", cmd.Long)
	assert.NotNil(t, cmd.RunE)
}

func TestAddFlags(t *testing.T) {
	o := &opts{}
	fs := &cobra.Command{}.Flags()
	o.AddFlags(fs)

	assert.NotNil(t, fs.Lookup("listenAddr"))
	assert.NotNil(t, fs.Lookup("dataProxyEndpoint"))
	assert.NotNil(t, fs.Lookup("startClient"))
	assert.NotNil(t, fs.Lookup("startDataMesh"))
	assert.NotNil(t, fs.Lookup("enableDataMeshTLS"))
	assert.NotNil(t, fs.Lookup("testDataType"))
	assert.NotNil(t, fs.Lookup("outputCSVFilePath"))
	assert.NotNil(t, fs.Lookup("ossDataSource"))
	assert.NotNil(t, fs.Lookup("ossEndpoint"))
	assert.NotNil(t, fs.Lookup("ossAccessKey"))
	assert.NotNil(t, fs.Lookup("ossAccessSecret"))
	assert.NotNil(t, fs.Lookup("ossBucket"))
	assert.NotNil(t, fs.Lookup("ossPrefix"))
	assert.NotNil(t, fs.Lookup("ossType"))
}
