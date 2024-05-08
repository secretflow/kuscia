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
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tidwall/gjson"

	"github.com/secretflow/kuscia/pkg/kusciastorage/common"
	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/model"
	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

// resourceCreateHandler defines the handler info for creating resource request.
type resourceCreateHandler struct {
	resourceService service.IService

	domain           string
	kind             string
	kindInstanceName string
	resourceName     string

	reqBodyMaxSize    int
	normalRequestBody *normalRequestBody
	refRequestBody    *refRequestBody
}

// NewResourceCreateHandler returns a resourceCreateHandler instance.
func NewResourceCreateHandler(reqBodyMaxSize int) api.ProtoHandler {
	return &resourceCreateHandler{
		reqBodyMaxSize: reqBodyMaxSize,
	}
}

func (h *resourceCreateHandler) Validate(ctx *api.BizContext, request api.ProtoRequest, errs *errorcode.Errs) {
	kind, _ := ctx.Params.Get(Kind)
	requestURI := ctx.Request.RequestURI

	err := validateRequestKind(kind, requestURI)
	if err != nil {
		errs.AppendErr(err)
		return
	}

	req, ok := request.(*api.AnyStringProto)
	if !ok || req == nil {
		errs.AppendErr(fmt.Errorf("request is invalid"))
		return
	}

	if isReferenceRequest(requestURI) {
		if len(req.String()) > h.reqBodyMaxSize {
			errs.AppendErr(fmt.Errorf(errRequestBodyTooLarge, h.reqBodyMaxSize))
			return
		}

		var refReqBody refRequestBody
		err = json.Unmarshal([]byte(req.String()), &refReqBody)
		if err != nil {
			errs.AppendErr(err)
			return
		}

		if refReqBody.Kind == "" || refReqBody.KindInstanceName == "" || refReqBody.ResourceName == "" {
			errs.AppendErr(fmt.Errorf(errInvalidRequestParams))
			return
		}

		if refReqBody.Domain != "" {
			if ok = validateDomainKind(refReqBody.Kind); !ok {
				errs.AppendErr(fmt.Errorf(fmt.Sprintf(errUnsupportedRefKind, kind, common.DomainKindList)))
				return
			}
		} else {
			if ok = validateClusterKind(refReqBody.Kind); !ok {
				errs.AppendErr(fmt.Errorf(fmt.Sprintf(errUnsupportedRefKind, kind, common.ClusterKindList)))
				return
			}
		}

		h.refRequestBody = &refReqBody
	} else {
		content := gjson.Get(req.String(), "content").String()
		if content == "" {
			errs.AppendErr(fmt.Errorf(errEmptyRequestBody))
			return
		}

		h.normalRequestBody = &normalRequestBody{
			Content: content,
		}
	}
}

// Handle is used to handle request to create resource.
func (h *resourceCreateHandler) Handle(ctx *api.BizContext, request api.ProtoRequest) api.ProtoResponse {
	h.domain, _ = ctx.Params.Get(Domain)
	h.kind, _ = ctx.Params.Get(Kind)
	h.kindInstanceName, _ = ctx.Params.Get(KindInstanceName)
	h.resourceName, _ = ctx.Params.Get(ResourceName)

	resourceServiceBean, _ := ctx.ConfBeanRegistry.GetBeanByName(common.BeanNameForResourceService)
	h.resourceService, _ = resourceServiceBean.(service.IService)

	switch h.refRequestBody != nil {
	case true:
		err := h.createForReferenceRequest()
		if err != nil {
			nlog.Errorf("Create resource failed: %v", err.Error())
			return buildResponse("", buildStatus(common.ErrorCodeForRefRequestCreate, err.Error()))
		}
	default:
		err := h.createForNormalRequest()
		if err != nil {
			nlog.Errorf("Create resource failed: %v", err.Error())
			return buildResponse("", buildStatus(common.ErrorCodeForNormalRequestCreate, err.Error()))
		}
	}

	return buildResponse("", buildStatus(common.ErrorCodeForSuccess, common.GetMsg(common.ErrorCodeForSuccess)))
}

// GetType is used to get request and response type.
func (h *resourceCreateHandler) GetType() (reqType, respType reflect.Type) {
	return reflect.TypeOf(api.AnyStringProto{}), reflect.TypeOf(api.AnyStringProto{})
}

// createForReferenceRequest is used to create resource for reference request.
func (h *resourceCreateHandler) createForReferenceRequest() error {
	resource := &model.Resource{
		Domain: h.domain,
		Kind:   h.kind,
		Name:   h.kindInstanceName + "/" + h.resourceName,
	}

	return h.resourceService.CreateForRefResource(
		h.refRequestBody.Domain,
		h.refRequestBody.Kind,
		h.refRequestBody.KindInstanceName+"/"+h.refRequestBody.ResourceName,
		resource)
}

// createForNormalRequest is used to create resource for normal request.
func (h *resourceCreateHandler) createForNormalRequest() error {
	resource := &model.Resource{
		Domain: h.domain,
		Kind:   h.kind,
		Name:   h.kindInstanceName + "/" + h.resourceName,
	}

	data := &model.Data{Content: h.normalRequestBody.Content}
	return h.resourceService.Create(resource, data)
}
