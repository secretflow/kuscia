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
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	consts "github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	pbv1alpha1 "github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type IDomainDataService interface {
	CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) *kusciaapi.CreateDomainDataResponse
	UpdateDomainData(ctx context.Context, request *kusciaapi.UpdateDomainDataRequest) *kusciaapi.UpdateDomainDataResponse
	DeleteDomainData(ctx context.Context, request *kusciaapi.DeleteDomainDataRequest) *kusciaapi.DeleteDomainDataResponse
	DeleteDomainDataAndSource(ctx context.Context, request *kusciaapi.DeleteDomainDataAndSourceRequest) *kusciaapi.DeleteDomainDataAndSourceResponse
	QueryDomainData(ctx context.Context, request *kusciaapi.QueryDomainDataRequest) *kusciaapi.QueryDomainDataResponse
	BatchQueryDomainData(ctx context.Context, request *kusciaapi.BatchQueryDomainDataRequest) *kusciaapi.BatchQueryDomainDataResponse
	ListDomainData(ctx context.Context, request *kusciaapi.ListDomainDataRequest) *kusciaapi.ListDomainDataResponse
}

type domainDataService struct {
	conf          *config.KusciaAPIConfig
	configService cmservice.IConfigService
}

func NewDomainDataService(config *config.KusciaAPIConfig, configService cmservice.IConfigService) IDomainDataService {
	return &domainDataService{
		conf:          config,
		configService: configService,
	}
}

func (s domainDataService) CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) *kusciaapi.CreateDomainDataResponse {
	// validate domainID
	if request.DomainId == "" {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "request domain id can not be empty"),
		}
	}
	if err := resources.ValidateK8sName(request.DomainId, "domain_id"); err != nil {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}
	// validate data type
	if request.Type == "" {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "request type can not be empty"),
		}
	}
	// validate relative_uri
	if request.RelativeUri == "" {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "request relative_uri can not be empty"),
		}
	}
	// validate lite request
	if err := s.validateRequestWhenLite(request); err != nil {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	// check whether domainData is existed
	if request.DomaindataId != "" {
		// do k8s validate
		if err := resources.ValidateK8sName(request.DomaindataId, "domaindata_id"); err != nil {
			return &kusciaapi.CreateDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
			}
		}
		domainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
		if err == nil && domainData != nil {
			// update domainData
			resp := s.UpdateDomainData(ctx, convert2UpdateReq(request))
			return convert2CreateResp(resp, request.DomaindataId)
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrAuthFailed, err.Error()),
		}
	}
	// normalization request
	s.normalizationCreateRequest(request)

	if len(request.DatasourceId) > 0 {
		kusciaErrorCode, msg := CheckDomainDataSourceExists(s.conf.KusciaClient, request.DomainId, request.DatasourceId)

		if pberrorcode.ErrorCode_SUCCESS != kusciaErrorCode {
			return &kusciaapi.CreateDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(kusciaErrorCode, msg),
			}
		}
	}

	// verdor priority using incoming
	customVendor := request.Vendor

	// build kuscia domain
	Labels := map[string]string{
		common.LabelDomainDataType:        request.Type,
		common.LabelDomainDataVendor:      customVendor,
		common.LabelInterConnProtocolType: "kuscia",
	}

	annotations := map[string]string{
		common.InitiatorAnnotationKey: request.DomainId,
	}

	kusciaDomainData := &v1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.DomaindataId,
			Labels:      Labels,
			Annotations: annotations,
		},
		Spec: v1alpha1.DomainDataSpec{
			RelativeURI: request.RelativeUri,
			Name:        request.Name,
			Type:        request.Type,
			DataSource:  request.DatasourceId,
			Attributes:  request.Attributes,
			Partition:   common.Convert2KubePartition(request.Partition),
			Columns:     common.Convert2KubeColumn(request.Columns),
			Vendor:      customVendor,
			Author:      request.DomainId,
			FileFormat:  common.Convert2KubeFileFormat(request.FileFormat),
		},
	}
	// create kuscia domain
	_, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Create(ctx, kusciaDomainData, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainData failed, error: %s", err.Error())
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.CreateDomainDataErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataFailed), err.Error()),
		}
	}
	return &kusciaapi.CreateDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.CreateDomainDataResponseData{
			DomaindataId: request.DomaindataId,
		},
	}
}

