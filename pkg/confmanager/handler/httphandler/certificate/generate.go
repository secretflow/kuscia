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

package certificate

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

type generateKeyCertsHandler struct {
	certificateService service.ICertificateService
}

func NewGenerateKeyCertsHandler(certificateService service.ICertificateService) api.ProtoHandler {
	return &generateKeyCertsHandler{
		certificateService: certificateService,
	}
}

func (h generateKeyCertsHandler) Validate(context *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	generateRequest, _ := request.(*confmanager.GenerateKeyCertsRequest)
	errs = h.certificateService.ValidateGenerateKeyCertsRequest(context, generateRequest)
}

func (h generateKeyCertsHandler) Handle(context *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	generateRequest, _ := request.(*confmanager.GenerateKeyCertsRequest)
	return h.certificateService.GenerateKeyCerts(context.Context, generateRequest)
}

func (h generateKeyCertsHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(confmanager.GenerateKeyCertsRequest{}), reflect.TypeOf(confmanager.GenerateKeyCertsResponse{})
}
