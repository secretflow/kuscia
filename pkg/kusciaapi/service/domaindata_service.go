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
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type IDomainDataService interface {
	CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) *kusciaapi.CreateDomainDataResponse
	UpdateDomainData(ctx context.Context, request *kusciaapi.UpdateDomainDataRequest) *kusciaapi.UpdateDomainDataResponse
	DeleteDomainData(ctx context.Context, request *kusciaapi.DeleteDomainDataRequest) *kusciaapi.DeleteDomainDataResponse
	QueryDomainData(ctx context.Context, request *kusciaapi.QueryDomainDataRequest) *kusciaapi.QueryDomainDataResponse
	BatchQueryDomainData(ctx context.Context, request *kusciaapi.BatchQueryDomainDataRequest) *kusciaapi.BatchQueryDomainDataResponse
	ListDomainData(ctx context.Context, request *kusciaapi.ListDomainDataRequest) *kusciaapi.ListDomainDataResponse
}

type DomainDataRequest interface {
	GetDomaindataId() string

	GetName() string

	GetType() string

	GetRelativeUri() string

	GetDomainId() string

	GetDatasourceId() string

	GetAttributes() map[string]string

	GetPartition() *v1alpha1.Partition

	GetColumns() []*v1alpha1.DataColumn

	GetVendor() string
}

type domainDataService struct {
	conf config.KusciaAPIConfig
}

func NewDomainDataService(config config.KusciaAPIConfig) IDomainDataService {
	return &domainDataService{
		conf: config,
	}
}

const (
	LabelDomainDataType   = "kuscia.secretflow/domaindata-type"
	LabelDomainDataVendor = "kuscia.secretflow/domaindata-vendor"
)

func (s domainDataService) CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) *kusciaapi.CreateDomainDataResponse {
	// do validate
	if request.DomainId == "" {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	// check whether domainData  is existed
	if request.DomaindataId != "" {
		domainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
		if err == nil && domainData != nil {
			// update domainData
			resp := s.UpdateDomainData(ctx, convert2UpdateReq(request))
			return convert2CreateResp(resp, request.DomaindataId)
		}
	}
	// normalization request
	s.normalizationCreateRequest(request)
	// build kuscia domain
	Labels := make(map[string]string)
	Labels[LabelDomainDataType] = request.Type
	Labels[LabelDomainDataVendor] = request.Vendor
	kusciaDomainData := &v1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:   request.DomaindataId,
			Labels: Labels,
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
		},
	}
	// create kuscia domain
	_, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Create(ctx, kusciaDomainData, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainData failed, error:%s", err.Error())
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, errorcode.ErrCreateDomainDataFailed), err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id and domaindata id can not be empty"),
		}
	}
	// get original domainData from k8s
	originalDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("UpdateDomainData failed, error:%s", err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrGetDomainDataFailed, err.Error()),
		}
	}
	s.normalizationUpdateRequest(request, originalDomainData.Spec)
	// build modified domainData
	Labels := make(map[string]string)
	Labels[LabelDomainDataType] = request.Type
	Labels[LabelDomainDataVendor] = request.Vendor
	modifiedDomainData := &v1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:            request.DomaindataId,
			Labels:          Labels,
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
			Vendor:      request.Vendor,
		},
	}
	// merge modifiedDomainData to originalDomainData
	patchBytes, originalBytes, modifiedBytes, err := common.MergeDomainData(originalDomainData, modifiedDomainData)
	if err != nil {
		nlog.Errorf("merge DomainData failed,request:%+v,error:%s.",
			request, err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrMergeDomainDataFailed, err.Error()),
		}
	}
	nlog.Debugf("update DomainData request:%+v, patchBytes:%s,originalDomainData:%s,modifiedDomainData:%s",
		request, patchBytes, originalBytes, modifiedBytes)
	// patch the merged domainData
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(originalDomainData.Namespace).Patch(ctx, originalDomainData.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		nlog.Debugf("patch DomainData failed, request:%+v, patchBytes:%s,originalDomainData:%s,modifiedDomainData:%s,error:%s",
			request, patchBytes, originalBytes, modifiedBytes, err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrPatchDomainDataFailed, err.Error()),
		}
	}
	// construct the response
	return &kusciaapi.UpdateDomainDataResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainDataService) DeleteDomainData(ctx context.Context, request *kusciaapi.DeleteDomainDataRequest) *kusciaapi.DeleteDomainDataResponse {
	// do validate
	if request.DomaindataId == "" || request.DomainId == "" {
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id and domaindata id can not be empty"),
		}
	}
	// record the delete operation
	nlog.Warnf("delete domainID:%s domainDataID:%s", request.DomainId, request.DomaindataId)
	// delete kuscia domainData
	err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Delete(ctx, request.DomaindataId, metav1.DeleteOptions{})
	if err != nil {
		nlog.Errorf("delete domainData:%s failed, detail:%s", request.DomaindataId, err.Error())
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, errorcode.ErrDeleteDomainDataFailed), err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id and domaindata id can not be empty"),
		}
	}
	// get kuscia domain
	kusciaDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.Data.DomainId).Get(ctx, request.Data.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("QueryDomainData failed, error:%s", err.Error())
		return &kusciaapi.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, errorcode.ErrGetDomainDataFailed), err.Error()),
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
		},
	}
}

func (s domainDataService) BatchQueryDomainData(ctx context.Context, request *kusciaapi.BatchQueryDomainDataRequest) *kusciaapi.BatchQueryDomainDataResponse {
	// get kuscia domain
	respDatas := make([]*kusciaapi.DomainData, 0, len(request.Data))
	for _, v := range request.Data {
		kusciaDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(v.DomainId).Get(ctx, v.DomaindataId, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			nlog.Errorf("QueryDomainData failed, error:%s", err.Error())
			return &kusciaapi.BatchQueryDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrGetDomainDataFailed, err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
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
		nlog.Errorf("List DomainData failed, error:%s", err.Error())
		return &kusciaapi.ListDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrListDomainDataFailed, err.Error()),
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
		Vendor:       createReq.Vendor,
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
	if request.Vendor == "" {
		request.Vendor = common.DefaultDomainDataVendor
	}
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
}