func (s domainDataService) UpdateDomainData(ctx context.Context, request *kusciaapi.UpdateDomainDataRequest) *kusciaapi.UpdateDomainDataResponse {
	// do validate
	if request.DomaindataId == "" || request.DomainId == "" {
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain id and domaindata id can not be empty"),
		}
	}

	if err := s.validateRequestWhenLite(request); err != nil {
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrAuthFailed, err.Error()),
		}
	}
	// get original domainData from k8s
	originalDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("UpdateDomainData failed, error: %s", err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrGetDomainDataFailed), err.Error()),
		}
	}

	s.normalizationUpdateRequest(request, originalDomainData.Spec)
	if len(request.DatasourceId) > 0 {
		kusciaErrorCode, msg := CheckDomainDataSourceExists(s.conf.KusciaClient, request.DomainId, request.DatasourceId)

		if pberrorcode.ErrorCode_SUCCESS != kusciaErrorCode {
			return &kusciaapi.UpdateDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(kusciaErrorCode, msg),
			}
		}
	}

	// build modified domainData
	labels := make(map[string]string)
	for key, value := range originalDomainData.Labels {
		labels[key] = value
	}

	// verdor priority using incoming
	customVendor := request.Vendor

	labels[common.LabelDomainDataType] = request.Type
	labels[common.LabelDomainDataVendor] = customVendor
	modifiedDomainData := &v1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:            request.DomaindataId,
			Labels:          labels,
			ResourceVersion: originalDomainData.ResourceVersion,
		},
		Spec: v1alpha1.DomainDataSpec{
			RelativeURI: request.RelativeUri,
			Name:        request.Name,
			Type:        request.Type,
			DataSource:  request.DatasourceId,
			Attributes:  request.Attributes,
			Partition:   common.Convert2KubePartition(request.Partition),
			Columns:     common.Convert2KubeColumn(request.Columns),
			Vendor:      customVendor,
			Author:      request.DomainId,
			FileFormat:  common.Convert2KubeFileFormat(request.FileFormat),
		},
	}
	// merge modifiedDomainData to originalDomainData
	patchBytes, originalBytes, modifiedBytes, err := service.MergeDomainData(originalDomainData, modifiedDomainData)
	if err != nil {
		nlog.Errorf("Merge DomainData failed, request: %+v,error: %s.",
			request, err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrMergeDomainDataFailed, err.Error()),
		}
	}
	nlog.Debugf("Update DomainData request: %+v, patchBytes: %s, originalDomainData: %s, modifiedDomainData: %s.",
		request, patchBytes, originalBytes, modifiedBytes)
	// patch the merged domainData
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(originalDomainData.Namespace).Patch(ctx, originalDomainData.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		nlog.Debugf("Patch DomainData failed, request: %+v, patchBytes: %s, originalDomainData: %s, modifiedDomainData: %s, error: %s.",
			request, patchBytes, originalBytes, modifiedBytes, err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrPatchDomainDataFailed), err.Error()),
		}
	}
	// construct the response
	return &kusciaapi.UpdateDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func deleteLocalFsFile(path, relativeUri string) error {
	// Check if the file exists
	filePath := filepath.Join(path, relativeUri)
	if _, err := os.Stat(filePath); err == nil {
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to delete file %s: %v", filePath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check file %s: %v", filePath, err)
	}
	nlog.Infof("File %s deleted successfully", filePath)
	return nil
}

