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

//nolint:dulp
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

var (
	errCreateDomainDataSource     = "CreateDomainDataSource failed, %v"
	errUpdateDomainDataSource     = "UpdateDomainDataSource failed, %v"
	errDeleteDomainDataSource     = "DeleteDomainDataSource failed, %v"
	errQueryDomainDataSource      = "QueryDomainDataSource failed, %v"
	errBatchQueryDomainDataSource = "BatchQueryDomainDataSource failed, %v"
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
}

type domainDataSourceService struct {
	conf                 *config.KusciaAPIConfig
	configurationService cmservice.IConfigurationService
}

func NewDomainDataSourceService(config *config.KusciaAPIConfig, configurationService cmservice.IConfigurationService) IDomainDataSourceService {
	return &domainDataSourceService{
		conf:                 config,
		configurationService: configurationService,
	}
}

func (s domainDataSourceService) CreateDomainDataSource(ctx context.Context, request *kusciaapi.CreateDomainDataSourceRequest) *kusciaapi.CreateDomainDataSourceResponse {
	var err error
	if err = s.validateRequestIdentity(request.DomainId); err != nil {
		nlog.Errorf(errCreateDomainDataSource, err.Error())
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}

	if request.DatasourceId == "" {
		name := ""
		if request.Name != nil {
			name = *request.Name
		}
		request.DatasourceId = common.GenDomainDataID(name)
	}

	if err = validateDataSourceType(request.Type); err != nil {
		nlog.Errorf(errCreateDomainDataSource, err.Error())
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}

	if (request.InfoKey == nil || *request.InfoKey == "") && request.Info == nil {
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrCreateDomainDataSource),
				fmt.Sprintf("domain data source info key and info all empty")),
		}
	}

	domainDataSource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Get(ctx, request.DatasourceId, metav1.GetOptions{})
	if domainDataSource != nil && err == nil {
		return &kusciaapi.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrCreateDomainDataSource),
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

	if request.InfoKey != nil {
		datasourceInfo, err := s.getDsInfoByKey(ctx, request.Type, *request.InfoKey)
		if err != nil {
			return &kusciaapi.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrCreateDomainDataSource),
					fmt.Sprintf("domain data source info key %s not exist", *request.InfoKey)),
			}
		}
		uri, err := parseDataSourceURI(request.Type, datasourceInfo)
		if err != nil {
			return &kusciaapi.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrCreateDomainDataSource),
					fmt.Sprintf("domain data source info key %s can not convert to datasource info", *request.InfoKey)),
			}
		}
		dataSource.Spec.InfoKey = *request.InfoKey
		dataSource.Spec.URI = uri
	} else {
		uri, encInfo, err := s.encryptInfo(request.Type, request.Info)
		if err != nil {
			nlog.Errorf(errCreateDomainDataSource, err.Error())
			return &kusciaapi.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrCreateDomainDataSource), err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.CreateDomainDataSourceErrorCode(err, errorcode.ErrCreateDomainDataSource), err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}

	if request.DatasourceId == "" {
		return &kusciaapi.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain data source id can not be empty"),
		}
	}

	curDataSource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Get(ctx, request.DatasourceId, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return &kusciaapi.UpdateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrUpdateDomainDataSource),
					fmt.Sprintf("domain %v data source %v doesn't exist", request.DomainId, request.DatasourceId)),
			}
		}
		nlog.Errorf(errUpdateDomainDataSource, err.Error())
		return &kusciaapi.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrUpdateDomainDataSource), err.Error()),
		}
	}

	dataSource := curDataSource.DeepCopy()

	updated, err := s.updateDataSource(dataSource, request)
	if err != nil {
		nlog.Errorf(errUpdateDomainDataSource, err.Error())
		return &kusciaapi.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrUpdateDomainDataSource), err.Error()),
		}
	}

	if updated {
		// update kuscia domain data source
		_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(request.DomainId).Update(ctx, dataSource, metav1.UpdateOptions{})
		if err != nil {
			nlog.Errorf(errUpdateDomainDataSource, err.Error())
			return &kusciaapi.UpdateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrUpdateDomainDataSource), err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}

	if request.DatasourceId == "" {
		return &kusciaapi.DeleteDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain data source id can not be empty"),
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
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrDeleteDomainDataSource), err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrQueryDomainDataSource), err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "request data can't be empty"),
		}
	}

	var data []*kusciaapi.DomainDataSource
	for _, reqData := range request.Data {
		dataSource, err := s.getDomainDataSource(ctx, reqData.DomainId, reqData.DatasourceId)
		if err != nil {
			nlog.Errorf(errBatchQueryDomainDataSource, err.Error())
			return &kusciaapi.BatchQueryDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrBatchQueryDomainDataSource), err.Error()),
			}
		}
		data = append(data, dataSource)
	}

	return &kusciaapi.BatchQueryDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   &kusciaapi.DomainDataSourceList{DatasourceList: data},
	}
}

