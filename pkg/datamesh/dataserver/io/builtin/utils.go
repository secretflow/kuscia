// Copyright 2025 Ant Group Co., Ltd.
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

package builtin

import (
	"fmt"
	"github.com/apache/arrow/go/v13/arrow/array"
	"strconv"
)

// following parse functions are copied and modified from arrow v13@v13.0.0 /csv/reader
func ParseBool(field array.Builder, str string) error {
	v, err := strconv.ParseBool(str)
	if err != nil {
		field.AppendNull()
		return fmt.Errorf("%w: unrecognized boolean: %s", err, str)
	}

	field.(*array.BooleanBuilder).Append(v)
	return nil
}

func ParseInt8(field array.Builder, str string) error {
	v, err := strconv.ParseInt(str, 10, 8)
	if err != nil {
		field.AppendNull()
		return err
	}

	field.(*array.Int8Builder).Append(int8(v))
	return nil
}

func ParseInt16(field array.Builder, str string) error {
	v, err := strconv.ParseInt(str, 10, 16)
	if err != nil {
		field.AppendNull()
		return err
	}

	field.(*array.Int16Builder).Append(int16(v))
	return nil
}

func ParseInt32(field array.Builder, str string) error {
	v, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		field.AppendNull()
		return err
	}
	field.(*array.Int32Builder).Append(int32(v))
	return nil
}

func ParseInt64(field array.Builder, str string) error {
	v, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		field.AppendNull()
		return err
	}

	field.(*array.Int64Builder).Append(v)
	return nil
}

func ParseUint8(field array.Builder, str string) error {
	v, err := strconv.ParseUint(str, 10, 8)
	if err != nil {
		field.AppendNull()
		return err
	}
	field.(*array.Uint8Builder).Append(uint8(v))
	return nil
}

func ParseUint16(field array.Builder, str string) error {
	v, err := strconv.ParseUint(str, 10, 16)
	if err != nil {
		field.AppendNull()
		return err
	}
	field.(*array.Uint16Builder).Append(uint16(v))
	return nil
}

func ParseUint32(field array.Builder, str string) error {
	v, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		field.AppendNull()
		return err
	}

	field.(*array.Uint32Builder).Append(uint32(v))
	return nil
}

func ParseUint64(field array.Builder, str string) error {
	v, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		field.AppendNull()
		return err
	}
	field.(*array.Uint64Builder).Append(v)
	return nil
}

func ParseFloat32(field array.Builder, str string) error {
	v, err := strconv.ParseFloat(str, 32)
	if err != nil {
		field.AppendNull()
		return err
	}
	field.(*array.Float32Builder).Append(float32(v))
	return nil
}

func ParseFloat64(field array.Builder, str string) error {
	v, err := strconv.ParseFloat(str, 64)
	if err != nil {
		field.AppendNull()
		return err
	}
	field.(*array.Float64Builder).Append(v)
	return nil
}