func deleteOSSFile(accessKey, secretKey, endpoint, bucketName, prefix, relativeURI string, virtualHost bool) error {

	nlog.Debugf("Open oss remote endpoint(%s), bucket(%s), relativeURI(%s)", endpoint, bucketName, relativeURI)
	// Create a new session
	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         &endpoint,
		Region:           aws.String("us-west-2"),
		S3ForcePathStyle: pointer.Bool(!virtualHost),
	})

	if err != nil {
		nlog.Warnf("OSS(%s) create session failed with error: %s", endpoint, err.Error())
	}
	client := s3.New(sess)

	// Check if the object exists
	_, err = client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(path.Join(prefix, relativeURI)),
	})
	if err != nil {
		if aerr, ok := err.(s3.RequestFailure); ok && aerr.StatusCode() == 404 {
			nlog.Infof("Object %s does not exist in bucket %s", path.Join(prefix, relativeURI), bucketName)
			return nil
		}
		return fmt.Errorf("failed to check existence of object %s in bucket %s: %v", path.Join(prefix, relativeURI), bucketName, err)
	}

	// Delete the object from the bucket
	_, err = client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(path.Join(prefix, relativeURI)),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %v", path.Join(prefix, relativeURI), bucketName, err)
	}
	nlog.Infof("Successfully deleted OSS file: %s", relativeURI)
	return nil
}

func deleteMysqlTable(user, passwd, endpoint, database, relativeURI string) error {
	nlog.Debugf("Open MySQL Session database(%s), user(%s)", database, user)

	// Build the connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, passwd, endpoint, database)

	// Open a connection to the database
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Drop the table
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", relativeURI)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop table %s: %v", relativeURI, err)
	}

	nlog.Infof("Successfully dropped table: %s", relativeURI)
	return nil
}

func deletePostgresqlTable(user, passwd, endpoint, database, relativeURI string) error {
	nlog.Debugf("Open Postgresql Session database(%s), user(%s)", database, user)

	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		if addrErr, ok := err.(*net.AddrError); ok && addrErr.Err == "missing port in address" {
			host = endpoint
			port = "5432"
			err = nil
		}
	}
	if err != nil {
		nlog.Errorf("Endpoint \"%s\" can't be resolved with net.SplitHostPort()", endpoint)
		return err
	}
	dsn := fmt.Sprintf("user=%s password=%s host=%s dbname=%s port=%s", user, passwd, host, database, port)

	// Open a connection to the database
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Drop the table
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", relativeURI)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop table %s: %v", relativeURI, err)
	}

	nlog.Infof("Successfully dropped table: %s", relativeURI)
	return nil
}

func (s domainDataService) getDsInfoByKey(ctx context.Context, sourceType string, infoKey string) (*kusciaapi.DataSourceInfo, error) {
	if s.configService == nil {
		return nil, fmt.Errorf("cm config service is empty, skip get datasource info by key")
	}

	response := s.configService.QueryConfig(ctx, &confmanager.QueryConfigRequest{Key: infoKey})
	if !utils.IsSuccessCode(response.Status.Code) {
		nlog.Errorf("Query info key failed, code: %d, message: %s", response.Status.Code, response.Status.Message)
		return nil, fmt.Errorf("query info key failed: %v", response.Status.Message)
	}
	info, err := decodeDataSourceInfo(sourceType, response.Value)
	if err != nil {
		nlog.Errorf("Decode datasource info for key %s failed: %v", infoKey, err)
	}
	return info, err
}

func (s domainDataService) decryptInfo(cipherInfo string) (*kusciaapi.DataSourceInfo, error) {
	plaintext, err := tls.DecryptOAEP(s.conf.DomainKey, cipherInfo)
	if err != nil {
		return nil, fmt.Errorf("decrypt data source info failed, %v", err)
	}
	info := &kusciaapi.DataSourceInfo{}
	if err = json.Unmarshal(plaintext, info); err != nil {
		return nil, fmt.Errorf("unmarshal data source info failed, %v", err)
	}
	return info, nil
}

