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
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dbInfoToDsnPg(info *datamesh.DatabaseDataSourceInfo) string {
	return fmt.Sprintf("user=%s password=%s host=%s dbname=%s port=5432",
		info.User,
		info.Password,
		info.Endpoint,
		info.Database,
	)
}

func initPostgresqlContextTestEnv(t *testing.T, domaindataSpec *v1alpha1.DomainDataSpec) *config.DataMeshConfig {
	conf := config.NewDefaultDataMeshConfig()

	assert.NotNil(t, conf)
	conf.KusciaClient = kusciafake.NewSimpleClientset()
	conf.KubeNamespace = "test-namespace"
	if domaindataSpec != nil && domaindataSpec.Author != "" {
		conf.KubeNamespace = domaindataSpec.Author
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	conf.DomainKey = privateKey

	return conf
}

func registerPostgresqlDomainDataSource(t *testing.T, conf *config.DataMeshConfig, dbInfo *datamesh.DatabaseDataSourceInfo, dsID string) {
	lfs, err := json.Marshal(&datamesh.DataSourceInfo{
		Database: &datamesh.DatabaseDataSourceInfo{
			Endpoint: dbInfo.Endpoint,
			User:     dbInfo.User,
			Password: dbInfo.Password,
			Database: dbInfo.Database,
		}})
	assert.NoError(t, err)

	strConfig, err := tls.EncryptOAEP(&conf.DomainKey.PublicKey, lfs)
	assert.NoError(t, err)

	_, err = conf.KusciaClient.KusciaV1alpha1().DomainDataSources(conf.KubeNamespace).Create(context.Background(), &v1alpha1.DomainDataSource{
		ObjectMeta: v1.ObjectMeta{
			Name: dsID,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			Name: dsID,
			Type: "postgresql",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)
}

// TEST ONLY! if domaindataSpec is nil, generate a basic spec
func initPostgresqlIOTestRequestContext(t *testing.T, tableName string, dbInfo *datamesh.DatabaseDataSourceInfo, domaindataSpec *v1alpha1.DomainDataSpec, isQuery bool) (*config.DataMeshConfig, *utils.DataMeshRequestContext, *sql.DB, sqlmock.Sqlmock) {
	conf := initPostgresqlContextTestEnv(t, domaindataSpec)
	dsID := "postgresql-data-source"
	domainDataID := "data-" + uuid.New().String()

	if domaindataSpec == nil {
		domaindataSpec = &v1alpha1.DomainDataSpec{
			RelativeURI: tableName,
			Name:        domainDataID,
			Type:        "TABLE",
			DataSource:  dsID,
			Author:      conf.KubeNamespace,
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
	} else {
		dsID = domaindataSpec.DataSource
		domainDataID = domaindataSpec.Name
	}

	assert.NotNil(t, conf)
	domainDataService := service.NewDomainDataService(conf)
	dataSourceService := service.NewDomainDataSourceService(conf, nil)

	assert.NotNil(t, domainDataService)
	assert.NotNil(t, dataSourceService)

	if dbInfo == nil {
		dbInfo = &datamesh.DatabaseDataSourceInfo{
			Endpoint: "127.0.0.1",
			User:     "user",
			Password: "password",
			Database: uuid.NewString(),
		}
	}
	registerPostgresqlDomainDataSource(t, conf, dbInfo, dsID)
	registerDomainData(t, conf, domaindataSpec)

	var msg protoreflect.ProtoMessage
	if isQuery {
		msg = &datamesh.CommandDomainDataQuery{
			DomaindataId: domainDataID,
		}
	} else {
		msg = &datamesh.CommandDomainDataUpdate{
			DomaindataId: domainDataID,
		}
	}
	ctx, err := utils.NewDataMeshRequestContext(domainDataService, dataSourceService, msg, common.DomainDataSourceTypePostgreSQL)
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	db, mock, err := sqlmock.NewWithDSN(dbInfoToDsnPg(dbInfo))
	assert.NoError(t, err)
	return conf, ctx, db, mock
}

func TestPostgresqlIOChannel_New(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewBuiltinPostgresqlIOChannel())
}

func TestPostgresqlIOChannel_Read_Success(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinPostgresqlIOChannel()
	assert.NotNil(t, channel)
	channel.(*BuiltinPostgresqlIO).driverName = "sqlmock"

	tableName := "testTable"
	ctx, _, mock, downloader, rc, err := initPostgresqlDownloader(t, tableName, nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	rows := mock.NewRows([]string{"name", "id"})
	rows.AddRow("alice", 1)
	rows.AddRow("bob", 2)

	mock.ExpectQuery("SELECT \"name\", \"id\" FROM \"" + tableName + "\"").WillReturnRows(rows)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := GenerateArrowSchemaExclude(dd, nil)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.NoError(t, channel.Read(ctx, rc, writer))
}

func TestPostgresqlIOChannel_Write_Success(t *testing.T) {
	t.Parallel()

	channel := NewBuiltinPostgresqlIOChannel()
	assert.NotNil(t, channel)
	channel.(*BuiltinPostgresqlIO).driverName = "sqlmock"

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

	ctx, _, mock, uploader, rc, err := initPostgresqlUploader(t, "\"output\"", domaindataSpec)
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
	dropSQL := regexp.QuoteMeta("DROP TABLE IF EXISTS \"output\"")
	expectSQL := regexp.QuoteMeta("CREATE TABLE \"output\" (\"name\" TEXT, \"id\" BIGINT)")
	mock.ExpectExec(dropSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(expectSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO \"output\"")
	prepare.ExpectExec().WithArgs("alice", "1").WillReturnResult(sqlmock.NewResult(1, 1))
	prepare.ExpectExec().WithArgs("bob", "2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	assert.NoError(t, channel.Write(ctx, rc, reader))
}

func TestPostgresqlIOChannel_Endpoint(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinPostgresqlIOChannel()
	assert.Equal(t, utils.BuiltinFlightServerEndpointURI, channel.GetEndpointURI())
}
