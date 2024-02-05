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
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	consts "github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"

	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	apiutils "github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/constants"
	weberrorcode "github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type IDomainService interface {
	CreateDomain(ctx context.Context, request *kusciaapi.CreateDomainRequest) *kusciaapi.CreateDomainResponse
	QueryDomain(ctx context.Context, request *kusciaapi.QueryDomainRequest) *kusciaapi.QueryDomainResponse
	UpdateDomain(ctx context.Context, request *kusciaapi.UpdateDomainRequest) *kusciaapi.UpdateDomainResponse
	DeleteDomain(ctx context.Context, request *kusciaapi.DeleteDomainRequest) *kusciaapi.DeleteDomainResponse
	BatchQueryDomain(ctx context.Context, request *kusciaapi.BatchQueryDomainRequest) *kusciaapi.BatchQueryDomainResponse
}

type domainService struct {
	kusciaClient kusciaclientset.Interface
	conf         *config.KusciaAPIConfig
}

func NewDomainService(config *config.KusciaAPIConfig) IDomainService {
	switch config.RunMode {
	case common.RunModeLite:
		return &domainServiceLite{
			kusciaAPIClient: proxy.NewKusciaAPIClient(""),
		}
	default:
		return &domainService{
			kusciaClient: config.KusciaClient,
			conf:         config,
		}
	}

}

func (s domainService) CreateDomain(ctx context.Context, request *kusciaapi.CreateDomainRequest) *kusciaapi.CreateDomainResponse {
	// do validate
	domainID := request.DomainId
	if domainID == "" {
		return &kusciaapi.CreateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	// do k8s validate
	if err := resources.ValidateK8sName(domainID, "doamin_id"); err != nil {
		return &kusciaapi.CreateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}

	role := request.Role
	if role != "" && role != string(v1alpha1.Partner) {
		return &kusciaapi.CreateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("role is invalid, must be empty or %s", v1alpha1.Partner)),
		}
	}
	// 1. role is empty, domain to create is located in local party
	// 2. role is partner, domain to create is located in remote party
	if request.MasterDomainId != "" && request.MasterDomainId != domainID {
		domain, err := s.kusciaClient.KusciaV1alpha1().Domains().Get(ctx, request.MasterDomainId, metav1.GetOptions{})
		if err != nil {
			return &kusciaapi.CreateDomainResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("check master domain id failed with err %v", err.Error())),
			}
		}
		if domain.Spec.Role != v1alpha1.DomainRole(request.Role) {
			return &kusciaapi.CreateDomainResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("master domain role is %s, but current role is %s", domain.Spec.Role, request.Role)),
			}
		}
	}
	// build kuscia domain
	kusciaDomain := &v1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainID,
		},
		Spec: v1alpha1.DomainSpec{
			Cert:         request.Cert,
			Role:         v1alpha1.DomainRole(request.Role),
			AuthCenter:   authCenterConverter(request.AuthCenter),
			MasterDomain: request.MasterDomainId,
		},
	}

	// currently just support kuscia protocol
	if role == string(v1alpha1.Partner) {
		kusciaDomain.Spec.InterConnProtocols = []v1alpha1.InterConnProtocolType{
			v1alpha1.InterConnKuscia,
		}
	}

	// create kuscia domain
	_, err := s.kusciaClient.KusciaV1alpha1().Domains().Create(ctx, kusciaDomain, metav1.CreateOptions{})
	if err != nil {
		return &kusciaapi.CreateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrCreateDomain), err.Error()),
		}
	}
	return &kusciaapi.CreateDomainResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainService) QueryDomain(ctx context.Context, request *kusciaapi.QueryDomainRequest) *kusciaapi.QueryDomainResponse {
	// do validate
	domainID := request.DomainId
	if domainID == "" {
		return &kusciaapi.QueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	if request.GetDomainId() == consts.KusciaMasterDomain {
		return s.queryKusciaMasterDomain()
	}
	// Auth Handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.QueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}
	// get kuscia domain
	kusciaDomain, err := s.kusciaClient.KusciaV1alpha1().Domains().Get(ctx, domainID, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.QueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrQueryDomain), err.Error()),
		}
	}
	// build domain response
	domainStatus, deployToken := s.buildDomain(kusciaDomain)
	// build authCenter
	var authCenter *kusciaapi.AuthCenter
	kusciaAuthCenter := kusciaDomain.Spec.AuthCenter
	if kusciaAuthCenter != nil {
		authCenter = &kusciaapi.AuthCenter{
			AuthenticationType: string(kusciaAuthCenter.AuthenticationType),
			TokenGenMethod:     string(kusciaAuthCenter.TokenGenMethod),
		}
	}
	return &kusciaapi.QueryDomainResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.QueryDomainResponseData{
			DomainId:            domainID,
			Cert:                kusciaDomain.Spec.Cert,
			Role:                string(kusciaDomain.Spec.Role),
			NodeStatuses:        domainStatus.NodeStatuses,
			DeployTokenStatuses: deployToken,
			Annotations:         kusciaDomain.Annotations,
			AuthCenter:          authCenter,
			MasterDomainId:      kusciaDomain.Spec.MasterDomain,
		},
	}
}

