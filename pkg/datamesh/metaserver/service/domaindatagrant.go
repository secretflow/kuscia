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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

type IDomainDataGrantService interface {
	CreateDomainDataGrant(ctx context.Context, request *datamesh.CreateDomainDataGrantRequest) *datamesh.CreateDomainDataGrantResponse
	QueryDomainDataGrant(ctx context.Context, request *datamesh.QueryDomainDataGrantRequest) *datamesh.QueryDomainDataGrantResponse
	UpdateDomainDataGrant(ctx context.Context, request *datamesh.UpdateDomainDataGrantRequest) *datamesh.UpdateDomainDataGrantResponse
	DeleteDomainDataGrant(ctx context.Context, request *datamesh.DeleteDomainDataGrantRequest) *datamesh.DeleteDomainDataGrantResponse
}

type domainDataGrantService struct {
	conf *config.DataMeshConfig
}

func NewDomainDataGrantService(config *config.DataMeshConfig) IDomainDataGrantService {
	return &domainDataGrantService{
		conf: config,
	}
}

func (s *domainDataGrantService) CreateDomainDataGrant(ctx context.Context, request *datamesh.CreateDomainDataGrantRequest) *datamesh.CreateDomainDataGrantResponse {

	if validateErr := validateCreateDomainDataGrantRequest(request); validateErr != nil {
		return &datamesh.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, validateErr.Error()),
		}
	}
	if request.GrantDomain == s.conf.KubeNamespace {
		return &datamesh.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "grantdomain cant be self"),
		}
	}

	dd, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(s.conf.KubeNamespace).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		return &datamesh.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, fmt.Sprintf("domaindata [%s] not exists", request.DomaindataId)),
		}
	}
	if request.DomaindatagrantId != "" {
		_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(s.conf.KubeNamespace).Get(ctx, request.DomaindatagrantId, metav1.GetOptions{})
		if err == nil {
			return &datamesh.CreateDomainDataGrantResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_DataMeshErrCreateDomainDataGrant), fmt.Sprintf("CreateDomainDataGrant failed, because domaindatagrant %s is exist", request.DomaindatagrantId)),
			}
		}
	}
	dg := &v1alpha1.DomainDataGrant{}
	dg.Labels = map[string]string{}
	dg.OwnerReferences = append(dg.OwnerReferences, *metav1.NewControllerRef(dd, v1alpha1.SchemeGroupVersion.WithKind("DomainData")))
	err = s.convertData2Spec(&datamesh.DomainDataGrantData{
		DomaindatagrantId: request.DomaindatagrantId,
		Author:            s.conf.KubeNamespace,
		DomaindataId:      request.DomaindataId,
		GrantDomain:       request.GrantDomain,
		Limit:             request.Limit,
		Description:       request.Description,
	}, dg)
	if err != nil {
		nlog.Errorf("CreateDomainDataGrant failed, error:%s", err.Error())
		return &datamesh.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_DataMeshErrCreateDomainDataGrant), err.Error()),
		}
	}

	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(s.conf.KubeNamespace).Create(ctx, dg, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainDataGrant failed, error:%s", err.Error())
		return &datamesh.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_DataMeshErrCreateDomainDataGrant), err.Error()),
		}
	}
	nlog.Infof("Create DomainDataGrant %s/%s", s.conf.KubeNamespace, dg.Name)
	return &datamesh.CreateDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &datamesh.CreateDomainDataGrantResponseData{
			DomaindatagrantId: dg.Name,
		},
	}
}

func (s *domainDataGrantService) QueryDomainDataGrant(ctx context.Context, request *datamesh.QueryDomainDataGrantRequest) *datamesh.QueryDomainDataGrantResponse {
	if request.DomaindatagrantId == "" {
		return &datamesh.QueryDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "domaindatagrantid cant be null"),
		}
	}
	dg, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(s.conf.KubeNamespace).Get(ctx, request.DomaindatagrantId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Query DomainDataGrant failed, error:%s", err.Error())
		return &datamesh.QueryDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrQueryDomainDataGrant, err.Error()),
		}
	}

	data := &datamesh.DomainDataGrantData{}
	s.convertSpec2Data(dg, data)
	return &datamesh.QueryDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   data,
	}
}

