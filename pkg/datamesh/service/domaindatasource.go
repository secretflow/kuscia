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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type IDomainDataSourceService interface {
	CreateDomainDataSource(ctx context.Context, request *datamesh.CreateDomainDataSourceRequest) *datamesh.CreateDomainDataSourceResponse
	QueryDomainDataSource(ctx context.Context, request *datamesh.QueryDomainDataSourceRequest) *datamesh.QueryDomainDataSourceResponse
	UpdateDomainDataSource(ctx context.Context, request *datamesh.UpdateDomainDataSourceRequest) *datamesh.UpdateDomainDataSourceResponse
	DeleteDomainDataSource(ctx context.Context, request *datamesh.DeleteDomainDataSourceRequest) *datamesh.DeleteDomainDataSourceResponse
}

type domainDataSourceService struct {
	conf *config.DataMeshConfig
}

// to be removed after use SecretBackend
var (
	dsInfo map[string]*datamesh.DataSourceInfo
)

func NewDomainDataSourceService(config *config.DataMeshConfig) IDomainDataSourceService {
	return &domainDataSourceService{
		conf: config,
	}
}

func (s domainDataSourceService) CreateDomainDataSource(ctx context.Context, request *datamesh.CreateDomainDataSourceRequest) *datamesh.CreateDomainDataSourceResponse {
	var (
		info *datamesh.DataSourceInfo
		err  error
	)

	var dsID string
	if request.DatasourceId != "" {
		dsID = request.GetDatasourceId()
	} else {
		dsID = common.GenDomainDataID(request.Name)
	}

	if len(request.InfoKey) > 0 {
		if info, err = s.getDsInfoByKey(request.Type, request.InfoKey); err != nil {
			return &datamesh.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrDecodeDomainDataSourceInfoFailed, err.Error()),
			}
		}
	} else {
		info = request.GetInfo()
		// todo use set infoKey to DomainDataSourceSpec{
		if _, err := s.setDsInfo(request.Type, request.Info, dsID); err != nil {
			return &datamesh.CreateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrEncodeDomainDataSourceInfoFailed, err.Error()),
			}
		}
		// todo request.InfoKey = dsID
	}

	// parse DataSource
	uri, jsonBytes, err := parseDataSource(request.Type, info)
	if err != nil {
		return &datamesh.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrParseDomainDataSourceFailed, err.Error()),
		}
	}
	// todo remove
	data := make(map[string]string)
	data["encryptInfo"] = string(encryptInfo(jsonBytes))

	// build kuscia domain DataSource
	Labels := make(map[string]string)
	Labels[common.LabelDomainDataSourceType] = request.Type
	kusciaDomainDataSource := &v1alpha1.DomainDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:   dsID,
			Labels: Labels,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			URI:            uri,
			Data:           data,
			Type:           request.Type,
			Name:           dsID,
			InfoKey:        request.InfoKey,
			AccessDirectly: request.AccessDirectly,
		},
	}

	// create kuscia domain datasource
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Create(ctx, kusciaDomainDataSource, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainDataSource failed, error:%s", err.Error())
		return &datamesh.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataSourceErrorCode(err, errorcode.ErrCreateDomainDataSource), err.Error()),
		}
	}

	return &datamesh.CreateDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &datamesh.CreateDomainDataSourceResponseData{
			DatasourceId: dsID,
		},
	}
}

func (s domainDataSourceService) QueryDomainDataSource(ctx context.Context, request *datamesh.QueryDomainDataSourceRequest) *datamesh.QueryDomainDataSourceResponse {
	var (
		kusciaDomainDataSource *v1alpha1.DomainDataSource
		info                   *datamesh.DataSourceInfo
		err                    error
	)

	// get kuscia domain datasource
	kusciaDomainDataSource, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Get(ctx,
		request.DatasourceId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("QueryDomainDataSource failed, error:%s", err.Error())
		return &datamesh.QueryDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrQueryDomainDataSource, err.Error()),
		}
	}

	if len(kusciaDomainDataSource.Spec.InfoKey) > 0 {
		info, err = s.getDsInfoByKey(kusciaDomainDataSource.Spec.Type, kusciaDomainDataSource.Spec.InfoKey)
	} else {
		info, err = decryptInfo([]byte(kusciaDomainDataSource.Spec.Data["encryptInfo"]))
	}

	if err != nil {
		return &datamesh.QueryDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrQueryDomainDataSource, err.Error()),
		}
	}

	// build domain response
	resp := &datamesh.QueryDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &datamesh.DomainDataSource{
			DatasourceId:   kusciaDomainDataSource.Name,
			Name:           kusciaDomainDataSource.Spec.Name,
			Type:           kusciaDomainDataSource.Spec.Type,
			AccessDirectly: kusciaDomainDataSource.Spec.AccessDirectly,
			Status:         "Available",
			Info:           info,
		},
	}
	return resp
}