func (s domainService) queryKusciaMasterDomain() *kusciaapi.QueryDomainResponse {
	caCert, err := s.queryKusciaMasterCert()
	if err != nil {
		return &kusciaapi.QueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrQueryDomain, err.Error()),
		}
	}
	return &kusciaapi.QueryDomainResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.QueryDomainResponseData{
			DomainId: consts.KusciaMasterDomain,
			Cert:     caCert,
		},
	}
}

func (s domainService) queryKusciaMasterCert() (string, error) {
	if s.conf.RootCA == nil {
		errMsg := fmt.Sprintf("master ca cert is nil")
		nlog.Errorf("Query kuscia-master cert failed, error: %s.", errMsg)
		return "", errors.New(errMsg)
	}
	caCert := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.conf.RootCA.Raw}))
	return caCert, nil
}

func (s domainService) UpdateDomain(ctx context.Context, request *kusciaapi.UpdateDomainRequest) *kusciaapi.UpdateDomainResponse {
	// do validate
	domainID := request.DomainId
	if domainID == "" {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	role := request.Role
	if role != "" && role != string(v1alpha1.Partner) {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("role is invalid, must be empty or %s", v1alpha1.Partner)),
		}
	}
	// Auth Handler
	if err := s.authHandler(ctx, request); err != nil {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}
	// 1. role is empty, domain to create is located in local party
	// 2. role is partner, domain to create is located in remote party
	if request.MasterDomainId != "" && request.MasterDomainId != domainID {
		domain, err := s.kusciaClient.KusciaV1alpha1().Domains().Get(ctx, request.MasterDomainId, metav1.GetOptions{})
		if err != nil {
			return &kusciaapi.UpdateDomainResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("check master domain id failed with err %v", err.Error())),
			}
		}
		if domain.Spec.Role != v1alpha1.DomainRole(request.Role) {
			return &kusciaapi.UpdateDomainResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("master domain role is %s, but current role is %s", domain.Spec.Role, request.Role)),
			}
		}
	}
	// get latest domain from k8s
	latestDomain, err := s.kusciaClient.KusciaV1alpha1().Domains().Get(ctx, domainID, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrUpdateDomain), err.Error()),
		}
	}
	// build kuscia domain
	kusciaDomain := &v1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name:            domainID,
			ResourceVersion: latestDomain.ResourceVersion,
		},
		Spec: v1alpha1.DomainSpec{
			Cert:         request.Cert,
			Role:         v1alpha1.DomainRole(role),
			AuthCenter:   authCenterConverter(request.AuthCenter),
			MasterDomain: request.MasterDomainId,
		},
	}
	// update kuscia domain
	_, err = s.kusciaClient.KusciaV1alpha1().Domains().Update(ctx, kusciaDomain, metav1.UpdateOptions{})
	if err != nil {
		return &kusciaapi.UpdateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrUpdateDomain, err.Error()),
		}
	}
	return &kusciaapi.UpdateDomainResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainService) DeleteDomain(ctx context.Context, request *kusciaapi.DeleteDomainRequest) *kusciaapi.DeleteDomainResponse {
	// do validate
	domainID := request.DomainId
	if domainID == "" {
		return &kusciaapi.DeleteDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain id can not be empty"),
		}
	}
	// delete kuscia domain
	err := s.kusciaClient.KusciaV1alpha1().Domains().Delete(ctx, domainID, metav1.DeleteOptions{})
	if err != nil {
		return &kusciaapi.DeleteDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrDeleteDomain), err.Error()),
		}
	}
	return &kusciaapi.DeleteDomainResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainService) BatchQueryDomain(ctx context.Context, request *kusciaapi.BatchQueryDomainRequest) *kusciaapi.BatchQueryDomainResponse {
	// do validate
	domainIDs := request.DomainIds
	if len(domainIDs) == 0 {
		return &kusciaapi.BatchQueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain ids can not be empty"),
		}
	}
	for i, domainID := range domainIDs {
		if domainID == "" {
			return &kusciaapi.BatchQueryDomainResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("domain id can not be empty on index %d", i)),
			}
		}
	}
	// build domain statuses
	domains := make([]*kusciaapi.Domain, len(domainIDs))
	for i, domainID := range domainIDs {
		if domainID == consts.KusciaMasterDomain {
			caCert, err := s.queryKusciaMasterCert()
			if err != nil {
				return &kusciaapi.BatchQueryDomainResponse{
					Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrQueryDomainStatus), err.Error()),
				}
			}
			domains[i] = &kusciaapi.Domain{
				DomainId: consts.KusciaMasterDomain,
				Cert:     caCert,
			}
			continue
		}
		kusciaDomain, err := s.kusciaClient.KusciaV1alpha1().Domains().Get(ctx, domainID, metav1.GetOptions{})
		if err != nil {
			return &kusciaapi.BatchQueryDomainResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrQueryDomainStatus), err.Error()),
			}
		}
		domains[i], _ = s.buildDomain(kusciaDomain)
	}
	return &kusciaapi.BatchQueryDomainResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.BatchQueryDomainResponseData{
			Domains: domains,
		},
	}
}

