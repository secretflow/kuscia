// Copyright 2023 Ant Group Co., Ltd.
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

//nolint:dupl
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/google/uuid"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

var (
	dsID          = common.DefaultDataSourceID
	domainID      = "domain-data-unit-test-namespace"
	mockInitiator = domainID
)

func makeDomainDataServiceConfig(t *testing.T) *config.KusciaAPIConfig {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &config.KusciaAPIConfig{
		DomainKey:    privateKey,
		KusciaClient: kusciafake.NewSimpleClientset(MakeMockDomain(domainID)),
		RunMode:      common.RunModeLite,
		Initiator:    mockInitiator,
		DomainID:     domainID,
	}
}

func mockCreateDomainDataSourceLocalFS(t *testing.T, conf *config.KusciaAPIConfig) {
	dataSourceService := makeDomainDataSourceService(t, conf)

	domainDataSource := dataSourceService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		DomainId:     domainID,
		DatasourceId: dsID,
		Type:         common.DomainDataSourceTypeLocalFS,
		Info: &kusciaapi.DataSourceInfo{
			Localfs: &kusciaapi.LocalDataSourceInfo{
				Path: "var/storage/data",
			},
		},
	})

	assert.NotNil(t, domainDataSource)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainDataSource.Status.Code)
}

func mockCreateDomainDataSourceOSS(t *testing.T, conf *config.KusciaAPIConfig, ts *httptest.Server) {
	dataSourceService := makeDomainDataSourceService(t, conf)

	domainDataSource := dataSourceService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		DomainId:     domainID,
		DatasourceId: dsID,
		Type:         common.DomainDataSourceTypeOSS,
		Info: &kusciaapi.DataSourceInfo{
			Oss: &kusciaapi.OssDataSourceInfo{
				Endpoint:        ts.URL,
				Bucket:          "test-bucket",
				Prefix:          "test-prefix",
				StorageType:     "minio",
				AccessKeyId:     "test-access-key",
				AccessKeySecret: "test-secret-key",
			},
		},
	})

	assert.NotNil(t, domainDataSource)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainDataSource.Status.Code)
}

func dbInfoToDsn(info *kusciaapi.DatabaseDataSourceInfo) string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s",
		info.User,
		info.Password,
		info.Endpoint,
		info.Database,
	)
}

func mockCreateDomainDataSourceMySQL(t *testing.T, conf *config.KusciaAPIConfig, dbInfo *kusciaapi.DatabaseDataSourceInfo) {
	dataSourceService := makeDomainDataSourceService(t, conf)

	domainDataSource := dataSourceService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		DomainId:     domainID,
		DatasourceId: dsID,
		Type:         common.DomainDataSourceTypeMysql,
		Info: &kusciaapi.DataSourceInfo{
			Database: dbInfo,
		},
	})

	assert.NotNil(t, domainDataSource)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainDataSource.Status.Code)
}

func mockCreateDomainDataSourcePostgresql(t *testing.T, conf *config.KusciaAPIConfig, dbInfo *kusciaapi.DatabaseDataSourceInfo) {
	dataSourceService := makeDomainDataSourceService(t, conf)

	domainDataSource := dataSourceService.CreateDomainDataSource(context.Background(), &kusciaapi.CreateDomainDataSourceRequest{
		DomainId:     domainID,
		DatasourceId: dsID,
		Type:         common.DomainDataSourceTypePostgreSQL,
		Info: &kusciaapi.DataSourceInfo{
			Database: dbInfo,
		},
	})

	assert.NotNil(t, domainDataSource)
	assert.Equal(t, kusciaAPISuccessStatusCode, domainDataSource.Status.Code)
}

func mockCreateDomainData(domainDataService IDomainDataService) *kusciaapi.CreateDomainDataResponse {
	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}

	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})

	return res
}

func TestCreateDomainData(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))

	mockCreateDomainDataSourceLocalFS(t, conf)

	res := mockCreateDomainData(domainDataService)

	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	res = domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/d.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)
}

