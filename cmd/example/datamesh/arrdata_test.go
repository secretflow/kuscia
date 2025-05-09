package main

import (
	"testing"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/stretchr/testify/assert"
)

func TestMakeNullRecords(t *testing.T) {
	// Call the function
	records := makeNullRecords()

	// Verify that the return value is a slice of Arrow records
	assert.NotNil(t, records, "records should not be nil")

	// Verify the number of records
	assert.Len(t, records, 3, "should have 3 records")

	// Verify the structure of each record
	for _, rec := range records {
		assert.NotNil(t, rec, "record should not be nil")
		assert.Equal(t, 1, len(rec.Schema().Fields()), "schema should have 1 field")
		assert.Equal(t, "nulls", rec.Schema().Field(0).Name, "field name should be 'nulls'")
		assert.Equal(t, arrow.Null, rec.Schema().Field(0).Type, "field type should be Null")
	}
}

func TestMakePrimitiveRecords(t *testing.T) {
	// Call the function
	records := MakePrimitiveRecords()

	// Verify that the return value is a slice of Arrow records
	assert.NotNil(t, records, "records should not be nil")

	// Verify the number of records
	assert.Len(t, records, 3, "should have 3 records")

	// Verify the structure of each record
	for _, rec := range records {
		assert.NotNil(t, rec, "record should not be nil")
		assert.Equal(t, 3, len(rec.Schema().Fields()), "schema should have 3 fields")
		assert.Equal(t, "bools", rec.Schema().Field(0).Name, "first field name should be 'bools'")
		assert.Equal(t, arrow.FixedWidthTypes.Boolean, rec.Schema().Field(0).Type, "first field type should be Boolean")
		assert.Equal(t, "int64s", rec.Schema().Field(1).Name, "second field name should be 'int64s'")
		assert.Equal(t, arrow.PrimitiveTypes.Int64, rec.Schema().Field(1).Type, "second field type should be Int64")
		assert.Equal(t, "float64s", rec.Schema().Field(2).Name, "third field name should be 'float64s'")
		assert.Equal(t, arrow.PrimitiveTypes.Float64, rec.Schema().Field(2).Type, "third field type should be Float64")
	}
}

func TestMakeStructsRecords(t *testing.T) {
	// Call the function
	records := makeStructsRecords()

	// Verify that the return value is a slice of Arrow records
	assert.NotNil(t, records, "records should not be nil")

	// Verify the number of records
	assert.Len(t, records, 2, "should have 2 records")

	// Verify the structure of each record
	for _, rec := range records {
		assert.NotNil(t, rec, "record should not be nil")
		assert.Equal(t, 1, len(rec.Schema().Fields()), "schema should have 1 field")
		assert.Equal(t, "struct_nullable", rec.Schema().Field(0).Name, "field name should be 'struct_nullable'")
		assert.Equal(t, arrow.StructOf(
			arrow.Field{Name: "f1", Type: arrow.PrimitiveTypes.Int32},
			arrow.Field{Name: "f2", Type: arrow.BinaryTypes.String},
		), rec.Schema().Field(0).Type, "field type should be Struct")
	}
}

func TestMakeStringsRecords(t *testing.T) {
	// Call the function
	records := makeStringsRecords()

	// Verify that the return value is a slice of Arrow records
	assert.NotNil(t, records, "records should not be nil")

	// Verify the number of records
	assert.Len(t, records, 3, "should have 3 records")

	// Verify the structure of each record
	for _, rec := range records {
		assert.NotNil(t, rec, "record should not be nil")
		assert.Equal(t, 2, len(rec.Schema().Fields()), "schema should have 2 fields")
		assert.Equal(t, "strings", rec.Schema().Field(0).Name, "first field name should be 'strings'")
		assert.Equal(t, arrow.BinaryTypes.String, rec.Schema().Field(0).Type, "first field type should be String")
		assert.Equal(t, "bytes", rec.Schema().Field(1).Name, "second field name should be 'bytes'")
		assert.Equal(t, arrow.BinaryTypes.Binary, rec.Schema().Field(1).Type, "second field type should be Binary")
	}
}

func TestMakeBinaryRecords(t *testing.T) {
	// Call the function
	records := MakeBinaryRecords()

	// Verify that the return value is a slice of Arrow records
	assert.NotNil(t, records, "records should not be nil")

	// Verify the number of records
	assert.Len(t, records, 3, "should have 3 records")

	// Verify the structure of each record
	for _, rec := range records {
		assert.NotNil(t, rec, "record should not be nil")
		assert.Equal(t, 1, len(rec.Schema().Fields()), "schema should have 1 field")
		assert.Equal(t, "binary_data", rec.Schema().Field(0).Name, "field name should be 'binary_data'")
		assert.Equal(t, arrow.BinaryTypes.Binary, rec.Schema().Field(0).Type, "field type should be Binary")
	}
}

