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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func NewDomainDataSourceService(config *config.DataMeshConfig) IDomainDataSourceService {
	return &domainDataSourceService{
		conf: config,
	}
}

func (s domainDataSourceService) CreateDomainDataSource(ctx context.Context, request *datamesh.CreateDomainDataSourceRequest) *datamesh.CreateDomainDataSourceResponse {
	// parse DataSource
	uri, jsonBytes, err := parseDataSource(request.Type, request.GetInfo())
	if err != nil {
		return &datamesh.CreateDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrParseDomainDataSourceFailed, err.Error()),
		}
	}
	data := make(map[string]string)
	data["encryptInfo"] = string(encryptInfo(jsonBytes))
	var dsID string
	if request.DatasourceId != "" {
		dsID = request.GetDatasourceId()
	} else {
		dsID = common.GenDomainDataID(request.Name)
	}
	// build kuscia domain DataSource
	Labels := make(map[string]string)
	Labels[common.LabelDomainDataSourceType] = request.Type
	kusciaDomainDataSource := &v1alpha1.DomainDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:   dsID,
			Labels: Labels,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			URI:  uri,
			Data: data,
			Type: request.Type,
			Name: dsID,
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
	// get kuscia domain datasource
	kusciaDomainDataSource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Get(ctx, request.DatasourceId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("QueryDomainData failed, error:%s", err.Error())
		return &datamesh.QueryDomainDataSourceResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrQueryDomainDataSource, err.Error()),
		}
	}
	// build domain response
	return &datamesh.QueryDomainDataSourceResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &datamesh.DomainDataSource{
			DatasourceId: kusciaDomainDataSource.Name,
			Name:         kusciaDomainDataSource.Spec.Name,
			Type:         kusciaDomainDataSource.Spec.Type,
			Status:       "Available",
			Info: &datamesh.DataSourceInfo{
				Localfs: &datamesh.LocalDataSourceInfo{
					Path: kusciaDomainDataSource.Spec.URI,
				},
			},
		},
	}
}

func (s domainDataSourceService) UpdateDomainDataSource(ctx context.Context, request *datamesh.UpdateDomainDataSourceRequest) *datamesh.UpdateDomainDataSourceResponse {
	// todo
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

func parseDataSource(sourceType string, info *datamesh.DataSourceInfo) (uri string, infoJSON []byte, err error) {
	if info == nil {
		return "", nil, errors.New("info is nil")
	}
	switch sourceType {
	case DomainDataSourceTypeOSS:
		uri = info.Oss.Bucket + "/" + info.Oss.Prefix
	case DomainDataSourceTypeLocalFS:
		uri = info.Localfs.Path
	default:
		errMsg := fmt.Sprintf("Datasource type:%s not support, only support [localfs,oss]", sourceType)
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
