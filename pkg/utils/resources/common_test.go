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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
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

func TestIsEmpty(t *testing.T) {
	var limitResource corev1.ResourceList
	assert.Equal(t, IsEmpty(limitResource), true, "IsEmpty() function cannot judge whether the data is empty. ")
	cpu := k8sresource.MustParse("100m")
	limitResource = corev1.ResourceList{}
	limitResource[corev1.ResourceCPU] = cpu
	assert.Equal(t, IsEmpty(limitResource), false, "IsEmpty() function cannot judge whether the data is not empty. ")
}

func TestSplitRSC(t *testing.T) {
	var input = "1000"
	output, err := SplitRSC(input, 5)
	assert.Equal(t, output, "200", "SplitRSC() function cannot handle the k8sresource.DecimalSI. ")
	assert.Nil(t, err, "SplitRSC() function cannot handle the k8sresource.DecimalSI. ")

	inputSlice := [6]string{"100Pi", "200T", "300Gi", "400M", "500Ki", "0.5Pi"}
	stdOutputSlice := [6]string{"102400Ti", "100T", "76800Mi", "50M", "32000", "16384Gi"}
	for k := range inputSlice {
		output, err = SplitRSC(inputSlice[k], 1<<k)
		assert.Equal(t, output, stdOutputSlice[k], "SplitRSC() function cannot handle the unit (Pi, Ti, Gi, Mi, Ki). ")
		assert.Nil(t, err, "SplitRSC() function cannot handle the unit (Pi, Ti, Gi, Mi, Ki). ")
	}

	input = "100KB"
	_, err = SplitRSC(input, 1<<5)
	assert.NotNil(t, err, "SplitRSC() function cannot handle the anomaly input. ")
}