func TestCreateDomainDataWithoutDatasource(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	res := mockCreateDomainData(domainDataService)

	assert.Equal(t, int32(errorcode.ErrorCode_KusciaAPIErrDomainDataSourceNotExists), res.Status.Code)
}

func TestCreateDomainDataWithVendor(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}

	mustomVendor := "mustom-vendor"

	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
		Vendor:  mustomVendor,
	})
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	res1 := domainDataService.QueryDomainData(context.Background(), &kusciaapi.QueryDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.QueryDomainDataRequestData{
			DomainId:     domainID,
			DomaindataId: res.Data.DomaindataId,
		},
	})
	assert.NotNil(t, res1)
	assert.Equal(t, mustomVendor, res1.Data.Vendor)
}

func TestQueryDomainData(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	res := mockCreateDomainData(domainDataService)

	res1 := domainDataService.QueryDomainData(context.Background(), &kusciaapi.QueryDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.QueryDomainDataRequestData{
			DomainId:     domainID,
			DomaindataId: res.Data.DomaindataId,
		},
	})
	assert.NotNil(t, res1)
	assert.Equal(t, common.DefaultDomainDataVendor, res1.Data.Vendor)
}

func TestUpdateDomainData(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	res := mockCreateDomainData(domainDataService)

	res1 := domainDataService.UpdateDomainData(context.Background(), &kusciaapi.UpdateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)
}

func TestUpdateDomainDataWithVendor(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	res := mockCreateDomainData(domainDataService)

	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	mustomVendor := "mustom-vendor"
	res1 := domainDataService.UpdateDomainData(context.Background(), &kusciaapi.UpdateDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b.csv",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
		Vendor:       mustomVendor,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)

	res2 := domainDataService.QueryDomainData(context.Background(), &kusciaapi.QueryDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.QueryDomainDataRequestData{
			DomainId:     domainID,
			DomaindataId: res.Data.DomaindataId,
		},
	})
	assert.NotNil(t, res1)
	assert.Equal(t, mustomVendor, res2.Data.Vendor)

}

func TestDeleteDomainData(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	res := mockCreateDomainData(domainDataService)

	res1 := domainDataService.DeleteDomainData(context.Background(), &kusciaapi.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)
}

func TestDeleteDomainDataAndRaw(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	// Create a mock domain data
	res := mockCreateDomainData(domainDataService)
	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	// Call DeleteDomainDataAndRaw
	res1 := domainDataService.DeleteDomainDataAndRaw(context.Background(), &kusciaapi.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)
}

func TestDeleteDomainDataAndRawWithOSS(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))

	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())
	defer ts.Close()
	mockCreateDomainDataSourceOSS(t, conf, ts)

	ds, err := conf.KusciaClient.KusciaV1alpha1().DomainDataSources(domainID).Get(context.Background(), dsID, v1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, ds)

	ossConfig := &kusciaapi.DataSourceInfo{
		Oss: &kusciaapi.OssDataSourceInfo{
			Endpoint:        ts.URL,
			Bucket:          "test-bucket",
			Prefix:          "test-prefix",
			StorageType:     "minio",
			AccessKeyId:     "test-access-key",
			AccessKeySecret: "test-secret-key",
		},
	}
	ossConfigBytes, err := json.Marshal(ossConfig)
	assert.NoError(t, err)

	encryptedConfig, err := tls.EncryptOAEP(&conf.DomainKey.PublicKey, ossConfigBytes)
	assert.NoError(t, err)

	ds.Spec.Data = map[string]string{
		"encryptedInfo": encryptedConfig,
	}
	_, err = conf.KusciaClient.KusciaV1alpha1().DomainDataSources(domainID).Update(context.Background(), ds, v1.UpdateOptions{})
	assert.NoError(t, err)

	err = backend.CreateBucket("test-bucket")
	assert.NoError(t, err)
	filePath := "test-prefix/test-file.txt"
	_, err = backend.PutObject("test-bucket", filePath, map[string]string{}, bytes.NewReader([]byte("test content")), int64(len("test content")))
	assert.NoError(t, err)

	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test-data",
		Type:         "file",
		RelativeUri:  "test-file.txt",
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	res1 := domainDataService.DeleteDomainDataAndRaw(context.Background(), &kusciaapi.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)

	_, err = backend.HeadObject("test-bucket", filePath)
	assert.Error(t, err)
}

