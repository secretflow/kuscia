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
	pb "github.com/secretflow/kuscia/pkg/transport/proto/mesh"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
)

func BuildInvokeOutboundByErr(err *transerr.TransError) *pb.Outbound {
	if err == nil {
		return BuildInvokeOutboundByPayload(nil)
	}

	return &pb.Outbound{
		Code:    err.Error(),
		Message: err.ErrorInfo(),
	}
}

func BuildInvokeOutboundByPayload(payload []byte) *pb.Outbound {
	return &pb.Outbound{
		Payload: payload,
		Code:    string(transerr.Success),
		Message: transerr.GetErrorInfo(transerr.Success),
	}
}

func BuildTransportOutboundByErr(err *transerr.TransError) *pb.TransportOutbound {
	if err == nil {
		return BuildTransportOutboundByPayload(nil)
	}

	return &pb.TransportOutbound{
		Code:    err.Error(),
		Message: err.ErrorInfo(),
	}
}

func BuildTransportOutboundByPayload(payload []byte) *pb.TransportOutbound {
	return &pb.TransportOutbound{
		Payload: payload,
		Code:    string(transerr.Success),
		Message: transerr.GetErrorInfo(transerr.Success),
	}
}