func TestArrayOf(t *testing.T) {
	mem := memory.NewGoAllocator()

	// Test the bool type
	bools := []bool{true, false, true, false, true}
	mask := []bool{true, false, false, true, true}
	arr := arrayOf(mem, bools, mask)
	assert.NotNil(t, arr, "array should not be nil")
	assert.Equal(t, 5, arr.Len(), "array length should be 5")

	// Test the int64 type
	ints := []int64{-1, -2, -3, -4, -5}
	arr = arrayOf(mem, ints, mask)
	assert.NotNil(t, arr, "array should not be nil")
	assert.Equal(t, 5, arr.Len(), "array length should be 5")

	// Test the float64 type
	floats := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	arr = arrayOf(mem, floats, mask)
	assert.NotNil(t, arr, "array should not be nil")
	assert.Equal(t, 5, arr.Len(), "array length should be 5")

	// Test the string type
	strs := []string{"1", "2", "3", "4", "5"}
	arr = arrayOf(mem, strs, mask)
	assert.NotNil(t, arr, "array should not be nil")
	assert.Equal(t, 5, arr.Len(), "array length should be 5")

	// Test the []byte type
	bytes := [][]byte{[]byte("1"), []byte("2"), []byte("3"), []byte("4"), []byte("5")}
	arr = arrayOf(mem, bytes, mask)
	assert.NotNil(t, arr, "array should not be nil")
	assert.Equal(t, 5, arr.Len(), "array length should be 5")
}

func TestStructOf(t *testing.T) {
	mem := memory.NewGoAllocator()

	fields := []arrow.Field{
		{Name: "f1", Type: arrow.PrimitiveTypes.Int32},
		{Name: "f2", Type: arrow.BinaryTypes.String},
	}
	dtype := arrow.StructOf(fields...)

	mask := []bool{true, false, true, true, true}
	fieldsData := [][]arrow.Array{
		{
			arrayOf(mem, []int32{-1, -2, -3, -4, -5}, mask),
			arrayOf(mem, []string{"111", "222", "333", "444", "555"}, mask),
		},
	}

	arr := structOf(mem, dtype, fieldsData, mask)
	assert.NotNil(t, arr, "array should not be nil")
	assert.Equal(t, 5, arr.Len(), "array length should be 5")
	assert.Equal(t, dtype, arr.DataType(), "data type should match")
}

func TestBuildArray(t *testing.T) {
	mem := memory.NewGoAllocator()

	// Test the bool type
	bools := []bool{true, false, true, false, true}
	mask := []bool{true, false, false, true, true}
	boolArr := arrayOf(mem, bools, mask)
	boolBldr := array.NewBooleanBuilder(mem)
	defer boolBldr.Release()
	buildArray(boolBldr, boolArr)
	boolResult := boolBldr.NewBooleanArray()
	assert.NotNil(t, boolResult, "bool array should not be nil")
	assert.Equal(t, 5, boolResult.Len(), "bool array length should be 5")

	// Test the int64 type
	ints := []int64{-1, -2, -3, -4, -5}
	intArr := arrayOf(mem, ints, mask)
	intBldr := array.NewInt64Builder(mem)
	defer intBldr.Release()
	buildArray(intBldr, intArr)
	intResult := intBldr.NewInt64Array()
	assert.NotNil(t, intResult, "int64 array should not be nil")
	assert.Equal(t, 5, intResult.Len(), "int64 array length should be 5")

	// Test the float64 type
	floats := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	floatArr := arrayOf(mem, floats, mask)
	floatBldr := array.NewFloat64Builder(mem)
	defer floatBldr.Release()
	buildArray(floatBldr, floatArr)
	floatResult := floatBldr.NewFloat64Array()
	assert.NotNil(t, floatResult, "float64 array should not be nil")
	assert.Equal(t, 5, floatResult.Len(), "float64 array length should be 5")

	// Test the string type
	strs := []string{"1", "2", "3", "4", "5"}
	strArr := arrayOf(mem, strs, mask)
	strBldr := array.NewStringBuilder(mem)
	defer strBldr.Release()
	buildArray(strBldr, strArr)
	strResult := strBldr.NewStringArray()
	assert.NotNil(t, strResult, "string array should not be nil")
	assert.Equal(t, 5, strResult.Len(), "string array length should be 5")

	// Test the []byte type
	bytes := [][]byte{[]byte("1"), []byte("2"), []byte("3"), []byte("4"), []byte("5")}
	byteArr := arrayOf(mem, bytes, mask)
	byteBldr := array.NewBinaryBuilder(mem, arrow.BinaryTypes.Binary)
	defer byteBldr.Release()
	buildArray(byteBldr, byteArr)
	byteResult := byteBldr.NewBinaryArray()
	assert.NotNil(t, byteResult, "binary array should not be nil")
}
