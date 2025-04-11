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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

var (
	errCreateDomainDataSource     = "CreateDomainDataSource failed, %v"
	errUpdateDomainDataSource     = "UpdateDomainDataSource failed, %v"
	errDeleteDomainDataSource     = "DeleteDomainDataSource failed, %v"
	errQueryDomainDataSource      = "QueryDomainDataSource failed, %v"
	errBatchQueryDomainDataSource = "BatchQueryDomainDataSource failed, %v"
	errListDomainDataSource       = "ListDomainDataSource failed, %v"
)

const (
	encryptedInfo = "encryptedInfo"
)

type IDomainDataSourceService interface {
	CreateDomainDataSource(ctx context.Context, request *kusciaapi.CreateDomainDataSourceRequest) *kusciaapi.CreateDomainDataSourceResponse
	UpdateDomainDataSource(ctx context.Context, request *kusciaapi.UpdateDomainDataSourceRequest) *kusciaapi.UpdateDomainDataSourceResponse
	DeleteDomainDataSource(ctx context.Context, request *kusciaapi.DeleteDomainDataSourceRequest) *kusciaapi.DeleteDomainDataSourceResponse
	QueryDomainDataSource(ctx context.Context, request *kusciaapi.QueryDomainDataSourceRequest) *kusciaapi.QueryDomainDataSourceResponse
	BatchQueryDomainDataSource(ctx context.Context, request *kusciaapi.BatchQueryDomainDataSourceRequest) *kusciaapi.BatchQueryDomainDataSourceResponse
	ListDomainDataSource(ctx context.Context, request *kusciaapi.ListDomainDataSourceRequest) *kusciaapi.ListDomainDataSourceResponse
}

type domainDataSourceService struct {
	conf          *config.KusciaAPIConfig
	configService cmservice.IConfigService
}

func NewDomainDataSourceService(config *config.KusciaAPIConfig, configService cmservice.IConfigService) IDomainDataSourceService {
	return &domainDataSourceService{
		conf:          config,
		configService: configService,
	}
}

func (s domainDataSourceService) CreateDomainDataSource(ctx context.Context, request *kusciaapi.CreateDomainDataSourceRequest) *kusciaapi.CreateDomainDataSourceResponse {
	var err error
	if err = s.validateRequestIdentity(request.DomainId); err != nil {
		nlog.Errorf(errCreateDomainDataSource, err.Error())
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	if err = validateDataSourceType(request.Type); err != nil {
		nlog.Errorf(errCreateDomainDataSource, err.Error())
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	if request.DatasourceId == "" {
		request.DatasourceId = common.GenDomainDataSourceID(request.Type)
	}

	if err = resources.ValidateK8sName(request.DatasourceId, "datasource_id"); err != nil {
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	if (request.InfoKey == nil || *request.InfoKey == "") && request.Info == nil {
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataSource),
				fmt.Sprintf("domain data source info key and info all empty")),
		}
	}

	domainDataSource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Get(ctx, request.DatasourceId, metav1.GetOptions{})
	if domainDataSource != nil && err == nil {
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataSource),
				fmt.Sprintf("domain data source %s already exist", request.DatasourceId)),
		}
	}

	dataSource := &v1alpha1.DomainDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.DatasourceId,
			Namespace: request.DomainId,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			Type: request.Type,
		},
	}

	if request.Name != nil {
		dataSource.Spec.Name = *request.Name
	}

	if request.InfoKey != nil && *request.InfoKey != "" {
		datasourceInfo, getErr := s.getDsInfoByKey(ctx, request.Type, *request.InfoKey)
		if getErr != nil {
			return &kusciaapi.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(getErr, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataSource),
					fmt.Sprintf("domain data source info key %s not exist", *request.InfoKey)),
			}
		}
		uri, parseErr := parseAndNormalizeDataSource(request.Type, datasourceInfo)
		if parseErr != nil {
			return &kusciaapi.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(parseErr, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataSource),
					fmt.Sprintf("domain data source info key %s can not convert to datasource info", *request.InfoKey)),
			}
		}
		dataSource.Spec.InfoKey = *request.InfoKey
		dataSource.Spec.URI = uri
	} else {

		if err = validateDataSourceInfo(request.Type, request.Info); err != nil {
			return &kusciaapi.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrRequestValidate), err.Error()),
			}
		}

		uri, encInfo, encryptErr := s.encryptInfo(request.Type, request.Info)
		if encryptErr != nil {
			nlog.Errorf(errCreateDomainDataSource, encryptErr.Error())
			return &kusciaapi.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(encryptErr, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataSource), encryptErr.Error()),
			}
		}
		dataSource.Spec.URI = uri
		dataSource.Spec.Data = map[string]string{encryptedInfo: encInfo}
	}

	if request.AccessDirectly != nil {
		dataSource.Spec.AccessDirectly = *request.AccessDirectly
	}

	// create kuscia domain data source
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Create(ctx, dataSource, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf(errCreateDomainDataSource, err.Error())
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.CreateDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataSource), err.Error()),
		}
	}

	return &kusciaapi.CreateDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.CreateDomainDataSourceResponseData{
			DatasourceId: request.DatasourceId,
		},
	}
}