func (s domainDataSourceService) getDomainDataSource(ctx context.Context, domainID, dataSourceID string) (*kusciaapi.DomainDataSource, error) {
	if err := s.validateRequestIdentity(domainID); err != nil {
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
	if len(kusciaDataSource.Spec.InfoKey) != 0 {
		info, err = s.getDsInfoByKey(ctx, kusciaDataSource.Spec.Type, kusciaDataSource.Spec.InfoKey)
		if err != nil {
			return nil, err
		}
	} else {
		encryptedInfo, exist := kusciaDataSource.Spec.Data[encryptedInfo]
		if !exist {
			return nil, fmt.Errorf("missing datasource info for %s", kusciaDataSource.Spec.Name)
		}
		info, err = s.decryptInfo(encryptedInfo)
		if err != nil {
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

func validateDataSourceType(t string) error {
	if t != common.DomainDataSourceTypeOSS &&
		t != common.DomainDataSourceTypeMysql &&
		t != common.DomainDataSourceTypeLocalFS {
		return fmt.Errorf("domain data source type %q doesn't support, the available types are [localfs,oss,mysql]", t)
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

// nolint:dulp
func (s domainDataSourceService) getDsInfoByKey(ctx context.Context, sourceType string, infoKey string) (*kusciaapi.DataSourceInfo, error) {
	response := s.configurationService.QueryConfiguration(ctx, &confmanager.QueryConfigurationRequest{
		Ids: []string{infoKey},
	}, s.conf.DomainID)
	if response.Status.Code != utils.ResponseCodeSuccess {
		nlog.Errorf("Query info key failed, code: %d, message: %s", response.Status.Code, response.Status.Message)
		return nil, fmt.Errorf("query info key failed")
	}
	if response.Configurations == nil || response.Configurations[infoKey] == nil {
		nlog.Errorf("Query info key success but info key %s not found", infoKey)
		return nil, fmt.Errorf("query info key not found")
	}
	infoKeyResult := response.Configurations[infoKey]
	if !infoKeyResult.Success {
		nlog.Errorf("Query info key %s failed: %s", infoKey, infoKeyResult.ErrMsg)
		return nil, fmt.Errorf("query info key failed")
	}
	info, err := decodeDataSourceInfo(sourceType, infoKeyResult.Content)
	if err != nil {
		nlog.Errorf("Decode datasource info for key %s fail: %v", infoKey, err)
	}
	return info, err
}

// nolint:dulp
func (s domainDataSourceService) encryptInfo(dataSourceType string, info *kusciaapi.DataSourceInfo) (uri string, encInfo string, err error) {
	uri, err = parseDataSourceURI(dataSourceType, info)
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

// nolint:dulp
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
func decodeDataSourceInfo(sourceType string, connectionStr string) (*kusciaapi.DataSourceInfo, error) {
	var dsInfo kusciaapi.DataSourceInfo
	var err error
	connectionBytes := []byte(connectionStr)
	switch sourceType {
	case common.DomainDataSourceTypeOSS:
		dsInfo.Oss = &kusciaapi.OssDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Oss)
	case common.DomainDataSourceTypeMysql:
		dsInfo.Database = &kusciaapi.DatabaseDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Database)
	case common.DomainDataSourceTypeLocalFS:
		dsInfo.Localfs = &kusciaapi.LocalDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Localfs)
	default:
		err = fmt.Errorf("invalid datasourceType:%s", sourceType)
	}
	if err != nil {
		return nil, err
	}

	return &dsInfo, nil
}

// nolint:dulp
func parseDataSourceURI(sourceType string, info *kusciaapi.DataSourceInfo) (uri string, err error) {
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
		uri = strings.TrimRight(info.Oss.Bucket, "/") + "/" + strings.TrimLeft(info.Oss.Prefix, "/")
	case common.DomainDataSourceTypeLocalFS:
		if isInvalid(info.Localfs == nil) {
			return
		}
		uri = info.Localfs.Path
	case common.DomainDataSourceTypeMysql:
		if isInvalid(info.Database == nil) {
			return
		}
		uri = ""
	default:
		err = fmt.Errorf("datasource type:%q not support, only support [localfs,oss,mysql]", sourceType)
		nlog.Error(err)
		return
	}
	return
}
