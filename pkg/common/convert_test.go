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
	"testing"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/stretchr/testify/assert"

	k8sv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	pbv1alpha1 "github.com/secretflow/kuscia/proto/api/v1alpha1"
)

// TestConvert2PbPartition_NilInput [Single Test Case] Test Scenario: Test converting an empty kube partition to a pb partition
func TestConvert2PbPartition_NilInput(t *testing.T) {
	result := Convert2PbPartition(nil)
	assert.Nil(t, result)
}

// TestConvert2PbPartition [Single Test Case] Test Scenario: Test converting kube Partition to pb Partition
func TestConvert2PbPartition(t *testing.T) {
	k8sPartition := &k8sv1alpha1.Partition{
		Type: "test",
		Fields: []k8sv1alpha1.DataColumn{
			{Name: "col1", Type: "int32", Comment: "comment1"},
			{Name: "col2", Type: "string", Comment: "comment2"},
		},
	}

	result := Convert2PbPartition(k8sPartition)
	assert.Equal(t, "test", result.Type)
	assert.Equal(t, 2, len(result.Fields))
	assert.Equal(t, "col1", result.Fields[0].Name)
	assert.Equal(t, "int32", result.Fields[0].Type)
	assert.Equal(t, "comment1", result.Fields[0].Comment)
}

// TestConvert2PbColumn [Single Test Use Case] Test Scenario: Test converts kube DataColumn array to pb DataColumn array
func TestConvert2PbColumn(t *testing.T) {
	k8sColumns := []k8sv1alpha1.DataColumn{
		{Name: "col1", Type: "int32", Comment: "comment1", NotNullable: true},
		{Name: "col2", Type: "string", Comment: "comment2", NotNullable: false},
	}

	result := Convert2PbColumn(k8sColumns)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "col1", result[0].Name)
	assert.Equal(t, "int32", result[0].Type)
	assert.Equal(t, "comment1", result[0].Comment)
	assert.True(t, result[0].NotNullable)
}

// TestConvert2KubePartition_NilInput [Single Test Case] Test Scenario: Test converting an empty pb partition to a kube partition
func TestConvert2KubePartition_NilInput(t *testing.T) {
	result := Convert2KubePartition(nil)
	assert.Nil(t, result)
}

// TestConvert2KubePartition [Single Test Case] Test Scenario: Test converting a pb partition to a kube partition
func TestConvert2KubePartition(t *testing.T) {
	pbPartition := &pbv1alpha1.Partition{
		Type: "test",
		Fields: []*pbv1alpha1.DataColumn{
			{Name: "col1", Type: "int32", Comment: "comment1"},
			{Name: "col2", Type: "string", Comment: "comment2"},
		},
	}

	result := Convert2KubePartition(pbPartition)
	assert.Equal(t, "test", result.Type)
	assert.Equal(t, 2, len(result.Fields))
	assert.Equal(t, "col1", result.Fields[0].Name)
	assert.Equal(t, "int32", result.Fields[0].Type)
	assert.Equal(t, "comment1", result.Fields[0].Comment)
}

// TestConvert2KubeColumn [Single Test Case] Test Scenario: Test converts a pb DataColumn array to a k8s DataColumn array
func TestConvert2KubeColumn(t *testing.T) {
	pbColumns := []*pbv1alpha1.DataColumn{
		{Name: "col1", Type: "int32", Comment: "comment1", NotNullable: true},
		{Name: "col2", Type: "string", Comment: "comment2", NotNullable: false},
	}

	result := Convert2KubeColumn(pbColumns)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "col1", result[0].Name)
	assert.Equal(t, "int32", result[0].Type)
	assert.Equal(t, "comment1", result[0].Comment)
	assert.True(t, result[0].NotNullable)
}

// TestConvert2KubeFileFormat [Single Test Use Case] Test Scenario: Test converting pb FileFormat to string format
func TestConvert2KubeFileFormat(t *testing.T) {
	assert.Equal(t, fileFormatCSV, Convert2KubeFileFormat(pbv1alpha1.FileFormat_CSV))
	assert.Equal(t, fileFormatBINARY, Convert2KubeFileFormat(pbv1alpha1.FileFormat_BINARY))
	assert.Equal(t, fileFormatUnKnown, Convert2KubeFileFormat(pbv1alpha1.FileFormat_UNKNOWN))
}

// TestConvert2PbFileFormat [Single Test Case] Test Scenario: Test converting a string format to pb FileFormat
func TestConvert2PbFileFormat(t *testing.T) {
	assert.Equal(t, pbv1alpha1.FileFormat_CSV, Convert2PbFileFormat(fileFormatCSV))
	assert.Equal(t, pbv1alpha1.FileFormat_BINARY, Convert2PbFileFormat(fileFormatBINARY))
	assert.Equal(t, pbv1alpha1.FileFormat_UNKNOWN, Convert2PbFileFormat("unknown_format"))
	assert.Equal(t, pbv1alpha1.FileFormat_CSV, Convert2PbFileFormat("CSV")) // 测试大小写不敏感
}

