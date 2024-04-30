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

func TestGetPrefixIfPresent(t *testing.T) {
	type args struct {
		endpoint v1alpha1.DomainEndpoint
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "case 0",
			args: args{
				endpoint: v1alpha1.DomainEndpoint{
					Host: "localhost",
					Ports: []v1alpha1.DomainPort{
						{
							Name:       "port",
							Protocol:   "http",
							PathPrefix: "/prefix",
							IsTLS:      false,
							Port:       8080,
						},
					},
				},
			},
			want: "/prefix",
		},
		{
			name: "case 1",
			args: args{
				endpoint: v1alpha1.DomainEndpoint{
					Host: "localhost",
					Ports: []v1alpha1.DomainPort{
						{
							Name:     "port",
							Protocol: "http",
							IsTLS:    false,
							Port:     8080,
						},
					},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPrefixIfPresent(tt.args.endpoint); got != tt.want {
				t.Errorf("GetPrefixIfPresent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHandshakePathSuffix(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// TODO: Add test cases.
		{
			name: "case 0",
			want: "/handshake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHandshakePathSuffix(); got != tt.want {
				t.Errorf("GetHandshakePathSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHandshakePathOfEndpoint(t *testing.T) {
	type args struct {
		endpoint v1alpha1.DomainEndpoint
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "case 0",
			args: args{
				endpoint: v1alpha1.DomainEndpoint{
					Host: "localhost",
					Ports: []v1alpha1.DomainPort{
						{
							Name:       "port",
							Protocol:   "http",
							PathPrefix: "/prefix",
							IsTLS:      false,
							Port:       8080,
						},
					},
				},
			},
			want: "/prefix/handshake",
		},
		{
			name: "case 1",
			args: args{
				endpoint: v1alpha1.DomainEndpoint{
					Host: "localhost",
					Ports: []v1alpha1.DomainPort{
						{
							Name:     "port",
							Protocol: "http",
							IsTLS:    false,
							Port:     8080,
						},
					},
				},
			},
			want: "/handshake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHandshakePathOfEndpoint(tt.args.endpoint); got != tt.want {
				t.Errorf("GetHandshakePathOfEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHandshakePathOfPrefix(t *testing.T) {
	type args struct {
		pathPrefix string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
		{
			name: "case 0",
			args: args{
				pathPrefix: "/prefix",
			},
			want: "/prefix/handshake",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetHandshakePathOfPrefix(tt.args.pathPrefix); got != tt.want {
				t.Errorf("GetHandshakePathOfPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
