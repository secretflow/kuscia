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
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
)

func initMySQLUploader(t *testing.T, tableName string, ddSpec *v1alpha1.DomainDataSpec) (context.Context, *sql.DB, sqlmock.Sqlmock, *MySQLUploader, *utils.DataMeshRequestContext, error) {
	ctx := context.Background()
	_, rc, db, mock := initMySQLIOTestRequestContext(t, tableName, nil, ddSpec, true)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	uploader := NewMySQLUploader(ctx, db, dd, rc.Query)
	return ctx, db, mock, uploader, rc, nil
}

func TestMySQLUploader_New(t *testing.T) {
	t.Parallel()
	_, _, _, uploader, _, err := initMySQLUploader(t, "", nil)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)
}

func TestMySQLUploader_Write_Commit(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "output",
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "name",
				Type: "str",
			},
			{
				Name: "id",
				Type: "int",
			},
			{
				Name: "boolTest",
				Type: "bool",
			},
			{
				Name: "int8Test",
				Type: "int8",
			},
			{
				Name: "int16Test",
				Type: "int16",
			},
			{
				Name: "int32Test",
				Type: "int32",
			},
			{
				Name: "int64Test",
				Type: "int64",
			},
			{
				Name: "uint8Test",
				Type: "uint8",
			},
			{
				Name: "uint16Test",
				Type: "uint16",
			},
			{
				Name: "uint32Test",
				Type: "uint32",
			},
			{
				Name: "uint64Test",
				Type: "uint64",
			},
			{
				Name: "uintTest",
				Type: "uint",
			},
			{
				Name: "float32Test",
				Type: "float32",
			},
			{
				Name: "float64Test",
				Type: "float64",
			},
		},
	}

	_, _, mock, uploader, rc, err := initMySQLUploader(t, "`output`", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)
	columnDefines := "`name` TEXT, `id` BIGINT SIGNED, `boolTest` TINYINT(1)"
	columnDefines += ", `int8Test` TINYINT SIGNED, `int16Test` SMALLINT SIGNED"
	columnDefines += ", `int32Test` INT SIGNED, `int64Test` BIGINT SIGNED"
	columnDefines += ", `uint8Test` TINYINT UNSIGNED, `uint16Test` SMALLINT UNSIGNED"
	columnDefines += ", `uint32Test` INT UNSIGNED, `uint64Test` BIGINT UNSIGNED"
	columnDefines += ", `uintTest` BIGINT UNSIGNED, `float32Test` FLOAT, `float64Test` DOUBLE"

	dropSQL := regexp.QuoteMeta("DROP TABLE IF EXISTS `output` ")
	sql := regexp.QuoteMeta("CREATE TABLE `output` (" + columnDefines + ")")

	mock.ExpectExec(dropSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(sql).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO `output`")
	prepare.ExpectExec().WithArgs("alice", "1", "1", "127", "32767", "2147483647",
		"2147483648", "255", "65535", "4294967295", "4294967296", "4294967295",
		"1.000001", "1.00000000000001", "bob", "2", "0", "127", "32767", "2147483647",
		"2147483648", "255", "65535", "4294967295", "4294967296", "4294967295",
		"1.000001", "1.00000000000001").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	colType := []arrow.DataType{arrow.BinaryTypes.String, arrow.PrimitiveTypes.Int64,
		arrow.FixedWidthTypes.Boolean, arrow.PrimitiveTypes.Int8,
		arrow.PrimitiveTypes.Int16, arrow.PrimitiveTypes.Int32,
		arrow.PrimitiveTypes.Int64, arrow.PrimitiveTypes.Uint8,
		arrow.PrimitiveTypes.Uint16, arrow.PrimitiveTypes.Uint32,
		arrow.PrimitiveTypes.Uint64, arrow.PrimitiveTypes.Uint64,
		arrow.PrimitiveTypes.Float32, arrow.PrimitiveTypes.Float64}

	dataRows := [][]any{
		{
			"alice", int64(1), true, int8(0x7f), int16(0x7fff), int32(0x7fffffff),
			int64(0x80000000), uint8(0xff), uint16(0xffff), uint32(0xffffffff),
			uint64(0x100000000), uint64(0xffffffff), float32(1.000001),
			float64(1.00000000000001),
		},
		{
			"bob", int64(2), false, int8(0x7f), int16(0x7fff), int32(0x7fffffff),
			int64(0x80000000), uint8(0xff), uint16(0xffff), uint32(0xffffffff),
			uint64(0x100000000), uint64(0xffffffff), float32(1.000001),
			float64(1.00000000000001),
		},
	}

	inputs := getTableFlightData(t, rc, colType, dataRows)

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})

	assert.NoError(t, err)
	assert.NotNil(t, reader)

	err = uploader.FlightStreamToDataProxyContentMySQL(reader)

	assert.NoError(t, err)
}