func (s domainDataSourceService) UpdateDomainDataSource(ctx context.Context, request *kusciaapi.UpdateDomainDataSourceRequest) *kusciaapi.UpdateDomainDataSourceResponse {
	var err error
	if err = s.validateRequestIdentity(request.DomainId); err != nil {
		nlog.Errorf(errUpdateDomainDataSource, err.Error())
		return &kusciaapi.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	if request.DatasourceId == "" {
		return &kusciaapi.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain data source id can not be empty"),
		}
	}

	curDataSource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Get(ctx, request.DatasourceId, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return &kusciaapi.UpdateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrUpdateDomainDataSource),
					fmt.Sprintf("domain %v data source %v doesn't exist", request.DomainId, request.DatasourceId)),
			}
		}
		nlog.Errorf(errUpdateDomainDataSource, err.Error())
		return &kusciaapi.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrUpdateDomainDataSource), err.Error()),
		}
	}

	dataSource := curDataSource.DeepCopy()

	updated, err := s.updateDataSource(dataSource, request)
	if err != nil {
		nlog.Errorf(errUpdateDomainDataSource, err.Error())
		return &kusciaapi.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrUpdateDomainDataSource), err.Error()),
		}
	}

	if updated {
		// update kuscia domain data source
		_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Update(ctx, dataSource, metav1.UpdateOptions{})
		if err != nil {
			nlog.Errorf(errUpdateDomainDataSource, err.Error())
			return &kusciaapi.UpdateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrUpdateDomainDataSource), err.Error()),
			}
		}
	}

	return &kusciaapi.UpdateDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataSourceService) updateDataSource(dataSource *v1alpha1.DomainDataSource, request *kusciaapi.UpdateDomainDataSourceRequest) (bool, error) {
	updated := false
	if request.Name != nil && *request.Name != dataSource.Spec.Name {
		dataSource.Spec.Name = *request.Name
		updated = true
	}

	if request.InfoKey != nil && *request.InfoKey != dataSource.Spec.InfoKey {
		dataSource.Spec.InfoKey = *request.InfoKey
		updated = true
	}

	if request.AccessDirectly != nil && *request.AccessDirectly != dataSource.Spec.AccessDirectly {
		dataSource.Spec.AccessDirectly = *request.AccessDirectly
		updated = true
	}

	if request.Type != "" && request.Type != dataSource.Spec.Type {
		if err := validateDataSourceType(request.Type); err != nil {
			return false, err
		}
		dataSource.Spec.Type = request.Type
		updated = true
	}

	if request.InfoKey == nil && request.Info != nil {
		infoType := request.Type
		if infoType == "" {
			infoType = dataSource.Spec.Type
		}

		if err := validateDataSourceInfo(request.Type, request.Info); err != nil {
			return false, err
		}

		uri, encInfo, err := s.encryptInfo(infoType, request.Info)
		if err != nil {
			return false, err
		}
		if uri != dataSource.Spec.URI {
			dataSource.Spec.URI = uri
			updated = true
		}

		if dataSource.Spec.Data == nil {
			dataSource.Spec.Data = map[string]string{}
		}
		if dataSource.Spec.Data[encryptedInfo] != encInfo {
			dataSource.Spec.Data[encryptedInfo] = encInfo
			updated = true
		}
	}
	return updated, nil
}

