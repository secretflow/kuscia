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

package config

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

type SourceType string

const (
	SourceFlag SourceType = "flag"
	SourceEnv  SourceType = "env"
)

type (
	// Setter sets the object with a string value.
	Setter interface {
		// Set sets the object with a string value.
		Set(value string) error
	}
)

// FlagEnvConfigLoader provides the function of obtaining config from command flags and environment variables.
type FlagEnvConfigLoader struct {
	// Config Source: env/flag
	Source SourceType
}

func (c *FlagEnvConfigLoader) RegisterFlags(conf framework.Config, name string, fs *pflag.FlagSet) {
	// Default source type - flag.
	if c.Source == "" {
		c.Source = SourceFlag
	}
	c.setDefaultValue(conf)
	// Do not inject flag when source is env.
	if c.Source == SourceFlag {
		flags(name, conf, fs)
	}
}

// Process Configuration Load. The env/envflag type config is obtained from the environment variable,
// and the value in the envflag type config is processed and replaced according to the priority.
func (c *FlagEnvConfigLoader) Process(conf framework.Config, name string, errs *errorcode.Errs) {
	if c.Source == SourceEnv {
		getFromEnv(name, conf, errs)
	}
}

// setDefaultValue get the value with "default" tag and put it into config
func (c *FlagEnvConfigLoader) setDefaultValue(conf framework.Config) {
	typeElem := reflect.TypeOf(conf).Elem()
	valueElem := reflect.ValueOf(conf).Elem()

	for i := 0; i < typeElem.NumField(); i++ {
		valueField := valueElem.Field(i)
		typeField := typeElem.Field(i)
		// Get tags. If no corresponding tags exist, skip.
		tag := typeField.Tag
		if _, ok := tag.Lookup("name"); !ok {
			continue
		}
		defaultVal, ok := tag.Lookup("default")
		if !ok {
			continue
		}
		_ = setValue(valueField, defaultVal)
	}
}

// Automatically inject flags for Config. Config must be of pointer type.
func flags(name string, config framework.Config, fs *pflag.FlagSet) {
	typeElem := reflect.TypeOf(config).Elem()
	valueElem := reflect.ValueOf(config).Elem()
	nfsValue := reflect.ValueOf(fs)

	for i := 0; i < typeElem.NumField(); i++ {
		valueField := valueElem.Field(i)
		typeField := typeElem.Field(i)
		// Get tags. If no corresponding tags exist, skip.
		tag := typeField.Tag
		fieldName, ok := tag.Lookup("name")
		if !ok {
			continue
		}
		usage := tag.Get("usage")
		usage = fmt.Sprintf("%s: %s", name, usage)

		// Select the injection method according to the field type.
		var funcName string
		switch valueField.Interface().(type) {
		case bool:
			funcName = "BoolVar"
		case []bool:
			funcName = "BoolSliceVar"
		case []byte:
			funcName = "BytesHexVar"
		case time.Duration:
			funcName = "DurationVar"
		case []time.Duration:
			funcName = "DurationSliceVar"
		case float32:
			funcName = "Float32Var"
		case []float32:
			funcName = "Float32SliceVar"
		case float64:
			funcName = "Float64Var"
		case []float64:
			funcName = "Float64SliceVar"
		case int:
			funcName = "IntVar"
		case []int:
			funcName = "IntSliceVar"
		case int8:
			funcName = "Int8Var"
		case []int8:
			funcName = "Int8SliceVar"
		case int16:
			funcName = "Int16Var"
		case []int16:
			funcName = "Int16SliceVar"
		case int32:
			funcName = "Int32Var"
		case []int32:
			funcName = "Int32SliceVar"
		case int64:
			funcName = "Int64Var"
		case []int64:
			funcName = "Int64SliceVar"
		case net.IP:
			funcName = "IPVar"
		case []net.IP:
			funcName = "IPSliceVar"
		case net.IPMask:
			funcName = "IPMaskVar"
		case []net.IPMask:
			funcName = "IPMaskSliceVar"
		case net.IPNet:
			funcName = "IPNetVar"
		case []net.IPNet:
			funcName = "IPNetSliceVar"
		case string:
			funcName = "StringVar"
		case []string:
			funcName = "StringSliceVar"
		case uint:
			funcName = "UintVar"
		case []uint:
			funcName = "Uint64Var"
		case uint8:
			funcName = "Uint8Var"
		case uint16:
			funcName = "Uint16Var"
		case uint32:
			funcName = "Uint32Var"
		case uint64:
			funcName = "Uint64Var"
		default:

		}
		if len(funcName) != 0 {
			_, required := tag.Lookup("required")
			if required {
				usage = usage + "\t[required]"
			}
			flagName := getFlagName(name, fieldName)
			nfsValue.MethodByName(funcName).Call([]reflect.Value{valueField.Addr(), reflect.ValueOf(flagName), valueField, reflect.ValueOf(usage)})
			// set required tag
			if required {
				cobra.MarkFlagRequired(fs, flagName)
			}
		}
	}
}

