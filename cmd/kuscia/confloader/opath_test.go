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
	"reflect"
	"testing"
)

type example struct {
	StrValue  string `yaml:"strValue,omitempty"`
	IntValue  int    `yaml:"intValue,omitempty"`
	BoolValue bool   `yaml:"boolValue,omitempty"`

	ArrayValue   []string            `yaml:"arrayValue,omitempty"`
	SliceValue   []int               `yaml:"sliceValue,omitempty"`
	MapStrValue  map[string]example2 `yaml:"mapStrValue,omitempty"`
	PointerValue *example            `yaml:"pointerValue,omitempty"`

	StructValue      example2              `yaml:"structValue,omitempty"`
	SliceMapStrValue []map[string]example2 `yaml:"sliceMapStrValue,omitempty"`

	NullValue *example `yaml:"nullValue,omitempty"`

	unexported string `yaml:"unexported,omitempty"`
}

type example2 struct {
	StrValue string `yaml:"strValue"`
}

func TestGetValue_Level_1(t *testing.T) {
	type args struct {
		obj         any
		fieldPath   string
		yamlTagPath string
	}
	tests := []struct {
		name    string
		args    args
		want    any
		wantErr bool
	}{
		{
			name: "string value level 1 should return success",
			args: args{
				obj: example{
					StrValue: "abc",
				},
				fieldPath:   "StrValue",
				yamlTagPath: "strValue",
			},
			want:    "abc",
			wantErr: false,
		},
		{
			name: "int value level 1 should return success",
			args: args{
				obj: example{
					IntValue: 123,
				},
				fieldPath:   "IntValue",
				yamlTagPath: "intValue",
			},
			want:    123,
			wantErr: false,
		},
		{
			name: "bool value level 1 should return success",
			args: args{
				obj: example{
					BoolValue: false,
				},
				fieldPath:   "BoolValue",
				yamlTagPath: "boolValue",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "array value level 1 should return success",
			args: args{
				obj: example{
					ArrayValue: []string{"abc", "def"},
				},
				fieldPath:   "ArrayValue",
				yamlTagPath: "arrayValue",
			},
			want:    []string{"abc", "def"},
			wantErr: false,
		},
		{
			name: "slice value level 1 should return success",
			args: args{
				obj: example{
					SliceValue: []int{123, 456},
				},
				fieldPath:   "SliceValue",
				yamlTagPath: "sliceValue",
			},
			want:    []int{123, 456},
			wantErr: false,
		},
		{
			name: "map value level 1 should return success",
			args: args{
				obj: example{
					MapStrValue: map[string]example2{
						"abc": {StrValue: "2abc"},
					},
				},
				fieldPath:   "MapStrValue",
				yamlTagPath: "mapStrValue",
			},
			want: map[string]example2{
				"abc": {StrValue: "2abc"},
			},
			wantErr: false,
		},
		{
			name: "pointer value level 1 should return success",
			args: args{
				obj: example{
					PointerValue: &example{
						StrValue: "2abc",
					},
				},
				fieldPath:   "PointerValue",
				yamlTagPath: "pointerValue",
			},
			want: &example{
				StrValue: "2abc",
			},
			wantErr: false,
		},
		{
			name: "null value level 1 should return success",
			args: args{
				obj:         example{},
				fieldPath:   "NullValue",
				yamlTagPath: "nullValue",
			},
			want:    (*example)(nil),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1, err := GetValue(tt.args.obj, tt.args.fieldPath, LookUpModeField)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got1, tt.want) {
					t.Errorf("GetValue() got1 = %v, want %v", got1, tt.want)
				}
			}
			got2, err := GetValue(tt.args.obj, tt.args.yamlTagPath, LookUpModeYamlTag)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got2, tt.want) {
					t.Errorf("GetValue() got2 = %v, want %v", got2, tt.want)
				}
			}
		})
	}
}

