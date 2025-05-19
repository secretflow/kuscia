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
	"github.com/apache/arrow/go/v13/arrow/flight"
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