func (s domainDataService) DeleteDomainDataAndSource(ctx context.Context, request *kusciaapi.DeleteDomainDataAndSourceRequest) *kusciaapi.DeleteDomainDataAndSourceResponse {
	// do validate
	if request.DomaindataId == "" || request.DomainId == "" {
		return &kusciaapi.DeleteDomainDataAndSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain id and domaindata id can not be empty"),
		}
	}
	if err := s.validateRequestWhenLite(request); err != nil {
		return &kusciaapi.DeleteDomainDataAndSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.DeleteDomainDataAndSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrAuthFailed, err.Error()),
		}
	}
	// record the delete operation
	nlog.Warnf("Delete domainID: %s domainDataID: %s", request.DomainId, request.DomaindataId)
	// Fetch the DomainData object based on domaindataId
	domainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Failed to get DomainData with ID %s: %v", request.DomaindataId, err)
		return &kusciaapi.DeleteDomainDataAndSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrGetDomainDataFailed), err.Error()),
		}
	}

	// Extract datasourceId and relativeUri from the DomainData object
	datasourceId := domainData.Spec.DataSource
	relativeUri := domainData.Spec.RelativeURI

	domaindatasource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(domainData.Namespace).Get(context.Background(), datasourceId, metav1.GetOptions{})

	var domaindatasourceInfo *kusciaapi.DataSourceInfo
	// decrypt the info
	if len(domaindatasource.Spec.InfoKey) != 0 {
		info, err := s.getDsInfoByKey(ctx, domaindatasource.Spec.Type, domaindatasource.Spec.InfoKey)
		if err != nil {
			nlog.Errorf("Failed to get DomainDataSource info by key %s: %v", domaindatasource.Spec.InfoKey, err)
			return &kusciaapi.DeleteDomainDataAndSourceResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, err.Error()),
			}
		}
		domaindatasourceInfo = info
	} else {
		encryptedInfo, exist := domaindatasource.Spec.Data[encryptedInfo]
		if !exist {
			nlog.Errorf("DomainDataSource %s does not have encrypted info", datasourceId)
			return &kusciaapi.DeleteDomainDataAndSourceResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, "DomainDataSource does not have encrypted info"),
			}
		}
		domaindatasourceInfo, err = s.decryptInfo(encryptedInfo)
		if err != nil {
			nlog.Errorf("Failed to decrypt DomainDataSource info: %v", err)
			return &kusciaapi.DeleteDomainDataAndSourceResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, err.Error()),
			}
		}
	}
	switch domaindatasource.Spec.Type {
	case "localfs":
		if err := deleteLocalFsFile(domaindatasourceInfo.Localfs.Path, relativeUri); err != nil {
			nlog.Errorf("Failed to delete local file at %s %s: %v", domaindatasourceInfo.Localfs.Path, relativeUri, err)
			return &kusciaapi.DeleteDomainDataAndSourceResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, err.Error()),
			}
		}
	case "oss":
		if err := deleteOSSFile(domaindatasourceInfo.Oss.AccessKeyId, domaindatasourceInfo.Oss.AccessKeySecret, domaindatasourceInfo.Oss.Endpoint, domaindatasourceInfo.Oss.Bucket, domaindatasourceInfo.Oss.Prefix, relativeUri, domaindatasourceInfo.Oss.Virtualhost); err != nil {
			nlog.Errorf("Failed to delete OSS file at %s: %v", relativeUri, err)
			return &kusciaapi.DeleteDomainDataAndSourceResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, err.Error()),
			}
		}
	case "mysql":
		if err := deleteMysqlTable(domaindatasourceInfo.Database.User, domaindatasourceInfo.Database.Password, domaindatasourceInfo.Database.Endpoint, domaindatasourceInfo.Database.Database, relativeUri); err != nil {
			nlog.Errorf("Failed to delete MySQL table at %s: %v", relativeUri, err)
			return &kusciaapi.DeleteDomainDataAndSourceResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, err.Error()),
			}
		}

	case "postgresql":
		if err := deletePostgresqlTable(domaindatasourceInfo.Database.User, domaindatasourceInfo.Database.Password, domaindatasourceInfo.Database.Endpoint, domaindatasourceInfo.Database.Database, relativeUri); err != nil {
			nlog.Errorf("Failed to delete PostgreSQL table at %s: %v", relativeUri, err)
			return &kusciaapi.DeleteDomainDataAndSourceResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, err.Error()),
			}
		}
	default:
		nlog.Warnf("Unsupported domainDataSource type: %s", domaindatasource.Spec.Type)
		return &kusciaapi.DeleteDomainDataAndSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed, "unsupported domainDataSource type"),
		}
	}
	// delete kuscia domainData
	err = s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Delete(ctx, request.DomaindataId, metav1.DeleteOptions{})
	if err != nil {
		nlog.Errorf("Delete domainData: %s failed, detail: %s", request.DomaindataId, err.Error())
		return &kusciaapi.DeleteDomainDataAndSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed), err.Error()),
		}
	}
	return &kusciaapi.DeleteDomainDataAndSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataService) DeleteDomainData(ctx context.Context, request *kusciaapi.DeleteDomainDataRequest) *kusciaapi.DeleteDomainDataResponse {
	// do validate
	if request.DomaindataId == "" || request.DomainId == "" {
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain id and domaindata id can not be empty"),
		}
	}
	if err := s.validateRequestWhenLite(request); err != nil {
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrAuthFailed, err.Error()),
		}
	}
	// record the delete operation
	nlog.Warnf("Delete domainID: %s domainDataID: %s", request.DomainId, request.DomaindataId)
	// delete kuscia domainData
	err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Delete(ctx, request.DomaindataId, metav1.DeleteOptions{})
	if err != nil {
		nlog.Errorf("Delete domainData: %s failed, detail: %s", request.DomaindataId, err.Error())
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataFailed), err.Error()),
		}
	}
	return &kusciaapi.DeleteDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataService) QueryDomainData(ctx context.Context, request *kusciaapi.QueryDomainDataRequest) *kusciaapi.QueryDomainDataResponse {
	// do validate
	if request.Data == nil || request.Data.DomaindataId == "" || request.Data.DomainId == "" {
		return &kusciaapi.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain id and domaindata id can not be empty"),
		}
	}
	if err := s.validateRequestWhenLite(request.Data); err != nil {
		return &kusciaapi.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request.Data); err != nil {
		return &kusciaapi.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrAuthFailed, err.Error()),
		}
	}
	// get kuscia domain
	kusciaDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.Data.DomainId).Get(ctx, request.Data.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("QueryDomainData failed, error: %s", err.Error())
		return &kusciaapi.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrGetDomainDataFailed), err.Error()),
		}
	}
	// build domain response
	return &kusciaapi.QueryDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.DomainData{
			DomaindataId: kusciaDomainData.Name,
			DomainId:     kusciaDomainData.Namespace,
			Name:         kusciaDomainData.Spec.Name,
			Type:         kusciaDomainData.Spec.Type,
			RelativeUri:  kusciaDomainData.Spec.RelativeURI,
			DatasourceId: kusciaDomainData.Spec.DataSource,
			Attributes:   kusciaDomainData.Spec.Attributes,
			Partition:    common.Convert2PbPartition(kusciaDomainData.Spec.Partition),
			Columns:      common.Convert2PbColumn(kusciaDomainData.Spec.Columns),
			Vendor:       kusciaDomainData.Spec.Vendor,
			Status:       constants.DomainDataStatusAvailable,
			Author:       kusciaDomainData.Spec.Author,
			FileFormat:   common.Convert2PbFileFormat(kusciaDomainData.Spec.FileFormat),
		},
	}
}