func (s domainDataSourceService) UpdateDomainDataSource(ctx context.Context, request *datamesh.UpdateDomainDataSourceRequest) *datamesh.UpdateDomainDataSourceResponse {
	var (
		info *datamesh.DataSourceInfo
		err  error
	)

	originalDataSource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Get(ctx,
		request.DatasourceId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("UpdateDomainDataSource failed, error:%s", err.Error())
		return &datamesh.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrGetDomainDataSourceFromKubeFailed, err.Error()),
		}
	}

	s.normalizationUpdateRequest(request, &originalDataSource.Spec)

	if len(request.InfoKey) > 0 {
		if info, err = s.getDsInfoByKey(request.Type, request.InfoKey); err != nil {
			return &datamesh.UpdateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrDecodeDomainDataSourceInfoFailed, err.Error()),
			}
		}
	} else {
		info = request.GetInfo()
		// todo use set infoKey to DomainDataSourceSpec{
		if _, err := s.setDsInfo(request.Type, request.Info, request.DatasourceId); err != nil {
			return &datamesh.UpdateDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrEncodeDomainDataSourceInfoFailed, err.Error()),
			}
		}
		// todo request.InfoKey = dsID
	}

	// parse DataSource
	data := make(map[string]string)
	uri, jsonBytes, err := parseDataSource(request.Type, info)
	if err != nil {
		return &datamesh.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrParseDomainDataSourceFailed, err.Error()),
		}
	}
	data["encryptInfo"] = string(encryptInfo(jsonBytes))
	modifiedDataSource := &v1alpha1.DomainDataSource{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            request.Name,
			ResourceVersion: originalDataSource.ResourceVersion,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			URI:            uri,
			Name:           request.Name,
			Data:           data,
			Type:           request.Type,
			InfoKey:        request.InfoKey,
			AccessDirectly: request.AccessDirectly,
		},
		Status: v1alpha1.DataStatus{},
	}

	patchBytes, originalBytes, modifiedBytes, err := common.MergeDomainDataSource(originalDataSource,
		modifiedDataSource)
	if err != nil {
		nlog.Errorf("Merge DomainDataSource failed,request:%+v,error:%s.",
			request, err.Error())
		return &datamesh.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrMergeDomainDataSourceFailed, err.Error()),
		}
	}
	nlog.Debugf("Update DomainDataSource request:%+v, patchBytes:%s,originalDataSource:%s,"+
		"modifiedDataSource:%s",
		request, patchBytes, originalBytes, modifiedBytes)
	// patch the merged domainDataSource
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(originalDataSource.Namespace).Patch(ctx,
		originalDataSource.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		// todo: retry if conflict
		nlog.Debugf("Patch DomainDataSource failed, request:%+v, patchBytes:%s,originalDataSource:%s,"+
			"modifiedDataSource:%s,error:%s",
			request, patchBytes, originalBytes, modifiedBytes, err.Error())
		return &datamesh.UpdateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrPatchDomainDataSourceFailed, err.Error()),
		}
	}
	// construct the response
	return &datamesh.UpdateDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataSourceService) DeleteDomainDataSource(ctx context.Context, request *datamesh.DeleteDomainDataSourceRequest) *datamesh.DeleteDomainDataSourceResponse {
	// record the delete operation
	nlog.Warnf("Delete domainDataSourceId %s", request.DatasourceId)
	// delete kuscia domainData
	err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Delete(ctx, request.DatasourceId, metav1.DeleteOptions{})
	if err != nil {
		nlog.Errorf("Delete domainDataSource:%s failed, detail:%s", request.DatasourceId, err.Error())
		return &datamesh.DeleteDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrDeleteDomainDataSourceFailed, err.Error()),
		}
	}

	return &datamesh.DeleteDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataSourceService) normalizationUpdateRequest(request *datamesh.UpdateDomainDataSourceRequest,
	spec *v1alpha1.DomainDataSourceSpec) {
	if request.Name == "" {
		request.Name = spec.Name
	}

	if request.Type == "" {
		request.Type = spec.Type
	}

	if request.InfoKey == "" {
		request.InfoKey = spec.InfoKey
	}
}

