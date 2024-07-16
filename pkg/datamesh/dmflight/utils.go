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

package dmflight

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const AliyunOssEndpointSuffix string = ".aliyuncs.com"
const BuiltinFlightServerEndpointURI string = "kuscia://datamesh"

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

func CreateDateMeshFlightInfo(tick []byte, endpointURI string) *flight.FlightInfo {
	info := &flight.FlightInfo{
		FlightDescriptor: &flight.FlightDescriptor{
			Type: flight.DescriptorCMD,
			Cmd:  []byte{},
			Path: []string{},
		},
		Endpoint: []*flight.FlightEndpoint{
			{
				Location: []*flight.Location{
					{
						Uri: endpointURI,
					},
				},
				Ticket: &flight.Ticket{
					Ticket: tick,
				},
			},
		},
	}

	return info
}

// GenerateArrowSchema generate schema from domaindata
func GenerateArrowSchema(domainData *datamesh.DomainData) (*arrow.Schema, error) {
	// get schema from query
	fields := make([]arrow.Field, 0)
	for i, column := range domainData.Columns {
		colType := common.Convert2ArrowColumnType(column.Type)
		if colType == nil {
			return nil, status.Errorf(codes.Internal, "invalid column(%s) with type(%s)", column.Name, column.Type)
		}
		fields = append(fields, arrow.Field{
			Name:     column.Name,
			Type:     colType,
			Nullable: !column.NotNullable,
		})
		nlog.Debugf("Columns[%d:%s].Nullable is :%v", i, column.Name, !column.NotNullable)
	}
	return arrow.NewSchema(fields, nil), nil
}

func GenerateBinaryDataArrowSchema() *arrow.Schema {
	return arrow.NewSchema([]arrow.Field{
		{
			Name:     "data",
			Type:     arrow.BinaryTypes.Binary,
			Nullable: false,
		},
	}, nil)
}

func PackActionResult(msg proto.Message) (*flight.Result, error) {
	var err error

	ret := &flight.Result{}
	if ret.Body, err = proto.Marshal(msg); err != nil {
		nlog.Warnf("Unable to marshal msg to flight.Result.Body: %v", err)
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("Unable to marshal final response: %v", err))
	}
	return ret, nil
}

func ParseRegionFromEndpoint(endpoint string) (string, error) {
	if strings.Contains(endpoint, AliyunOssEndpointSuffix) {
		// aliyun oss
		url, err := url.Parse(endpoint)
		if err != nil {
			nlog.Warnf("Parse endpoint(%s) failed", endpoint)
			return "", err
		}

		if strings.HasSuffix(url.Host, AliyunOssEndpointSuffix) {
			return strings.TrimSuffix(url.Host, AliyunOssEndpointSuffix), nil
		}
	}

	return "", nil
}