func (s domainDataSourceService) DeleteDomainDataSource(ctx context.Context, request *kusciaapi.DeleteDomainDataSourceRequest) *kusciaapi.DeleteDomainDataSourceResponse {
	var err error
	if err = s.validateRequestIdentity(request.DomainId); err != nil {
		nlog.Errorf(errDeleteDomainDataSource, err.Error())
		return &kusciaapi.DeleteDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error()),
		}
	}

	if request.DatasourceId == "" {
		return &kusciaapi.DeleteDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain data source id can not be empty"),
		}
	}

	// delete kuscia domain data source
	if err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Delete(ctx, request.DatasourceId, metav1.DeleteOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return &kusciaapi.DeleteDomainDataSourceResponse{
				Status: utils.BuildSuccessResponseStatus(),
			}
		}
		nlog.Errorf(errDeleteDomainDataSource, err.Error())
		return &kusciaapi.DeleteDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataSource), err.Error()),
		}
	}

	return &kusciaapi.DeleteDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataSourceService) QueryDomainDataSource(ctx context.Context, request *kusciaapi.QueryDomainDataSourceRequest) *kusciaapi.QueryDomainDataSourceResponse {
	dataSource, err := s.getDomainDataSource(ctx, request.DomainId, request.DatasourceId)
	if err != nil {
		nlog.Errorf(errQueryDomainDataSource, err.Error())
		return &kusciaapi.QueryDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrQueryDomainDataSource), err.Error()),
		}
	}

	return &kusciaapi.QueryDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   dataSource,
	}
}

func (s domainDataSourceService) BatchQueryDomainDataSource(ctx context.Context, request *kusciaapi.BatchQueryDomainDataSourceRequest) *kusciaapi.BatchQueryDomainDataSourceResponse {
	if request.Data == nil {
		return &kusciaapi.BatchQueryDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "request data can't be empty"),
		}
	}

	var data []*kusciaapi.DomainDataSource
	for _, reqData := range request.Data {
		dataSource, err := s.getDomainDataSource(ctx, reqData.DomainId, reqData.DatasourceId)
		if err != nil {
			nlog.Errorf(errBatchQueryDomainDataSource, err.Error())
			return &kusciaapi.BatchQueryDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrBatchQueryDomainDataSource), err.Error()),
			}
		}
		data = append(data, dataSource)
	}

	return &kusciaapi.BatchQueryDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   &kusciaapi.DomainDataSourceList{DatasourceList: data},
	}
}

func (s domainDataSourceService) ListDomainDataSource(ctx context.Context, request *kusciaapi.ListDomainDataSourceRequest) *kusciaapi.ListDomainDataSourceResponse {
	if request.DomainId == "" {
		return &kusciaapi.ListDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "request domainID can't be empty"),
		}
	}
	if err := s.validateRetrieveRequest(request.DomainId); err != nil {
		nlog.Errorf(errListDomainDataSource, err.Error())
		return &kusciaapi.ListDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrListDomainDataSource, err.Error()),
		}
	}
	var data []*kusciaapi.DomainDataSource
	dsList, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.GetDomainId()).List(ctx, metav1.ListOptions{})
	if err != nil {
		nlog.Errorf("List DomainDataSource failed, %v", err.Error())
		goto returnErr

	}
	for _, ds := range dsList.Items {
		var info *kusciaapi.DataSourceInfo
		// master mode could not decrypt Info, only autonomy and lite need decrypt Info
		if s.conf.RunMode != common.RunModeMaster {
			if info, err = s.decryptDatasourceInfo(ctx, &ds); err != nil {
				goto returnErr
			}
		}
		ids := &kusciaapi.DomainDataSource{
			DomainId:       ds.Namespace,
			DatasourceId:   ds.Name,
			Name:           ds.Spec.Name,
			Type:           ds.Spec.Type,
			Info:           info,
			InfoKey:        ds.Spec.InfoKey,
			AccessDirectly: ds.Spec.AccessDirectly}
		data = append(data, ids)
	}

	return &kusciaapi.ListDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   &kusciaapi.DomainDataSourceList{DatasourceList: data},
	}
