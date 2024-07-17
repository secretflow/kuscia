// Copyright 2024 Ant Group Co., Ltd.
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

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/datamesh/metaserver/service"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type CustomAction interface {
	DoActionCreateDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error)
	DoActionQueryDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error)
	DoActionUpdateDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error)
	DoActionDeleteDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error)
	DoActionQueryDomainDataSourceRequest(ctx context.Context, body []byte) (*flight.Result, error)
}

type dmCustomAction struct {
	domainDataService       service.IDomainDataService
	domainDataSourceService service.IDomainDataSourceService
}

func NewCustomActionService(domainDataService service.IDomainDataService, dataSourceService service.IDomainDataSourceService) CustomAction {
	return &dmCustomAction{
		domainDataService:       domainDataService,
		domainDataSourceService: dataSourceService,
	}
}

func (s *dmCustomAction) DoActionCreateDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error) {
	request := &datamesh.CreateDomainDataRequest{}

	if err := proto.Unmarshal(body, request); err != nil {
		return nil, err
	}

	domainDataResp, err := s.createDomainData(ctx, request)
	if err != nil {
		return nil, err
	}

	return utils.PackActionResult(domainDataResp)
}

func (s *dmCustomAction) DoActionQueryDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error) {
	request := &datamesh.QueryDomainDataRequest{}

	if err := proto.Unmarshal(body, request); err != nil {
		return nil, err
	}

	domainDataResp, err := s.getDomainData(ctx, request.DomaindataId)
	if err != nil {
		return nil, err
	}

	return utils.PackActionResult(domainDataResp)
}

func (s *dmCustomAction) DoActionUpdateDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error) {
	request := &datamesh.UpdateDomainDataRequest{}

	if err := proto.Unmarshal(body, request); err != nil {
		return nil, err
	}

	domainDataResp := s.domainDataService.UpdateDomainData(ctx, request)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, common.BuildGrpcErrorf(appStatus, codes.Internal, "update domain data with id(%s) fail", request.DomaindataId)
	}

	return utils.PackActionResult(domainDataResp)
}

func (s *dmCustomAction) DoActionDeleteDomainDataRequest(ctx context.Context, body []byte) (*flight.Result, error) {
	request := &datamesh.DeleteDomainDataRequest{}
	if err := proto.Unmarshal(body, request); err != nil {
		return nil, err
	}

	domainDataResp := s.domainDataService.DeleteDomainData(ctx, request)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, common.BuildGrpcErrorf(appStatus, codes.Internal, "delete domain data with id(%s) fail",
			request.DomaindataId)
	}

	return utils.PackActionResult(domainDataResp)
}

func (s *dmCustomAction) DoActionQueryDomainDataSourceRequest(ctx context.Context, body []byte) (*flight.Result, error) {
	request := &datamesh.QueryDomainDataSourceRequest{}
	domainSourceResp := s.domainDataSourceService.QueryDomainDataSource(ctx, request)
	if domainSourceResp == nil || domainSourceResp.GetStatus() == nil || domainSourceResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainSourceResp.GetStatus() != nil {
			appStatus = domainSourceResp.GetStatus()
		}
		return nil, common.BuildGrpcErrorf(appStatus, codes.Internal, "query datasource with id(%s) fail",
			request.DatasourceId)
	}

	return utils.PackActionResult(domainSourceResp)
}

func (s *dmCustomAction) getDomainData(ctx context.Context, id string) (*datamesh.QueryDomainDataResponse, error) {
	domainDataReq := &datamesh.QueryDomainDataRequest{
		DomaindataId: id,
	}

	domainDataResp := s.domainDataService.QueryDomainData(ctx, domainDataReq)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, common.BuildGrpcErrorf(appStatus, codes.Internal, "query domain data by id(%s) fail", id)
	}
	return domainDataResp, nil
}

func (s *dmCustomAction) createDomainData(ctx context.Context,
	domainDataReq *datamesh.CreateDomainDataRequest) (*datamesh.CreateDomainDataResponse, error) {
	domainDataResp := s.domainDataService.CreateDomainData(ctx, domainDataReq)
	if domainDataResp == nil || domainDataResp.GetStatus() == nil || domainDataResp.GetStatus().GetCode() != 0 {
		var appStatus *v1alpha1.Status
		if domainDataResp.GetStatus() != nil {
			appStatus = domainDataResp.GetStatus()
		}
		return nil, common.BuildGrpcErrorf(appStatus, codes.Internal, "create domain data with id(%s) fail",
			domainDataReq.DomaindataId)
	}
	return domainDataResp, nil
}
