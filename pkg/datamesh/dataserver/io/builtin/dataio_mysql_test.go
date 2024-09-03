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
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/google/uuid"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dbInfoToDsn(info *datamesh.DatabaseDataSourceInfo) string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s",
		info.User,
		info.Password,
		info.Endpoint,
		info.Database,
	)
}

// GenerateArrowSchema generate schema from domaindata
func GenerateArrowSchemaExclude(domainData *datamesh.DomainData, exclude map[string]struct{}) (*arrow.Schema, error) {
	// get schema from Query
	fields := make([]arrow.Field, 0)
	for i, column := range domainData.Columns {
		if _, ok := exclude[column.Name]; ok {
			continue
		}

		colType := common.Convert2ArrowColumnType(column.Type)
		if colType == nil {
			return nil, status.Errorf(codes.Internal, "invalid column(%s) with type(%s)", column.Name, column.Type)
		}
		fields = append(fields, arrow.Field{
			Name:     column.Name,
			Type:     colType,
			Nullable: !column.NotNullable,
		})
		nlog.Debugf("Columns[%d:%s][src:%s,dst:%s].Nullable is :%v", i, column.Name, column.Type, colType.Name(), !column.NotNullable)
	}
	return arrow.NewSchema(fields, nil), nil
}

func initMySQLContextTestEnv(t *testing.T, domaindataSpec *v1alpha1.DomainDataSpec) *config.DataMeshConfig {
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

func registerMySQLDomainDataSource(t *testing.T, conf *config.DataMeshConfig, dbInfo *datamesh.DatabaseDataSourceInfo, dsID string) {
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
			Type: "mysql",
			Data: map[string]string{
				"encryptedInfo": strConfig,
			},
		},
	}, v1.CreateOptions{})

	assert.NoError(t, err)
}

func registerDomainData(t *testing.T, conf *config.DataMeshConfig, domaindataSpec *v1alpha1.DomainDataSpec) *v1alpha1.DomainData {
	domainDataID := domaindataSpec.Name
	dd, err := conf.KusciaClient.KusciaV1alpha1().DomainDatas(conf.KubeNamespace).Create(context.Background(), &v1alpha1.DomainData{
		ObjectMeta: v1.ObjectMeta{
			Name: domainDataID,
		},
		Spec: *domaindataSpec,
	}, v1.CreateOptions{})
	assert.NoError(t, err)

	return dd
}

// TEST ONLY! if domaindataSpec is nil, generate a basic spec
func initMySQLIOTestRequestContext(t *testing.T, tableName string, dbInfo *datamesh.DatabaseDataSourceInfo, domaindataSpec *v1alpha1.DomainDataSpec, isQuery bool) (*config.DataMeshConfig, *utils.DataMeshRequestContext, *sql.DB, sqlmock.Sqlmock) {
	conf := initMySQLContextTestEnv(t, domaindataSpec)
	dsID := "mysql-data-source"
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
	registerMySQLDomainDataSource(t, conf, dbInfo, dsID)
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
	ctx, err := utils.NewDataMeshRequestContext(domainDataService, dataSourceService, msg, common.DomainDataSourceTypeMysql)
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	db, mock, err := sqlmock.NewWithDSN(dbInfoToDsn(dbInfo))
	assert.NoError(t, err)
	return conf, ctx, db, mock
}

