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

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

type args struct {
	transit *v1alpha1.Transit
}

var (
	argsWithReverseTunnel = args{
		transit: &v1alpha1.Transit{
			TransitMethod: v1alpha1.TransitMethodReverseTunnel,
		},
	}
	argsWithThirdDomain = args{
		transit: &v1alpha1.Transit{
			TransitMethod: v1alpha1.TransitMethodThirdDomain,
		},
	}
	argsWithEmpty = args{
		transit: &v1alpha1.Transit{},
	}
	argsWithNil = args{nil}
)

func TestIsThirdPartyTransit(t *testing.T) {

	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "case 0",
			args: argsWithReverseTunnel,
			want: false,
		},
		{
			name: "case 1",
			args: argsWithThirdDomain,
			want: true,
		},
		{
			name: "case 2",
			args: argsWithNil,
			want: false,
		},
		{
			name: "case 3",
			args: argsWithEmpty,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsThirdPartyTransit(tt.args.transit); got != tt.want {
				t.Errorf("IsThirdPartyTransit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsReverseTunnelTransit(t *testing.T) {

	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "case 0",
			args: argsWithReverseTunnel,
			want: true,
		},
		{
			name: "case 1",
			args: argsWithThirdDomain,
			want: false,
		},
		{
			name: "case 2",
			args: argsWithNil,
			want: false,
		},
		{
			name: "case 3",
			args: argsWithEmpty,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsReverseTunnelTransit(tt.args.transit); got != tt.want {
				t.Errorf("IsReverseTunnelTransit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTransit(t *testing.T) {

	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "case 0",
			args: argsWithReverseTunnel,
			want: true,
		},
		{
			name: "case 1",
			args: argsWithThirdDomain,
			want: true,
		},
		{
			name: "case 2",
			args: argsWithNil,
			want: false,
		},
		{
			name: "case 3",
			args: argsWithEmpty,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTransit(tt.args.transit); got != tt.want {
				t.Errorf("IsTransit() = %v, want %v", got, tt.want)
			}
		})
	}
}
