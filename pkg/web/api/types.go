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

package api

import (
	"reflect"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

type ProtoRequest = proto.Message

type ProtoResponse = proto.Message

type BizContext struct {
	*gin.Context
	Store            map[string]interface{}
	ConfBeanRegistry framework.ConfBeanRegistry
	BizType          string
	Logger           *nlog.NLog
}

// ProtoHandler Is the service handler interface.
type ProtoHandler interface {
	// Validate is used for request verification.
	Validate(*BizContext, ProtoRequest, *errorcode.Errs)
	// Handle is used to process business logic
	Handle(*BizContext, ProtoRequest) ProtoResponse
	// GetType returns the type of request and response.
	GetType() (reqType, respType reflect.Type)
}