func TestDeleteDomainDataAndRawWithMysql(t *testing.T) {
	// Step 1: Initialize configuration and service
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))

	dbInfo := &kusciaapi.DatabaseDataSourceInfo{
		Endpoint: "127.0.0.1",
		User:     "user",
		Password: "password",
		Database: uuid.NewString(),
	}
	// Step 2: Use sqlmock to simulate MySQL database
	db, mock, err := sqlmock.NewWithDSN(dbInfoToDsn(dbInfo))
	assert.NoError(t, err)
	defer db.Close()

	patches := gomonkey.ApplyFunc(sql.Open, func(driverName, dataSourceName string) (*sql.DB, error) {
		if driverName == "mysql" {
			return db, nil
		}
		return nil, fmt.Errorf("unexpected driverName: %s", driverName)
	})
	defer patches.Reset()
	// Step 3: Initialize MySQL data source
	mockCreateDomainDataSourceMySQL(t, conf, dbInfo)

	// Step 4: Simulate MySQL table and data
	tableName := "test_table"
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName))).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 5: Create domain data
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test-data",
		Type:         "table",
		RelativeUri:  tableName,
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	// Step 6: Call DeleteDomainDataAndRaw to delete domain data and MySQL table
	res1 := domainDataService.DeleteDomainDataAndRaw(context.Background(), &kusciaapi.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)

	// Step 7: Verify MySQL table was deleted
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteDomainDataAndRawWithPostgres(t *testing.T) {
	// Step 1: Initialize configuration and service
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))

	dbInfo := &kusciaapi.DatabaseDataSourceInfo{
		Endpoint: "127.0.0.1",
		User:     "user",
		Password: "password",
		Database: uuid.NewString(),
	}
	// Step 2: Use sqlmock to simulate PostgreSQL database
	db, mock, err := sqlmock.NewWithDSN(dbInfoToDsn(dbInfo))
	assert.NoError(t, err)
	defer db.Close()

	patches := gomonkey.ApplyFunc(sql.Open, func(driverName, dataSourceName string) (*sql.DB, error) {
		if driverName == "postgres" {
			return db, nil
		}
		return nil, fmt.Errorf("unexpected driverName: %s", driverName)
	})
	defer patches.Reset()
	// Step 3: Initialize PostgreSQL data source
	mockCreateDomainDataSourcePostgresql(t, conf, dbInfo)

	// Step 4: Simulate PostgreSQL table and data
	tableName := "test_table"
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("DROP TABLE IF EXISTS \"%s\"", tableName))).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 5: Create domain data
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test-data",
		Type:         "table",
		RelativeUri:  tableName,
		DatasourceId: dsID,
		Attributes:   nil,
		Partition:    nil,
		Columns:      nil,
	})
	assert.NotNil(t, res)
	assert.Equal(t, kusciaAPISuccessStatusCode, res.Status.Code)

	// Step 6: Call DeleteDomainDataAndRaw to delete domain data and PostgreSQL table
	res1 := domainDataService.DeleteDomainDataAndRaw(context.Background(), &kusciaapi.DeleteDomainDataRequest{
		Header:       nil,
		DomaindataId: res.Data.DomaindataId,
		DomainId:     domainID,
	})
	assert.NotNil(t, res1)
	assert.Equal(t, kusciaAPISuccessStatusCode, res1.Status.Code)

	// Step 7: Verify PostgreSQL table was deleted
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchQueryDomainData(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	res := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	res1 := domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns: col,
	})
	datas := make([]*kusciaapi.QueryDomainDataRequestData, 2)
	datas[0] = &kusciaapi.QueryDomainDataRequestData{
		DomainId:     domainID,
		DomaindataId: res.Data.DomaindataId,
	}
	datas[1] = &kusciaapi.QueryDomainDataRequestData{
		DomainId:     domainID,
		DomaindataId: res1.Data.DomaindataId,
	}
	res2 := domainDataService.BatchQueryDomainData(context.Background(), &kusciaapi.BatchQueryDomainDataRequest{
		Header: nil,
		Data:   datas})
	assert.NotNil(t, res2)
}

