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

package controller

import (
	"reflect"

	"github.com/secretflow/kuscia/pkg/kusciastorage/common"
	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

// resourceDeleteHandler defines the handler info for deleting resource request.
type resourceDeleteHandler struct{}

// NewResourceDeleteHandler returns a resourceDeleteHandler instance.
func NewResourceDeleteHandler() api.ProtoHandler {
	return &resourceDeleteHandler{}
}

// Validate is used to validate request to delete resource.
func (h *resourceDeleteHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	kind, _ := ctx.Params.Get(Kind)
	requestURI := ctx.Request.RequestURI
	err := validateRequestKind(kind, requestURI)
	if err != nil {
		errs.AppendErr(err)
	}
}

// Handle is used to handle request to delete resource.
func (h *resourceDeleteHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	domain, _ := ctx.Params.Get(Domain)
	kind, _ := ctx.Params.Get(Kind)
	kindInstanceName, _ := ctx.Params.Get(KindInstanceName)
	resourceName, _ := ctx.Params.Get(ResourceName)
	resourceNameInDB := kindInstanceName + "/" + resourceName

	resourceServiceBean, _ := ctx.ConfBeanRegistry.GetBeanByName(common.BeanNameForResourceService)
	rs, _ := resourceServiceBean.(service.IService)

	err := rs.Delete(domain, kind, resourceNameInDB)
	if err != nil {
		nlog.Errorf("Delete resource failed: %v", err.Error())
		return buildResponse("", buildStatus(common.ErrorCodeForDeleteResource, err.Error()))
	}

	return buildResponse("", buildStatus(common.ErrorCodeForSuccess, common.GetMsg(common.ErrorCodeForSuccess)))
}

// GetType is used to get request and response type.
func (h *resourceDeleteHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(api.AnyStringProto{}), reflect.TypeOf(api.AnyStringProto{})
}
