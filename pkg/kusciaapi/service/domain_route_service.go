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
	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type IDomainRouteService interface {
	CreateDomainRoute(ctx context.Context, request *kusciaapi.CreateDomainRouteRequest) *kusciaapi.CreateDomainRouteResponse
	DeleteDomainRoute(ctx context.Context, request *kusciaapi.DeleteDomainRouteRequest) *kusciaapi.DeleteDomainRouteResponse
	QueryDomainRoute(ctx context.Context, request *kusciaapi.QueryDomainRouteRequest) *kusciaapi.QueryDomainRouteResponse
	BatchQueryDomainRouteStatus(ctx context.Context, request *kusciaapi.BatchQueryDomainRouteStatusRequest) *kusciaapi.BatchQueryDomainRouteStatusResponse
}

type domainRouteService struct {
	kusciaClient kusciaclientset.Interface
}

func NewDomainRouteService(config config.KusciaAPIConfig) IDomainRouteService {
	return &domainRouteService{
		kusciaClient: config.KusciaClient,
	}
}

func (s domainRouteService) CreateDomainRoute(ctx context.Context, request *kusciaapi.CreateDomainRouteRequest) *kusciaapi.CreateDomainRouteResponse {
	// do validate
	source := request.Source
	if source == "" {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "source can not be empty"),
		}
	}
	destination := request.Destination
	if destination == "" {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "destination can not be empty"),
		}
	}
	endpoint := request.Endpoint
	if endpoint == nil || len(endpoint.Ports) == 0 {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "endpoint can not be empty"),
		}
	}
	authenticationType := request.AuthenticationType
	if authenticationType == "" {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "authentication type can not be empty"),
		}
	}
	// build cdr kusciaAPIDomainRoute endpoint
	cdrEndpoint := v1alpha1.DomainEndpoint{
		Host: endpoint.Host,
	}
	cdrEndpoint.Ports = make([]v1alpha1.DomainPort, len(endpoint.Ports))
	for i, port := range endpoint.Ports {
		cdrEndpoint.Ports[i] = v1alpha1.DomainPort{
			Name:     port.Name,
			Port:     int(port.Port),
			Protocol: v1alpha1.DomainRouteProtocolType(port.Protocol),
		}
	}
	// build cdr token config or mtls config
	var cdrTokenConfig *v1alpha1.TokenConfig
	var cdrMtlsConfig *v1alpha1.DomainRouteMTLSConfig
	var cdrAuthenticationType v1alpha1.DomainAuthenticationType
	switch authenticationType {
	case string(v1alpha1.DomainAuthenticationToken):
		cdrAuthenticationType = v1alpha1.DomainAuthenticationToken
		// build cdr token config
		tokenConfig := request.TokenConfig
		cdrTokenConfig = &v1alpha1.TokenConfig{
			SourcePublicKey:      tokenConfig.SourcePublicKey,
			DestinationPublicKey: tokenConfig.DestinationPublicKey,
			TokenGenMethod:       v1alpha1.TokenGenMethodType(tokenConfig.TokenGenMethod),
		}
	case string(v1alpha1.DomainAuthenticationMTLS):
		cdrAuthenticationType = v1alpha1.DomainAuthenticationMTLS
		// build cdr mtls config
		mtlsConfig := request.MtlsConfig
		cdrMtlsConfig = &v1alpha1.DomainRouteMTLSConfig{
			TLSCA:                  mtlsConfig.TlsCa,
			SourceClientPrivateKey: mtlsConfig.SourceClientPrivateKey,
			SourceClientCert:       mtlsConfig.SourceClientCert,
		}
	case string(v1alpha1.DomainAuthenticationNone):
		cdrAuthenticationType = v1alpha1.DomainAuthenticationNone
	}
	// build cdr
	clusterDomainRoute := &v1alpha1.ClusterDomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildRouteName(source, destination),
		},
		Spec: v1alpha1.ClusterDomainRouteSpec{
			DomainRouteSpec: v1alpha1.DomainRouteSpec{
				Source:             source,
				Destination:        destination,
				Endpoint:           cdrEndpoint,
				AuthenticationType: cdrAuthenticationType,
				TokenConfig:        cdrTokenConfig,
				MTLSConfig:         cdrMtlsConfig,
			},
		},
	}
	// create cdr
	_, err := s.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(ctx, clusterDomainRoute, metav1.CreateOptions{})
	if err != nil {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainRouteErrorCode(err, errorcode.ErrCreateDomainRoute), err.Error()),
		}
	}
	return &kusciaapi.CreateDomainRouteResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainRouteService) DeleteDomainRoute(ctx context.Context, request *kusciaapi.DeleteDomainRouteRequest) *kusciaapi.DeleteDomainRouteResponse {
	// do validate
	source := request.Source
	if source == "" {
		return &kusciaapi.DeleteDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "source can not be empty"),
		}
	}
	destination := request.Destination
	if destination == "" {
		return &kusciaapi.DeleteDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "destination can not be empty"),
		}
	}
	// delete cluster domain kusciaAPIDomainRoute
	name := buildRouteName(source, destination)
	err := s.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return &kusciaapi.DeleteDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainRouteErrorCode(err, errorcode.ErrDeleteDomainRoute), err.Error()),
		}
	}
	return &kusciaapi.DeleteDomainRouteResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s domainRouteService) QueryDomainRoute(ctx context.Context, request *kusciaapi.QueryDomainRouteRequest) *kusciaapi.QueryDomainRouteResponse {
	// do validate
	source := request.Source
	if source == "" {
		return &kusciaapi.QueryDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "source can not be empty"),
		}
	}
	destination := request.Destination
	if destination == "" {
		return &kusciaapi.QueryDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "destination can not be empty"),
		}
	}
	// get cdr from k8s
	name := buildRouteName(source, destination)
	cdr, err := s.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.QueryDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainRouteErrorCode(err, errorcode.ErrQueryDomainRoute), err.Error()),
		}
	}
	cdrSpec := cdr.Spec
	// build kusciaAPIDomainRoute endpoint
	cdrEndpoint := cdrSpec.Endpoint
	routePorts := make([]*kusciaapi.EndpointPort, len(cdrEndpoint.Ports))
	for i, port := range cdrEndpoint.Ports {
		routePorts[i] = &kusciaapi.EndpointPort{
			Name:     port.Name,
			Port:     int32(port.Port),
			Protocol: string(port.Protocol),
		}
	}
	routeEndpoint := &kusciaapi.RouteEndpoint{
		Host:  cdrEndpoint.Host,
		Ports: routePorts,
	}
	var routeTokenConfig *kusciaapi.TokenConfig
	var routeMtlsConfig *kusciaapi.MTLSConfig
	// build kusciaAPIDomainRoute token config
	switch cdrSpec.AuthenticationType {
	case v1alpha1.DomainAuthenticationToken:
		cdrTokenConfig := cdrSpec.TokenConfig
		routeTokenConfig = &kusciaapi.TokenConfig{
			DestinationPublicKey: cdrTokenConfig.DestinationPublicKey,
			RollingUpdatePeriod:  int64(cdrTokenConfig.RollingUpdatePeriod),
			SourcePublicKey:      cdrTokenConfig.SourcePublicKey,
			TokenGenMethod:       string(cdrTokenConfig.TokenGenMethod),
		}
	case v1alpha1.DomainAuthenticationMTLS:
		cdrMtlsConfig := cdrSpec.MTLSConfig
		routeMtlsConfig = &kusciaapi.MTLSConfig{
			TlsCa:                  cdrMtlsConfig.TLSCA,
			SourceClientPrivateKey: cdrMtlsConfig.SourceClientPrivateKey,
			SourceClientCert:       cdrMtlsConfig.SourceClientCert,
		}
	case v1alpha1.DomainAuthenticationNone:
	}
	// build kusciaAPIDomainRoute mtls config
	return &kusciaapi.QueryDomainRouteResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.QueryDomainRouteResponseData{
			Name:               name,
			AuthenticationType: string(cdrSpec.AuthenticationType),
			Source:             source,
			Endpoint:           routeEndpoint,
			Destination:        destination,
			TokenConfig:        routeTokenConfig,
			MtlsConfig:         routeMtlsConfig,
			Status:             buildRouteStatus(cdr),
		},
	}
}