func TestListDomainData(t *testing.T) {
	conf := makeDomainDataServiceConfig(t)
	domainDataService := NewDomainDataService(conf, makeConfigService(t))
	mockCreateDomainDataSourceLocalFS(t, conf)

	attr := make(map[string]string)
	attr["rows"] = "100"
	col := make([]*v1alpha1.DataColumn, 2)
	col[0] = &v1alpha1.DataColumn{Name: "id", Type: "string"}
	col[1] = &v1alpha1.DataColumn{Name: "date", Type: "string"}
	_ = domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns:    col,
		FileFormat: v1alpha1.FileFormat_CSV,
	})
	_ = domainDataService.CreateDomainData(context.Background(), &kusciaapi.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: "",
		DomainId:     domainID,
		Name:         "test",
		Type:         "table",
		RelativeUri:  "a/b/c.csv",
		DatasourceId: dsID,
		Attributes:   attr,
		Partition: &v1alpha1.Partition{
			Type:   "path",
			Fields: col[1:],
		},
		Columns:    col,
		FileFormat: v1alpha1.FileFormat_CSV,
	})
	res2 := domainDataService.ListDomainData(context.Background(), &kusciaapi.ListDomainDataRequest{
		Header: nil,
		Data: &kusciaapi.ListDomainDataRequestData{
			DomainId:         domainID,
			DomaindataType:   "table",
			DomaindataVendor: "manual",
		},
	},
	)
	assert.NotNil(t, res2)
}

func TestDeletePostgresqlTableSuccessWithStandardConnection(t *testing.T) {
	// Test scenario: Successfully delete PostgreSQL table with standard connection
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	patches := gomonkey.ApplyFunc(sql.Open, func(driverName, dataSourceName string) (*sql.DB, error) {
		if driverName == "postgres" {
			return db, nil
		}
		return nil, fmt.Errorf("unexpected driverName: %s", driverName)
	})
	defer patches.Reset()

	tableName := "test_table"
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("DROP TABLE IF EXISTS \"%s\"", tableName))).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = deletePostgresqlTable("user", "password", "127.0.0.1:5432", "testdb", tableName)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeletePostgresqlTableSuccessWithConnectionParams(t *testing.T) {
	// Test scenario: Successfully delete PostgreSQL table with connection parameters
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	patches := gomonkey.ApplyFunc(sql.Open, func(driverName, dataSourceName string) (*sql.DB, error) {
		if driverName == "postgres" {
			// Verify DSN correctly handles parameters
			assert.Contains(t, dataSourceName, "sslmode=disable")
			assert.Contains(t, dataSourceName, "connect_timeout=10")
			return db, nil
		}
		return nil, fmt.Errorf("unexpected driverName: %s", driverName)
	})
	defer patches.Reset()

	tableName := "test_table_with_params"
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("DROP TABLE IF EXISTS \"%s\"", tableName))).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = deletePostgresqlTable("user", "password", "127.0.0.2:5432?sslmode=disable&connect_timeout=10", "testdb", tableName)
	assert.NoError(t, err)
}

func TestDeletePostgresqlTableSuccessWithoutPortInEndpoint(t *testing.T) {
	// Test scenario: Successfully delete PostgreSQL table when endpoint doesn't contain port
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	patches := gomonkey.ApplyFunc(sql.Open, func(driverName, dataSourceName string) (*sql.DB, error) {
		if driverName == "postgres" {
			// Verify DSN uses default port 5432
			assert.Contains(t, dataSourceName, "port=5432")
			return db, nil
		}
		return nil, fmt.Errorf("unexpected driverName: %s", driverName)
	})
	defer patches.Reset()

	tableName := "test_table_no_port"
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("DROP TABLE IF EXISTS \"%s\"", tableName))).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = deletePostgresqlTable("user", "password", "127.0.0.3", "testdb", tableName)
	assert.NoError(t, err)
}

