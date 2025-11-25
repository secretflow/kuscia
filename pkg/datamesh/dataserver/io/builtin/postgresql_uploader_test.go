// Copyright 2025 Ant Group Co., Ltd.
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
	"database/sql/driver"
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

func initPostgresqlUploader(t *testing.T, tableName string, ddSpec *v1alpha1.DomainDataSpec) (context.Context, *sql.DB, sqlmock.Sqlmock, *PostgresqlUploader, *utils.DataMeshRequestContext, error) {
	ctx := context.Background()
	_, rc, db, mock := initPostgresqlIOTestRequestContext(t, tableName, nil, ddSpec, true)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	uploader := NewPostgresqlUploader(ctx, db, dd, rc.Query)
	return ctx, db, mock, uploader, rc, nil
}

func TestPostgresqlUploader_New(t *testing.T) {
	t.Parallel()
	_, _, _, uploader, _, err := initPostgresqlUploader(t, "", nil)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)
}

func TestPostgresqlUploader_Write_Commit(t *testing.T) {
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

	_, _, mock, uploader, rc, err := initPostgresqlUploader(t, "\"output\"", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)
	columnDefines := "\"name\" TEXT, \"id\" BIGINT, \"boolTest\" BOOLEAN"
	columnDefines += ", \"int8Test\" SMALLINT, \"int16Test\" SMALLINT"
	columnDefines += ", \"int32Test\" INTEGER, \"int64Test\" BIGINT"
	columnDefines += ", \"uint8Test\" SMALLINT, \"uint16Test\" INTEGER"
	columnDefines += ", \"uint32Test\" BIGINT, \"uint64Test\" DECIMAL(20, 0)"
	columnDefines += ", \"uintTest\" DECIMAL(20, 0), \"float32Test\" REAL, \"float64Test\" DOUBLE PRECISION"

	dropSQL := regexp.QuoteMeta("DROP TABLE IF EXISTS \"output\" ")
	sql := regexp.QuoteMeta("CREATE TABLE \"output\" (" + columnDefines + ")")

	mock.ExpectExec(dropSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(sql).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO \"output\"")
	prepare.ExpectExec().WithArgs("alice", "1", true, "127", "32767", "2147483647",
		"2147483648", "255", "65535", "4294967295", "4294967296", "4294967295",
		"1.000001", "1.00000000000001",
		"bob", "2", false, "127", "32767", "2147483647",
		"2147483648", "255", "65535", "4294967295", "4294967296", "4294967295",
		"1.000001", "1.00000000000001",
	).WillReturnResult(sqlmock.NewResult(2, 2))
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

	err = uploader.FlightStreamToDataProxyContentPostgresql(reader)

	assert.NoError(t, err)
}

func TestPostgresqlUplaoder_Write_Delete(t *testing.T) {
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

	_, _, mock, uploader, rc, err := initPostgresqlUploader(t, "\"output\"", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)

	dropSQL := regexp.QuoteMeta("DROP TABLE IF EXISTS \"output\" ")
	deleteSQL := regexp.QuoteMeta("DELETE FROM \"output\"")

	mock.ExpectExec(dropSQL).WithoutArgs().WillReturnError(fmt.Errorf("DROP permission denied.")).WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(deleteSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO \"output\"")
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

	err = uploader.FlightStreamToDataProxyContentPostgresql(reader)

	assert.NoError(t, err)
}

func TestPostgresqlUploader_Write_UnsafeColumn(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "output",
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "\"name",
				Type: "str",
			},
			{
				Name: "id",
				Type: "int",
			},
		},
	}

	_, _, _, uploader, rc, err := initPostgresqlUploader(t, "\"output\"", domaindataSpec)
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

	err = uploader.FlightStreamToDataProxyContentPostgresql(reader)

	assert.Error(t, err)
}

func TestPostgresqlUploader_Write_UnsafeTable(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "\"output",
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

	_, _, _, uploader, rc, err := initPostgresqlUploader(t, "\"output\"", domaindataSpec)
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

	err = uploader.FlightStreamToDataProxyContentPostgresql(reader)

	assert.Error(t, err)
}

func TestPostgresqlUploader_Write_Rollback(t *testing.T) {
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

	_, _, mock, uploader, rc, err := initPostgresqlUploader(t, "\"output\"", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)

	// must use regexp to escape
	expectSQL := regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS \"output\" (\"name\" TEXT, \"id\" BIGINT SIGNED)")
	mock.ExpectExec(expectSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO \"output\"")
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

	err = uploader.FlightStreamToDataProxyContentPostgresql(reader)

	assert.Error(t, err)
}

