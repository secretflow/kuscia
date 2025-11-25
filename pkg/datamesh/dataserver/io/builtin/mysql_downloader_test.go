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

func initMySQLDownloader(t *testing.T, tableName string, ddSpec *v1alpha1.DomainDataSpec, queryColumn []string) (context.Context, *sql.DB, sqlmock.Sqlmock, *MySQLDownloader, *utils.DataMeshRequestContext, error) {
	ctx := context.Background()
	_, rc, db, mock := initMySQLIOTestRequestContext(t, tableName, nil, ddSpec, true)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)

	// if queryColumn is not null, change query columns
	if queryColumn != nil {
		rc.Query.Columns = queryColumn
	}

	downloader := NewMySQLDownloader(ctx, db, dd, rc.Query)
	return ctx, db, mock, downloader, rc, nil
}

func TestMySQLDownloader_New(t *testing.T) {
	t.Parallel()
	_, _, _, downloader, _, err := initMySQLDownloader(t, "", nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)
}

func TestMySQLDownloader_Read_Success(t *testing.T) {
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

	ctx, _, mock, downloader, rc, err := initMySQLDownloader(t, "", domaindataSpec, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	columns := make([]string, len(domaindataSpec.Columns))
	for idx, col := range domaindataSpec.Columns {
		columns[idx] = "`" + col.Name + "`"
	}

	sql := sqlbuilder.Select(columns...).From("`" + domaindataSpec.RelativeURI + "`").String()

	resultRow := mock.NewRows(columns)
	resultRow.AddRow("alice", int64(1), true, int8(0x7f), int16(0x7fff), int32(0x7fffffff),
		int64(0x80000000), uint8(0xff), uint16(0xffff), uint32(0xffffffff),
		uint64(0x100000000), uint64(0xffffffff), float32(1.000001),
		float64(1.00000000000001), "2023-12-25", "2023-12-25 14:30:45", "14:30:45", "14:30:45.123456", "2023-12-25 14:30:45")

	mock.ExpectQuery(sql).WillReturnRows(resultRow)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)

	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.NoError(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestMySQLDownloader_Read_PartialColumn(t *testing.T) {
	t.Parallel()
	tableName := "testTable"
	ctx, _, mock, downloader, rc, err := initMySQLDownloader(t, tableName, nil, []string{"id"})
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	rows := mock.NewRows([]string{"id"})
	// can't parse 1.0 to int
	rows.AddRow(1)

	mock.ExpectQuery("SELECT `id` FROM `" + tableName + "`").WillReturnRows(rows)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := GenerateArrowSchemaExclude(dd, map[string]struct{}{"name": {}})
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.NoError(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestMySQLDownloader_Read_QueryColumnError(t *testing.T) {
	t.Parallel()
	tableName := "testTable"
	_, _, _, downloader, _, err := initMySQLDownloader(t, tableName, nil, []string{"id", "age"})
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

func TestMySQLDownloader_Read_ParseError(t *testing.T) {
	t.Parallel()
	tableName := "testTable"

	ctx, _, mock, downloader, rc, err := initMySQLDownloader(t, tableName, nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	parseErrorRows := mock.NewRows([]string{"name", "id"})
	// can't parse 1.0 to int
	parseErrorRows.AddRow("alice", "1.0")

	mock.ExpectQuery("SELECT `name`, `id` FROM `" + tableName + "`").WillReturnRows(parseErrorRows)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.Error(t, downloader.DataProxyContentToFlightStreamSQL(writer))
}

func TestMySQLDownloader_Read_UnsafeTable(t *testing.T) {
	t.Parallel()
	tableName := "`testTable"

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
	ctx, _, _, downloader, rc, err := initMySQLDownloader(t, tableName, domaindataSpec, nil)
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

func TestMySQLDownloader_Read_UnsupportColumnType(t *testing.T) {
	t.Parallel()
	tableName := "`testTable"

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
	ctx, _, _, downloader, rc, err := initMySQLDownloader(t, tableName, domaindataSpec, nil)
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

func TestMySQLDownloader_Read_QueryError(t *testing.T) {
	t.Parallel()
	tableName := "testTable"
	ctx, _, mock, downloader, rc, err := initMySQLDownloader(t, tableName, nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	sqlErr := fmt.Errorf("test error")
	mock.ExpectQuery("SELECT `name`, `id` FROM `" + tableName + "`").WillReturnError(sqlErr)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := utils.GenerateArrowSchema(dd)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.ErrorIs(t, downloader.DataProxyContentToFlightStreamSQL(writer), sqlErr)

}

func TestInitFieldConverter(t *testing.T) {
	// Create test MySQLDownloader
	d := &MySQLDownloader{
		err: nil,
	}

	tests := []struct {
		name     string
		dataType arrow.DataType
		input    string
		expected interface{}
		hasError bool
	}{
		// [Test Case] Test scenario: Boolean type conversion
		{
			name:     "test boolean type",
			dataType: &arrow.BooleanType{},
			input:    "true",
			expected: true,
			hasError: false,
		},
		{
			name:     "test invalid boolean",
			dataType: &arrow.BooleanType{},
			input:    "not_a_boolean",
			hasError: true,
		},

		// [Test Case] Test scenario: Int8 type conversion
		{
			name:     "test int8 type",
			dataType: &arrow.Int8Type{},
			input:    "127",
			expected: int8(127),
			hasError: false,
		},
		{
			name:     "test int8 overflow",
			dataType: &arrow.Int8Type{},
			input:    "128", // Out of int8 range
			hasError: true,
		},

		// [Test Case] Test scenario: Int16 type conversion
		{
			name:     "test int16 type",
			dataType: &arrow.Int16Type{},
			input:    "32767",
			expected: int16(32767),
			hasError: false,
		},

		// [Test Case] Test scenario: Int32 type conversion
		{
			name:     "test int32 type",
			dataType: &arrow.Int32Type{},
			input:    "2147483647",
			expected: int32(2147483647),
			hasError: false,
		},

		// [Test Case] Test scenario: Int64 type conversion
		{
			name:     "test int64 type",
			dataType: &arrow.Int64Type{},
			input:    "9223372036854775807",
			expected: int64(9223372036854775807),
			hasError: false,
		},

		// [Test Case] Test scenario: Uint8 type conversion
		{
			name:     "test uint8 type",
			dataType: &arrow.Uint8Type{},
			input:    "255",
			expected: uint8(255),
			hasError: false,
		},

		// [Test Case] Test scenario: Uint16 type conversion
		{
			name:     "test uint16 type",
			dataType: &arrow.Uint16Type{},
			input:    "65535",
			expected: uint16(65535),
			hasError: false,
		},

		// [Test Case] Test scenario: Uint32 type conversion
		{
			name:     "test uint32 type",
			dataType: &arrow.Uint32Type{},
			input:    "4294967295",
			expected: uint32(4294967295),
			hasError: false,
		},

		// [Test Case] Test scenario: Uint64 type conversion
		{
			name:     "test uint64 type",
			dataType: &arrow.Uint64Type{},
			input:    "18446744073709551615",
			expected: uint64(18446744073709551615),
			hasError: false,
		},

		// [Test Case] Test scenario: Float32 type conversion
		{
			name:     "test float32 type",
			dataType: &arrow.Float32Type{},
			input:    "3.1415926",
			expected: float32(3.1415926),
			hasError: false,
		},
		{
			name:     "test invalid float32",
			dataType: &arrow.Float32Type{},
			input:    "not_a_float",
			hasError: true,
		},

		// [Test Case] Test scenario: Float64 type conversion
		{
			name:     "test float64 type",
			dataType: &arrow.Float64Type{},
			input:    "3.141592653589793",
			expected: 3.141592653589793,
			hasError: false,
		},

		// [Test Case] Test scenario: Date32 type conversion
		{
			name:     "test date32 type",
			dataType: &arrow.Date32Type{},
			input:    "2023-12-25",
			expected: arrow.Date32(19716), // 2023-12-25 converted to the number of days since 1970-01-01
			hasError: false,
		},
		{
			name:     "test invalid date32 format",
			dataType: &arrow.Date32Type{},
			input:    "invalid-date",
			hasError: true,
		},

		// [Test Case] Test scenario: Date64 type conversion
		{
			name:     "test date64 type",
			dataType: &arrow.Date64Type{},
			input:    "2023-12-25",
			expected: arrow.Date64(1703462400000), // 2023-12-25 00:00:00 ms
			hasError: false,
		},
		{
			name:     "test date64 with datetime",
			dataType: &arrow.Date64Type{},
			input:    "2023-12-25 14:30:45",
			expected: arrow.Date64(1703514645000), // 2023-12-25 14:30:45 ms
			hasError: false,
		},

		// [Test Case] Test scenario: Time32 type conversion
		{
			name:     "test time32 type - seconds",
			dataType: &arrow.Time32Type{Unit: arrow.Second},
			input:    "14:30:45",
			expected: arrow.Time32(52245), // 14*3600 + 30*60 + 45 = 52245s
			hasError: false,
		},
		{
			name:     "test time32 invalid format",
			dataType: &arrow.Time32Type{Unit: arrow.Second},
			input:    "25:70:90",
			hasError: true,
		},

		// [Test Case] Test scenario: Time64 type conversion
		{
			name:     "test time64 type - microseconds",
			dataType: &arrow.Time64Type{Unit: arrow.Microsecond},
			input:    "14:30:45.123456",
			expected: arrow.Time64(52245123456), // (14*3600+30*60+45)*1e6+123456 us
			hasError: false,
		},
		{
			name:     "test time64 invalid format",
			dataType: &arrow.Time64Type{Unit: arrow.Microsecond},
			input:    "invalid-time",
			hasError: true,
		},

		// [Test Case] Test scenario: Timestamp type conversion
		{
			name:     "test timestamp type - seconds",
			dataType: &arrow.TimestampType{Unit: arrow.Second, TimeZone: "UTC"},
			input:    "2023-12-25 14:30:45",
			expected: arrow.Timestamp(1703514645), // 2023-12-25 14:30:45 s
			hasError: false,
		},
		{
			name:     "test timestamp unix string",
			dataType: &arrow.TimestampType{Unit: arrow.Second},
			input:    "1703514645",
			expected: arrow.Timestamp(1703514645),
			hasError: false,
		},
		{
			name:     "test invalid timestamp format",
			dataType: &arrow.TimestampType{Unit: arrow.Second},
			input:    "invalid-timestamp",
			hasError: true,
		},

		// [Test Case] Test scenario: String type conversion
		{
			name:     "test string type",
			dataType: &arrow.StringType{},
			input:    "test string",
			expected: "test string",
			hasError: false,
		},

		// [Test Case] Test scenario: LargeString type conversion
		{
			name:     "test large string type",
			dataType: &arrow.LargeStringType{},
			input:    "test large string",
			expected: "test large string",
			hasError: false,
		},
	}

	mem := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer mem.AssertSize(t, 0)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bldr := array.NewBuilder(mem, tt.dataType)
			defer bldr.Release()

			d.err = nil
			converter := d.initFieldConverter(bldr)
			converter(tt.input)

			if tt.hasError {
				assert.NotNil(t, d.err, "expected error but got none")
				return
			}

			assert.Nil(t, d.err, "unexpected error: %v", d.err)

			arr := bldr.NewArray()
			defer arr.Release()

			switch arr := arr.(type) {
			case *array.Boolean:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Int8:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Int16:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Int32:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Int64:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Uint8:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Uint16:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Uint32:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Uint64:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Float32:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Float64:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.String:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.LargeString:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Date32:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Date64:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Time32:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Time64:
				assert.Equal(t, tt.expected, arr.Value(0))
			case *array.Timestamp:
				assert.Equal(t, tt.expected, arr.Value(0))
			}
		})
	}
}