func (s *domainDataGrantService) UpdateDomainDataGrant(ctx context.Context, request *datamesh.UpdateDomainDataGrantRequest) *datamesh.UpdateDomainDataGrantResponse {
	if request.DomaindatagrantId == "" {
		return &datamesh.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "domaindatagrantid cant be null"),
		}
	}

	if request.DomaindataId == "" {
		return &datamesh.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "domaindata cant be null"),
		}
	}

	_, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(s.conf.KubeNamespace).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		return &datamesh.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "domaindata cant be found"),
		}
	}

	if request.GrantDomain == "" {
		return &datamesh.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "grantdomain cant be null"),
		}
	}
	if request.GrantDomain == s.conf.KubeNamespace {
		return &datamesh.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "grantdomain cant be self"),
		}
	}
	dg, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(s.conf.KubeNamespace).Get(ctx, request.DomaindatagrantId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Get DomainDataGrant failed, error:%s", err.Error())
		return &datamesh.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_DataMeshErrUpdateDomainDataGrant), err.Error()),
		}
	}

	_ = s.convertData2Spec(&datamesh.DomainDataGrantData{
		DomaindatagrantId: request.DomaindatagrantId,
		Author:            s.conf.KubeNamespace,
		DomaindataId:      request.DomaindataId,
		GrantDomain:       request.GrantDomain,
		Limit:             request.Limit,
		Description:       request.Description,
	}, dg)
	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(s.conf.KubeNamespace).Update(ctx, dg, metav1.UpdateOptions{})
	if err != nil {
		nlog.Errorf("Update DomainDataGrant failed, error:%s", err.Error())
		return &datamesh.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_DataMeshErrUpdateDomainDataGrant), err.Error()),
		}
	}
	nlog.Infof("Update DomainDataGrant %s/%s", s.conf.KubeNamespace, request.DomaindatagrantId)
	return &datamesh.UpdateDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s *domainDataGrantService) DeleteDomainDataGrant(ctx context.Context, request *datamesh.DeleteDomainDataGrantRequest) *datamesh.DeleteDomainDataGrantResponse {
	if request.DomaindatagrantId == "" {
		return &datamesh.DeleteDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrRequestInvalidate, "domaindatagrantid cant be null"),
		}
	}
	nlog.Warnf("Delete domainDataGrantId %s/%s", s.conf.KubeNamespace, request.DomaindatagrantId)
	err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(s.conf.KubeNamespace).Delete(ctx, request.DomaindatagrantId, metav1.DeleteOptions{})
	if err != nil {
		return &datamesh.DeleteDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_DataMeshErrDeleteDomainDataGrant, fmt.Sprintf("Delete domainDataGrantId:%s failed, detail:%s", request.DomaindatagrantId, err.Error())),
		}
	}
	return &datamesh.DeleteDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s *domainDataGrantService) signDomainDataGrant(dg *v1alpha1.DomainDataGrantSpec) error {
	dg.Signature = ""
	bs, err := json.Marshal(dg)
	if err != nil {
		return err
	}
	h := sha256.New()
	h.Write(bs)
	digest := h.Sum(nil)
	sign, err := rsa.SignPKCS1v15(rand.Reader, s.conf.DomainKey, crypto.SHA256, digest)
	if err != nil {
		return err
	}
	dg.Signature = base64.StdEncoding.EncodeToString(sign)
	return nil
}

func (s *domainDataGrantService) convertData2Spec(reqdata *datamesh.DomainDataGrantData, v *v1alpha1.DomainDataGrant) error {
	var limit *v1alpha1.GrantLimit
	if reqdata.Limit != nil {
		grantMode := []v1alpha1.GrantType{"normal"}
		limit = &v1alpha1.GrantLimit{
			FlowID:      reqdata.Limit.FlowId,
			UseCount:    int(reqdata.Limit.UseCount),
			Initiator:   reqdata.Limit.Initiator,
			InputConfig: reqdata.Limit.InputConfig,
			Components:  reqdata.Limit.Components,
			GrantMode:   grantMode,
		}
		if reqdata.Limit.ExpirationTime > 0 {
			mt := metav1.NewTime(time.Unix(reqdata.Limit.ExpirationTime/int64(time.Second), reqdata.Limit.ExpirationTime%int64(time.Second)))
			limit.ExpirationTime = &mt
		}
	}

	dgID := reqdata.DomaindatagrantId
	if dgID == "" {
		dgID = common.GenDomainDataID("domaindatagrant")
	}
	v.Name = dgID
	v.Spec = v1alpha1.DomainDataGrantSpec{
		Author:       reqdata.Author,
		DomainDataID: reqdata.DomaindataId,
		Signature:    reqdata.Signature,
		GrantDomain:  reqdata.GrantDomain,
		Limit:        limit,
		Description:  reqdata.Description,
	}

	return s.signDomainDataGrant(&v.Spec)
}

func (s *domainDataGrantService) convertSpec2Data(v *v1alpha1.DomainDataGrant, domaindata *datamesh.DomainDataGrantData) {
	domaindata.Author = v.Spec.Author
	domaindata.DomaindataId = v.Spec.DomainDataID
	domaindata.DomaindatagrantId = v.Name
	domaindata.GrantDomain = v.Spec.GrantDomain
	domaindata.Signature = v.Spec.Signature
	if v.Spec.Limit != nil {
		domaindata.Limit = &datamesh.GrantLimit{
			Components:  v.Spec.Limit.Components,
			FlowId:      v.Spec.Limit.FlowID,
			UseCount:    int32(v.Spec.Limit.UseCount),
			InputConfig: v.Spec.Limit.InputConfig,
		}
		if v.Spec.Limit.ExpirationTime != nil {
			domaindata.Limit.ExpirationTime = v.Spec.Limit.ExpirationTime.UnixNano()
		}
	}
}

func validateCreateDomainDataGrantRequest(request *datamesh.CreateDomainDataGrantRequest) error {
	if request.GrantDomain == "" {
		return fmt.Errorf("grantdomain cant be null")
	}

	if request.DomaindataId == "" {
		return fmt.Errorf("domaindata cant be null")
	}

	// do k8s validate
	if err := resources.ValidateK8sName(request.DomaindataId, "domaindata_id"); err != nil {
		return err
	}

	if request.GetDomaindatagrantId() != "" {
		return resources.ValidateK8sName(request.GetDomaindatagrantId(), "domaindatagrant_id")
	}
	return nil
}