// TestConvert2ArrowColumnType [Single Test Use Case] Test Scenario: Test converting a string type to an Arrow data type
func TestConvert2ArrowColumnType(t *testing.T) {
	assert.Equal(t, arrow.PrimitiveTypes.Int8, Convert2ArrowColumnType("int8"))
	assert.Equal(t, arrow.PrimitiveTypes.Int16, Convert2ArrowColumnType("int16"))
	assert.Equal(t, arrow.PrimitiveTypes.Int32, Convert2ArrowColumnType("int32"))
	assert.Equal(t, arrow.PrimitiveTypes.Int64, Convert2ArrowColumnType("int64"))
	assert.Equal(t, arrow.PrimitiveTypes.Int64, Convert2ArrowColumnType("int"))
	assert.Equal(t, arrow.PrimitiveTypes.Uint8, Convert2ArrowColumnType("uint8"))
	assert.Equal(t, arrow.PrimitiveTypes.Uint16, Convert2ArrowColumnType("uint16"))
	assert.Equal(t, arrow.PrimitiveTypes.Uint32, Convert2ArrowColumnType("uint32"))
	assert.Equal(t, arrow.PrimitiveTypes.Uint64, Convert2ArrowColumnType("uint64"))
	assert.Equal(t, arrow.PrimitiveTypes.Uint64, Convert2ArrowColumnType("uint"))
	assert.Equal(t, arrow.PrimitiveTypes.Float32, Convert2ArrowColumnType("float32"))
	assert.Equal(t, arrow.PrimitiveTypes.Float64, Convert2ArrowColumnType("float64"))
	assert.Equal(t, arrow.PrimitiveTypes.Float64, Convert2ArrowColumnType("float"))
	assert.Equal(t, arrow.PrimitiveTypes.Date32, Convert2ArrowColumnType("date32"))
	assert.Equal(t, arrow.PrimitiveTypes.Date64, Convert2ArrowColumnType("date64"))

	assert.Equal(t, arrow.PrimitiveTypes.Date32, Convert2ArrowColumnType("date"))
	assert.Equal(t, arrow.PrimitiveTypes.Date64, Convert2ArrowColumnType("datetime"))
	assert.Equal(t, arrow.FixedWidthTypes.Time32s, Convert2ArrowColumnType("time32"))
	assert.Equal(t, arrow.FixedWidthTypes.Time64us, Convert2ArrowColumnType("time64"))
	assert.Equal(t, arrow.FixedWidthTypes.Timestamp_s, Convert2ArrowColumnType("timestamp"))
	assert.Equal(t, arrow.FixedWidthTypes.Duration_s, Convert2ArrowColumnType("duration"))

	assert.Equal(t, arrow.FixedWidthTypes.Boolean, Convert2ArrowColumnType("bool"))
	assert.Equal(t, arrow.BinaryTypes.String, Convert2ArrowColumnType("string"))
	assert.Equal(t, arrow.BinaryTypes.String, Convert2ArrowColumnType("str"))
	// Test LargeString mappings
	assert.Equal(t, arrow.BinaryTypes.LargeString, Convert2ArrowColumnType("large_string"))
	assert.Equal(t, arrow.BinaryTypes.LargeString, Convert2ArrowColumnType("large_str"))
	assert.Equal(t, arrow.BinaryTypes.LargeString, Convert2ArrowColumnType("large_utf8"))
	assert.Equal(t, arrow.BinaryTypes.Binary, Convert2ArrowColumnType("binary"))
	assert.Nil(t, Convert2ArrowColumnType("unknown_type"))
}

