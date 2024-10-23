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

package confloader

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	LookUpModeYamlTag = "yaml"
	LookUpModeField   = "field"
)

var lookUpFuncs = map[string]structForwardFunc{
	LookUpModeYamlTag: structYamlTagForward,
	LookUpModeField:   structFiledForward,
}

// GetValue get value from obj via fieldPath which is like jsonpath.
func GetValue(obj any, oPath string, mode string) (value any, err error) {
	defer func() {
		if r := recover(); r != nil {
			nlog.Errorf("GetValue failed with obj=%s, fieldPath=%s, recover msg=%s", obj, oPath, r)
			err = fmt.Errorf("unexpected error: %s", r)
		}
	}()
	loopUpFunc, exist := lookUpFuncs[mode]
	if !exist {
		return nil, fmt.Errorf("can not found lookup func")
	}
	reflectValue, err := getReflectValue(obj, oPath, loopUpFunc)
	if err != nil {
		return nil, err
	}
	if !reflectValue.CanInterface() {
		return nil, fmt.Errorf("got value but can not interface")
	}
	return reflectValue.Interface(), err
}

// SetValue set value to obj via fieldPath which is like jsonpath. The value type should be same as the target type.
// The obj should be addressable.
func SetValue(obj any, oPath string, mode string, value any) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			nlog.Errorf("GetValue failed with obj=%s, fieldPath=%s, recover msg=%s", obj, oPath, r)
			err = fmt.Errorf("unexpected error: %s", r)
		}
	}()
	loopUpFunc, exist := lookUpFuncs[mode]
	if !exist {
		return fmt.Errorf("can not found lookup func")
	}
	reflectValue, err := getReflectValue(obj, oPath, loopUpFunc)
	if err != nil {
		return err
	}
	if !reflectValue.CanSet() {
		return fmt.Errorf("got value but can not set")
	}
	if reflectValue.Type() != reflect.ValueOf(value).Type() {
		return fmt.Errorf("got value but type not match")
	}
	reflectValue.Set(reflect.ValueOf(value))
	return err
}

func getReflectValue(obj any, oPath string, structForward structForwardFunc) (reflect.Value, error) {
	subPaths := strings.Split(oPath, ".")
	confValue := reflect.ValueOf(obj)
	var err error
	for _, p := range subPaths {
		confValue, err = forward(confValue, p, structForward)
		if err != nil {
			return reflect.Value{}, err
		}
	}
	return confValue, nil
}

func isTerminatedKind(v reflect.Value) bool {
	kind := v.Type().Kind()
	if kind == reflect.Pointer {
		kind = v.Elem().Kind()
	}
	if kind == reflect.Array || kind == reflect.Slice || kind == reflect.Map || kind == reflect.Struct {
		return false
	}
	return true
}

func forward(v reflect.Value, subPath string, structForward structForwardFunc) (reflect.Value, error) {
	if isTerminatedKind(v) {
		return reflect.Value{}, fmt.Errorf("unexpect terminated kind when forward")
	}
	if v.Type().Kind() == reflect.Pointer {
		v = v.Elem()
	}
	switch v.Type().Kind() {
	case reflect.Array, reflect.Slice:
		index, err := strconv.Atoi(subPath)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("find array/slice kind but subPath=%s is not int", subPath)
		}
		if index < 0 || index >= v.Len() {
			return reflect.Value{}, fmt.Errorf("find array/slice kind but out of index, len=%d", v.Len())
		}
		return v.Index(index), nil
	case reflect.Map:
		if !mapHasKey(v, subPath) {
			return reflect.Value{}, fmt.Errorf("find map kind but key not exist")
		}
		return v.MapIndex(reflect.ValueOf(subPath)), nil
	case reflect.Struct:
		return structForward(v, subPath)
	default:
		return reflect.Value{}, fmt.Errorf("unexpect terminated kind when forward, got type=%s, subPath=%s", v.Type().Kind(), subPath)
	}
}

// param v's must be Map.
func mapHasKey(v reflect.Value, key any) bool {
	keys := v.MapKeys()
	for _, k := range keys {
		if k.CanInterface() && k.Interface() == key {
			return true
		}
	}
	return false
}

func structFiledForward(v reflect.Value, subPath string) (reflect.Value, error) {
	field, found := v.Type().FieldByName(subPath)
	if !found {
		return reflect.Value{}, fmt.Errorf("find struct kind but has no field named=%s", subPath)
	}
	if !field.IsExported() {
		return reflect.Value{}, fmt.Errorf("find struct kind but field is not exported")
	}
	return v.FieldByName(subPath), nil
}

func structYamlTagForward(v reflect.Value, subPath string) (reflect.Value, error) {
	for i := 0; i < v.Type().NumField(); i++ {
		value, exist := v.Type().Field(i).Tag.Lookup("yaml")
		if strings.Contains(value, ",") {
			value = strings.Split(value, ",")[0]
		}
		if exist && value == subPath {
			if !v.Type().Field(i).IsExported() {
				return reflect.Value{}, fmt.Errorf("find struct kind but field is not exported")
			}
			return v.Field(i), nil
		}
	}

	return reflect.Value{}, fmt.Errorf("for sturct tag=%s not found", subPath)
}

type structForwardFunc func(v reflect.Value, subPath string) (reflect.Value, error)