func (s domainDataSourceService) getDsInfoByKey(sourceType string, infoKey string) (*datamesh.DataSourceInfo, error) {
	connectionStr := infoKey
	info, err := decodeDataSourceInfo(sourceType, connectionStr)
	if err != nil {
		nlog.Warnf("decode datasource info fail: %v", err)
	}
	return info, err
}

func (s domainDataSourceService) setDsInfo(sourceType string, info *datamesh.DataSourceInfo, dsID string) (string,
	error) {
	if info == nil {
		return "", fmt.Errorf("info is nil")
	}
	_, err := encodeDataSourceInfo(sourceType, info)
	if err != nil {
		nlog.Warnf("encode datasource info fail: %v", err)
		return "", err
	}
	// todo call secretBackend to store dsInfo with key = dsID
	return "", nil
}

func parseDataSource(sourceType string, info *datamesh.DataSourceInfo) (uri string, infoJSON []byte, err error) {
	if info == nil {
		return "", nil, errors.New("info is nil")
	}

	isInvalid := func(invalid bool) bool {
		if invalid {
			errMsg := fmt.Sprintf("Invalid datasource info")
			nlog.Error(errMsg)
			err = errors.New(errMsg)
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
		errMsg := fmt.Sprintf("Datasource type:%s not support, only support [localfs,oss,mysql]", sourceType)
		nlog.Error(errMsg)
		err = errors.New(errMsg)
		return
	}
	infoJSON, err = json.Marshal(info)
	if err != nil {
		nlog.Errorf("Marshal info:%+v failed,error:%s", info, err.Error())
	}
	return
}

func encryptInfo(originalBytes []byte) (encryptBytes []byte) {
	// todo encrypt by private key
	return originalBytes
}

func decryptInfo(encryptBytes []byte) (*datamesh.DataSourceInfo, error) {
	info := &datamesh.DataSourceInfo{}
	if err := json.Unmarshal(encryptBytes, info); err != nil {
		nlog.Warnf("Unmarshal to DataSourceInfo fail: %v", err)
		return nil, err
	}

	return info, nil
}

// connectionStr in secretBackend is a json format string
func decodeDataSourceInfo(sourceType string, connectionStr string) (*datamesh.DataSourceInfo, error) {
	var dsInfo datamesh.DataSourceInfo
	var err error
	connectionBytes := []byte(connectionStr)
	switch sourceType {
	case common.DomainDataSourceTypeOSS:
		dsInfo.Oss = &datamesh.OssDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Oss)
	case common.DomainDataSourceTypeMysql:
		dsInfo.Database = &datamesh.DatabaseDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Database)
	case common.DomainDataSourceTypeLocalFS:
		dsInfo.Localfs = &datamesh.LocalDataSourceInfo{}
		err = json.Unmarshal(connectionBytes, dsInfo.Localfs)
	default:
		err = fmt.Errorf("invalid datasourceType:%s", sourceType)
	}
	if err != nil {
		return nil, err
	}

	return &dsInfo, nil
}

// connectionStr in secretBackend is a value string like
func encodeDataSourceInfo(sourceType string, dsInfo *datamesh.DataSourceInfo) (string, error) {
	var err error
	var connectionStr []byte
	switch sourceType {
	case common.DomainDataSourceTypeOSS:
		connectionStr, err = json.Marshal(dsInfo.Oss)
	case common.DomainDataSourceTypeMysql:
		connectionStr, err = json.Marshal(dsInfo.Database)
	case common.DomainDataSourceTypeLocalFS:
		connectionStr, err = json.Marshal(dsInfo.Localfs)
	default:
		err = fmt.Errorf("invalid datasourceType:%s", sourceType)
	}
	if err != nil {
		return "", err
	}

	return string(connectionStr), nil
}

func isFSDataSource(dsType string) bool {
	return dsType == common.DomainDataSourceTypeLocalFS || dsType == common.DomainDataSourceTypeOSS
}
