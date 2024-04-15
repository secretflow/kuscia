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
	"path"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	cmservice "github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const (
	encryptedInfo = "encryptedInfo"
)

type IDomainDataSourceService interface {
	CreateDefaultDomainDataSource(ctx context.Context) error
	CreateDefaultDataProxyDomainDataSource(ctx context.Context) error
	QueryDomainDataSource(ctx context.Context, request *datamesh.QueryDomainDataSourceRequest) *datamesh.QueryDomainDataSourceResponse
}

type domainDataSourceService struct {
	conf                 *config.DataMeshConfig
	configurationService cmservice.IConfigurationService
}

func NewDomainDataSourceService(config *config.DataMeshConfig, configurationService cmservice.IConfigurationService) IDomainDataSourceService {
	return &domainDataSourceService{
		conf:                 config,
		configurationService: configurationService,
	}
}

func (s domainDataSourceService) CreateDefaultDomainDataSource(ctx context.Context) error {
	nlog.Infof("Create default datasource %s.", common.DefaultDataSourceID)

	kusciaDomainDataSource, err := s.generateDefaultDataSource(common.DefaultDataSourceID)
	if err != nil {
		nlog.Errorf("GenerateDefaultDataSource %s failed, error:%s", common.DefaultDataSourceID, err.Error())
		return err
	}
	// create kuscia domain datasource
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Create(ctx, kusciaDomainDataSource, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainDataSource %s failed, error:%s", common.DefaultDataSourceID, err.Error())
		return err
	}
	return nil
}

func (s domainDataSourceService) CreateDefaultDataProxyDomainDataSource(ctx context.Context) error {
	nlog.Infof("Create default datasource: %s.", common.DefaultDataProxyDataSourceID)
	kusciaDomainDataSource, err := s.generateDefaultDataSource(common.DefaultDataProxyDataSourceID)
	if err != nil {
		nlog.Errorf("GenerateDefaultDataSource %s failed, error:%s", common.DefaultDataProxyDataSourceID, err.Error())
		return err
	}
	kusciaDomainDataSource.Spec.AccessDirectly = false
	// create kuscia domain datasource
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Create(ctx, kusciaDomainDataSource, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainDataSource %s failed, error:%s", common.DefaultDataProxyDataSourceID, err.Error())
		return err
	}
	return nil
}

func (s domainDataSourceService) generateDefaultDataSource(dsID string) (*v1alpha1.DomainDataSource, error) {
	info := &datamesh.DataSourceInfo{
		Localfs: &datamesh.LocalDataSourceInfo{
			Path: path.Join(s.conf.RootDir, common.DefaultDomainDataSourceLocalFSPath),
		},
	}

	// parse DataSource
	uri, encInfo, err := s.encryptInfo(common.DomainDataSourceTypeLocalFS, info)
	if err != nil {
		return nil, err
	}

	// build kuscia domain DataSource
	Labels := make(map[string]string)
	Labels[common.LabelDomainDataSourceType] = common.DomainDataSourceTypeLocalFS
	kusciaDomainDataSource := &v1alpha1.DomainDataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:   dsID,
			Labels: Labels,
		},
		Spec: v1alpha1.DomainDataSourceSpec{
			URI:            uri,
			Type:           common.DomainDataSourceTypeLocalFS,
			Name:           dsID,
			Data:           map[string]string{encryptedInfo: encInfo},
			AccessDirectly: true,
		},
	}
	return kusciaDomainDataSource, nil
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

	if len(kusciaDomainDataSource.Spec.InfoKey) != 0 {
		info, err = s.getDsInfoByKey(ctx, kusciaDomainDataSource.Spec.Type, kusciaDomainDataSource.Spec.InfoKey)
		if err != nil {
			return &datamesh.QueryDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrQueryDomainDataSource, err.Error()),
			}
		}
	} else {
		encryptedInfo, exist := kusciaDomainDataSource.Spec.Data[encryptedInfo]
		if !exist {
			return &datamesh.QueryDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrQueryDomainDataSource, "datasource crd encryptedInfo field is not exist"),
			}
		}
		info, err = s.decryptInfo(encryptedInfo)
		if err != nil {
			return &datamesh.QueryDomainDataSourceResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrQueryDomainDataSource, err.Error()),
			}
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

// nolint:dulp
func (s domainDataSourceService) getDsInfoByKey(ctx context.Context, sourceType string, infoKey string) (*datamesh.DataSourceInfo, error) {
	response := s.configurationService.QueryConfiguration(ctx, &confmanager.QueryConfigurationRequest{
		Ids: []string{infoKey},
	}, s.conf.KubeNamespace)
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
func parseDataSourceURI(sourceType string, info *datamesh.DataSourceInfo) (uri string, err error) {
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

// nolint:dulp
func (s domainDataSourceService) encryptInfo(dataSourceType string, info *datamesh.DataSourceInfo) (uri string, encInfo string, err error) {
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
func (s domainDataSourceService) decryptInfo(cipherInfo string) (*datamesh.DataSourceInfo, error) {
	plaintext, err := tls.DecryptOAEP(s.conf.DomainKey, cipherInfo)
	if err != nil {
		return nil, fmt.Errorf("decrypt data source info failed, %v", err)
	}
	info := &datamesh.DataSourceInfo{}
	if err = json.Unmarshal(plaintext, info); err != nil {
		return nil, fmt.Errorf("unmarshal data source info failed, %v", err)
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

func isFSDataSource(dsType string) bool {
	return dsType == common.DomainDataSourceTypeLocalFS || dsType == common.DomainDataSourceTypeOSS
}
