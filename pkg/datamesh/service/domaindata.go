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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/utils"
	pbv1alpha1 "github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

type IDomainDataService interface {
	CreateDomainData(ctx context.Context, request *datamesh.CreateDomainDataRequest) *datamesh.CreateDomainDataResponse
	QueryDomainData(ctx context.Context, request *datamesh.QueryDomainDataRequest) *datamesh.QueryDomainDataResponse
	UpdateDomainData(ctx context.Context, request *datamesh.UpdateDomainDataRequest) *datamesh.UpdateDomainDataResponse
	DeleteDomainData(ctx context.Context, request *datamesh.DeleteDomainDataRequest) *datamesh.DeleteDomainDataResponse
}

type domainDataService struct {
	conf *config.DataMeshConfig
}

func NewDomainDataService(config *config.DataMeshConfig) IDomainDataService {
	return &domainDataService{
		conf: config,
	}
}

func (s domainDataService) CreateDomainData(ctx context.Context, request *datamesh.CreateDomainDataRequest) *datamesh.CreateDomainDataResponse {
	// check whether domainData  is existed
	if request.DomaindataId != "" {
		// do k8s validate
		if err := resources.ValidateK8sName(request.DomaindataId, "domaindata_id"); err != nil {
			return &datamesh.CreateDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrRequestInvalidate, err.Error()),
			}
		}
		domainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(s.conf.KubeNamespace).Get(ctx, request.DomaindataId, metav1.GetOptions{})
		if err == nil && domainData != nil {
			// update domainData
			resp := s.UpdateDomainData(ctx, convert2UpdateReq(request))
			return convert2CreateResp(resp, request.DomaindataId)
		}
	}

	// normalization request
	s.normalizationCreateRequest(request)

	// check datasource
	datasource, err := s.checkDataSource(ctx, request.DatasourceId, request.Columns)
	if err != nil {
		return &datamesh.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrGetDomainDataSourceFromKubeFailed, err.Error()),
		}
	}
	if !isFSDataSource(datasource.Spec.Type) {
		request.FileFormat = pbv1alpha1.FileFormat_UNKNOWN
	}

	// build kuscia domain
	Labels := make(map[string]string)
	Labels[common.LabelDomainDataType] = request.Type
	Labels[common.LabelDomainDataVendor] = request.Vendor
	Labels[common.LabelInterConnProtocolType] = "kuscia"

	annotations := make(map[string]string)
	annotations[common.InitiatorAnnotationKey] = request.DomaindataId

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
			Vendor:      request.Vendor,
			Author:      s.conf.KubeNamespace,
			FileFormat:  common.Convert2KubeFileFormat(request.FileFormat),
		},
	}

	// create kuscia domain
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(s.conf.KubeNamespace).Create(ctx, kusciaDomainData, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainData failed, error: %s", err.Error())
		return &datamesh.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrCreateDomainData, err.Error()),
		}
	}
	return &datamesh.CreateDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &datamesh.CreateDomainDataResponseData{
			DomaindataId: request.DomaindataId,
		},
	}
}

func (s domainDataService) QueryDomainData(ctx context.Context, request *datamesh.QueryDomainDataRequest) *datamesh.QueryDomainDataResponse {
	// get kuscia domain
	kusciaDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(s.conf.KubeNamespace).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("QueryDomainData failed, error: %s", err.Error())
		return &datamesh.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrQueryDomainData, err.Error()),
		}
	}
	// build domain response
	return &datamesh.QueryDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &datamesh.DomainData{
			DomaindataId: kusciaDomainData.Name,
			Name:         kusciaDomainData.Spec.Name,
			Type:         kusciaDomainData.Spec.Type,
			RelativeUri:  kusciaDomainData.Spec.RelativeURI,
			DatasourceId: kusciaDomainData.Spec.DataSource,
			Attributes:   kusciaDomainData.Spec.Attributes,
			Partition:    common.Convert2PbPartition(kusciaDomainData.Spec.Partition),
			Columns:      common.Convert2PbColumn(kusciaDomainData.Spec.Columns),
			Vendor:       kusciaDomainData.Spec.Vendor,
			FileFormat:   common.Convert2PbFileFormat(kusciaDomainData.Spec.FileFormat),
			Author:       kusciaDomainData.Spec.Author,
		},
	}
}