returnErr:
	nlog.Error(err.Error())
	return &kusciaapi.ListDomainDataSourceResponse{
		Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrListDomainDataSource, err.Error()),
	}
}

func (s domainDataSourceService) decryptDatasourceInfo(ctx context.Context, ds *v1alpha1.DomainDataSource) (info *kusciaapi.DataSourceInfo, err error) {
	if len(ds.Spec.InfoKey) != 0 {
		info, err = s.getDsInfoByKey(ctx, ds.Spec.Type, ds.Spec.InfoKey)
		if err != nil {
			return nil, err
		}
	} else {
		encryptedInfo, exist := ds.Spec.Data[encryptedInfo]
		if !exist {
			return nil, fmt.Errorf("missing datasource info for %s", ds.Spec.Name)
		}
		info, err = s.decryptInfo(encryptedInfo)
		if err != nil {
			return nil, err
		}
	}
	return info, err
}

func (s domainDataSourceService) getDomainDataSource(ctx context.Context, domainID, dataSourceID string) (*kusciaapi.DomainDataSource, error) {
	if err := s.validateRetrieveRequest(domainID); err != nil {
		return nil, err
	}

	if dataSourceID == "" {
		return nil, fmt.Errorf("domain %v data source id can not be empty", domainID)
	}

	kusciaDataSource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(domainID).Get(ctx, dataSourceID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get domain %v kuscia data source %v failed, %v", domainID, dataSourceID, err.Error())
	}

	var info *kusciaapi.DataSourceInfo
	// master mode could not decrypt Info, only autonomy and lite need decrypt Info
	if s.conf.RunMode != common.RunModeMaster {
		if info, err = s.decryptDatasourceInfo(ctx, kusciaDataSource); err != nil {
			return nil, err
		}
	}

	return &kusciaapi.DomainDataSource{
		DomainId:       kusciaDataSource.Namespace,
		DatasourceId:   kusciaDataSource.Name,
		Name:           kusciaDataSource.Spec.Name,
		Type:           kusciaDataSource.Spec.Type,
		Info:           info,
		InfoKey:        kusciaDataSource.Spec.InfoKey,
		AccessDirectly: kusciaDataSource.Spec.AccessDirectly}, nil
}

func CheckDomainDataSourceExists(kusciaClient kusciaclientset.Interface, domainID, domainDataSourceID string) (kusciaError pberrorcode.ErrorCode, errorMsg string) {

	_, err := kusciaClient.KusciaV1alpha1().DomainDataSources(domainID).Get(context.Background(), domainDataSourceID, metav1.GetOptions{})
	if err != nil {
		errorCode := errorcode.GetDomainDataSourceErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrQueryDomainDataSource)
		if pberrorcode.ErrorCode_KusciaAPIErrDomainDataSourceNotExists == errorCode {
			return errorCode, fmt.Sprintf("domain data source `%s` is not exists in domain `%s`", domainDataSourceID, domainID)
		}

		return errorCode, err.Error()
	}
	return pberrorcode.ErrorCode_SUCCESS, ""
}

func validateDataSourceType(t string) error {
	if t != common.DomainDataSourceTypeOSS &&
		t != common.DomainDataSourceTypeMysql &&
		t != common.DomainDataSourceTypeLocalFS &&
		t != common.DomainDataSourceTypeODPS &&
		t != common.DomainDataSourceTypePostgreSQL {
		return fmt.Errorf("domain data source type %q doesn't support, the available types are [localfs,oss,mysql,odps,postgresql]", t)
	}
	return nil
}

func (s domainDataSourceService) validateRequestIdentity(domainID string) error {
	if s.conf.RunMode == common.RunModeMaster {
		return errors.New("master's kuscia api can't operate domain data source")
	}
	if domainID == "" {
		return errors.New("domain id can not be empty")
	}
	// do k8s validate
	if err := resources.ValidateK8sName(domainID, "domain_id"); err != nil {
		return err
	}
	if domainID != s.conf.Initiator {
		return fmt.Errorf("domain %v can't operate domain %v data source", s.conf.Initiator, domainID)
	}
	return nil
}