func CheckDomainExists(kusciaclient kusciaclientset.Interface, domainId string) (kusciaError weberrorcode.KusciaErrorCode, errorMsg string) {
	_, err := kusciaclient.KusciaV1alpha1().Domains().Get(context.Background(), domainId, metav1.GetOptions{})
	if err != nil {
		return errorcode.GetDomainErrorCode(err, errorcode.ErrQueryDomain), err.Error()
	}
	return utils.ResponseCodeSuccess, ""
}

func (s domainService) buildDomain(kusciaDomain *v1alpha1.Domain) (*kusciaapi.Domain, []*kusciaapi.DeployTokenStatus) {
	domain := &kusciaapi.Domain{}
	if kusciaDomain == nil {
		return domain, nil
	}
	domain.DomainId = kusciaDomain.Name
	domain.Role = string(kusciaDomain.Spec.Role)
	domain.Cert = kusciaDomain.Spec.Cert
	// build node status
	kusciaDomainStatus := kusciaDomain.Status
	if kusciaDomainStatus == nil {
		return domain, nil
	}
	kds := kusciaDomainStatus.NodeStatuses
	nodeStatuses := make([]*kusciaapi.NodeStatus, 0)
	for _, node := range kds {
		nodeStatuses = append(nodeStatuses, &kusciaapi.NodeStatus{
			Name:               node.Name,
			Status:             node.Status,
			Version:            node.Version,
			LastHeartbeatTime:  apiutils.TimeRfc3339String(&node.LastHeartbeatTime),
			LastTransitionTime: apiutils.TimeRfc3339String(&node.LastTransitionTime),
		})
	}
	domain.NodeStatuses = nodeStatuses
	// build deploy token status
	dts := kusciaDomainStatus.DeployTokenStatuses
	tokenStatuses := make([]*kusciaapi.DeployTokenStatus, 0)
	for _, token := range dts {
		tokenStatuses = append(tokenStatuses, &kusciaapi.DeployTokenStatus{
			Token:              token.Token,
			State:              token.State,
			LastTransitionTime: apiutils.TimeRfc3339String(&token.LastTransitionTime),
		})
	}
	return domain, tokenStatuses
}

func authCenterConverter(authCenter *kusciaapi.AuthCenter) *v1alpha1.AuthCenter {
	if authCenter != nil {
		return &v1alpha1.AuthCenter{
			AuthenticationType: v1alpha1.DomainAuthenticationType(authCenter.AuthenticationType),
			TokenGenMethod:     v1alpha1.TokenGenMethodType(authCenter.TokenGenMethod),
		}
	}
	return nil
}

type RequestWithDomainID interface {
	GetDomainId() string
}

func (s domainService) authHandler(ctx context.Context, request RequestWithDomainID) error {
	role, domainId := GetRoleAndDomainFromCtx(ctx)
	if role == constants.AuthRoleDomain {
		if request.GetDomainId() != domainId {
			return fmt.Errorf("domain's kusciaAPI could only operate its own Domain, request.DomainID must be %s not %s", domainId, request.GetDomainId())
		}
	}
	return nil
}
