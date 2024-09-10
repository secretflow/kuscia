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

package grpchandler

import (
	"context"
	"encoding/json"

	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

// certificateHandler GRPC Handler
type certificateHandler struct {
	certificateService service.ICertificateService
	kusciaapi.UnimplementedCertificateServiceServer
}

func NewCertificateHandler(certService service.ICertificateService) kusciaapi.CertificateServiceServer {
	return &certificateHandler{
		certificateService: certService,
	}
}

func (h *certificateHandler) GenerateKeyCerts(ctx context.Context, request *kusciaapi.GenerateKeyCertsRequest) (*kusciaapi.GenerateKeyCertsResponse, error) {
	cmReq := &confmanager.GenerateKeyCertsRequest{}
	kapiResp := &kusciaapi.GenerateKeyCertsResponse{}
	if err := CopyValue(request, cmReq); err != nil {
		return &kusciaapi.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrForUnexpected, err.Error()),
		}, nil
	}
	cmResp := h.certificateService.GenerateKeyCerts(ctx, cmReq)
	if err := CopyValue(cmResp, kapiResp); err != nil {
		return &kusciaapi.GenerateKeyCertsResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrForUnexpected, err.Error()),
		}, nil
	}
	return kapiResp, nil
}

func CopyValue(src interface{}, dst interface{}) error {
	jsonBytes, _ := json.Marshal(src)
	return json.Unmarshal(jsonBytes, dst)
}
