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
	"github.com/secretflow/kuscia/pkg/transport/proto/ptp"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
)

// header
const (
	PtpSessionID    = "x-ptp-session-id"
	PtpTargetNodeID = "x-ptp-target-node-id"
	PtpSourceNodeID = "x-ptp-source-node-id"
	PtpTraceID      = "x-ptp-trace-id"
	PtpTopicID      = "x-ptp-topic"
)

type Outbound ptp.TransportOutbound

func BuildOutboundByErr(err *transerr.TransError) *Outbound {
	if err == nil {
		return BuildOutboundByPayload(nil)
	}

	return &Outbound{
		Payload: nil,
		Code:    err.Error(),
		Message: err.ErrorInfo(),
	}
}

func BuildOutboundByPayload(payload []byte) *Outbound {
	return &Outbound{
		Payload: payload,
		Code:    string(transerr.Success),
		Message: transerr.GetErrorInfo(transerr.Success),
	}
}