func (s domainRouteService) BatchQueryDomainRouteStatus(ctx context.Context, request *kusciaapi.BatchQueryDomainRouteStatusRequest) *kusciaapi.BatchQueryDomainRouteStatusResponse {
	// do validate
	routeKeys := request.RouteKeys
	if len(routeKeys) == 0 {
		return &kusciaapi.BatchQueryDomainRouteStatusResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "kusciaAPIDomainRoute keys can not be empty"),
		}
	}
	for i, key := range routeKeys {
		if key.Source == "" {
			return &kusciaapi.BatchQueryDomainRouteStatusResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("source can not be empty on index %d", i)),
			}
		}
	}
	// build kusciaAPIDomainRoute statuses
	routeStatuses := make([]*kusciaapi.DomainRouteStatus, len(routeKeys))
	for i, key := range routeKeys {
		name := buildRouteName(key.Source, key.Destination)
		// get cdr from lister
		cdr, err := s.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return &kusciaapi.BatchQueryDomainRouteStatusResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainRouteErrorCode(err, errorcode.ErrQueryDomainRouteStatus), err.Error()),
			}
		}
		routeStatuses[i] = &kusciaapi.DomainRouteStatus{
			Name:        name,
			Source:      key.Source,
			Destination: key.Destination,
			Status:      buildRouteStatus(cdr),
		}
	}
	return &kusciaapi.BatchQueryDomainRouteStatusResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.BatchQueryDomainRouteStatusResponseData{
			Routes: routeStatuses,
		},
	}
}

func buildRouteStatus(cdr *v1alpha1.ClusterDomainRoute) *kusciaapi.RouteStatus {
	cdrTokenStatus := cdr.Status.TokenStatus
	status := constants.RouteFailed
	if len(cdrTokenStatus.DestinationTokens) > 0 && len(cdrTokenStatus.SourceTokens) > 0 {
		status = constants.RouteSucceeded
	}
	return &kusciaapi.RouteStatus{
		Status: status,
	}
}

func buildRouteName(source, destination string) string {
	return fmt.Sprintf("%s-%s", source, destination)
}
