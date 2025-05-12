// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	fs := (&cobra.Command{}).Flags()
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
