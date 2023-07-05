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