func (s domainDataService) BatchQueryDomainData(ctx context.Context, request *kusciaapi.BatchQueryDomainDataRequest) *kusciaapi.BatchQueryDomainDataResponse {
	// do validate
	for _, v := range request.Data {
		if v.GetDomainId() == "" || v.GetDomaindataId() == "" {
			return &kusciaapi.BatchQueryDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain id and domaindata id can not be empty"),
			}
		}
		// check the request when this is kuscia lite api
		if err := s.validateRequestWhenLite(v); err != nil {
			return &kusciaapi.BatchQueryDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
			}
		}
		// auth pre handler
		if err := s.authHandler(ctx, v); err != nil {
			return &kusciaapi.BatchQueryDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrAuthFailed, err.Error()),
			}
		}
	}
	// get kuscia domain
	respDatas := make([]*kusciaapi.DomainData, 0, len(request.Data))
	for _, v := range request.Data {
		kusciaDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(v.DomainId).Get(ctx, v.DomaindataId, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			nlog.Errorf("QueryDomainData failed, error: %s", err.Error())
			return &kusciaapi.BatchQueryDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrGetDomainDataFailed, err.Error()),
			}
		}
		domainData := kusciaapi.DomainData{
			DomaindataId: kusciaDomainData.Name,
			DomainId:     kusciaDomainData.Namespace,
			Name:         kusciaDomainData.Spec.Name,
			Type:         kusciaDomainData.Spec.Type,
			RelativeUri:  kusciaDomainData.Spec.RelativeURI,
			DatasourceId: kusciaDomainData.Spec.DataSource,
			Attributes:   kusciaDomainData.Spec.Attributes,
			Partition:    common.Convert2PbPartition(kusciaDomainData.Spec.Partition),
			Columns:      common.Convert2PbColumn(kusciaDomainData.Spec.Columns),
			Vendor:       kusciaDomainData.Spec.Vendor,
			Status:       constants.DomainDataStatusAvailable,
			Author:       kusciaDomainData.Spec.Author,
			FileFormat:   common.Convert2PbFileFormat(kusciaDomainData.Spec.FileFormat),
		}
		respDatas = append(respDatas, &domainData)
	}
	// build domain response
	return &kusciaapi.BatchQueryDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.DomainDataList{
			DomaindataList: respDatas,
		},
	}
}

