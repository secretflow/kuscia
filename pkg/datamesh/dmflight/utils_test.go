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

package dmflight

import (
	"testing"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func TestGenerateArrowSchema_ValidSchema(t *testing.T) {
	t.Parallel()
	data := &datamesh.DomainData{
		Columns: []*v1alpha1.DataColumn{
			{Name: "id", Type: "int64", NotNullable: true},
			{Name: "name", Type: "string", NotNullable: false},
		},
	}

	expectedFields := []arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
	}

	schema, err := GenerateArrowSchema(data)
	assert.NoError(t, err)
	assert.Equal(t, expectedFields, schema.Fields())
}

func TestGenerateArrowSchema_InValidSchema(t *testing.T) {
	t.Parallel()
	data := &datamesh.DomainData{
		Columns: []*v1alpha1.DataColumn{
			{Name: "invalid", Type: "unknown", NotNullable: true},
		},
	}

	_, err := GenerateArrowSchema(data)
	assert.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGenerateArrowSchema_EmptySchema(t *testing.T) {
	t.Parallel()
	data := &datamesh.DomainData{
		Columns: []*v1alpha1.DataColumn{},
	}

	expectedFields := []arrow.Field{}

	schema, err := GenerateArrowSchema(data)
	assert.NoError(t, err)
	assert.Equal(t, expectedFields, schema.Fields())
}

func TestCreateDateMeshFlightInfo(t *testing.T) {
	t.Parallel()
	info := CreateDateMeshFlightInfo([]byte("xyz"), "url")

	assert.NotNil(t, info)
	assert.Equal(t, "url", info.Endpoint[0].Location[0].Uri)
	assert.Equal(t, "xyz", string(info.Endpoint[0].Ticket.Ticket))
	assert.Equal(t, flight.DescriptorCMD, info.FlightDescriptor.Type)
}

func TestParseRegionFromEndpoint(t *testing.T) {
	t.Parallel()
	region, err := ParseRegionFromEndpoint("xyz")
	assert.NoError(t, err)
	assert.Equal(t, "", region)

	region, err = ParseRegionFromEndpoint("http://shanghai.aliyuncs.com/")
	assert.NoError(t, err)
	assert.Equal(t, "shanghai", region)

	region, err = ParseRegionFromEndpoint("http://shanghai.aliyuncs.com.cn:9000/")
	assert.NoError(t, err)
	assert.Equal(t, "", region)

	region, err = ParseRegionFromEndpoint("http://shanghai.aliyuncs.com.cn/")
	assert.NoError(t, err)
	assert.Equal(t, "", region)

	region, err = ParseRegionFromEndpoint("http://shanghai.aliyuncs.com.cn/http://xyz")
	assert.NoError(t, err)
	assert.Equal(t, "", region)
}

func TestPackActionResult(t *testing.T) {
	t.Parallel()
	r, err := PackActionResult(nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestGenerateBinaryDataArrowSchema(t *testing.T) {
	t.Parallel()
	schema := GenerateBinaryDataArrowSchema()
	assert.NotNil(t, schema)

	assert.NotNil(t, schema.Fields())
	assert.Equal(t, 1, len(schema.Fields()))
	assert.Equal(t, arrow.BinaryTypes.Binary, schema.Fields()[0].Type)
}

func TestDescForCommand(t *testing.T) {
	t.Parallel()
	desc, err := DescForCommand(nil)

	assert.Error(t, err)
	assert.Nil(t, desc)

	desc, err = DescForCommand(&datamesh.CommandDomainDataQuery{
		DomaindataId: "xyz",
	})
	assert.NoError(t, err)
	assert.NotNil(t, desc)
	assert.Equal(t, flight.DescriptorCMD, desc.Type)

	var any anypb.Any
	err = proto.Unmarshal(desc.Cmd, &any)
	assert.NoError(t, err)

	msg, err := any.UnmarshalNew()
	assert.NoError(t, err)
	assert.NotNil(t, msg)

	m, ok := msg.(*datamesh.CommandDomainDataQuery)
	assert.True(t, ok)
	assert.NotNil(t, m)
	assert.Equal(t, "xyz", m.DomaindataId)
}