func TestMySQLUplaoder_Write_Delete(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "output",
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "name",
				Type: "str",
			},
		},
	}

	_, _, mock, uploader, rc, err := initMySQLUploader(t, "`output`", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)

	dropSQL := regexp.QuoteMeta("DROP TABLE IF EXISTS `output` ")
	deleteSQL := regexp.QuoteMeta("DELETE FROM `output`")

	mock.ExpectExec(dropSQL).WithoutArgs().WillReturnError(fmt.Errorf("DROP permission denied.")).WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(deleteSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO `output`")
	prepare.ExpectExec().WithArgs("alice").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	colType := []arrow.DataType{arrow.BinaryTypes.String}

	dataRows := [][]any{
		{
			"alice",
		},
	}

	inputs := getTableFlightData(t, rc, colType, dataRows)

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})

	assert.NoError(t, err)
	assert.NotNil(t, reader)

	err = uploader.FlightStreamToDataProxyContentMySQL(reader)

	assert.NoError(t, err)
}

func TestMySQLUploader_Write_UnsafeColumn(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "output",
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "`name",
				Type: "str",
			},
			{
				Name: "id",
				Type: "int",
			},
		},
	}

	_, _, _, uploader, rc, err := initMySQLUploader(t, "`output`", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)

	colType := make([]arrow.DataType, 2)
	colType[0] = arrow.BinaryTypes.String
	colType[1] = arrow.PrimitiveTypes.Int64

	dataRows := make([][]any, 2)
	dataRows[0] = []any{"alice", int64(1)}
	dataRows[1] = []any{"bob", int64(2)}

	inputs := getTableFlightData(t, rc, colType, dataRows)

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})

	assert.NoError(t, err)
	assert.NotNil(t, reader)

	err = uploader.FlightStreamToDataProxyContentMySQL(reader)

	assert.Error(t, err)
}

func TestMySQLUploader_Write_UnsafeTable(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "`output",
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "name",
				Type: "str",
			},
			{
				Name: "id",
				Type: "int",
			},
		},
	}

	_, _, _, uploader, rc, err := initMySQLUploader(t, "`output`", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)

	colType := make([]arrow.DataType, 2)
	colType[0] = arrow.BinaryTypes.String
	colType[1] = arrow.PrimitiveTypes.Int64

	dataRows := make([][]any, 2)
	dataRows[0] = []any{"alice", int64(1)}
	dataRows[1] = []any{"bob", int64(2)}

	inputs := getTableFlightData(t, rc, colType, dataRows)

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})

	assert.NoError(t, err)
	assert.NotNil(t, reader)

	err = uploader.FlightStreamToDataProxyContentMySQL(reader)

	assert.Error(t, err)
}

func TestMySQLUploader_Write_Rollback(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "output",
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "name",
				Type: "str",
			},
			{
				Name: "id",
				Type: "int",
			},
		},
	}

	_, _, mock, uploader, rc, err := initMySQLUploader(t, "`output`", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)

	// must use regexp to escape
	expectSQL := regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS `output` (`name` TEXT, `id` BIGINT SIGNED)")
	mock.ExpectExec(expectSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO `output`")
	prepare.ExpectExec().WithArgs("alice", "1").WillReturnResult(sqlmock.NewResult(1, 1))
	prepare.ExpectExec().WithArgs("bob", "2").WillReturnError(errors.Errorf("test error"))
	mock.ExpectRollback()

	colType := make([]arrow.DataType, 2)
	colType[0] = arrow.BinaryTypes.String
	colType[1] = arrow.PrimitiveTypes.Int64

	dataRows := make([][]any, 2)
	dataRows[0] = []any{"alice", int64(1)}
	dataRows[1] = []any{"bob", int64(2)}

	inputs := getTableFlightData(t, rc, colType, dataRows)

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})

	assert.NoError(t, err)
	assert.NotNil(t, reader)

	err = uploader.FlightStreamToDataProxyContentMySQL(reader)

	assert.Error(t, err)
}