func (s domainDataService) UpdateDomainData(ctx context.Context, request *datamesh.UpdateDomainDataRequest) *datamesh.UpdateDomainDataResponse {
	// get original domainData from k8s
	originalDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(s.conf.KubeNamespace).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("UpdateDomainData failed, error: %s", err.Error())
		return &datamesh.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrGetDomainDataFromKubeFailed, err.Error()),
		}
	}

	// normalize request
	s.normalizationUpdateRequest(request, originalDomainData.Spec)

	// check DataSource
	if request.DatasourceId != originalDomainData.Spec.DataSource {
		datasource, err := s.checkDataSource(ctx, request.DatasourceId, request.Columns)
		if err != nil {
			nlog.Errorf("Query DataSource %s of DomainData %s fail: %v", request.DatasourceId, request.DomaindataId, err)
			return &datamesh.UpdateDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrGetDomainDataSourceFromKubeFailed, err.Error()),
			}
		}
		if !isFSDataSource(datasource.Spec.Type) {
			request.FileFormat = pbv1alpha1.FileFormat_UNKNOWN
		}
	}

	// build modified domainData
	labels := make(map[string]string)
	for key, value := range originalDomainData.Labels {
		labels[key] = value
	}
	labels[common.LabelDomainDataType] = request.Type
	labels[common.LabelDomainDataVendor] = request.Vendor

	// build modified domainData
	modifiedDomainData := &v1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:            request.DomaindataId,
			ResourceVersion: originalDomainData.ResourceVersion,
			Labels:          labels,
		},
		Spec: v1alpha1.DomainDataSpec{
			RelativeURI: request.RelativeUri,
			Name:        request.Name,
			Type:        request.Type,
			DataSource:  request.DatasourceId,
			Attributes:  request.Attributes,
			Partition:   common.Convert2KubePartition(request.Partition),
			Columns:     common.Convert2KubeColumn(request.Columns),
			Vendor:      request.Vendor,
			FileFormat:  common.Convert2KubeFileFormat(request.FileFormat),
			Author:      s.conf.KubeNamespace,
		},
	}
	// merge modifiedDomainData to originalDomainData
	patchBytes, originalBytes, modifiedBytes, err := MergeDomainData(originalDomainData, modifiedDomainData)
	if err != nil {
		nlog.Errorf("Merge DomainData failed, request: %+v,error: %s.",
			request, err.Error())
		return &datamesh.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrMergeDomainDataFailed, err.Error()),
		}
	}
	nlog.Debugf("Update DomainData request: %+v, patchBytes: %s, originalDomainData: %s, modifiedDomainData: %s",
		request, patchBytes, originalBytes, modifiedBytes)
	// patch the merged domainData
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(originalDomainData.Namespace).Patch(ctx, originalDomainData.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		// todo: retry if conflict
		nlog.Debugf("Patch DomainData failed, request: %+v, patchBytes: %s, originalDomainData: %s, modifiedDomainData: %s, error: %s",
			request, patchBytes, originalBytes, modifiedBytes, err.Error())
		return &datamesh.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrPatchDomainDataFailed, err.Error()),
		}
	}
	// construct the response
	return &datamesh.UpdateDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataService) DeleteDomainData(ctx context.Context, request *datamesh.DeleteDomainDataRequest) *datamesh.DeleteDomainDataResponse {
	// record the delete operation
	nlog.Warnf("Delete domainDataID %s", request.DomaindataId)
	// delete kuscia domainData
	err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(s.conf.KubeNamespace).Delete(ctx, request.DomaindataId, metav1.DeleteOptions{})
	if err != nil {
		nlog.Errorf("Delete domainData: %s failed, detail: %s", request.DomaindataId, err.Error())
		return &datamesh.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_DataMeshErrDeleteDomainDataFailed, err.Error()),
		}
	}
	return &datamesh.DeleteDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataService) normalizationCreateRequest(request *datamesh.CreateDomainDataRequest) {
	// normalization domaindata name
	if request.Name == "" {
		uris := strings.Split(request.RelativeUri, DomainDataURIDelimiter)
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
	if request.Vendor == "" {
		request.Vendor = common.DefaultDomainDataVendor
	}
}

func (s domainDataService) normalizationUpdateRequest(request *datamesh.UpdateDomainDataRequest, data v1alpha1.DomainDataSpec) {
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

func (s domainDataService) checkDataSource(ctx context.Context, dsID string,
	cols []*pbv1alpha1.DataColumn) (*v1alpha1.DomainDataSource, error) {
	datasource, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(s.conf.KubeNamespace).Get(ctx,
		dsID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("query DataSource %s of DomainData fail: %v", dsID, err)
	}

	if !datasource.Spec.AccessDirectly {
		return datasource, CheckColType(cols, datasource.Spec.Type)
	}
	return datasource, nil
}

func convert2UpdateReq(createReq *datamesh.CreateDomainDataRequest) (updateReq *datamesh.UpdateDomainDataRequest) {
	updateReq = &datamesh.UpdateDomainDataRequest{
		Header:       createReq.Header,
		DomaindataId: createReq.DomaindataId,
		Name:         createReq.Name,
		Type:         createReq.Type,
		RelativeUri:  createReq.RelativeUri,
		DatasourceId: createReq.DatasourceId,
		Attributes:   createReq.Attributes,
		Partition:    createReq.Partition,
		Columns:      createReq.Columns,
		Vendor:       createReq.Vendor,
		FileFormat:   createReq.FileFormat,
	}
	return
}

func convert2CreateResp(updateResp *datamesh.UpdateDomainDataResponse, domainDataID string) (createResp *datamesh.CreateDomainDataResponse) {
	createResp = &datamesh.CreateDomainDataResponse{
		Status: updateResp.Status,
		Data: &datamesh.CreateDomainDataResponseData{
			DomaindataId: domainDataID,
		},
	}
	return
}

func CheckColType(cols []*pbv1alpha1.DataColumn, dsType string) error {
	for _, col := range cols {
		col.Type = strings.ToLower(col.Type)
		if dsType == common.DomainDataSourceTypeMysql {
			switch col.Type {
			case "int64":
			case "float64":
			case "bool":
			case "string":
				return nil
			default:
				err := fmt.Errorf("Col[%s].Type=%s is invalid for mysql", col.Name, col.Type)
				nlog.Error(err)
				return err
			}

			return nil
		}

		arrowType := common.Convert2ArrowColumnType(col.Type)
		if arrowType == nil {
			err := fmt.Errorf("Col[%s].Type=%s is invalid for DataProxy", col.Name, col.Type)
			nlog.Error(err)
			return err
		}
	}
	return nil
}
