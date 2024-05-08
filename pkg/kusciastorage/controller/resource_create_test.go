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
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"

	"github.com/secretflow/kuscia/pkg/kusciastorage/common"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

func TestValidateForResourceCreate(t *testing.T) {
	h := resourceCreateHandler{reqBodyMaxSize: 1 << 27}
	testCases := []struct {
		name        string
		ctx         *api.BizContext
		params      map[string]string
		method      string
		path        string
		request     *api.AnyStringProto
		errs        *errorcode.Errs
		expectedErr bool
	}{
		{
			name: "request kind is invalid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params: map[string]string{Kind: "kusciatasks"},
			method: "POST",
			path:   invalidReqForDomainResourcePath,
			request: &api.AnyStringProto{
				Content: "test",
			},
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "request body is nil",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params:      map[string]string{"kind": "pods"},
			method:      "POST",
			path:        validReqForDomainResourcePath,
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "normal request body format is invalid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params: map[string]string{Kind: "pods"},
			method: "POST",
			path:   validReqForDomainResourcePath,
			request: &api.AnyStringProto{
				Content: "test",
			},
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "reference request body format is invalid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params: map[string]string{Kind: "pods"},
			method: "POST",
			path:   validRefReqForDomainResourcePath,
			request: &api.AnyStringProto{
				Content: "test",
			},
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "domain kind in reference request body is invalid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params: map[string]string{Kind: "pods"},
			method: "POST",
			path:   validRefReqForDomainResourcePath,
			request: &api.AnyStringProto{
				Content: (&refRequestBody{
					Domain:           "bob-test-b912bea48f6f5630",
					Kind:             "kusciatasks",
					KindInstanceName: "secretflow-cdgeljbv23fid2snn9hg",
					ResourceName:     "test",
				}).String(),
			},
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "cluster kind in reference request body is invalid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params: map[string]string{Kind: "pods"},
			method: "POST",
			path:   validRefReqForDomainResourcePath,
			request: &api.AnyStringProto{
				Content: (&refRequestBody{
					Kind:             "pods",
					KindInstanceName: "secretflow-cdgeljbv23fid2snn9hg",
					ResourceName:     "test",
				}).String(),
			},
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "reference request is valid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params: map[string]string{Kind: "pods"},
			method: "POST",
			path:   validRefReqForDomainResourcePath,
			request: &api.AnyStringProto{
				Content: (&refRequestBody{
					Domain:           "bob-test-b912bea48f6f5630",
					Kind:             "pods",
					KindInstanceName: "secretflow-cdgeljbv23fid2snn9hg",
					ResourceName:     "test",
				}).String(),
			},
			errs:        &errorcode.Errs{},
			expectedErr: false,
		},
		{
			name: "normal request is valid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params: map[string]string{Kind: "pods"},
			method: "POST",
			path:   validReqForDomainResourcePath,
			request: &api.AnyStringProto{
				Content: (&normalRequestBody{
					Content: "test",
				}).String(),
			},
			errs:        &errorcode.Errs{},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setGinParams(tc.ctx.Context, tc.params)
			if tc.request != nil {
				setHTTPRequest(tc.ctx.Context, tc.method, tc.path, tc.request.String())
			} else {
				setHTTPRequest(tc.ctx.Context, tc.method, tc.path, "")
			}

			h.Validate(tc.ctx, tc.request, tc.errs)
			if tc.expectedErr {
				assert.Equal(t, 1, len(*tc.errs))
			} else {
				assert.Equal(t, 0, len(*tc.errs))
			}
		})
	}
}

func TestHandleForResourceCreate(t *testing.T) {
	domainResourceParams := map[string]string{
		Domain:           "alice-test-9d060f86b01acbb3",
		Kind:             "pods",
		KindInstanceName: "secretflow-5655d668fc-g8j8j",
		ResourceName:     "test",
	}

	testCases := []struct {
		name        string
		ctx         *api.BizContext
		params      map[string]string
		h           *resourceCreateHandler
		request     *api.AnyStringProto
		expectedErr bool
	}{
		{
			name: "create resource failed for normal request",
			ctx: &api.BizContext{
				ConfBeanRegistry: &mockServiceBean{
					createErr: true,
				},
				Context: &gin.Context{},
			},
			params: domainResourceParams,
			h: &resourceCreateHandler{
				normalRequestBody: &normalRequestBody{
					Content: "test",
				},
			},
			request: &api.AnyStringProto{
				Content: "test",
			},
			expectedErr: true,
		},
		{
			name: "create resource failed for reference request",
			ctx: &api.BizContext{
				ConfBeanRegistry: &mockServiceBean{
					createForRefResource: true,
				},
				Context: &gin.Context{},
			},
			params: domainResourceParams,
			h: &resourceCreateHandler{
				refRequestBody: &refRequestBody{
					Domain:           "bob-test-b912bea48f6f5630",
					Kind:             "kusciatasks",
					KindInstanceName: "secretflow-cdgeljbv23fid2snn9hg",
					ResourceName:     "test",
				},
			},
			request: &api.AnyStringProto{
				Content: (&refRequestBody{
					Domain:           "bob-test-b912bea48f6f5630",
					Kind:             "kusciatasks",
					KindInstanceName: "secretflow-cdgeljbv23fid2snn9hg",
					ResourceName:     "test",
				}).String(),
			},
			expectedErr: true,
		},
		{
			name: "normal request is successful",
			ctx: &api.BizContext{
				ConfBeanRegistry: &mockServiceBean{},
				Context:          &gin.Context{},
			},
			params: domainResourceParams,
			h: &resourceCreateHandler{
				normalRequestBody: &normalRequestBody{
					Content: "test",
				},
			},
			request: &api.AnyStringProto{
				Content: "test",
			},
			expectedErr: false,
		},
		{
			name: "reference request is successful",
			ctx: &api.BizContext{
				ConfBeanRegistry: &mockServiceBean{},
				Context:          &gin.Context{},
			},
			params: domainResourceParams,
			h: &resourceCreateHandler{
				refRequestBody: &refRequestBody{
					Domain:           "bob-test-b912bea48f6f5630",
					Kind:             "kusciatasks",
					KindInstanceName: "secretflow-cdgeljbv23fid2snn9hg",
					ResourceName:     "test",
				},
			},
			request: &api.AnyStringProto{
				Content: (&refRequestBody{
					Domain:           "bob-test-b912bea48f6f5630",
					Kind:             "kusciatasks",
					KindInstanceName: "secretflow-cdgeljbv23fid2snn9hg",
					ResourceName:     "test",
				}).String(),
			},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setGinParams(tc.ctx.Context, tc.params)
			resp := tc.h.Handle(tc.ctx, tc.request)
			code := gjson.Get(resp.(*api.AnyStringProto).String(), "status.code").Int()
			if tc.expectedErr {
				assert.NotEqual(t, common.ErrorCodeForSuccess, int32(code))
			} else {
				assert.Equal(t, common.ErrorCodeForSuccess, int32(code))
			}
		})
	}
}