// TestConvert2DataMeshColumnType [Single Test Use Case] Test Scenario: Test converting the Arrow data type to a string type
func TestConvert2DataMeshColumnType(t *testing.T) {
	assert.Equal(t, "int8", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Int8))
	assert.Equal(t, "int16", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Int16))
	assert.Equal(t, "int32", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Int32))
	assert.Equal(t, "int64", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Int64))
	assert.Equal(t, "uint8", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Uint8))
	assert.Equal(t, "uint16", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Uint16))
	assert.Equal(t, "uint32", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Uint32))
	assert.Equal(t, "uint64", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Uint64))
	assert.Equal(t, "float32", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Float32))
	assert.Equal(t, "float64", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Float64))
	assert.Equal(t, "date32", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Date32))
	assert.Equal(t, "date64", Convert2DataMeshColumnType(arrow.PrimitiveTypes.Date64))

	assert.Equal(t, "time32", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Time32s))
	assert.Equal(t, "time32", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Time32ms))
	assert.Equal(t, "time64", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Time64us))
	assert.Equal(t, "time64", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Time64ns))

	assert.Equal(t, "timestamp", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Timestamp_s))
	assert.Equal(t, "timestamp", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Timestamp_ms))
	assert.Equal(t, "timestamp", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Timestamp_us))
	assert.Equal(t, "timestamp", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Timestamp_ns))

	assert.Equal(t, "duration", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Duration_s))
	assert.Equal(t, "duration", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Duration_ms))
	assert.Equal(t, "duration", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Duration_us))
	assert.Equal(t, "duration", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Duration_ns))

	assert.Equal(t, "bool", Convert2DataMeshColumnType(arrow.FixedWidthTypes.Boolean))
	assert.Equal(t, "string", Convert2DataMeshColumnType(arrow.BinaryTypes.String))
	assert.Equal(t, "large_string", Convert2DataMeshColumnType(arrow.BinaryTypes.LargeString))
	assert.Equal(t, "binary", Convert2DataMeshColumnType(arrow.BinaryTypes.Binary))
	assert.Empty(t, Convert2DataMeshColumnType(arrow.Null))
}

// TestArrowSchema2DataMeshColumns [Single Test Use Case] Test Scenario: Test converting an Arrow Schema to a DataMesh column
func TestArrowSchema2DataMeshColumns(t *testing.T) {
	fields := []arrow.Field{
		{Name: "col1", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "col2", Type: arrow.BinaryTypes.String, Nullable: true},
	}
	schema := arrow.NewSchema(fields, nil)

	columns, err := ArrowSchema2DataMeshColumns(schema)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(columns))
	assert.Equal(t, "col1", columns[0].Name)
	assert.Equal(t, "int32", columns[0].Type)
	assert.True(t, columns[0].NotNullable)
	assert.Equal(t, "col2", columns[1].Name)
	assert.Equal(t, "string", columns[1].Type)
	assert.False(t, columns[1].NotNullable)
}

// TestArrowSchema2DataMeshColumns_InvalidType [Single Test Use Case] Test Scenario: Test a case where converting an Arrow Schema to a DataMesh column fails
func TestArrowSchema2DataMeshColumns_InvalidType(t *testing.T) {
	fields := []arrow.Field{
		{Name: "col1", Type: arrow.Null, Nullable: false},
	}
	schema := arrow.NewSchema(fields, nil)

	_, err := ArrowSchema2DataMeshColumns(schema)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid arrow.DataType")
}

// TestCopySameMemberTypeStruct Test scenario: Test replication structs of the same member type
func TestCopySameMemberTypeStruct(t *testing.T) {
	type TestStruct struct {
		Field1 string
		Field2 int
	}

	src := TestStruct{Field1: "test", Field2: 42}
	dst := TestStruct{}

	err := CopySameMemberTypeStruct(&dst, &src)
	assert.NoError(t, err)
	assert.Equal(t, "test", dst.Field1)
	assert.Equal(t, 42, dst.Field2)
}

// TestCopySameMemberTypeStruct_DifferentFieldCount Test scenario: Test replication structs with different number of members
func TestCopySameMemberTypeStruct_DifferentFieldCount(t *testing.T) {
	type SrcStruct struct {
		Field1 string
		Field2 int
	}
	type DstStruct struct {
		Field1 string
	}

	src := SrcStruct{Field1: "test", Field2: 42}
	dst := DstStruct{}

	err := CopySameMemberTypeStruct(&dst, &src)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "do not match")
}

// TestCopySameMemberTypeStruct_DifferentFieldTypes Test scenario: Test replication of structs of different member types
func TestCopySameMemberTypeStruct_DifferentFieldTypes(t *testing.T) {
	type SrcStruct struct {
		Field1 string
		Field2 int
	}
	type DstStruct struct {
		Field1 string
		Field2 string // different type
	}

	src := SrcStruct{Field1: "test", Field2: 42}
	dst := DstStruct{}

	err := CopySameMemberTypeStruct(&dst, &src)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

// TestProtocol_UnmarshalYAML [Single test case] Test scenario: Test the YAML deserialization of the protocol
func TestProtocol_UnmarshalYAML(t *testing.T) {
	var p Protocol

	// Test valid values
	tests := []struct {
		input    string
		expected Protocol
	}{
		{"notls", NOTLS},
		{"TLS", TLS},
		{"mtls", MTLS},
		{"NotLs", NOTLS}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := p.UnmarshalYAML(func(v interface{}) error {
				*v.(*string) = tt.input
				return nil
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, p)
		})
	}

	// Test invalid value
	err := p.UnmarshalYAML(func(v interface{}) error {
		*v.(*string) = "invalid"
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}
