// Copyright 2024 Ant Group Co., Ltd.
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

package utils

import (
	"reflect"
	"strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const desensitizedValue = "***"

// StructToMap is a function that converts a struct to a map with sensitive data obfuscated.
func StructToMap(s interface{}, sensitiveFields ...string) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{}
	}
	return structToMap(s, getFieldTypeNameWithPbJSONTag, sensitiveFields...)
}

type fieldTypeNameFunc func(t reflect.StructField) string

func getFieldTypeNameWithPbJSONTag(fieldType reflect.StructField) string {
	jsonTag := fieldType.Tag.Get("json")
	protobufTag := fieldType.Tag.Get("protobuf")

	if jsonTag == "" || jsonTag == "-" || protobufTag == "" {
		return ""
	}
	return strings.Split(jsonTag, ",")[0]
}

func structToMap(s any, nameFunc fieldTypeNameFunc, sensitiveFields ...string) map[string]interface{} {
	var result = map[string]interface{}{}
	// Get the reflection value of a struct.
	value := reflect.ValueOf(s)
	// Get the reflection type of struct.
	var typeof reflect.Type
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
		if !value.IsValid() {
			return result
		}
	}
	typeof = value.Type()

	if typeof.Kind() == reflect.Map {
		for _, key := range value.MapKeys() {
			keyStr := key.String()
			val := value.MapIndex(key).Interface()
			result[keyStr] = val
		}
		filterMapDesensitizeKey(result, nameFunc, sensitiveFields)
		return result
	}

	if typeof.Kind() != reflect.Struct {
		return result
	}
	// Traverse all fields of a struct.
	for i := 0; i < typeof.NumField(); i++ {
		// Get the reflection value of a field.
		fieldValue := value.Field(i)
		// Get the reflection type of field.
		fieldType := typeof.Field(i)
		fieldName := nameFunc(fieldType)
		fieldTypeKind := fieldType.Type.Kind()

		if fieldName == "" {
			continue
		}
		if fieldTypeKind == reflect.Pointer {
			fieldValue = fieldValue.Elem()
			fieldTypeKind = fieldType.Type.Elem().Kind()
		}

		if !fieldValue.IsValid() {
			continue
		}
		fieldInterface := fieldValue.Interface()
		switch fieldTypeKind {
		case reflect.Struct:
			result[fieldName] = structToMap(fieldInterface, nameFunc, sensitiveFields...)
		case reflect.Map:
			if stringMap, ok := fieldInterface.(map[string]string); ok {
				interfaceMap := make(map[string]interface{})
				for key, value := range stringMap {
					interfaceMap[key] = value
				}
				result[fieldName] = filterMapDesensitizeKey(interfaceMap, nameFunc, sensitiveFields)
			} else if interfaceMap, ok := fieldInterface.(map[string]interface{}); ok {
				result[fieldName] = filterMapDesensitizeKey(interfaceMap, nameFunc, sensitiveFields)
			} else {
				nlog.Warnf("FieldName: %s, can not transform to `map[string]string` or `map[string]interface{}`: %v", fieldName, fieldInterface)
			}
		case reflect.String:
			if str := desensitizeFieldOrKeyValue(fieldName, fieldInterface, sensitiveFields); str != "" {
				result[fieldName] = str
			}
		case reflect.Slice:
			interfaceMap := map[string]interface{}{
				fieldName: fieldInterface,
			}
			result[fieldName] = filterMapDesensitizeKey(interfaceMap, nameFunc, sensitiveFields)[fieldName]
		default:
			result[fieldName] = desensitizeFieldOrKeyValue(fieldName, fieldInterface, sensitiveFields)
		}
	}
	return result
}

func filterMapDesensitizeKey(mapData map[string]interface{}, nameFunc fieldTypeNameFunc, sensitiveFields []string) map[string]interface{} {
	if len(sensitiveFields) == 0 {
		return mapData
	}
	for key, val := range mapData {

		switch value := val.(type) {
		case string:
			if value != "" {
				mapData[key] = desensitizeFieldOrKeyValue(key, value, sensitiveFields)
			}
		case map[string]interface{}:
			mapData[key] = filterMapDesensitizeKey(value, nameFunc, sensitiveFields)
		case []interface{}:
			var newSubSlice []map[string]interface{}
			for _, sliceValue := range value {
				newSubSlice = append(newSubSlice, structToMap(sliceValue, nameFunc, sensitiveFields...))
			}
			mapData[key] = newSubSlice
		case []map[string]interface{}:
			var newSubSlice []map[string]interface{}
			for _, sliceValue := range value {
				newSubSlice = append(newSubSlice, filterMapDesensitizeKey(sliceValue, nameFunc, sensitiveFields))
			}
			mapData[key] = newSubSlice
		default:
			if reflect.ValueOf(val).Kind() == reflect.Struct {
				mapData[key] = structToMap(val, nameFunc, sensitiveFields...)
			} else {
				mapData[key] = desensitizeFieldOrKeyValue(key, value, sensitiveFields)
			}
		}
	}
	return mapData
}

func desensitizeFieldOrKeyValue(fieldOrKeyName string, v any, sensitiveFields []string) any {
	fieldInterface := v
	for _, sensitiveField := range sensitiveFields {
		if sensitiveField == fieldOrKeyName {
			fieldInterface = desensitizedValue
			break
		}
	}
	return fieldInterface
}
