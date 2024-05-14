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

package resources

import (
	"fmt"
	"testing"
)

func TestCompareResourceVersion(t *testing.T) {
	tests := []struct {
		name string
		rv1  string
		rv2  string
		want bool
	}{
		{
			name: "rv1 is greater than rv2",
			rv1:  "2",
			rv2:  "1",
			want: true,
		},
		{
			name: "rv1 is equal to rv2",
			rv1:  "1",
			rv2:  "1",
			want: false,
		},
		{
			name: "rv1 is less than rv2",
			rv1:  "1",
			rv2:  "2",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareResourceVersion(tt.rv1, tt.rv2)
			if got != tt.want {
				t.Errorf(" got %v, want %v", got, tt.want)
			}
		})
	}
}

type domainStruct struct {
	Domain_name string
	Domain_id   string
	want        bool
}

var domains = []domainStruct{
	{
		Domain_name: "1",
		Domain_id:   "QWERdasda.12.23",
		want:        false,
	},
	{
		Domain_name: "2",
		Domain_id:   "adsdsada.12.23",
		want:        true,
	},
	{
		Domain_name: "lenth=63 is true",
		Domain_id:   "adsdsada.12.23wqdqdqdqwdqwddddddddddddddddddddddddddddddddddddd",
		want:        true,
	},
	{
		Domain_name: "lenth=64 is false",
		Domain_id:   "adsdsada.12.23wqdqdqdqwdqwdddddddddddddddddddddddddddddddddddddd",
		want:        false,
	},
	{
		Domain_name: "5",
		Domain_id:   "adswqdqdqdqwdqwddddddddddddddddddddddddddddddddddddddddd",
		want:        true,
	},
	{
		Domain_name: "empty",
		Domain_id:   "",
		want:        false,
	},
	{
		Domain_name: "Chinese is false",
		Domain_id:   "中文",
		want:        false,
	},
	{
		Domain_name: "8",
		Domain_id:   "!@#$%^&*()！@#￥%……&*（）——+",
		want:        false,
	},
	{
		Domain_name: "9",
		Domain_id:   "qwe_qwe",
		want:        false,
	},
	{
		Domain_name: "10",
		Domain_id:   "qwe.",
		want:        false,
	},
	{
		Domain_name: "11",
		Domain_id:   "qwe-",
		want:        false,
	},
	{
		Domain_name: "12",
		Domain_id:   "_qwer",
		want:        false,
	},
	{
		Domain_name: "13",
		Domain_id:   "-qwer",
		want:        false,
	},
}

func TestValidateK8sName(t *testing.T) {

	for _, domain := range domains {
		t.Run(domain.Domain_name, func(t *testing.T) {

			err := ValidateK8sName(domain.Domain_id, "domain_id")
			got := err == nil
			if !got {
				fmt.Printf("%s error message: %s \n", domain.Domain_name, err.Error())
			}
			if got != domain.want {
				t.Errorf(" got %v, want %v", got, domain.want)
			}
		})
	}
}