func (s domainDataSourceService) validateRetrieveRequest(domainID string) error {
	if s.conf.RunMode == common.RunModeMaster {
		return nil
	}
	if domainID == "" {
		return errors.New("domain id can not be empty")
	}
	// do k8s validate
	if err := resources.ValidateK8sName(domainID, "domain_id"); err != nil {
		return err
	}
	if domainID != s.conf.Initiator {
		return fmt.Errorf("domain %v can't operate domain %v data source", s.conf.Initiator, domainID)
	}
	return nil
}

//nolint:dupl
func (s domainDataSourceService) getDsInfoByKey(ctx context.Context, sourceType string, infoKey string) (*kusciaapi.DataSourceInfo, error) {
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

//nolint:dupl
func (s domainDataSourceService) encryptInfo(dataSourceType string, info *kusciaapi.DataSourceInfo) (uri string, encInfo string, err error) {
	uri, err = parseAndNormalizeDataSource(dataSourceType, info)
	if err != nil {
		return "", "", fmt.Errorf("parse data source failed, %v", err)
	}

	var infoBytes []byte
	infoBytes, err = json.Marshal(info)
	if err != nil {
		return "", "", err
	}

	// encrypt
	encInfo, err = tls.EncryptOAEP(&s.conf.DomainKey.PublicKey, infoBytes)
	if err != nil {
		return "", "", fmt.Errorf("encrypt plaintext failed, %v", err)
	}

	return
}

//nolint:dupl
func (s domainDataSourceService) decryptInfo(cipherInfo string) (*kusciaapi.DataSourceInfo, error) {
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

// connectionStr in secretBackend is a json format string
// nolint:dulp
func decodeDataSourceInfo(sourceType string, connectionStr string) (*kusciaapi.DataSourceInfo, error) {
	var dsInfo kusciaapi.DataSourceInfo
	var err error
	connectionBytes := []byte(connectionStr)
	switch sourceType {
	case common.DomainDataSourceTypeOSS:
		dsInfo.Oss = &kusciaapi.OssDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Oss)
	case common.DomainDataSourceTypeMysql, common.DomainDataSourceTypePostgreSQL:
		dsInfo.Database = &kusciaapi.DatabaseDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Database)
	case common.DomainDataSourceTypeLocalFS:
		dsInfo.Localfs = &kusciaapi.LocalDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Localfs)
	case common.DomainDataSourceTypeODPS:
		dsInfo.Odps = &kusciaapi.OdpsDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Odps)
	default:
		err = fmt.Errorf("invalid datasourceType:%s", sourceType)
	}
	if err != nil {
		return nil, err
	}

	return &dsInfo, nil
}

//nolint:dupl
func parseAndNormalizeDataSource(sourceType string, info *kusciaapi.DataSourceInfo) (uri string, err error) {
	if info == nil {
		return "", errors.New("info is nil")
	}

	isInvalid := func(invalid bool) bool {
		if invalid {
			err = fmt.Errorf("invalid datasource info")
			nlog.Error(err)
		}
		return invalid
	}

	switch sourceType {
	case common.DomainDataSourceTypeOSS:
		if isInvalid(info.Oss == nil) {
			return
		}
		// truncate slash
		// fix Issue:
		// datasource-path: "/home/admin" ; datasource-prefix: "/test/" ; domainData-relativeURI: "/data/alice.csv"
		// os.path.join(path,prefix,uri) would be /data/alice.csv , this is not expect, expect is /home/admin/test/data/alice.csv
		// so trim the prefix filepath.Separator
		info.Oss.Bucket = strings.Trim(info.Oss.Bucket, string(filepath.Separator))
		info.Oss.Prefix = strings.Trim(info.Oss.Prefix, string(filepath.Separator))
		uri = filepath.Join(info.Oss.Bucket, info.Oss.Prefix)
	case common.DomainDataSourceTypeLocalFS:
		if isInvalid(info.Localfs == nil) {
			return
		}
		// truncate slash
		info.Localfs.Path = strings.TrimRight(info.Localfs.Path, string(filepath.Separator))
		uri = info.Localfs.Path
	case common.DomainDataSourceTypeMysql, common.DomainDataSourceTypePostgreSQL:
		if isInvalid(info.Database == nil) {
			return
		}
		uri = ""
	case common.DomainDataSourceTypeODPS:
		if isInvalid(info.Odps == nil) {
			return
		}
		uri = info.Odps.Endpoint + "/" + info.Odps.Project
	default:
		err = fmt.Errorf("datasource type:%q not support, only support [localfs,oss,mysql,odps,postgresql]", sourceType)
		nlog.Error(err)
		return
	}
	return
}

