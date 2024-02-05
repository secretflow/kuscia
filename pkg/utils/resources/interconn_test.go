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

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestGetInterConnParties(t *testing.T) {
	tests := []struct {
		name       string
		annotation map[string]string
		want       int
	}{
		{
			name: "annotation is empty",
			want: 0,
		},
		{
			name: "annotation is not empty",
			annotation: map[string]string{
				common.InterConnBFIAPartyAnnotationKey:   "alice",
				common.InterConnKusciaPartyAnnotationKey: "bob",
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetInterConnParties(tt.annotation)
			assert.Equal(t, len(got), tt.want)
		})
	}
}

func TestGetInterConnProtocolTypeByPartyAnnotation(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want kusciaapisv1alpha1.InterConnProtocolType
	}{
		{
			name: "key is bfia",
			key:  common.InterConnBFIAPartyAnnotationKey,
			want: kusciaapisv1alpha1.InterConnBFIA,
		},
		{
			name: "key is kuscia",
			key:  common.InterConnKusciaPartyAnnotationKey,
			want: kusciaapisv1alpha1.InterConnKuscia,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetInterConnProtocolTypeByPartyAnnotation(tt.key)
			assert.Equal(t, got, tt.want)
		})
	}
}
