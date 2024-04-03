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

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers/clusterdomainroute"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	consts "github.com/secretflow/kuscia/pkg/web/constants"
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

func NewDomainRouteService(config *config.KusciaAPIConfig) IDomainRouteService {
	switch config.RunMode {
	case common.RunModeLite:
		return &domainRouteServiceLite{
			kusciaAPIClient: proxy.NewKusciaAPIClient(""),
		}
	default:
		return &domainRouteService{
			kusciaClient: config.KusciaClient,
		}
	}
}

func (s domainRouteService) CreateDomainRoute(ctx context.Context, request *kusciaapi.CreateDomainRouteRequest) *kusciaapi.CreateDomainRouteResponse {
	// do validate
	if err := validateCreateDomainRouteRequest(request); err != nil {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandlerViaDestination(ctx, request); err != nil {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}

	// check source domain exists
	if errorCode, errMsg := CheckDomainExists(s.kusciaClient, request.GetSource()); utils.ResponseCodeSuccess != errorCode {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorCode, errMsg),
		}
	}
	// check destination domain exists
	if errorCode, errMsg := CheckDomainExists(s.kusciaClient, request.GetDestination()); utils.ResponseCodeSuccess != errorCode {
		return &kusciaapi.CreateDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorCode, errMsg),
		}
	}

	endpoint := request.Endpoint
	// build cdr kusciaAPIDomainRoute endpoint
	cdrEndpoint := v1alpha1.DomainEndpoint{
		Host: endpoint.Host,
	}
	cdrEndpoint.Ports = make([]v1alpha1.DomainPort, len(endpoint.Ports))
	for i, port := range endpoint.Ports {
		// TODO: Converted `isTLS` is about to be removed
		drProtocol, isTLS, err := convert2DomainRouteProtocol(port.Protocol)
		if err != nil {
			return &kusciaapi.CreateDomainRouteResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainRouteErrorCode(err, errorcode.ErrCreateDomainRoute), err.Error()),
			}
		}
		cdrEndpoint.Ports[i] = v1alpha1.DomainPort{
			Name:     port.Name,
			Port:     int(port.Port),
			Protocol: drProtocol,
			IsTLS:    isTLS || port.IsTLS,
		}
	}
	// build cdr token config or mtls config
	var cdrTokenConfig *v1alpha1.TokenConfig
	var cdrMtlsConfig *v1alpha1.DomainRouteMTLSConfig
	var cdrAuthenticationType v1alpha1.DomainAuthenticationType
	switch request.AuthenticationType {
	case string(v1alpha1.DomainAuthenticationToken):
		cdrAuthenticationType = v1alpha1.DomainAuthenticationToken
		// build cdr token config
		tokenConfig := request.TokenConfig
		if tokenConfig == nil {
			// set default token config
			tokenConfig = &kusciaapi.TokenConfig{
				TokenGenMethod: v1alpha1.TokenGenMethodRSA,
			}
		}
		cdrTokenConfig = &v1alpha1.TokenConfig{
			SourcePublicKey:      tokenConfig.SourcePublicKey,
			DestinationPublicKey: tokenConfig.DestinationPublicKey,
			TokenGenMethod:       v1alpha1.TokenGenMethodType(tokenConfig.TokenGenMethod),
		}
	case string(v1alpha1.DomainAuthenticationMTLS):
		cdrAuthenticationType = v1alpha1.DomainAuthenticationMTLS
		// build cdr mtls config
		mtlsConfig := request.MtlsConfig
		if mtlsConfig != nil {
			cdrMtlsConfig = &v1alpha1.DomainRouteMTLSConfig{
				TLSCA:                  mtlsConfig.TlsCa,
				SourceClientPrivateKey: mtlsConfig.SourceClientPrivateKey,
				SourceClientCert:       mtlsConfig.SourceClientCert,
			}
		}
	case string(v1alpha1.DomainAuthenticationNone):
		cdrAuthenticationType = v1alpha1.DomainAuthenticationNone
	}
	// build cdr
	clusterDomainRoute := &v1alpha1.ClusterDomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name: buildRouteName(request.Source, request.Destination),
		},
		Spec: v1alpha1.ClusterDomainRouteSpec{
			DomainRouteSpec: v1alpha1.DomainRouteSpec{
				Source:             request.Source,
				Destination:        request.Destination,
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
	if err := validateDomainRouteRequest(request); err != nil {
		return &kusciaapi.DeleteDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandlerViaDestination(ctx, request); err != nil {
		return &kusciaapi.DeleteDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}
	// delete cluster domain kusciaAPIDomainRoute
	name := buildRouteName(request.Source, request.Destination)
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
	if err := validateDomainRouteRequest(request); err != nil {
		return &kusciaapi.QueryDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
		}
	}
	// auth pre handler
	if err := s.authHandlerViaDstAndSrc(ctx, request); err != nil {
		return &kusciaapi.QueryDomainRouteResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
		}
	}
	// get cdr from k8s
	name := buildRouteName(request.Source, request.Destination)
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
			Source:             request.Source,
			Endpoint:           routeEndpoint,
			Destination:        request.Destination,
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
			Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "DomainRoute keys can not be empty"),
		}
	}
	for i, key := range routeKeys {
		if err := validateDomainRouteRequest(key); err != nil {
			nlog.Errorf("Validate BatchQueryDomainRouteStatusRequest the index: %d of route key, failed: %s.", i, err.Error())
			return &kusciaapi.BatchQueryDomainRouteStatusResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
			}
		}
		// auth handler
		if err := s.authHandlerViaDstAndSrc(ctx, key); err != nil {
			return &kusciaapi.BatchQueryDomainRouteStatusResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.ErrAuthFailed, err.Error()),
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
	status := constants.RouteFailed

	if clusterdomainroute.IsReady(&cdr.Status) {
		status = constants.RouteSucceeded
	}

	return &kusciaapi.RouteStatus{
		Status: status,
	}
}

