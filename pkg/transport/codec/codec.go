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
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/transport/proto/ptp"
)

type Codec interface {
	Marshal(out *Outbound) ([]byte, error)
	UnMarshal([]byte) (*Outbound, error)
	ContentType() string
}

type ProtoCodec struct {
}

func NewProtoCodec() *ProtoCodec {
	return &ProtoCodec{}
}

func (c *ProtoCodec) Marshal(out *Outbound) ([]byte, error) {
	return proto.Marshal((*ptp.TransportOutbound)(out))
}

func (c *ProtoCodec) UnMarshal(b []byte) (*Outbound, error) {
	outbound := &ptp.TransportOutbound{}
	if err := proto.Unmarshal(b, outbound); err != nil {
		return nil, err
	}

	return (*Outbound)(outbound), nil
}

func (c *ProtoCodec) ContentType() string {
	return "application/proto"
}
