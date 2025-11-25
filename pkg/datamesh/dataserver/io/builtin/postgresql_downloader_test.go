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
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
)

func initPostgresqlDownloader(t *testing.T, tableName string, ddSpec *v1alpha1.DomainDataSpec, queryColumn []string) (context.Context, *sql.DB, sqlmock.Sqlmock, *PostgresqlDownloader, *utils.DataMeshRequestContext, error) {
	ctx := context.Background()
	_, rc, db, mock := initPostgresqlIOTestRequestContext(t, tableName, nil, ddSpec, true)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)

	// if queryColumn is not null, change query columns
	if queryColumn != nil {
		rc.Query.Columns = queryColumn
	}

	downloader := NewPostgresqlDownloader(ctx, db, dd, rc.Query)
	return ctx, db, mock, downloader, rc, nil
}

func TestPostgresqlDownloader_New(t *testing.T) {
	t.Parallel()
	_, _, _, downloader, _, err := initPostgresqlDownloader(t, "", nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)
}

func TestPostgresqlDownloader_Read_Success(t *testing.T) {
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
			{
				Name: "date32Test",
				Type: "date32",
			},
			{
				Name: "date64Test",
				Type: "date64",
			},
			{
				Name: "time32Test",
				Type: "time32",
			},
			{
				Name: "time64Test",
				Type: "time64",
			},
			{
				Name: "timestampTest",
				Type: "timestamp",
			},
		},
	}

	ctx, _, mock, downloader, rc, err := initPostgresqlDownloader(t, "", domaindataSpec, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	columns := make([]string, len(domaindataSpec.Columns))
	for idx, col := range domaindataSpec.Columns {
		columns[idx] = "\"" + col.Name + "\""
	}

	sql := sqlbuilder.Select(columns...).From("\"" + domaindataSpec.RelativeURI + "\"").String()

	resultRow := mock.NewRows(columns)
	resultRow.AddRow("alice", int64(1), true, int8(0x7f), int16(0x7fff), int32(0x7fffffff),
		int64(0x80000000), uint8(0xff), uint16(0xffff), uint32(0xffffffff),
		uint64(0x100000000), uint64(0xffffffff), float32(1.000001),
		float64(1.00000000000001), "2023-12-25", "2023-12-25 14:30:45",
		"14:30:45", "14:30:45.123456", "2023-12-25 14:30:45")

	mock.ExpectQuery(sql).WillReturnRows(resultRow)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)

	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.NoError(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestPostgresqlDownloader_Read_PartialColumn(t *testing.T) {
	t.Parallel()
	tableName := "testTable"
	ctx, _, mock, downloader, rc, err := initPostgresqlDownloader(t, tableName, nil, []string{"id"})
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	rows := mock.NewRows([]string{"id"})
	// can't parse 1.0 to int
	rows.AddRow(1)

	mock.ExpectQuery("SELECT \"id\" FROM \"" + tableName + "\"").WillReturnRows(rows)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := GenerateArrowSchemaExclude(dd, map[string]struct{}{"name": {}})
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.NoError(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestPostgresqlDownloader_Read_QueryColumnError(t *testing.T) {
	t.Parallel()
	tableName := "testTable"
	_, _, _, downloader, _, err := initPostgresqlDownloader(t, tableName, nil, []string{"id", "age"})
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	// check column before write, writer doesn't matter here
	schema := &arrow.Schema{}
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.Error(t, downloader.DataProxyContentToFlightStreamSQL(nil))
}

func TestPostgresqlDownloader_Read_ParseError(t *testing.T) {
	t.Parallel()
	tableName := "testTable"

	ctx, _, mock, downloader, rc, err := initPostgresqlDownloader(t, tableName, nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	parseErrorRows := mock.NewRows([]string{"name", "id"})
	// can't parse 1.0 to int
	parseErrorRows.AddRow("alice", "1.0")

	mock.ExpectQuery("SELECT \"name\", \"id\" FROM \"" + tableName + "\"").WillReturnRows(parseErrorRows)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.Error(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestPostgresqlDownloader_Read_UnsafeTable(t *testing.T) {
	t.Parallel()
	tableName := "\"testTable"

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: tableName,
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
	ctx, _, _, downloader, rc, err := initPostgresqlDownloader(t, tableName, domaindataSpec, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.Error(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestPostgresqlDownloader_Read_UnsupportColumnType(t *testing.T) {
	t.Parallel()
	tableName := "\"testTable"

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: tableName,
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "name",
				Type: "binary",
			},
			{
				Name: "id",
				Type: "int",
			},
		},
	}
	ctx, _, _, downloader, rc, err := initPostgresqlDownloader(t, tableName, domaindataSpec, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.Error(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestPostgresqlDownloader_Read_TimeColumn(t *testing.T) {
	t.Parallel()

	domaindataSpec := &v1alpha1.DomainDataSpec{
		RelativeURI: "output",
		Name:        "alice-table",
		Type:        "TABLE",
		DataSource:  "data-" + uuid.New().String(),
		Author:      "alice",
		Columns: []v1alpha1.DataColumn{
			{
				Name: "id",
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
		},
	}

	ctx, _, mock, downloader, rc, err := initPostgresqlDownloader(t, "", domaindataSpec, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	columns := make([]string, len(domaindataSpec.Columns))
	for idx, col := range domaindataSpec.Columns {
		columns[idx] = "\"" + col.Name + "\""
	}

	sql := sqlbuilder.Select(columns...).From("\"" + domaindataSpec.RelativeURI + "\"").String()

	resultRow := mock.NewRows(columns)
	resultRow.AddRow(int64(1001), "2023-12-25", "09:15:30", "2023-12-25 09:15:30", "2023-12-25 14:30:45.123")
	resultRow.AddRow(int64(1002), "2024-01-01", "23:59:59", "2024-01-01 00:00:00", "2024-01-01 12:00:00.000")

	mock.ExpectQuery(sql).WillReturnRows(resultRow)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)

	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.NoError(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestPostgresqlDownloader_Read_QueryError(t *testing.T) {
	t.Parallel()
	tableName := "testTable"
	ctx, _, mock, downloader, rc, err := initPostgresqlDownloader(t, tableName, nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	sqlErr := fmt.Errorf("test error")
	mock.ExpectQuery("SELECT \"name\", \"id\" FROM \"" + tableName + "\"").WillReturnError(sqlErr)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.ErrorIs(t, downloader.DataProxyContentToFlightStreamSQL(writer), sqlErr)
}
func TestPostgresqlDownloader_initFieldConverter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		fieldType arrow.DataType
		value     string
		expectErr bool
	}{
		{"bool_true", &arrow.BooleanType{}, "true", false},
		{"bool_false", &arrow.BooleanType{}, "false", false},
		{"bool_invalid", &arrow.BooleanType{}, "not_bool", true},

		{"int8_valid", &arrow.Int8Type{}, "127", false},
		{"int8_invalid", &arrow.Int8Type{}, "128", true},

		{"int16_valid", &arrow.Int16Type{}, "32767", false},
		{"int16_invalid", &arrow.Int16Type{}, "32768", true},

		{"int32_valid", &arrow.Int32Type{}, "2147483647", false},
		{"int32_invalid", &arrow.Int32Type{}, "2147483648", true},

		{"int64_valid", &arrow.Int64Type{}, "9223372036854775807", false},
		{"int64_invalid", &arrow.Int64Type{}, "9223372036854775808", true},

		{"uint8_valid", &arrow.Uint8Type{}, "255", false},
		{"uint8_invalid", &arrow.Uint8Type{}, "256", true},

		{"uint16_valid", &arrow.Uint16Type{}, "65535", false},
		{"uint16_invalid", &arrow.Uint16Type{}, "65536", true},

		{"uint32_valid", &arrow.Uint32Type{}, "4294967295", false},
		{"uint32_invalid", &arrow.Uint32Type{}, "4294967296", true},

		{"uint64_valid", &arrow.Uint64Type{}, "18446744073709551615", false},
		{"uint64_invalid", &arrow.Uint64Type{}, "18446744073709551616", true},

		{"float32_valid", &arrow.Float32Type{}, "3.14159", false},
		{"float32_invalid", &arrow.Float32Type{}, "not_float", true},

		{"float64_valid", &arrow.Float64Type{}, "3.141592653589793", false},
		{"float64_invalid", &arrow.Float64Type{}, "not_double", true},

		// [Test Case] Test scenario: Date32 type conversion
		{"date32_valid", &arrow.Date32Type{}, "2023-12-25", false},
		{"date32_invalid", &arrow.Date32Type{}, "invalid-date", true},

		// [Test Case] Test scenario: Date64 type conversion
		{"date64_valid_date", &arrow.Date64Type{}, "2023-12-25", false},
		{"date64_valid_datetime", &arrow.Date64Type{}, "2023-12-25 14:30:45", false},
		{"date64_invalid", &arrow.Date64Type{}, "invalid-datetime", true},

		// [Test Case] Test scenario: Time32 type conversion
		{"time32_valid", &arrow.Time32Type{Unit: arrow.Second}, "14:30:45", false},
		{"time32_invalid", &arrow.Time32Type{Unit: arrow.Second}, "25:70:90", true},

		// [Test Case] Test scenario: Time64 type conversion
		{"time64_valid", &arrow.Time64Type{Unit: arrow.Microsecond}, "14:30:45.123456", false},
		{"time64_invalid", &arrow.Time64Type{Unit: arrow.Microsecond}, "invalid-time", true},

		// [Test Case] Test scenario: Timestamp type conversion
		{"timestamp_valid", &arrow.TimestampType{Unit: arrow.Second, TimeZone: "UTC"}, "2023-12-25 14:30:45", false},
		{"timestamp_unix_string", &arrow.TimestampType{Unit: arrow.Second}, "1703514645", false},
		{"timestamp_invalid", &arrow.TimestampType{Unit: arrow.Second}, "invalid-timestamp", true},

		{"string_valid", &arrow.StringType{}, "test_string", false},
		{"large_string_valid", &arrow.LargeStringType{}, "test_large_string", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bldr array.Builder
			switch tt.fieldType.(type) {
			case *arrow.BooleanType:
				bldr = array.NewBooleanBuilder(memory.DefaultAllocator)
			case *arrow.Int8Type:
				bldr = array.NewInt8Builder(memory.DefaultAllocator)
			case *arrow.Int16Type:
				bldr = array.NewInt16Builder(memory.DefaultAllocator)
			case *arrow.Int32Type:
				bldr = array.NewInt32Builder(memory.DefaultAllocator)
			case *arrow.Int64Type:
				bldr = array.NewInt64Builder(memory.DefaultAllocator)
			case *arrow.Uint8Type:
				bldr = array.NewUint8Builder(memory.DefaultAllocator)
			case *arrow.Uint16Type:
				bldr = array.NewUint16Builder(memory.DefaultAllocator)
			case *arrow.Uint32Type:
				bldr = array.NewUint32Builder(memory.DefaultAllocator)
			case *arrow.Uint64Type:
				bldr = array.NewUint64Builder(memory.DefaultAllocator)
			case *arrow.Float32Type:
				bldr = array.NewFloat32Builder(memory.DefaultAllocator)
			case *arrow.Float64Type:
				bldr = array.NewFloat64Builder(memory.DefaultAllocator)
			case *arrow.StringType:
				bldr = array.NewStringBuilder(memory.DefaultAllocator)
			case *arrow.LargeStringType:
				bldr = array.NewLargeStringBuilder(memory.DefaultAllocator)
			case *arrow.Date32Type:
				bldr = array.NewDate32Builder(memory.DefaultAllocator)
			case *arrow.Date64Type:
				bldr = array.NewDate64Builder(memory.DefaultAllocator)
			case *arrow.Time32Type:
				bldr = array.NewTime32Builder(memory.DefaultAllocator, arrow.FixedWidthTypes.Time32s.(*arrow.Time32Type))
			case *arrow.Time64Type:
				bldr = array.NewTime64Builder(memory.DefaultAllocator, arrow.FixedWidthTypes.Time64us.(*arrow.Time64Type))
			case *arrow.TimestampType:
				bldr = array.NewTimestampBuilder(memory.DefaultAllocator, arrow.FixedWidthTypes.Timestamp_s.(*arrow.TimestampType))
			default:
				bldr = array.NewStringBuilder(memory.DefaultAllocator) // fallback
			}

			defer bldr.Release()

			downloader := &PostgresqlDownloader{}
			converter := downloader.initFieldConverter(bldr)
			converter(tt.value)

			if tt.expectErr {
				assert.Error(t, downloader.err)
			} else {
				assert.NoError(t, downloader.err)
			}
		})
	}
}