func buildRouteName(source, destination string) string {
	return fmt.Sprintf("%s-%s", source, destination)
}

type RequestWithDstAndSrc interface {
	GetDestination() string
	GetSource() string
}

func (s domainRouteService) authHandlerViaDestination(ctx context.Context, request RequestWithDstAndSrc) error {
	role, domainId := GetRoleAndDomainFromCtx(ctx)
	if role == consts.AuthRoleDomain && request.GetDestination() != domainId {
		return fmt.Errorf("domain's kusciaAPI could only operate DomainRoute with destination is itself, request.Destination must be %s not %s", domainId, request.GetDestination())
	}
	return nil
}

func (s domainRouteService) authHandlerViaDstAndSrc(ctx context.Context, request RequestWithDstAndSrc) error {
	role, domainId := GetRoleAndDomainFromCtx(ctx)
	if role == consts.AuthRoleDomain && request.GetDestination() != domainId && request.GetSource() != domainId {
		return fmt.Errorf("domain's kusciaAPI could only query DomainRoute with itself, domain:%s ,destination:%s,source:%s", domainId, request.GetDestination(), request.GetSource())
	}
	return nil
}

func GetRoleAndDomainFromCtx(ctx context.Context) (role, domain string) {
	authRole := ctx.Value(consts.AuthRole)
	if strRole, ok := authRole.(string); ok {
		role = strRole
	}
	sourceDomain := ctx.Value(consts.SourceDomainKey)
	if strDomain, ok := sourceDomain.(string); ok {
		domain = strDomain
	}
	return
}

func validateCreateDomainRouteRequest(request *kusciaapi.CreateDomainRouteRequest) error {
	if request.Source == "" {
		return fmt.Errorf("source can not be empty")
	}
	if request.Destination == "" {
		return fmt.Errorf("destination can not be empty")
	}
	if request.Endpoint == nil || len(request.Endpoint.Ports) == 0 {
		return fmt.Errorf("endpoint can not be empty")
	}
	if request.AuthenticationType == "" {
		return fmt.Errorf("authentication type can not be empty")
	}
	return nil
}

func validateDomainRouteRequest(request RequestWithDstAndSrc) error {
	if request.GetSource() == "" {
		return fmt.Errorf("source can not be empty")
	}
	if request.GetDestination() == "" {
		return fmt.Errorf("destination can not be empty")
	}
	return nil
}

func convert2DomainRouteProtocol(protocol string) (drProtocol v1alpha1.DomainRouteProtocolType, isTLS bool, err error) {
	protocol = strings.ToUpper(protocol)
	isTLS = false
	if protocol == "HTTPS" || protocol == "GRPCS" {
		isTLS = true
		protocol = strings.TrimRight(protocol, "S")
	}

	err = nil
	switch protocol {
	case "HTTP":
		drProtocol = v1alpha1.DomainRouteProtocolHTTP
	case "GRPC":
		drProtocol = v1alpha1.DomainRouteProtocolGRPC
	default:
		err = fmt.Errorf("DomainRoute Port Protocol should be HTTP or HTTPS or GRPC or GRPCS")
	}
	return
}
