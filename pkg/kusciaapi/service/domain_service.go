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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	apiutils "github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type IDomainService interface {
	CreateDomain(ctx context.Context, request *kusciaapi.CreateDomainRequest) *kusciaapi.CreateDomainResponse
	QueryDomain(ctx context.Context, request *kusciaapi.QueryDomainRequest) *kusciaapi.QueryDomainResponse
	UpdateDomain(ctx context.Context, request *kusciaapi.UpdateDomainRequest) *kusciaapi.UpdateDomainResponse
	DeleteDomain(ctx context.Context, request *kusciaapi.DeleteDomainRequest) *kusciaapi.DeleteDomainResponse
	BatchQueryDomainStatus(ctx context.Context, request *kusciaapi.BatchQueryDomainStatusRequest) *kusciaapi.BatchQueryDomainStatusResponse
}

type domainService struct {
	kusciaClient kusciaclientset.Interface
}

func NewDomainService(config config.KusciaAPIConfig) IDomainService {
	return &domainService{
		kusciaClient: config.KusciaClient,
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
	role := request.Role
	if role != "" && role != string(v1alpha1.Partner) {
		return &kusciaapi.CreateDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("role is invalid, must be empty or %s", v1alpha1.Partner)),
		}
	}
	// build kuscia domain
	kusciaDomain := &v1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainID,
		},
		Spec: v1alpha1.DomainSpec{
			Cert:       request.Cert,
			Role:       v1alpha1.DomainRole(request.Role),
			AuthCenter: authCenterConverter(request.AuthCenter),
		},
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
	// get kuscia domain
	kusciaDomain, err := s.kusciaClient.KusciaV1alpha1().Domains().Get(ctx, domainID, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.QueryDomainResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrQueryDomain), err.Error()),
		}
	}
	// build domain response
	domainStatus := s.buildDomainStatus(kusciaDomain)
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
			DeployTokenStatuses: domainStatus.DeployTokenStatuses,
			Annotations:         kusciaDomain.Annotations,
			AuthCenter:          authCenter,
		},
	}
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
			Cert:       request.Cert,
			Role:       v1alpha1.DomainRole(role),
			AuthCenter: authCenterConverter(request.AuthCenter),
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

func (s domainService) BatchQueryDomainStatus(ctx context.Context, request *kusciaapi.BatchQueryDomainStatusRequest) *kusciaapi.BatchQueryDomainStatusResponse {
	// do validate
	domainIDs := request.DomainIds
	if len(domainIDs) == 0 {
		return &kusciaapi.BatchQueryDomainStatusResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "domain ids can not be empty"),
		}
	}
	for i, domainID := range domainIDs {
		if domainID == "" {
			return &kusciaapi.BatchQueryDomainStatusResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("domain id can not be empty on index %d", i)),
			}
		}
	}
	// build domain statuses
	domainStatuses := make([]*kusciaapi.DomainStatus, len(domainIDs))
	for i, domainID := range domainIDs {
		kusciaDomain, err := s.kusciaClient.KusciaV1alpha1().Domains().Get(ctx, domainID, metav1.GetOptions{})
		if err != nil {
			return &kusciaapi.BatchQueryDomainStatusResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainErrorCode(err, errorcode.ErrQueryDomainStatus), err.Error()),
			}
		}
		domainStatuses[i] = s.buildDomainStatus(kusciaDomain)
	}
	return &kusciaapi.BatchQueryDomainStatusResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.BatchQueryDomainStatusResponseData{
			Domains: domainStatuses,
		},
	}
}

func (s domainService) buildDomainStatus(kusciaDomain *v1alpha1.Domain) *kusciaapi.DomainStatus {
	domainStatus := &kusciaapi.DomainStatus{}
	if kusciaDomain == nil {
		return domainStatus
	}
	domainStatus.DomainId = kusciaDomain.Name
	// build node status
	kusciaDomainStatus := kusciaDomain.Status
	if kusciaDomainStatus == nil {
		return domainStatus
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
	domainStatus.NodeStatuses = nodeStatuses
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
	domainStatus.DeployTokenStatuses = tokenStatuses
	return domainStatus
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