func (s domainDataService) ListDomainData(ctx context.Context, request *kusciaapi.ListDomainDataRequest) *kusciaapi.ListDomainDataResponse {
	// do validate
	if request.Data == nil || request.Data.DomainId == "" {
		return &kusciaapi.ListDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain id can not be empty"),
		}
	}
	if err := s.validateRequestWhenLite(request.Data); err != nil {
		return &kusciaapi.ListDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request.Data); err != nil {
		return &kusciaapi.ListDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrAuthFailed, err.Error()),
		}
	}
	// construct label selector
	var (
		selector    fields.Selector
		selectorStr string
	)
	if request.Data.DomaindataType != "" {
		typeSelector := fields.OneTermEqualSelector(common.LabelDomainDataType, request.Data.DomaindataType)
		selector = typeSelector
		selectorStr = selector.String()
	}
	if request.Data.DomaindataVendor != "" {
		vendorSelector := fields.OneTermEqualSelector(common.LabelDomainDataVendor, request.Data.DomaindataVendor)
		if selector != nil {
			selector = fields.AndSelectors(selector, vendorSelector)
		} else {
			selector = vendorSelector
		}
		selectorStr = selector.String()
	}
	// get kuscia domain
	// todo support limit and continue
	dataList, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.Data.DomainId).List(ctx, metav1.ListOptions{
		TypeMeta:       metav1.TypeMeta{},
		LabelSelector:  selectorStr,
		TimeoutSeconds: nil,
		Limit:          0,
		Continue:       "",
	})
	if err != nil {
		nlog.Errorf("List DomainData failed, error: %s", err.Error())
		return &kusciaapi.ListDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrListDomainDataFailed, err.Error()),
		}
	}
	respDatas := make([]*kusciaapi.DomainData, len(dataList.Items))
	for i, v := range dataList.Items {
		domaindata := kusciaapi.DomainData{
			DomaindataId: v.Name,
			DomainId:     v.Namespace,
			Name:         v.Spec.Name,
			Type:         v.Spec.Type,
			RelativeUri:  v.Spec.RelativeURI,
			DatasourceId: v.Spec.DataSource,
			Attributes:   v.Spec.Attributes,
			Partition:    common.Convert2PbPartition(v.Spec.Partition),
			Columns:      common.Convert2PbColumn(v.Spec.Columns),
			Vendor:       v.Spec.Vendor,
			Status:       constants.DomainDataStatusAvailable,
			Author:       v.Spec.Author,
			FileFormat:   common.Convert2PbFileFormat(v.Spec.FileFormat),
		}
		respDatas[i] = &domaindata
	}
	// build domain response
	return &kusciaapi.ListDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.DomainDataList{
			DomaindataList: respDatas,
		},
	}
}

