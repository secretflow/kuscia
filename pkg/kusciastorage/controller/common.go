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
	"strings"

	"github.com/secretflow/kuscia/pkg/kusciastorage/common"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
)

const (
	Domain           = "domain"
	Kind             = "kind"
	KindInstanceName = "kind_instance_name"
	ResourceName     = "resource_name"
)

const (
	errUnsupportedKind      = "request kind %v is invalid, supported kind list: %v"
	errUnsupportedRefKind   = "request reference kind %v is invalid, supported kind list: %v"
	errRequestBodyTooLarge  = "request body is larger than the limit size: %d"
	errEmptyRequestBody     = "request body can't be empty"
	errInvalidRequestParams = "request params is invalid"
)

// refRequestBody defines reference request body.
type refRequestBody struct {
	Domain           string `json:"domain"`
	Kind             string `json:"kind"`
	KindInstanceName string `json:"kind_instance_name"`
	ResourceName     string `json:"resource_name"`
}

// String returns reference request body with string format.
func (b *refRequestBody) String() string {
	body, _ := json.Marshal(b)
	return string(body)
}

// normalRequestBody defines normal request body.
type normalRequestBody struct {
	Content string `json:"content"`
}

// String returns normal request body with string format.
func (b *normalRequestBody) String() string {
	body, _ := json.Marshal(b)
	return string(body)
}

// Response defines response for request.
type Response struct {
	Status  *v1alpha1.Status `json:"status"`
	Content string           `json:"content,omitempty"`
}

// String is used to get response with string format.
func (r *Response) String() string {
	resp, _ := json.Marshal(r)
	return string(resp)
}

// buildResponse is used to build response.
func buildResponse(content string, status *v1alpha1.Status) api.ProtoResponse {
	resp := &Response{
		Status:  status,
		Content: content,
	}
	return &api.AnyStringProto{Content: resp.String()}
}

// buildStatus is used to build status.
func buildStatus(code int32, msg string) *v1alpha1.Status {
	return &v1alpha1.Status{
		Code:    code,
		Message: msg,
	}
}

// isDomainResourceRequest is used to check whether it's domain resource request.
func isDomainResourceRequest(path string) bool {
	return strings.HasPrefix(path, "/api/v1/namespaces")
}

// isReferenceRequest is used to check whether it's reference request.
func isReferenceRequest(path string) bool {
	fields := strings.Split(path, "/")
	return fields[len(fields)-1] == "reference"
}

// validateRequestKind is used to check request kind.
func validateRequestKind(kind, apiPath string) error {
	if isDomainResourceRequest(apiPath) {
		if ok := validateDomainKind(kind); !ok {
			return fmt.Errorf(errUnsupportedKind, kind, common.DomainKindList)
		}
	} else {
		if ok := validateClusterKind(kind); !ok {
			return fmt.Errorf(errUnsupportedKind, kind, common.ClusterKindList)
		}
	}
	return nil
}

// validateDomainKind is used to validate domain kind.
func validateDomainKind(kind string) bool {
	for _, k := range common.DomainKindList {
		if k == kind {
			return true
		}
	}
	return false
}

// validateClusterKind is used to validate cluster kind.
func validateClusterKind(kind string) bool {
	for _, k := range common.ClusterKindList {
		if k == kind {
			return true
		}
	}
	return false
}