// validateDataSourceInfo validate data source info
func validateDataSourceInfo(sourceType string, info *kusciaapi.DataSourceInfo) error {

	if info == nil {
		return fmt.Errorf("data source info cannot be nil")
	}

	switch sourceType {
	case common.DomainDataSourceTypeLocalFS:
		if info.Localfs == nil {
			return fmt.Errorf("localfs info is nil")
		}
		if info.Localfs.Path == "" {
			return fmt.Errorf("localfs 'path' is empty")
		}
	case common.DomainDataSourceTypeOSS:
		if info.Oss == nil {
			return fmt.Errorf("oss info is nil")
		}
		if info.Oss.Endpoint == "" {
			return fmt.Errorf("oss 'endpoint' is empty")
		}
		if info.Oss.Bucket == "" {
			return fmt.Errorf("oss 'bucket' is empty")
		}
		if info.Oss.AccessKeyId == "" {
			return fmt.Errorf("oss 'access_key_id' is empty")
		}
		if info.Oss.AccessKeySecret == "" {
			return fmt.Errorf("oss 'access_key_secret' is empty")
		}
	case common.DomainDataSourceTypeMysql:
		if info.Database == nil {
			return fmt.Errorf("mysql info is nil")
		}
		if info.Database.Endpoint == "" {
			return fmt.Errorf("mysql 'endpoint' is empty")
		}
		if info.Database.Database == "" {
			return fmt.Errorf("mysql 'database' is empty")
		}
		if info.Database.User == "" {
			return fmt.Errorf("mysql 'user' is empty")
		}
		if info.Database.Password == "" {
			return fmt.Errorf("mysql 'password' is empty")
		}
	case common.DomainDataSourceTypePostgreSQL:
		if info.Database == nil {
			return fmt.Errorf("postgresql info is nil")
		}
		if info.Database.Endpoint == "" {
			return fmt.Errorf("postgresql 'endpoint' is empty")
		}
		if info.Database.Database == "" {
			return fmt.Errorf("postgresql 'database' is empty")
		}
		if info.Database.User == "" {
			return fmt.Errorf("postgresql 'user' is empty")
		}
		if info.Database.Password == "" {
			return fmt.Errorf("postgresql 'password' is empty")
		}
	case common.DomainDataSourceTypeODPS:
		if info.Odps == nil {
			return fmt.Errorf("odps info is nil")
		}
		if info.Odps.Endpoint == "" {
			return fmt.Errorf("odps 'endpoint' is empty")
		}
		if !isValidURL(info.Odps.Endpoint) {
			return fmt.Errorf("odps 'endpoint' is invalid, should contain the prefix: http or https")
		}
		if info.Odps.Project == "" {
			return fmt.Errorf("odps 'project' is empty")
		}
		if info.Odps.AccessKeyId == "" {
			return fmt.Errorf("odps 'access_key_id' is empty")
		}
		if info.Odps.AccessKeySecret == "" {
			return fmt.Errorf("odps 'access_key_secret' is empty")
		}
	default:
		return fmt.Errorf("invalid datasource type:%s", sourceType)

	}
	return nil
}

// isValidURL checks if the endpoint is a valid URL.
func isValidURL(endpoint string) bool {

	parse, err := url.Parse(endpoint)
	if err != nil {
		return false
	}

	if parse.Host == "" {
		return false
	}

	return parse.Scheme == constants.SchemaHTTP || parse.Scheme == constants.SchemaHTTPS
}