func TestDeletePostgresqlTableErrorWhenDatabaseConnectionFails(t *testing.T) {
	// Test scenario: Database connection failure should return error
	patches := gomonkey.ApplyFunc(sql.Open, func(driverName, dataSourceName string) (*sql.DB, error) {
		if driverName == "postgres" {
			return nil, fmt.Errorf("connection refused")
		}
		return nil, fmt.Errorf("unexpected driverName: %s", driverName)
	})
	defer patches.Reset()

	err := deletePostgresqlTable("user", "password", "127.0.0.4:5432", "testdb", "test_table")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to database")
}

func TestDeleteLocalFsFile(t *testing.T) {
	// Setup test directory structure
	basePath := "var/storage/data"
	testSubDir := "testdata"

	// Convert to absolute path for file operations
	absBasePath, err := filepath.Abs(basePath)
	assert.NoError(t, err)
	absTestDir := filepath.Join(absBasePath, testSubDir)

	// Ensure test directory exists
	err = os.MkdirAll(absTestDir, 0755)
	assert.NoError(t, err)
	defer os.RemoveAll(absBasePath)

	t.Run("success when file exists", func(t *testing.T) {
		tempFile, err := os.CreateTemp(absTestDir, "testfile")
		assert.NoError(t, err)
		tempFileName := tempFile.Name()
		tempFile.Close()

		// Get relative path from basePath
		relPath, err := filepath.Rel(absBasePath, tempFileName)
		assert.NoError(t, err)

		err = deleteLocalFsFile(basePath, relPath)
		assert.NoError(t, err)

		// Verify file deleted
		_, err = os.Stat(tempFileName)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("success when deleting subdirectory file", func(t *testing.T) {
		subDir := filepath.Join(absTestDir, "subdir")
		err := os.MkdirAll(subDir, 0755)
		assert.NoError(t, err)

		tempFile, err := os.CreateTemp(subDir, "testfile")
		assert.NoError(t, err)
		tempFileName := tempFile.Name()
		tempFile.Close()

		relPath, err := filepath.Rel(absBasePath, tempFileName)
		assert.NoError(t, err)

		err = deleteLocalFsFile(basePath, relPath)
		assert.NoError(t, err)

		_, err = os.Stat(tempFileName)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("success when file not exists", func(t *testing.T) {
		err := deleteLocalFsFile(basePath, "nonexistent_file")
		assert.NoError(t, err)
	})

	t.Run("error when base path is invalid", func(t *testing.T) {
		invalidBasePath := "/invalid/path"
		err := deleteLocalFsFile(invalidBasePath, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid base path")
	})

	t.Run("error when relative path escapes base", func(t *testing.T) {
		tempFile, err := os.CreateTemp(absTestDir, "testfile")
		assert.NoError(t, err)
		tempFileName := tempFile.Name()
		tempFile.Close()

		// Create malicious relative path
		relPath := filepath.Join("../", filepath.Base(tempFileName))

		err = deleteLocalFsFile(basePath, relPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid relative URI")
	})

	t.Run("error when stat fails", func(t *testing.T) {
		patches := gomonkey.ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
			return nil, fmt.Errorf("mock stat error")
		})
		defer patches.Reset()

		err := deleteLocalFsFile(basePath, "testfile")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check file")
	})

	t.Run("error when remove fails", func(t *testing.T) {
		tempFile, err := os.CreateTemp(absTestDir, "testfile")
		assert.NoError(t, err)
		tempFileName := tempFile.Name()
		tempFile.Close()

		relPath, err := filepath.Rel(absBasePath, tempFileName)
		assert.NoError(t, err)

		patches := gomonkey.ApplyFunc(os.Remove, func(name string) error {
			return fmt.Errorf("mock remove error")
		})
		defer patches.Reset()

		err = deleteLocalFsFile(basePath, relPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete file")
	})
}
