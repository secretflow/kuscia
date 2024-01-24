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

package flight

import (
	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

func DescForCommand(cmd proto.Message) (*flight.FlightDescriptor, error) {
	var any anypb.Any
	if err := any.MarshalFrom(cmd); err != nil {
		return nil, err
	}

	data, err := proto.Marshal(&any)
	if err != nil {
		return nil, err
	}
	return &flight.FlightDescriptor{
		Type: flight.DescriptorCMD,
		Cmd:  data,
	}, nil
}

func GetTicketDomainDataQuery(in *flight.Ticket) (*datamesh.TicketDomainDataQuery, error) {
	var any anypb.Any
	if err := proto.Unmarshal(in.Ticket, &any); err != nil {
		nlog.Warnf("Unmarshal Ticket to any fail: %v", err)
		return nil, err
	}

	var out datamesh.TicketDomainDataQuery
	if err := any.UnmarshalTo(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
