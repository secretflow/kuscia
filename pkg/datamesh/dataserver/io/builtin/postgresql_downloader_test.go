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
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/stretchr/testify/assert"
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
		float64(1.00000000000001))

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

	mock.ExpectQuery("SELECT \"i\" FROM \"" + tableName + "\"").WillReturnRows(rows)

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
