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

func TestValidateForResourceQuery(t *testing.T) {
	h := NewResourceQueryHandler()
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
			name: "request domain kind is invalid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params:      map[string]string{Kind: "kusciatasks"},
			method:      "GET",
			path:        invalidReqForDomainResourcePath,
			request:     &api.AnyStringProto{},
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "request cluster kind is invalid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params:      map[string]string{Kind: "pods"},
			method:      "GET",
			path:        invalidReqForClusterResourcePath,
			request:     &api.AnyStringProto{},
			errs:        &errorcode.Errs{},
			expectedErr: true,
		},
		{
			name: "request domain kind is valid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params:      map[string]string{Kind: "pods"},
			method:      "GET",
			path:        validReqForDomainResourcePath,
			request:     &api.AnyStringProto{},
			errs:        &errorcode.Errs{},
			expectedErr: false,
		},
		{
			name: "request cluster kind is valid",
			ctx: &api.BizContext{
				Context: &gin.Context{},
			},
			params:      map[string]string{Kind: "kusciatasks"},
			method:      "GET",
			path:        validReqForClusterResourcePath,
			request:     &api.AnyStringProto{},
			errs:        &errorcode.Errs{},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setGinParams(tc.ctx.Context, tc.params)
			setHTTPRequest(tc.ctx.Context, tc.method, tc.path, "")
			h.Validate(tc.ctx, tc.request, tc.errs)
			if tc.expectedErr {
				assert.Equal(t, 1, len(*tc.errs))
			} else {
				assert.Equal(t, 0, len(*tc.errs))
			}
		})
	}
}

func TestHandleForResourceQuery(t *testing.T) {
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
		h           *resourceQueryHandler
		request     *api.AnyStringProto
		expectedErr bool
	}{
		{
			name: "find failed",
			ctx: &api.BizContext{
				ConfBeanRegistry: &mockServiceBean{
					findErr: true,
				},
				Context: &gin.Context{},
			},
			params:      domainResourceParams,
			h:           &resourceQueryHandler{},
			request:     &api.AnyStringProto{},
			expectedErr: true,
		},
		{
			name: "request is successful",
			ctx: &api.BizContext{
				ConfBeanRegistry: &mockServiceBean{},
				Context:          &gin.Context{},
			},
			params:      domainResourceParams,
			h:           &resourceQueryHandler{},
			request:     &api.AnyStringProto{},
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
