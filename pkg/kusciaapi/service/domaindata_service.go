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
	"github.com/secretflow/kuscia/pkg/utils/resources"
	consts "github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	pbv1alpha1 "github.com/secretflow/kuscia/proto/api/v1alpha1"
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

type domainDataService struct {
	conf *config.KusciaAPIConfig
}

func NewDomainDataService(config *config.KusciaAPIConfig) IDomainDataService {
	return &domainDataService{
		conf: config,
	}
}

func (s domainDataService) CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) *kusciaapi.CreateDomainDataResponse {
	// do validate
	if request.DomainId == "" {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	// do k8s validate
	if err := resources.ValidateK8sName(request.DomainId, "domain_id"); err != nil {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	if err := s.validateRequestWhenLite(request); err != nil {
		return &kusciaapi.CreateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}

	// check whether domainData  is existed
	if request.DomaindataId != "" {
		// do k8s validate
		if err := resources.ValidateK8sName(request.DomaindataId, "domaindata_id"); err != nil {
			return &kusciaapi.CreateDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}

	// normalization request
	s.normalizationCreateRequest(request)
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
			Status: utils.BuildErrorResponseStatus(errorcode.CreateDomainDataErrorCode(err, errorcode.ErrCreateDomainDataFailed), err.Error()),
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

	if err := s.validateRequestWhenLite(request); err != nil {
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}
	// get original domainData from k8s
	originalDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("UpdateDomainData failed, error: %s", err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, errorcode.ErrGetDomainDataFailed), err.Error()),
		}
	}

	s.normalizationUpdateRequest(request, originalDomainData.Spec)
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
	patchBytes, originalBytes, modifiedBytes, err := common.MergeDomainData(originalDomainData, modifiedDomainData)
	if err != nil {
		nlog.Errorf("Merge DomainData failed, request: %+v,error: %s.",
			request, err.Error())
		return &kusciaapi.UpdateDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrMergeDomainDataFailed, err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataErrorCode(err, errorcode.ErrPatchDomainDataFailed), err.Error()),
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
	if err := s.validateRequestWhenLite(request); err != nil {
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.DeleteDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}
	// record the delete operation
	nlog.Warnf("Delete domainID: %s domainDataID: %s", request.DomainId, request.DomaindataId)
	// delete kuscia domainData
	err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Delete(ctx, request.DomaindataId, metav1.DeleteOptions{})
	if err != nil {
		nlog.Errorf("Delete domainData: %s failed, detail: %s", request.DomaindataId, err.Error())
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
	if err := s.validateRequestWhenLite(request.Data); err != nil {
		return &kusciaapi.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request.Data); err != nil {
		return &kusciaapi.QueryDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}
	// get kuscia domain
	kusciaDomainData, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.Data.DomainId).Get(ctx, request.Data.DomaindataId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("QueryDomainData failed, error: %s", err.Error())
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
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id and domaindata id can not be empty"),
			}
		}
		// check the request when this is kuscia lite api
		if err := s.validateRequestWhenLite(v); err != nil {
			return &kusciaapi.BatchQueryDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
			}
		}
		// auth pre handler
		if err := s.authHandler(ctx, v); err != nil {
			return &kusciaapi.BatchQueryDomainDataResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	if err := s.validateRequestWhenLite(request.Data); err != nil {
		return &kusciaapi.ListDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandler(ctx, request.Data); err != nil {
		return &kusciaapi.ListDomainDataResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
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
	role, domainId := GetRoleAndDomainFromCtx(ctx)
	if role == consts.AuthRoleDomain && request.GetDomainId() != domainId {
		return fmt.Errorf("domain's kusciaAPI could only operate its own DomainData, request.DomainID must be %s not %s", domainId, request.GetDomainId())
	}
	return nil
}

func (s domainDataService) validateRequestWhenLite(request RequestWithDomainID) error {
	if s.conf.RunMode == common.RunModeLite && request.GetDomainId() != s.conf.Initiator {
		return fmt.Errorf("kuscia lite api could only operate it's own domaindata, the domainid of request must be %s, not %s", s.conf.Initiator, request.GetDomainId())
	}
	return nil
}
