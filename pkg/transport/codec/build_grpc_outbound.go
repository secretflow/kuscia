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

package codec

import (
	pb "github.com/secretflow/kuscia/pkg/transport/proto/grpcPtp"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
)

//type Outbound grpcPtp.TransportOutbound

func BuildGrpcOutboundByErr(err *transerr.TransError) *pb.TransportOutbound {
	if err == nil {
		return BuildGrpcOutboundByPayload(nil)
	}

	return &pb.TransportOutbound{
		Payload: nil,
		Code:    err.Error(),
		Message: err.ErrorInfo(),
	}
}

func BuildGrpcOutboundByPayload(payload []byte) *pb.TransportOutbound {
	return &pb.TransportOutbound{
		Payload: payload,
		Code:    string(transerr.Success),
		Message: transerr.GetErrorInfo(transerr.Success),
	}
}
