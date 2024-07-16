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

package common

import (
	"fmt"

	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/proto/api/v1alpha1"
)

func BuildGrpcErrorf(appStatus *v1alpha1.Status, code codes.Code, format string, a ...interface{}) error {
	if appStatus == nil {
		return status.Errorf(code, format, a...)
	}

	s := &spb.Status{
		Code: int32(code),
	}
	if len(appStatus.Message) > 0 {
		s.Message = appStatus.Message
	} else {
		s.Message = fmt.Sprintf(format, a...)
	}

	if detail, err := anypb.New(appStatus); err != nil {
		s.Details = []*anypb.Any{
			detail,
		}
	}
	return status.ErrorProto(s)
}
