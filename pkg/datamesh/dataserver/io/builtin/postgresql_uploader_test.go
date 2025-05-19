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
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
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
	columnDefines := "\"name\" TEXT, \"id\" BIGINT, \"boolTest\" SMALLINT"
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
	prepare.ExpectExec().WithArgs("alice", "1", "1", "127", "32767", "2147483647",
		"2147483648", "255", "65535", "4294967295", "4294967296", "4294967295",
		"1.000001", "1.00000000000001").WillReturnResult(sqlmock.NewResult(1, 1))
	prepare.ExpectExec().WithArgs("bob", "2", "0", "127", "32767", "2147483647",
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