// [Unit Test] Test scenario: Test transformColToStringArr for various arrow data types
func TestMySQLUploader_transformColToStringArr(t *testing.T) {
	t.Parallel()

	uploader := &MySQLUploader{nullValue: "NULL"}

	tests := []struct {
		name   string
		typ    arrow.DataType
		values []interface{}
		want   []string
	}{
		// Boolean type
		{
			name:   "boolean",
			typ:    arrow.FixedWidthTypes.Boolean,
			values: []interface{}{true, false, nil},
			want:   []string{"1", "0", "NULL"},
		},
		// Integer types
		{
			name:   "int8",
			typ:    arrow.PrimitiveTypes.Int8,
			values: []interface{}{int8(1), int8(2), nil},
			want:   []string{"1", "2", "NULL"},
		},
		{
			name:   "int16",
			typ:    arrow.PrimitiveTypes.Int16,
			values: []interface{}{int16(100), int16(200), nil},
			want:   []string{"100", "200", "NULL"},
		},
		{
			name:   "int32",
			typ:    arrow.PrimitiveTypes.Int32,
			values: []interface{}{int32(1000), int32(2000), nil},
			want:   []string{"1000", "2000", "NULL"},
		},
		{
			name:   "int64",
			typ:    arrow.PrimitiveTypes.Int64,
			values: []interface{}{int64(10000), int64(20000), nil},
			want:   []string{"10000", "20000", "NULL"},
		},
		// Unsigned integer types
		{
			name:   "uint8",
			typ:    arrow.PrimitiveTypes.Uint8,
			values: []interface{}{uint8(255), uint8(128), nil},
			want:   []string{"255", "128", "NULL"},
		},
		{
			name:   "uint16",
			typ:    arrow.PrimitiveTypes.Uint16,
			values: []interface{}{uint16(65535), uint16(32768), nil},
			want:   []string{"65535", "32768", "NULL"},
		},
		{
			name:   "uint32",
			typ:    arrow.PrimitiveTypes.Uint32,
			values: []interface{}{uint32(4294967295), uint32(2147483648), nil},
			want:   []string{"4294967295", "2147483648", "NULL"},
		},
		{
			name:   "uint64",
			typ:    arrow.PrimitiveTypes.Uint64,
			values: []interface{}{uint64(18446744073709551615), uint64(9223372036854775808), nil},
			want:   []string{"18446744073709551615", "9223372036854775808", "NULL"},
		},
		// Floating point types
		{
			name:   "float32",
			typ:    arrow.PrimitiveTypes.Float32,
			values: []interface{}{float32(1.234), float32(5.678), nil},
			want:   []string{"1.234", "5.678", "NULL"},
		},
		{
			name:   "float64",
			typ:    arrow.PrimitiveTypes.Float64,
			values: []interface{}{float64(1.23456789), float64(9.87654321), nil},
			want:   []string{"1.23456789", "9.87654321", "NULL"},
		},
		// String types
		{
			name:   "string",
			typ:    arrow.BinaryTypes.String,
			values: []interface{}{"hello", "world", nil},
			want:   []string{"hello", "world", "NULL"},
		},
		{
			name:   "large string",
			typ:    arrow.BinaryTypes.LargeString,
			values: []interface{}{"large hello", "large world", nil},
			want:   []string{"large hello", "large world", "NULL"},
		},
		// Time types
		{
			name:   "date32",
			typ:    arrow.PrimitiveTypes.Date32,
			values: []interface{}{int32(19555), int32(19566), nil}, // 2023-07-25, 2023-08-05
			want:   []string{"2023-07-17", "2023-07-28", "NULL"},
		},
		{
			name:   "date64",
			typ:    arrow.PrimitiveTypes.Date64,
			values: []interface{}{int64(1690204800000), int64(1691112000000), nil}, // 2023-07-25 08:00:00, 2023-08-04 12:00:00
			want:   []string{"2023-07-24 13:20:00", "2023-08-04 01:20:00", "NULL"},
		},
		{
			name:   "time32_second",
			typ:    arrow.FixedWidthTypes.Time32s,
			values: []interface{}{int32(3600), int32(7200), nil}, // 01:00:00, 02:00:00
			want:   []string{"01:00:00", "02:00:00", "NULL"},
		},
		{
			name:   "time32_millisecond",
			typ:    arrow.FixedWidthTypes.Time32ms,
			values: []interface{}{int32(3600000), int32(7200000), nil}, // 01:00:00, 02:00:00 (in milliseconds)
			want:   []string{"01:00:00", "02:00:00", "NULL"},
		},
		{
			name:   "time64_microsecond",
			typ:    arrow.FixedWidthTypes.Time64us,
			values: []interface{}{int64(3600000000), int64(7200000000), nil}, // 01:00:00, 02:00:00 (in microseconds)
			want:   []string{"01:00:00.000000", "02:00:00.000000", "NULL"},
		},
		{
			name:   "time64_nanosecond",
			typ:    arrow.FixedWidthTypes.Time64ns,
			values: []interface{}{int64(3600000000000), int64(7200000000000), nil}, // 01:00:00, 02:00:00 (in nanoseconds)
			want:   []string{"01:00:00.000000", "02:00:00.000000", "NULL"},
		},
		{
			name:   "timestamp_second",
			typ:    arrow.FixedWidthTypes.Timestamp_s,
			values: []interface{}{int64(1690204800), int64(1691112000), nil}, // 2023-07-25 08:00:00, 2023-08-04 12:00:00
			want:   []string{"2023-07-24 13:20:00", "2023-08-04 01:20:00", "NULL"},
		},
		{
			name:   "timestamp_millisecond",
			typ:    arrow.FixedWidthTypes.Timestamp_ms,
			values: []interface{}{int64(1690204800000), int64(1691112000000), nil}, // 2023-07-25 08:00:00, 2023-08-04 12:00:00
			want:   []string{"2023-07-24 13:20:00", "2023-08-04 01:20:00", "NULL"},
		},
		{
			name:   "timestamp_microsecond",
			typ:    arrow.FixedWidthTypes.Timestamp_us,
			values: []interface{}{int64(1690204800000000), int64(1691112000000000), nil}, // 2023-07-25 08:00:00, 2023-08-04 12:00:00
			want:   []string{"2023-07-24 13:20:00", "2023-08-04 01:20:00", "NULL"},
		},
		{
			name:   "timestamp_nanosecond",
			typ:    arrow.FixedWidthTypes.Timestamp_ns,
			values: []interface{}{int64(1690204800000000000), int64(1691112000000000000), nil}, // 2023-07-25 08:00:00, 2023-08-04 12:00:00
			want:   []string{"2023-07-24 13:20:00", "2023-08-04 01:20:00", "NULL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test array
			builder := array.NewBuilder(memory.DefaultAllocator, tt.typ)
			defer builder.Release()

			for _, v := range tt.values {
				if v == nil {
					builder.AppendNull()
				} else {
					switch b := builder.(type) {
					case *array.BooleanBuilder:
						b.Append(v.(bool))
					case *array.Int8Builder:
						b.Append(v.(int8))
					case *array.Int16Builder:
						b.Append(v.(int16))
					case *array.Int32Builder:
						b.Append(v.(int32))
					case *array.Int64Builder:
						b.Append(v.(int64))
					case *array.Uint8Builder:
						b.Append(v.(uint8))
					case *array.Uint16Builder:
						b.Append(v.(uint16))
					case *array.Uint32Builder:
						b.Append(v.(uint32))
					case *array.Uint64Builder:
						b.Append(v.(uint64))
					case *array.Float32Builder:
						b.Append(v.(float32))
					case *array.Float64Builder:
						b.Append(v.(float64))
					case *array.StringBuilder:
						b.Append(v.(string))
					case *array.LargeStringBuilder:
						b.Append(v.(string))
					case *array.Date32Builder:
						b.Append(arrow.Date32(v.(int32)))
					case *array.Date64Builder:
						b.Append(arrow.Date64(v.(int64)))
					case *array.Time32Builder:
						b.Append(arrow.Time32(v.(int32)))
					case *array.Time64Builder:
						b.Append(arrow.Time64(v.(int64)))
					case *array.TimestampBuilder:
						b.Append(arrow.Timestamp(v.(int64)))
					}
				}
			}

			arr := builder.NewArray()
			defer arr.Release()

			// Test transformation
			got := uploader.transformColToStringArr(tt.typ, arr)
			assert.Equal(t, tt.want, got)
		})
	}

	// Test unsupported data type
	t.Run("unsupported type", func(t *testing.T) {
		unsupportedType := &arrow.DurationType{}
		builder := array.NewBuilder(memory.DefaultAllocator, unsupportedType)
		defer builder.Release()
		builder.AppendNull()
		arr := builder.NewArray()
		defer arr.Release()

		assert.Panics(t, func() {
			uploader.transformColToStringArr(unsupportedType, arr)
		}, "Should panic for unsupported type")
	})
}
