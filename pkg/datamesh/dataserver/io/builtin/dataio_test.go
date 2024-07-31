// Copyright 2024 Ant Group Co., Ltd.
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

package builtin

import (
	"bytes"
	"errors"
	"testing"

	gomonkeyv2 "github.com/agiledragon/gomonkey/v2"
	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func getTestDomainData() *datamesh.DomainData {
	return &datamesh.DomainData{
		DomaindataId: "test-domain-data",
		DatasourceId: common.DefaultDataSourceID,
	}
}

func TestDataProxyContentToFlightStreamBinary_WriteFailed(t *testing.T) {
	data := getTestDomainData()

	reader := bytes.NewReader([]byte("abcdefg"))

	writer := &flight.Writer{}
	// mock writer.write

	patch := gomonkeyv2.ApplyMethod(writer, "Write", func(w *flight.Writer, rec arrow.Record) error {
		return errors.New("write failed")
	})
	defer patch.Reset()

	assert.Error(t, DataProxyContentToFlightStreamBinary(data, reader, writer, 1024))
}

func TestDataProxyContentToFlightStreamBinary_WriteSuccess(t *testing.T) {
	t.Parallel()
	data := getTestDomainData()

	reader := bytes.NewReader([]byte("abcdefg"))

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))

	assert.NoError(t, DataProxyContentToFlightStreamBinary(data, reader, writer, 1024))

	assert.Len(t, mgs.dataList, 2)

	res, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: mgs.dataList,
	})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.Next())

	rec := res.Record()
	assert.NotNil(t, rec)

	assert.NotNil(t, rec)
	assert.Equal(t, int64(1), rec.NumCols())
	assert.Equal(t, "data", rec.ColumnName(0))

	col := rec.Columns()[0]
	assert.NotNil(t, col)

	assert.Equal(t, arrow.BinaryTypes.Binary, col.DataType())

	b, ok := col.(*array.Binary)
	assert.True(t, ok)
	assert.NotNil(t, b)
	assert.Equal(t, 1, b.Len())
	assert.Equal(t, "abcdefg", string(b.Value(0)))
}

func TestDataProxyContentToFlightStreamBinary_Loop(t *testing.T) {
	t.Parallel()
	data := getTestDomainData()

	reader := bytes.NewReader(make([]byte, 100))

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))

	assert.NoError(t, DataProxyContentToFlightStreamBinary(data, reader, writer, 50))
	// first data is schema
	assert.Equal(t, 3, len(mgs.dataList))
}

func getFlightData(t *testing.T, inputs [][]byte) []*flight.FlightData {
	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(utils.GenerateBinaryDataArrowSchema()))

	for i := 0; i < len(inputs); i++ {

		builder := array.NewBinaryBuilder(memory.NewGoAllocator(), arrow.BinaryTypes.Binary)
		builder.Append(inputs[i])

		assert.NoError(t, writer.Write(array.NewRecord(utils.GenerateBinaryDataArrowSchema(), []arrow.Array{builder.NewArray()}, 1)))
	}

	writer.Close()

	return mgs.dataList
}

func TestFlightStreamToDataProxyContentBinary_NoData(t *testing.T) {
	t.Parallel()
	data := getTestDomainData()

	inputs := getFlightData(t, [][]byte{})

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})
	assert.NoError(t, err)

	writer := bytes.NewBuffer([]byte{})

	assert.NoError(t, FlightStreamToDataProxyContentBinary(data, writer, reader))
}

func TestFlightStreamToDataProxyContentBinary_Success(t *testing.T) {
	t.Parallel()
	data := getTestDomainData()

	writer := bytes.NewBuffer([]byte{})

	inputs := getFlightData(t, [][]byte{[]byte("xyz")})

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})
	assert.NoError(t, err)

	assert.NoError(t, FlightStreamToDataProxyContentBinary(data, writer, reader))
	assert.Equal(t, "xyz", writer.String())
}

func TestFlightStreamToDataProxyContentBinary_ErrorFormat(t *testing.T) {
	data := getTestDomainData()

	writer := bytes.NewBuffer([]byte{})

	reader := &flight.Reader{}

	times := 0
	patch := gomonkeyv2.ApplyMethod(reader.Reader, "Next", func(*ipc.Reader) bool {
		times++
		// first return true, second return false
		return times == 1
	})

	patch.ApplyMethod(reader.Reader, "Record", func(*ipc.Reader) arrow.Record {
		// column error
		builder := array.NewBinaryBuilder(memory.NewGoAllocator(), arrow.BinaryTypes.Binary)
		builder.Append([]byte("xyz"))
		schema, _ := utils.GenerateBinaryDataArrowSchema().AddField(1, arrow.Field{
			Name:     "invalidate",
			Type:     &arrow.BooleanType{},
			Nullable: false,
		})

		builder2 := array.NewBooleanBuilder(memory.NewGoAllocator())
		builder2.Append(true)

		return array.NewRecord(schema, []arrow.Array{builder.NewArray(), builder2.NewArray()}, 1)

	})
	defer patch.Reset()

	assert.Error(t, FlightStreamToDataProxyContentBinary(data, writer, reader))
	assert.Equal(t, "xyz", writer.String())
}