func TestGetValue_Level_Multi(t *testing.T) {
	type args struct {
		obj         any
		fieldPath   string
		yamlTagPath string
	}
	tests := []struct {
		name    string
		args    args
		want    any
		wantErr bool
	}{
		{
			name: "array value level 2 should return success",
			args: args{
				obj: example{
					ArrayValue: []string{"abc", "def"},
				},
				fieldPath:   "ArrayValue.1",
				yamlTagPath: "arrayValue.1",
			},
			want:    "def",
			wantErr: false,
		},
		{
			name: "slice value level 2 should return success",
			args: args{
				obj: example{
					SliceValue: []int{123, 456},
				},
				fieldPath:   "SliceValue.1",
				yamlTagPath: "sliceValue.1",
			},
			want:    456,
			wantErr: false,
		},
		{
			name: "map value level 3 should return success",
			args: args{
				obj: example{
					MapStrValue: map[string]example2{
						"abc": {StrValue: "2abc"},
					},
				},
				fieldPath:   "MapStrValue.abc.StrValue",
				yamlTagPath: "mapStrValue.abc.strValue",
			},
			want:    "2abc",
			wantErr: false,
		},
		{
			name: "map value level 4 should return success",
			args: args{
				obj: example{
					SliceMapStrValue: []map[string]example2{
						{"abc": {StrValue: "2abc"}},
						{"def": {StrValue: "3def"}},
					},
				},

				fieldPath:   "SliceMapStrValue.1.def.StrValue",
				yamlTagPath: "sliceMapStrValue.1.def.strValue",
			},
			want:    "3def",
			wantErr: false,
		},
		{
			name: "pointer value level 2 should return success",
			args: args{
				obj: example{
					PointerValue: &example{
						StrValue: "2abc",
					},
				},
				fieldPath:   "PointerValue.StrValue",
				yamlTagPath: "pointerValue.strValue",
			},
			want:    "2abc",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1, err := GetValue(tt.args.obj, tt.args.fieldPath, LookUpModeField)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got1, tt.want) {
					t.Errorf("GetValue() got1 = %v, want %v", got1, tt.want)
				}
			}
			got2, err := GetValue(tt.args.obj, tt.args.yamlTagPath, LookUpModeYamlTag)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got2, tt.want) {
					t.Errorf("GetValue() got2 = %v, want %v", got2, tt.want)
				}
			}
		})
	}
}

