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

package common

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"

	k8sv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	pbv1alpha1 "github.com/secretflow/kuscia/proto/api/v1alpha1"
)

const (
	fileFormatUnKnown = "unknown"
	fileFormatCSV     = "csv"
	fileFormatBINARY  = "binary"
)

func Convert2PbPartition(partition *k8sv1alpha1.Partition) (pbPartition *pbv1alpha1.Partition) {
	if partition == nil {
		return nil
	}
	cols := make([]*pbv1alpha1.DataColumn, len(partition.Fields))
	for i, v := range partition.Fields {
		cols[i] = &pbv1alpha1.DataColumn{
			Name:    v.Name,
			Type:    v.Type,
			Comment: v.Comment,
		}
	}
	return &pbv1alpha1.Partition{
		Type:   partition.Type,
		Fields: cols,
	}
}

func Convert2PbColumn(cols []k8sv1alpha1.DataColumn) []*pbv1alpha1.DataColumn {
	colsV1 := make([]*pbv1alpha1.DataColumn, len(cols))
	for i, v := range cols {
		colsV1[i] = &pbv1alpha1.DataColumn{
			Name:        v.Name,
			Type:        v.Type,
			Comment:     v.Comment,
			NotNullable: v.NotNullable,
		}
	}
	return colsV1
}

func Convert2KubePartition(partition *pbv1alpha1.Partition) *k8sv1alpha1.Partition {
	if partition == nil {
		return nil
	}
	cols := make([]k8sv1alpha1.DataColumn, len(partition.Fields))
	for i, v := range partition.Fields {
		cols[i] = k8sv1alpha1.DataColumn{
			Name:    v.Name,
			Type:    v.Type,
			Comment: v.Comment,
		}
	}
	return &k8sv1alpha1.Partition{
		Type:   partition.Type,
		Fields: cols,
	}
}

func Convert2KubeColumn(cols []*pbv1alpha1.DataColumn) []k8sv1alpha1.DataColumn {
	colsV1 := make([]k8sv1alpha1.DataColumn, len(cols))
	for i, v := range cols {
		colsV1[i] = k8sv1alpha1.DataColumn{
			Name:        v.Name,
			Type:        v.Type,
			Comment:     v.Comment,
			NotNullable: v.NotNullable,
		}
	}
	return colsV1
}
func Convert2KubeFileFormat(format pbv1alpha1.FileFormat) string {
	switch format {
	case pbv1alpha1.FileFormat_CSV:
		return fileFormatCSV
	case pbv1alpha1.FileFormat_BINARY:
		return fileFormatBINARY
	}
	return fileFormatUnKnown
}

func Convert2PbFileFormat(format string) pbv1alpha1.FileFormat {
	switch strings.ToLower(format) {
	case fileFormatCSV:
		return pbv1alpha1.FileFormat_CSV
	case fileFormatBINARY:
		return pbv1alpha1.FileFormat_BINARY
	}
	return pbv1alpha1.FileFormat_UNKNOWN
}

func Convert2ArrowColumnType(colType string) arrow.DataType {
	switch strings.ToLower(colType) {
	case "int8":
		return arrow.PrimitiveTypes.Int8
	case "int16":
		return arrow.PrimitiveTypes.Int16
	case "int32":
		return arrow.PrimitiveTypes.Int32
	case "int64", "int":
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
	case "float64", "float":
		return arrow.PrimitiveTypes.Float64
	case "date32":
		return arrow.PrimitiveTypes.Date32
	case "date64":
		return arrow.PrimitiveTypes.Date64
	case "bool":
		return arrow.FixedWidthTypes.Boolean
	// STRING UTF8 variable-length string as List<Char>
	case "string", "str":
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

func CopySameMemberTypeStruct(dst, src interface{}) error {
	dVal := reflect.ValueOf(dst).Elem()
	sVal := reflect.ValueOf(src).Elem()

	if dVal.NumField() != sVal.NumField() {
		return fmt.Errorf("number of fields in dst and src struct do not match")
	}

	for i := 0; i < dVal.NumField(); i++ {
		dField := dVal.Field(i)
		sField := sVal.Field(i)

		if dField.Type() != sField.Type() {
			return fmt.Errorf("type of field %v does not match", dVal.Type().Field(i).Name)
		}

		dField.Set(sField)
	}

	return nil
}

// yaml profile conversion for Protocol
func (protocol *Protocol) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val string
	err := unmarshal(&val)
	if err != nil {
		return err
	}

	upval := Protocol(strings.ToUpper(val))

	switch upval {
	case NOTLS, TLS, MTLS:
		*protocol = upval
	default:
		return fmt.Errorf("Kuscia configuration: protocol: %s is unsupported, supported: NOTLS/TLS/MTLS", val)
	}
	return nil
}