// [Test Case] Test scenario: Test transformColToStringArr function for all Arrow data types
func TestPostgresqlUploader_TransformColToStringArr(t *testing.T) {
	t.Parallel()

	// Initialize an empty PostgresqlUploader
	uploader := &PostgresqlUploader{}

	tests := []struct {
		name     string
		typ      arrow.DataType
		col      arrow.Array
		expected []driver.Value
	}{
		// Test boolean type
		{
			name: "boolean type",
			typ:  arrow.FixedWidthTypes.Boolean,
			col: func() arrow.Array {
				builder := array.NewBooleanBuilder(memory.DefaultAllocator)
				builder.AppendValues([]bool{true, false}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{true, false},
		},
		// Test null boolean type
		{
			name: "null boolean type",
			typ:  arrow.FixedWidthTypes.Boolean,
			col: func() arrow.Array {
				builder := array.NewBooleanBuilder(memory.DefaultAllocator)
				builder.AppendValues([]bool{true, false}, []bool{false, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{sql.NullBool{}, false},
		},
		// Test int8 type
		{
			name: "int8 type",
			typ:  arrow.PrimitiveTypes.Int8,
			col: func() arrow.Array {
				builder := array.NewInt8Builder(memory.DefaultAllocator)
				builder.AppendValues([]int8{1, -1}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1", "-1"},
		},
		// Test null int8 type
		{
			name: "null int8 type",
			typ:  arrow.PrimitiveTypes.Int8,
			col: func() arrow.Array {
				builder := array.NewInt8Builder(memory.DefaultAllocator)
				builder.AppendValues([]int8{1, -1}, []bool{false, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{sql.Null[int8]{}, "-1"},
		},
		// Test int16 type
		{
			name: "int16 type",
			typ:  arrow.PrimitiveTypes.Int16,
			col: func() arrow.Array {
				builder := array.NewInt16Builder(memory.DefaultAllocator)
				builder.AppendValues([]int16{100, -100}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"100", "-100"},
		},
		// Test int32 type
		{
			name: "int32 type",
			typ:  arrow.PrimitiveTypes.Int32,
			col: func() arrow.Array {
				builder := array.NewInt32Builder(memory.DefaultAllocator)
				builder.AppendValues([]int32{1000, -1000}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1000", "-1000"},
		},
		// Test int64 type
		{
			name: "int64 type",
			typ:  arrow.PrimitiveTypes.Int64,
			col: func() arrow.Array {
				builder := array.NewInt64Builder(memory.DefaultAllocator)
				builder.AppendValues([]int64{1000000, -1000000}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1000000", "-1000000"},
		},
		// Test uint8 type
		{
			name: "uint8 type",
			typ:  arrow.PrimitiveTypes.Uint8,
			col: func() arrow.Array {
				builder := array.NewUint8Builder(memory.DefaultAllocator)
				builder.AppendValues([]uint8{1, 255}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1", "255"},
		},
		// Test uint16 type
		{
			name: "uint16 type",
			typ:  arrow.PrimitiveTypes.Uint16,
			col: func() arrow.Array {
				builder := array.NewUint16Builder(memory.DefaultAllocator)
				builder.AppendValues([]uint16{100, 65535}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"100", "65535"},
		},
		// Test uint32 type
		{
			name: "uint32 type",
			typ:  arrow.PrimitiveTypes.Uint32,
			col: func() arrow.Array {
				builder := array.NewUint32Builder(memory.DefaultAllocator)
				builder.AppendValues([]uint32{1000, 4294967295}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1000", "4294967295"},
		},
		// Test uint64 type
		{
			name: "uint64 type",
			typ:  arrow.PrimitiveTypes.Uint64,
			col: func() arrow.Array {
				builder := array.NewUint64Builder(memory.DefaultAllocator)
				builder.AppendValues([]uint64{1000000, 18446744073709551615}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1000000", "18446744073709551615"},
		},
		// Test float32 type
		{
			name: "float32 type",
			typ:  arrow.PrimitiveTypes.Float32,
			col: func() arrow.Array {
				builder := array.NewFloat32Builder(memory.DefaultAllocator)
				builder.AppendValues([]float32{1.234, -5.678}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1.234", "-5.678"},
		},
		// Test float64 type
		{
			name: "float64 type",
			typ:  arrow.PrimitiveTypes.Float64,
			col: func() arrow.Array {
				builder := array.NewFloat64Builder(memory.DefaultAllocator)
				builder.AppendValues([]float64{1.23456789, -5.67890123}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"1.23456789", "-5.67890123"},
		},
		// Test string type
		{
			name: "string type",
			typ:  arrow.BinaryTypes.String,
			col: func() arrow.Array {
				builder := array.NewStringBuilder(memory.DefaultAllocator)
				builder.AppendValues([]string{"hello", "world"}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"hello", "world"},
		},
		// Test largestring type
		{
			name: "largestring type",
			typ:  arrow.BinaryTypes.LargeString,
			col: func() arrow.Array {
				builder := array.NewLargeStringBuilder(memory.DefaultAllocator)
				builder.AppendValues([]string{"hello", "world"}, []bool{true, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{"hello", "world"},
		},
		// Test null string type
		{
			name: "null string type",
			typ:  arrow.BinaryTypes.String,
			col: func() arrow.Array {
				builder := array.NewStringBuilder(memory.DefaultAllocator)
				builder.AppendValues([]string{"hello", "world"}, []bool{false, true})
				return builder.NewArray()
			}(),
			expected: []driver.Value{sql.NullString{}, "world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uploader.transformColToStringArr(tt.typ, tt.col)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test unsupported type
	unsupportedType := &arrow.Decimal128Type{Precision: 10, Scale: 2}
	builder := array.NewBooleanBuilder(memory.DefaultAllocator)
	builder.Append(true)
	t.Run("unsupported type", func(t *testing.T) {
		assert.Panics(t, func() {
			uploader.transformColToStringArr(unsupportedType, builder.NewArray())
		})
	})
}

func TestPostgresqlUploader_Write_TimeColumn(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "event_log",
		Name:        "user-events",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "system",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "event_id",
				Type: "int",
			},
			{
				Name: "event_date",
				Type: "date32",
			},
			{
				Name: "event_time",
				Type: "time32",
			},
			{
				Name: "created_at",
				Type: "timestamp",
			},
			{
				Name: "updated_at",
				Type: "date64",
			},
			{
				Name: "log_time",
				Type: "time64",
			},
		},
	}

	_, _, mock, uploader, rc, err := initPostgresqlUploader(t, "\"event_log\"", domaindataSpec)
	assert.NotNil(t, uploader)
	assert.NoError(t, err)
	columnDefines := `"event_id" BIGINT, "event_date" DATE, "event_time" TIME WITHOUT TIME ZONE`
	columnDefines += `, "created_at" TIMESTAMPTZ, "updated_at" TIMESTAMP, "log_time" TIME WITHOUT TIME ZONE`

	dropSQL := regexp.QuoteMeta("DROP TABLE IF EXISTS \"event_log\" ")
	sql := regexp.QuoteMeta("CREATE TABLE \"event_log\" (" + columnDefines + ")")

	mock.ExpectExec(dropSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(sql).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO \"event_log\"")
	prepare.ExpectExec().WithArgs("1001", "2023-12-25", "09:00:00", "2023-12-25 14:30:45", "2023-12-25 08:00:00", "14:15:45.123000",
		"1002", "2024-01-01", "23:59:59", "2024-01-01 00:00:00", "2024-01-01 00:00:00", "23:59:59.999999",
	).WillReturnResult(sqlmock.NewResult(2, 2))
	mock.ExpectCommit()

	colType := []arrow.DataType{arrow.PrimitiveTypes.Int64,
		&arrow.Date32Type{}, &arrow.Time32Type{Unit: arrow.Second},
		&arrow.TimestampType{Unit: arrow.Second}, &arrow.Date64Type{},
		&arrow.Time64Type{Unit: arrow.Microsecond}}

	dataRows := [][]any{
		{
			int64(1001), int32(19716), int32(32400), int64(1703514645),
			int64(1703491200000), int64(51345123000),
		},
		{
			int64(1002), int32(19723), int32(86399), int64(1704067200),
			int64(1704067200000), int64(86399999999),
		},
	}

	inputs := getTableFlightData(t, rc, colType, dataRows)

	reader, err := flight.NewRecordReader(&mockDoPutServer{
		ServerStream: &mockGrpcServerStream{},
		nextDataList: inputs,
	})

	assert.NoError(t, err)
	assert.NotNil(t, reader)
	err = uploader.FlightStreamToDataProxyContentPostgresql(reader)
	assert.NoError(t, err)
}
