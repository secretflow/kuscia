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

// this file is copy from https://github.com/apache/arrow

package example

import (
	"fmt"
	"sort"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/float16"
	"github.com/apache/arrow/go/v13/arrow/memory"
)

var (
	Records     = make(map[string][]arrow.Record)
	RecordNames []string
)

type (
	nullT       struct{}
	time32s     arrow.Time32
	time32ms    arrow.Time32
	time64ns    arrow.Time64
	time64us    arrow.Time64
	timestamps  arrow.Timestamp
	timestampms arrow.Timestamp
	timestampus arrow.Timestamp
	timestampns arrow.Timestamp
)

var (
	null nullT
)

func init() {
	Records["nulls"] = makeNullRecords()
	Records["primitives"] = MakePrimitiveRecords()
	Records["structs"] = makeStructsRecords()
	Records["strings"] = makeStringsRecords()
	Records["binary"] = MakeBinaryRecords()

	for k := range Records {
		RecordNames = append(RecordNames, k)
	}
	sort.Strings(RecordNames)
}

func makeNullRecords() []arrow.Record {
	mem := memory.NewGoAllocator()

	meta := arrow.NewMetadata(
		[]string{"k1", "k2", "k3"},
		[]string{"v1", "v2", "v3"},
	)

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "nulls", Type: arrow.Null, Nullable: true},
		}, &meta,
	)

	mask := []bool{true, false, false, true, true}
	chunks := [][]arrow.Array{
		{
			arrayOf(mem, []nullT{null, null, null, null, null}, mask),
		},
		{
			arrayOf(mem, []nullT{null, null, null, null, null}, mask),
		},
		{
			arrayOf(mem, []nullT{null, null, null, null, null}, mask),
		},
	}

	defer func() {
		for _, chunk := range chunks {
			for _, col := range chunk {
				col.Release()
			}
		}
	}()

	recs := make([]arrow.Record, len(chunks))
	for i, chunk := range chunks {
		recs[i] = array.NewRecord(schema, chunk, -1)
	}

	return recs
}

func MakePrimitiveRecords() []arrow.Record {
	mem := memory.NewGoAllocator()

	meta := arrow.NewMetadata(
		[]string{"k1", "k2", "k3"},
		[]string{"v1", "v2", "v3"},
	)

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "bools", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
			{Name: "int8s", Type: arrow.PrimitiveTypes.Int8, Nullable: true},
			{Name: "float64s", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		}, &meta,
	)

	mask := []bool{true, false, false, true, true}
	chunks := [][]arrow.Array{
		{
			arrayOf(mem, []bool{true, false, true, false, true}, mask),
			arrayOf(mem, []int8{-1, -2, -3, -4, -5}, mask),
			arrayOf(mem, []float64{+1, +2, +3, +4, +5}, mask),
		},
		{
			arrayOf(mem, []bool{true, false, true, false, true}, mask),
			arrayOf(mem, []int8{-11, -12, -13, -14, -15}, mask),
			arrayOf(mem, []float64{+11, +12, +13, +14, +15}, mask),
		},
		{
			arrayOf(mem, []bool{true, false, true, false, true}, mask),
			arrayOf(mem, []int8{-21, -22, -23, -24, -25}, mask),
			arrayOf(mem, []float64{+21, +22, +23, +24, +25}, mask),
		},
	}

	defer func() {
		for _, chunk := range chunks {
			for _, col := range chunk {
				col.Release()
			}
		}
	}()

	recs := make([]arrow.Record, len(chunks))
	for i, chunk := range chunks {
		recs[i] = array.NewRecord(schema, chunk, -1)
	}

	return recs
}