func convert2UpdateReq(createReq *kusciaapi.CreateDomainDataRequest) (updateReq *kusciaapi.UpdateDomainDataRequest) {
	updateReq = &kusciaapi.UpdateDomainDataRequest{
		Header:       createReq.Header,
		DomainId:     createReq.DomainId,
		DomaindataId: createReq.DomaindataId,
		Name:         createReq.Name,
		Type:         createReq.Type,
		RelativeUri:  createReq.RelativeUri,
		DatasourceId: createReq.DatasourceId,
		Attributes:   createReq.Attributes,
		Partition:    createReq.Partition,
		Columns:      createReq.Columns,
		FileFormat:   createReq.FileFormat,
	}
	return
}

func convert2CreateResp(updateResp *kusciaapi.UpdateDomainDataResponse, domainDataID string) (createResp *kusciaapi.CreateDomainDataResponse) {
	createResp = &kusciaapi.CreateDomainDataResponse{
		Status: updateResp.Status,
		Data: &kusciaapi.CreateDomainDataResponseData{
			DomaindataId: domainDataID,
		},
	}
	return
}

func (s domainDataService) normalizationCreateRequest(request *kusciaapi.CreateDomainDataRequest) {
	// normalization domaindata name
	if request.Name == "" {
		uris := strings.Split(request.RelativeUri, "/")
		if len(uris) > 0 {
			request.Name = uris[len(uris)-1]
		}
	}
	// normalization domaindata id
	if request.DomaindataId == "" {
		request.DomaindataId = common.GenDomainDataID(request.Name)
	}
	//fill default datasource id
	if request.DatasourceId == "" {
		request.DatasourceId = common.DefaultDataSourceID
	}
	//fill default vendor
	if request.Vendor == "" {
		request.Vendor = common.DefaultDomainDataVendor
	}
	// truncate slash
	// fix Issue:
	// datasource-path: "/home/admin" ; datasource-prefix: "/test/" ; domainData-relativeURI: "/data/alice.csv"
	// os.path.join(path,prefix,uri) would be /data/alice.csv , this is not expect, expect is /home/admin/test/data/alice.csv
	// so trim the prefix filepath.Separator
	request.RelativeUri = strings.TrimPrefix(request.RelativeUri, string(filepath.Separator))
}

func (s domainDataService) normalizationUpdateRequest(request *kusciaapi.UpdateDomainDataRequest, data v1alpha1.DomainDataSpec) {
	if request.Name == "" {
		request.Name = data.Name
	}
	if request.Type == "" {
		request.Type = data.Type
	}
	if request.RelativeUri == "" {
		request.RelativeUri = data.RelativeURI
	}
	if request.DatasourceId == "" {
		request.DatasourceId = data.DataSource
	}
	if len(request.Columns) == 0 {
		request.Columns = common.Convert2PbColumn(data.Columns)
	}
	if request.Partition == nil {
		request.Partition = common.Convert2PbPartition(data.Partition)
	}
	if len(request.Attributes) == 0 {
		request.Attributes = data.Attributes
	}
	if request.Vendor == "" {
		request.Vendor = data.Vendor
	}
	if request.FileFormat == pbv1alpha1.FileFormat_UNKNOWN {
		request.FileFormat = common.Convert2PbFileFormat(data.FileFormat)
	}
}

func (s domainDataService) authHandler(ctx context.Context, request RequestWithDomainID) error {
	role, domainID := GetRoleAndDomainFromCtx(ctx)
	if role == consts.AuthRoleDomain && request.GetDomainId() != domainID {
		return fmt.Errorf("domain's kusciaAPI could only operate its own DomainData, request.DomainID must be %s not %s", domainID, request.GetDomainId())
	}
	return nil
}

func (s domainDataService) validateRequestWhenLite(request RequestWithDomainID) error {
	if s.conf.RunMode == common.RunModeLite && request.GetDomainId() != s.conf.Initiator {
		return fmt.Errorf("kuscia lite api could only operate it's own domaindata, the domainid of request must be %s, not %s", s.conf.Initiator, request.GetDomainId())
	}
	return nil
}