func TestGetValue_Invalid(t *testing.T) {
	type args struct {
		obj         any
		fieldPath   string
		yamlTagPath string
	}
	tests := []struct {
		name    string
		args    args
		want    any
		wantErr bool
	}{
		{
			name: "struct unexported field should return error",
			args: args{
				obj: example{
					unexported: "abc",
				},
				fieldPath:   "unexported",
				yamlTagPath: "unexported",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct no existed field should return error",
			args: args{
				obj: example{
					unexported: "abc",
				},
				fieldPath:   "NotExisted.1",
				yamlTagPath: "notExisted.1",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "array value out of index should return error",
			args: args{
				obj: example{
					ArrayValue: []string{"abc", "def"},
				},
				fieldPath:   "ArrayValue.2",
				yamlTagPath: "arrayValue.2",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "slice value out of index should return error",
			args: args{
				obj: example{
					SliceValue: []int{123, 456},
				},
				fieldPath:   "SliceValue.2",
				yamlTagPath: "sliceValue.2",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "map value not exist should return error",
			args: args{
				obj: example{
					MapStrValue: map[string]example2{
						"abc": {},
					},
				},
				fieldPath:   "MapStrValue.def",
				yamlTagPath: "mapStrValue.def",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "null value filed should return error",
			args: args{
				obj:         example{},
				fieldPath:   "NullValue.StrValue",
				yamlTagPath: "nullValue.strValue",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetValue(tt.args.obj, tt.args.fieldPath, LookUpModeField)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("GetValue() got = %v, want %v", got, tt.want)
				}
			}
			got2, err := GetValue(tt.args.obj, tt.args.fieldPath, LookUpModeField)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("GetValue() got2 = %v, want %v", got2, tt.want)
				}
			}
		})
	}
}

func TestSetValue_Level_1(t *testing.T) {
	type args struct {
		obj         any
		fieldPath   string
		yamlTagPath string
		value       any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "string value level 1 should return success",
			args: args{
				obj:         &example{},
				fieldPath:   "StrValue",
				yamlTagPath: "strValue",
				value:       "def",
			},
			wantErr: false,
		},
		{
			name: "int value level 1 should return success",
			args: args{
				obj:         &example{},
				fieldPath:   "IntValue",
				yamlTagPath: "intValue",
				value:       456,
			},
			wantErr: false,
		},
		{
			name: "bool value level 1 should return success",
			args: args{
				obj:         &example{},
				fieldPath:   "BoolValue",
				yamlTagPath: "boolValue",
				value:       true,
			},
			wantErr: false,
		},
		{
			name: "array value level 1 should return success",
			args: args{
				obj:         &example{},
				fieldPath:   "ArrayValue",
				yamlTagPath: "arrayValue",
				value:       []string{"def", "abc"},
			},
			wantErr: false,
		},
		{
			name: "slice value level 1 should return success",
			args: args{
				obj:         &example{},
				fieldPath:   "SliceValue",
				yamlTagPath: "sliceValue",
				value:       []int{456, 123},
			},
			wantErr: false,
		},
		{
			name: "map value level 1 should return success",
			args: args{
				obj:         &example{},
				fieldPath:   "MapStrValue",
				yamlTagPath: "mapStrValue",
				value: map[string]example2{
					"def": {StrValue: "2def"},
				},
			},
			wantErr: false,
		},
		{
			name: "map value level 1 should return success",
			args: args{
				obj:         &example{},
				fieldPath:   "PointerValue",
				yamlTagPath: "pointerValue",
				value: &example{
					StrValue: "2def",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetValue(tt.args.obj, tt.args.fieldPath, LookUpModeField, tt.args.value); (err != nil) != tt.wantErr {
				t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := SetValue(tt.args.obj, tt.args.yamlTagPath, LookUpModeYamlTag, tt.args.value); (err != nil) != tt.wantErr {
				t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetValue_Level_Multi(t *testing.T) {
	type args struct {
		obj         any
		fieldPath   string
		yamlTagPath string
		value       any
	}
	example1 := example{
		ArrayValue: []string{"abc", "def"},
		SliceValue: []int{123, 456},
		MapStrValue: map[string]example2{
			"abc": {StrValue: "2abc"},
		},
		SliceMapStrValue: []map[string]example2{
			{"abc": {StrValue: "2abc"}},
			{"def": {StrValue: "3def"}},
		},
		PointerValue: &example{
			StrValue: "2abc",
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "array value level 2 should return success",
			args: args{
				obj:         &example1,
				fieldPath:   "ArrayValue.1",
				yamlTagPath: "arrayValue.1",
				value:       "hgt",
			},
			wantErr: false,
		},
		{
			name: "slice value level 2 should return success",
			args: args{
				obj:         &example1,
				fieldPath:   "SliceValue.1",
				yamlTagPath: "sliceValue.1",
				value:       789,
			},
			wantErr: false,
		},
		{
			name: "pointer value level 2 should return success",
			args: args{
				obj:         &example1,
				fieldPath:   "PointerValue.StrValue",
				yamlTagPath: "pointerValue.strValue",
				value:       "3dc",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				if err := SetValue(tt.args.obj, tt.args.fieldPath, LookUpModeField, tt.args.value); (err != nil) != tt.wantErr {
					t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
				}
				if err := SetValue(tt.args.obj, tt.args.yamlTagPath, LookUpModeYamlTag, tt.args.value); (err != nil) != tt.wantErr {
					t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		})
	}
}

func TestSetValue_Level_Invalid(t *testing.T) {
	type args struct {
		obj         any
		fieldPath   string
		yamlTagPath string
		value       any
	}
	example1 := example{
		ArrayValue: []string{"abc", "def"},
		MapStrValue: map[string]example2{
			"abc": {StrValue: "2abc"},
		},
		SliceMapStrValue: []map[string]example2{
			{"abc": {StrValue: "2abc"}},
			{"def": {StrValue: "3def"}},
		},
		PointerValue: &example{
			StrValue: "2abc",
		},
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "slice value level 2 should return error because of slice is not init",
			args: args{
				obj:         &example1,
				fieldPath:   "SliceValue.1",
				yamlTagPath: "sliceValue.1",
				value:       "2hgt",
			},
			wantErr: true,
		},
		{
			name: "map value level 3 should return error because of map element can not set",
			args: args{
				obj:         &example1,
				fieldPath:   "MapStrValue.abc.StrValue",
				yamlTagPath: "mapStrValue.abc.strValue",
				value:       "2hgt",
			},
			wantErr: true,
		},
		{
			name: "map value level 4 should return success because map element can not set",
			args: args{
				obj:         &example1,
				fieldPath:   "SliceMapStrValue.1.def.StrValue",
				yamlTagPath: "sliceMapStrValue.1.def.strValue",
				value:       "3hgt",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				if err := SetValue(tt.args.obj, tt.args.fieldPath, LookUpModeField, tt.args.value); (err != nil) != tt.wantErr {
					t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
				}
				if err := SetValue(tt.args.obj, tt.args.yamlTagPath, LookUpModeYamlTag, tt.args.value); (err != nil) != tt.wantErr {
					t.Errorf("SetValue() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		})
	}
}