func getTableFlightData(t *testing.T, ctx *utils.DataMeshRequestContext, colType []arrow.DataType, dataRows [][]any) []*flight.FlightData {
	// use a writer to store input data
	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}
	dd, _, err := ctx.GetDomainDataAndSource(context.Background())
	assert.NoError(t, err)
	schema, err := utils.GenerateArrowSchema(dd)
	assert.NoError(t, err)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))

	recordBuilder := make([]array.Builder, len(colType))
	builderFunc := make([]func(array.Builder, any), len(colType))
	for idx := range recordBuilder {
		switch colType[idx].(type) {
		case *arrow.StringType:
			recordBuilder[idx] = array.NewStringBuilder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.StringBuilder).Append(val.(string))
			}
		case *arrow.BooleanType:
			recordBuilder[idx] = array.NewBooleanBuilder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.BooleanBuilder).Append(val.(bool))
			}
		case *arrow.Float32Type:
			recordBuilder[idx] = array.NewFloat32Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Float32Builder).Append(val.(float32))
			}
		case *arrow.Float64Type:
			recordBuilder[idx] = array.NewFloat64Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Float64Builder).Append(val.(float64))
			}
		case *arrow.Int8Type:
			recordBuilder[idx] = array.NewInt8Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Int8Builder).Append(val.(int8))
			}
		case *arrow.Int16Type:
			recordBuilder[idx] = array.NewInt16Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Int16Builder).Append(val.(int16))
			}
		case *arrow.Int32Type:
			recordBuilder[idx] = array.NewInt32Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Int32Builder).Append(val.(int32))
			}
		case *arrow.Int64Type:
			recordBuilder[idx] = array.NewInt64Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Int64Builder).Append(val.(int64))
			}
		case *arrow.Uint8Type:
			recordBuilder[idx] = array.NewUint8Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Uint8Builder).Append(val.(uint8))
			}
		case *arrow.Uint16Type:
			recordBuilder[idx] = array.NewUint16Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Uint16Builder).Append(val.(uint16))
			}
		case *arrow.Uint32Type:
			recordBuilder[idx] = array.NewUint32Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Uint32Builder).Append(val.(uint32))
			}
		case *arrow.Uint64Type:
			recordBuilder[idx] = array.NewUint64Builder(memory.DefaultAllocator)
			builderFunc[idx] = func(bld array.Builder, val any) {
				bld.(*array.Uint64Builder).Append(val.(uint64))
			}
		default:
			panic("invalid unit test data type")
		}

	}

	for _, row := range dataRows {
		for idx, val := range row {
			builderFunc[idx](recordBuilder[idx], val)
		}
	}

	// prepare record
	recordData := make([]arrow.Array, len(colType))
	for idx, builder := range recordBuilder {
		recordData[idx] = builder.NewArray()
	}

	// prepare dataList
	assert.NoError(t, writer.Write(array.NewRecord(schema, recordData, int64(len(dataRows)))))

	writer.Close()

	return mgs.dataList
}

func TestMySQLIOChannel_New(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewBuiltinMySQLIOChannel())
}

func TestMySQLIOChannel_Read_Success(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinMySQLIOChannel()
	assert.NotNil(t, channel)
	channel.(*BuiltinMySQLIO).driverName = "sqlmock"

	tableName := "testTable"
	ctx, _, mock, downloader, rc, err := initMySQLDownloader(t, tableName, nil, nil)
	assert.NotNil(t, downloader)
	assert.NoError(t, err)

	mgs := &mockDoGetServer{
		ServerStream: &mockGrpcServerStream{},
	}

	rows := mock.NewRows([]string{"name", "id"})
	rows.AddRow("alice", 1)
	rows.AddRow("bob", 2)

	mock.ExpectQuery("SELECT `name`, `id` FROM `" + tableName + "`").WillReturnRows(rows)

	dd, _, err := rc.GetDomainDataAndSource(ctx)
	assert.NoError(t, err)
	schema, _ := GenerateArrowSchemaExclude(dd, nil)
	writer := flight.NewRecordWriter(mgs, ipc.WithSchema(schema))
	assert.NotNil(t, writer)
	assert.NoError(t, channel.Read(ctx, rc, writer))
}

func TestMySQLIOChannel_Write_Success(t *testing.T) {
	t.Parallel()

	channel := NewBuiltinMySQLIOChannel()
	assert.NotNil(t, channel)
	channel.(*BuiltinMySQLIO).driverName = "sqlmock"

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

	ctx, _, mock, uploader, rc, err := initMySQLUploader(t, "`output`", domaindataSpec)
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

	expectSQL := regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS `output` (`name` TEXT, `id` BIGINT SIGNED)")
	mock.ExpectExec(expectSQL).WithoutArgs().WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectBegin()
	prepare := mock.ExpectPrepare("INSERT INTO `output`")
	prepare.ExpectExec().WithArgs("alice", "1").WillReturnResult(sqlmock.NewResult(1, 1))
	prepare.ExpectExec().WithArgs("bob", "2").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	assert.NoError(t, channel.Write(ctx, rc, reader))
}

func TestMySQLIOChannel_Endpoint(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinMySQLIOChannel()
	assert.Equal(t, utils.BuiltinFlightServerEndpointURI, channel.GetEndpointURI())
}
