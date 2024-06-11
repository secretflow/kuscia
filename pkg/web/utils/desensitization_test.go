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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type testStruct struct {
	Name     string
	Age      int
	Address  string
	Password string                 `protobuf:"bytes,2,opt,name=password,json=password,proto3" json:"password,omitempty"`
	Number1  int                    `protobuf:"bytes,2,opt,name=number1,json=number1,proto3" json:"number1,omitempty"`
	Map      map[string]interface{} `protobuf:"bytes,2,opt,name=map,json=map,proto3" json:"map,omitempty"`
	Slice    []int                  `protobuf:"bytes,2,opt,name=slice,json=slice,proto3" json:"slice,omitempty"`
}

func TestStructToMap_Pb(t *testing.T) {

	dataSourceName := "test"

	createDomainDataSourceRequest := kusciaapi.CreateDomainDataSourceRequest{
		Header:   nil,
		DomainId: "",
		Name:     &dataSourceName,
		Info: &kusciaapi.DataSourceInfo{
			Oss: &kusciaapi.OssDataSourceInfo{
				Endpoint:        "endpoint",
				Bucket:          "bucket",
				AccessKeyId:     "accessKeyId",
				AccessKeySecret: "accessKeySecret",
			},
		},
	}
	var result = StructToMap(&createDomainDataSourceRequest, "bucket", "access_key_id", "access_key_secret")

	expected := map[string]interface{}{
		"name": dataSourceName,
		"info": map[string]interface{}{
			"oss": map[string]interface{}{
				"endpoint":          "endpoint",
				"bucket":            desensitizedValue,
				"access_key_id":     desensitizedValue,
				"access_key_secret": desensitizedValue,
				"virtualhost":       false,
			},
		},
	}
	assert.EqualValues(t, result, expected)
}
func TestStructToMap_Map(t *testing.T) {

	var m = make(map[string]interface{})
	m["a"] = 1
	m["b"] = 2
	m["c"] = 3

	var result = StructToMap(m, "b")
	expected := map[string]interface{}{
		"a": 1,
		"b": desensitizedValue,
		"c": 3,
	}
	assert.EqualValues(t, result, expected)
}
func TestStructToMap_MapInterface(t *testing.T) {

	var m = make(map[string]interface{})
	m["a"] = 1
	m["b"] = map[string]string{
		"d": "d",
		"e": "e",
	}
	m["c"] = 3

	var result = StructToMap(m, "b", "d")
	expected := map[string]interface{}{
		"a": 1,
		"b": desensitizedValue,
		"c": 3,
	}
	assert.EqualValues(t, result, expected)
}

func TestStructToMap_MapArray(t *testing.T) {

	var m = make(map[string]interface{})
	m["a"] = 1
	m["b"] = []map[string]interface{}{
		{
			"d": "d",
			"e": "e",
		},
	}
	m["c"] = 3

	var result = StructToMap(m, "d")
	expected := map[string]interface{}{
		"a": 1,
		"b": []map[string]interface{}{
			{
				"d": desensitizedValue,
				"e": "e",
			},
		},
		"c": 3,
	}
	assert.EqualValues(t, result, expected)
}

func TestStructToMap_MapStruct(t *testing.T) {
	dataSourceName := "test"

	m := map[string]interface{}{
		"a": 1,
		"b": []map[string]interface{}{
			{
				"d": "d",
				"e": "e",
			},
		},
		"c": kusciaapi.CreateDomainDataSourceRequest{
			Header:   nil,
			DomainId: "",
			Name:     &dataSourceName,
			Info: &kusciaapi.DataSourceInfo{
				Oss: &kusciaapi.OssDataSourceInfo{
					Endpoint:        "endpoint",
					Bucket:          "bucket",
					AccessKeyId:     "accessKeyId",
					AccessKeySecret: "accessKeySecret",
				},
			},
		},
	}

	var result = StructToMap(m, "d", "bucket", "access_key_id", "access_key_secret")
	expected := map[string]interface{}{
		"a": 1,
		"b": []map[string]interface{}{
			{
				"d": desensitizedValue,
				"e": "e",
			},
		},
		"c": map[string]interface{}{
			"name": dataSourceName,
			"info": map[string]interface{}{
				"oss": map[string]interface{}{
					"endpoint":          "endpoint",
					"bucket":            desensitizedValue,
					"access_key_id":     desensitizedValue,
					"access_key_secret": desensitizedValue,
					"virtualhost":       false,
				},
			},
		},
	}
	assert.EqualValues(t, result, expected)
}
func TestStructToMap_StructMap(t *testing.T) {
	myStruct := testStruct{
		Name:     "John",
		Age:      30,
		Address:  "123 Main St",
		Password: "password",
		Map: map[string]interface{}{
			"a": "1",
			"b": "2",
		},
		Slice: make([]int, 0),
	}

	result := StructToMap(myStruct, "password", "a")
	expected := map[string]interface{}{
		"password": desensitizedValue,
		"number1":  0,
		"map": map[string]interface{}{
			"a": desensitizedValue,
			"b": "2",
		},
		"slice": []int{},
	}
	assert.EqualValues(t, result, expected)
}

func TestStructToMap_PbStream(t *testing.T) {

	requestHeader := v1alpha1.RequestHeader{
		CustomHeaders: map[string]string{
			"hello": "hello",
		},
	}
	request := kusciaapi.WatchJobRequest{
		TimeoutSeconds: 10,
		Header:         &requestHeader,
	}

	var result = StructToMap(&request, "bucket", "access_key_id", "access_key_secret")

	expected := map[string]interface{}{
		"header": map[string]interface{}{
			"custom_headers": map[string]interface{}{
				"hello": "hello",
			},
		},
		"timeout_seconds": int64(10),
	}
	assert.EqualValues(t, result, expected)

}

func TestStructToMap_StructWithNumberSlice(t *testing.T) {
	myStruct := testStruct{
		Name:     "John",
		Age:      30,
		Address:  "123 Main St",
		Password: "password",
		Number1:  1,
		Slice:    []int{1, 2, 3},
		Map:      map[string]interface{}{},
	}

	result := StructToMap(myStruct, "password", "number1")
	expected := map[string]interface{}{
		"password": desensitizedValue,
		"number1":  desensitizedValue,
		"slice":    []int{1, 2, 3},
		"map":      map[string]interface{}{},
	}
	assert.EqualValues(t, result, expected)
}
