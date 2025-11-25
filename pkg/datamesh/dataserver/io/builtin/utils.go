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
	"strconv"
	"time"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
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

func ParseStr2Date32(bldr array.Builder, str string) error {
	if str == "" || str == "NULL" {
		bldr.(*array.Date32Builder).AppendNull()
		return nil
	}

	t, err := time.Parse("2006-01-02", str)
	if err != nil {
		return err
	}
	bldr.(*array.Date32Builder).Append(arrow.Date32(t.Unix() / (24 * 3600)))
	return nil
}
func ParseStr2Date64(bldr array.Builder, str string) error {
	if str == "" || str == "NULL" {
		bldr.(*array.Date64Builder).AppendNull()
		return nil
	}

	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999",
		"2006-01-02",
	}

	var t time.Time
	var err error
	for _, format := range formats {
		t, err = time.Parse(format, str)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	bldr.(*array.Date64Builder).Append(arrow.Date64(t.UnixMilli()))
	return nil
}
func ParseStr2Time32(bldr array.Builder, str string) error {
	if str == "" || str == "NULL" {
		bldr.(*array.Time32Builder).AppendNull()
		return nil
	}

	t, err := time.Parse("15:04:05", str)
	if err != nil {
		return err
	}
	seconds := int32(t.Hour()*3600 + t.Minute()*60 + t.Second())
	bldr.(*array.Time32Builder).Append(arrow.Time32(seconds))
	return nil
}
func ParseStr2Time64(bldr array.Builder, str string) error {
	if str == "" || str == "NULL" {
		bldr.(*array.Time64Builder).AppendNull()
		return nil
	}

	t, err := time.Parse("15:04:05.999999", str)
	if err != nil {
		t, err = time.Parse("15:04:05", str)
		if err != nil {
			return err
		}
	}

	microseconds := int64(t.Hour()*3600+t.Minute()*60+t.Second())*1_000_000 + int64(t.Nanosecond()/1000)
	bldr.(*array.Time64Builder).Append(arrow.Time64(microseconds))
	return nil
}

func ParseStr2Timestamp(bldr array.Builder, str string) error {
	if str == "" || str == "NULL" {
		bldr.(*array.TimestampBuilder).AppendNull()
		return nil
	}

	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02",
	}

	var t time.Time
	var err error
	for _, format := range formats {
		t, err = time.Parse(format, str)
		if err == nil {
			break
		}
	}
	if err != nil {
		// Try the digital format
		if ts, parseErr := strconv.ParseInt(str, 10, 64); parseErr == nil {
			bldr.(*array.TimestampBuilder).Append(arrow.Timestamp(ts))
			return nil
		}
		return err
	}

	bldr.(*array.TimestampBuilder).Append(arrow.Timestamp(t.Unix()))
	return nil
}
