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
	"fmt"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	pbv1alpha1 "github.com/secretflow/kuscia/proto/api/v1alpha1"
)

func Convert2ArrowColumnType(colType string) arrow.DataType {
	switch strings.ToLower(colType) {
	case "int8":
		return arrow.PrimitiveTypes.Int8
	case "int16":
		return arrow.PrimitiveTypes.Int16
	case "int32":
		return arrow.PrimitiveTypes.Int32
	case "int64":
		return arrow.PrimitiveTypes.Int64
	case "uint8":
		return arrow.PrimitiveTypes.Uint8
	case "uint16":
		return arrow.PrimitiveTypes.Uint16
	case "uint32":
		return arrow.PrimitiveTypes.Uint32
	case "uint64":
		return arrow.PrimitiveTypes.Uint64
	case "float32":
		return arrow.PrimitiveTypes.Float32
	case "float64":
		return arrow.PrimitiveTypes.Float64
	case "date32":
		return arrow.PrimitiveTypes.Date32
	case "date64":
		return arrow.PrimitiveTypes.Date64
	case "bool":
		return arrow.FixedWidthTypes.Boolean
	// STRING UTF8 variable-length string as List<Char>
	case "string":
		return arrow.BinaryTypes.String
	// Variable-length bytes (no guarantee of UTF8-ness)
	case "binary":
		return arrow.BinaryTypes.Binary
	}
	return nil
}

func Convert2DataMeshColumnType(colType arrow.DataType) string {
	switch colType {
	case arrow.PrimitiveTypes.Int8:
		return "int8"
	case arrow.PrimitiveTypes.Int16:
		return "int16"
	case arrow.PrimitiveTypes.Int32:
		return "int32"
	case arrow.PrimitiveTypes.Int64:
		return "int64"
	case arrow.PrimitiveTypes.Uint8:
		return "uint8"
	case arrow.PrimitiveTypes.Uint16:
		return "uint16"
	case arrow.PrimitiveTypes.Uint32:
		return "uint32"
	case arrow.PrimitiveTypes.Uint64:
		return "uint64"
	case arrow.PrimitiveTypes.Float32:
		return "float32"
	case arrow.PrimitiveTypes.Float64:
		return "float64"
	case arrow.PrimitiveTypes.Date32:
		return "date32"
	case arrow.PrimitiveTypes.Date64:
		return "date64"
	case arrow.FixedWidthTypes.Boolean:
		return "bool"
	// STRING UTF8 variable-length string as List<Char>
	case arrow.BinaryTypes.String:
		return "string"
	// Variable-length bytes (no guarantee of UTF8-ness)
	case arrow.BinaryTypes.Binary:
		return "binary"
	}
	return ""
}

func ArrowSchema2DataMeshColumns(schema *arrow.Schema) ([]*pbv1alpha1.DataColumn, error) {
	var res []*pbv1alpha1.DataColumn
	for i, field := range schema.Fields() {
		colType := Convert2DataMeshColumnType(field.Type)
		if len(colType) == 0 {
			return nil, fmt.Errorf("invalid arrow.DataType: %v of field[%d]: %s", field.Type, i, field.Name)
		}
		res = append(res, &pbv1alpha1.DataColumn{
			Name:        field.Name,
			Type:        colType,
			NotNullable: !field.Nullable,
		})
	}
	return res, nil
}

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

	nlog.Warnf("type is %s", any.GetTypeUrl())
	var out datamesh.TicketDomainDataQuery
	if err := any.UnmarshalTo(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