func makeStructsRecords() []arrow.Record {
	mem := memory.NewGoAllocator()

	fields := []arrow.Field{
		{Name: "f1", Type: arrow.PrimitiveTypes.Int32},
		{Name: "f2", Type: arrow.BinaryTypes.String},
	}
	dtype := arrow.StructOf(fields...)
	schema := arrow.NewSchema([]arrow.Field{{Name: "struct_nullable", Type: dtype, Nullable: true}}, nil)

	mask := []bool{true, false, false, true, true, true, false, true}
	chunks := [][]arrow.Array{
		{
			structOf(mem, dtype, [][]arrow.Array{
				{
					arrayOf(mem, []int32{-1, -2, -3, -4, -5}, mask[:5]),
					arrayOf(mem, []string{"111", "222", "333", "444", "555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{-11, -12, -13, -14, -15}, mask[:5]),
					arrayOf(mem, []string{"1111", "1222", "1333", "1444", "1555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{-21, -22, -23, -24, -25}, mask[:5]),
					arrayOf(mem, []string{"2111", "2222", "2333", "2444", "2555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{-31, -32, -33, -34, -35}, mask[:5]),
					arrayOf(mem, []string{"3111", "3222", "3333", "3444", "3555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{-41, -42, -43, -44, -45}, mask[:5]),
					arrayOf(mem, []string{"4111", "4222", "4333", "4444", "4555"}, mask[:5]),
				},
			}, []bool{true, false, true, true, true}),
		},
		{
			structOf(mem, dtype, [][]arrow.Array{
				{
					arrayOf(mem, []int32{1, 2, 3, 4, 5}, mask[:5]),
					arrayOf(mem, []string{"-111", "-222", "-333", "-444", "-555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{11, 12, 13, 14, 15}, mask[:5]),
					arrayOf(mem, []string{"-1111", "-1222", "-1333", "-1444", "-1555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{21, 22, 23, 24, 25}, mask[:5]),
					arrayOf(mem, []string{"-2111", "-2222", "-2333", "-2444", "-2555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{31, 32, 33, 34, 35}, mask[:5]),
					arrayOf(mem, []string{"-3111", "-3222", "-3333", "-3444", "-3555"}, mask[:5]),
				},
				{
					arrayOf(mem, []int32{41, 42, 43, 44, 45}, mask[:5]),
					arrayOf(mem, []string{"-4111", "-4222", "-4333", "-4444", "-4555"}, mask[:5]),
				},
			}, []bool{true, false, false, true, true}),
		},
	}

	defer func() {
		for _, chunk := range chunks {
			for _, col := range chunk {
				col.Release()
			}
		}
	}()

	recs := make([]arrow.Record, len(chunks))
	for i, chunk := range chunks {
		recs[i] = array.NewRecord(schema, chunk, -1)
	}

	return recs
}

func makeStringsRecords() []arrow.Record {
	mem := memory.NewGoAllocator()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "strings", Type: arrow.BinaryTypes.String},
		{Name: "bytes", Type: arrow.BinaryTypes.Binary},
	}, nil)

	mask := []bool{true, false, false, true, true}
	chunks := [][]arrow.Array{
		{
			arrayOf(mem, []string{"1é", "2", "3", "4", "5"}, mask),
			arrayOf(mem, [][]byte{[]byte("1é"), []byte("2"), []byte("3"), []byte("4"), []byte("5")}, mask),
		},
		{
			arrayOf(mem, []string{"11", "22", "33", "44", "55"}, mask),
			arrayOf(mem, [][]byte{[]byte("11"), []byte("22"), []byte("33"), []byte("44"), []byte("55")}, mask),
		},
		{
			arrayOf(mem, []string{"111", "222", "333", "444", "555"}, mask),
			arrayOf(mem, [][]byte{[]byte("111"), []byte("222"), []byte("333"), []byte("444"), []byte("555")}, mask),
		},
	}

	defer func() {
		for _, chunk := range chunks {
			for _, col := range chunk {
				col.Release()
			}
		}
	}()

	recs := make([]arrow.Record, len(chunks))
	for i, chunk := range chunks {
		recs[i] = array.NewRecord(schema, chunk, -1)
	}

	return recs
}

func MakeBinaryRecords() []arrow.Record {
	mem := memory.NewGoAllocator()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "binary_data", Type: arrow.BinaryTypes.Binary},
	}, nil)

	mask := []bool{true, false, false, true, true}
	chunks := [][]arrow.Array{
		{
			arrayOf(mem, [][]byte{[]byte("1é"), []byte("2"), []byte("3"), []byte("4"), []byte("5")}, mask),
		},
		{
			arrayOf(mem, [][]byte{[]byte("11"), []byte("22"), []byte("33"), []byte("44"), []byte("55")}, mask),
		},
		{
			arrayOf(mem, [][]byte{[]byte("111"), []byte("222"), []byte("333"), []byte("444"), []byte("555")}, mask),
		},
	}

	defer func() {
		for _, chunk := range chunks {
			for _, col := range chunk {
				col.Release()
			}
		}
	}()

	recs := make([]arrow.Record, len(chunks))
	for i, chunk := range chunks {
		recs[i] = array.NewRecord(schema, chunk, -1)
	}

	return recs
}

func arrayOf(mem memory.Allocator, a interface{}, valids []bool) arrow.Array {
	if mem == nil {
		mem = memory.NewGoAllocator()
	}

	switch a := a.(type) {
	case []nullT:
		return array.NewNull(len(a))

	case []bool:
		bldr := array.NewBooleanBuilder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewBooleanArray()

	case []int8:
		bldr := array.NewInt8Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewInt8Array()

	case []int16:
		bldr := array.NewInt16Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewInt16Array()

	case []int32:
		bldr := array.NewInt32Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewInt32Array()

	case []int64:
		bldr := array.NewInt64Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewInt64Array()

	case []uint8:
		bldr := array.NewUint8Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewUint8Array()

	case []uint16:
		bldr := array.NewUint16Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewUint16Array()

	case []uint32:
		bldr := array.NewUint32Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewUint32Array()

	case []uint64:
		bldr := array.NewUint64Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewUint64Array()

	case []float16.Num:
		bldr := array.NewFloat16Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewFloat16Array()

	case []float32:
		bldr := array.NewFloat32Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewFloat32Array()

	case []float64:
		bldr := array.NewFloat64Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewFloat64Array()

	case []string:
		bldr := array.NewStringBuilder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewStringArray()

	case [][]byte:
		bldr := array.NewBinaryBuilder(mem, arrow.BinaryTypes.Binary)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewBinaryArray()

	case []time32s:
		bldr := array.NewTime32Builder(mem, arrow.FixedWidthTypes.Time32s.(*arrow.Time32Type))
		defer bldr.Release()

		vs := make([]arrow.Time32, len(a))
		for i, v := range a {
			vs[i] = arrow.Time32(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []time32ms:
		bldr := array.NewTime32Builder(mem, arrow.FixedWidthTypes.Time32ms.(*arrow.Time32Type))
		defer bldr.Release()

		vs := make([]arrow.Time32, len(a))
		for i, v := range a {
			vs[i] = arrow.Time32(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []time64ns:
		bldr := array.NewTime64Builder(mem, arrow.FixedWidthTypes.Time64ns.(*arrow.Time64Type))
		defer bldr.Release()

		vs := make([]arrow.Time64, len(a))
		for i, v := range a {
			vs[i] = arrow.Time64(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []time64us:
		bldr := array.NewTime64Builder(mem, arrow.FixedWidthTypes.Time64us.(*arrow.Time64Type))
		defer bldr.Release()

		vs := make([]arrow.Time64, len(a))
		for i, v := range a {
			vs[i] = arrow.Time64(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []timestamps:
		bldr := array.NewTimestampBuilder(mem, arrow.FixedWidthTypes.Timestamp_s.(*arrow.TimestampType))
		defer bldr.Release()

		vs := make([]arrow.Timestamp, len(a))
		for i, v := range a {
			vs[i] = arrow.Timestamp(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []timestampms:
		bldr := array.NewTimestampBuilder(mem, arrow.FixedWidthTypes.Timestamp_ms.(*arrow.TimestampType))
		defer bldr.Release()

		vs := make([]arrow.Timestamp, len(a))
		for i, v := range a {
			vs[i] = arrow.Timestamp(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []timestampus:
		bldr := array.NewTimestampBuilder(mem, arrow.FixedWidthTypes.Timestamp_us.(*arrow.TimestampType))
		defer bldr.Release()

		vs := make([]arrow.Timestamp, len(a))
		for i, v := range a {
			vs[i] = arrow.Timestamp(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []timestampns:
		bldr := array.NewTimestampBuilder(mem, arrow.FixedWidthTypes.Timestamp_ns.(*arrow.TimestampType))
		defer bldr.Release()

		vs := make([]arrow.Timestamp, len(a))
		for i, v := range a {
			vs[i] = arrow.Timestamp(v)
		}
		bldr.AppendValues(vs, valids)
		return bldr.NewArray()

	case []arrow.Date32:
		bldr := array.NewDate32Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewArray()

	case []arrow.Date64:
		bldr := array.NewDate64Builder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewArray()

	case []arrow.MonthInterval:
		bldr := array.NewMonthIntervalBuilder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewArray()

	case []arrow.DayTimeInterval:
		bldr := array.NewDayTimeIntervalBuilder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewArray()

	case []arrow.MonthDayNanoInterval:
		bldr := array.NewMonthDayNanoIntervalBuilder(mem)
		defer bldr.Release()

		bldr.AppendValues(a, valids)
		return bldr.NewArray()

	default:
		panic(fmt.Errorf("arrdata: invalid data slice type %T", a))
	}
}

func structOf(mem memory.Allocator, dtype *arrow.StructType, fields [][]arrow.Array, valids []bool) *array.Struct {
	if mem == nil {
		mem = memory.NewGoAllocator()
	}

	bldr := array.NewStructBuilder(mem, dtype)
	defer bldr.Release()

	if valids == nil {
		valids = make([]bool, fields[0][0].Len())
		for i := range valids {
			valids[i] = true
		}
	}

	for i := range fields {
		bldr.AppendValues(valids)
		for j := range dtype.Fields() {
			fbldr := bldr.FieldBuilder(j)
			buildArray(fbldr, fields[i][j])
		}
	}

	return bldr.NewStructArray()
}

func buildArray(bldr array.Builder, data arrow.Array) {
	defer data.Release()

	switch bldr := bldr.(type) {
	case *array.BooleanBuilder:
		data := data.(*array.Boolean)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Int8Builder:
		data := data.(*array.Int8)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Int16Builder:
		data := data.(*array.Int16)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Int32Builder:
		data := data.(*array.Int32)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Int64Builder:
		data := data.(*array.Int64)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Uint8Builder:
		data := data.(*array.Uint8)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Uint16Builder:
		data := data.(*array.Uint16)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Uint32Builder:
		data := data.(*array.Uint32)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Uint64Builder:
		data := data.(*array.Uint64)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Float32Builder:
		data := data.(*array.Float32)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.Float64Builder:
		data := data.(*array.Float64)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.StringBuilder:
		data := data.(*array.String)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}

	case *array.LargeStringBuilder:
		data := data.(*array.LargeString)
		for i := 0; i < data.Len(); i++ {
			switch {
			case data.IsValid(i):
				bldr.Append(data.Value(i))
			default:
				bldr.AppendNull()
			}
		}
	}
}