// getFromEnv gets the environment variable to replace the value in config.
func getFromEnv(name string, config framework.Config, errs *errorcode.Errs) {
	typeElem := reflect.TypeOf(config).Elem()
	valueElem := reflect.ValueOf(config).Elem()

	for i := 0; i < typeElem.NumField(); i++ {
		valueField := valueElem.Field(i)
		typeField := typeElem.Field(i)
		if !valueField.CanSet() {
			continue
		}
		// Get tags. If no corresponding tags exist, skip.
		tag := typeField.Tag
		fieldName, ok := tag.Lookup("name")
		if !ok {
			continue
		}

		envName := getEnvName(name, fieldName)
		val, ok := os.LookupEnv(envName)
		if !ok {
			if _, ok = tag.Lookup("required"); ok {
				errs.AppendErr(fmt.Errorf("can not get equired config (%s)", envName))
			}
			continue
		}
		errs.AppendErr(setValue(valueField, val))
	}
}

func getEnvName(group string, key string) string {
	return fmt.Sprintf("%s_%s", strings.ToUpper(strings.ReplaceAll(group, "-", "_")), strings.ToUpper(strings.ReplaceAll(key, "-", "_")))
}

func getFlagName(group string, key string) string {
	return fmt.Sprintf("%s-%s", group, key)
}

// setValue assigns a string value to a reflection value using appropriate string parsing and conversion logic.
func setValue(rval reflect.Value, value string) error {
	rval = indirect(rval)
	rtype := rval.Type()

	if !rval.CanAddr() {
		return errors.New("the value is unaddressable")
	}

	// if the reflection value implements supported interface, use the interface to set the value
	pval := rval.Addr().Interface()
	if p, ok := pval.(Setter); ok {
		return p.Set(value)
	}
	if p, ok := pval.(encoding.TextUnmarshaler); ok {
		return p.UnmarshalText([]byte(value))
	}
	if p, ok := pval.(encoding.BinaryUnmarshaler); ok {
		return p.UnmarshalBinary([]byte(value))
	}

	// parse the string according to the type of the reflection value and assign it
	switch rtype.Kind() {
	case reflect.String:
		rval.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(value, 0, rtype.Bits())
		if err != nil {
			return err
		}

		rval.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(value, 0, rtype.Bits())
		if err != nil {
			return err
		}
		rval.SetUint(val)
	case reflect.Bool:
		val, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		rval.SetBool(val)
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(value, rtype.Bits())
		if err != nil {
			return err
		}
		rval.SetFloat(val)
	case reflect.Slice:
		if rtype.Elem().Kind() == reflect.Uint8 {
			sl := reflect.ValueOf([]byte(value))
			rval.Set(sl)
			return nil
		}
		fallthrough
	default:
		// assume the string is in JSON format for non-basic types
		return json.Unmarshal([]byte(value), rval.Addr().Interface())
	}

	return nil
}

// indirect dereferences pointers and returns the actual value it points to.
// If a pointer is nil, it will be initialized with a new value.
func indirect(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}
