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

package secretbackend

import (
	"testing"
)

func TestRegister(t *testing.T) {
	type args struct {
		name    string
		factory RegistryFactory
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "register a fake driver should return no error",
			args: args{
				name:    "fake",
				factory: NewFake,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Register(tt.args.name, tt.args.factory)
		})
	}
}

func TestNewSecretBackendWith(t *testing.T) {
	type args struct {
		name   string
		config map[string]any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new fake secret backend should return no error",
			args: args{
				name:   "fake",
				config: map[string]any{},
			},
			wantErr: false,
		},
		{
			name: "new fake2 secret backend should return error",
			args: args{
				name:   "fake2",
				config: map[string]any{},
			},
			wantErr: true,
		},
	}
	Register("fake", NewFake)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSecretBackendWith(tt.args.name, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSecretBackendWith() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

type Fake struct{}

func NewFake(configMap map[string]any) (SecretDriver, error) {
	return &Fake{}, nil
}

func (f *Fake) Set(confID string, value string) error {
	return nil
}

func (f *Fake) Get(confID string) (string, error) {
	return "", nil
}

func (f *Fake) GetByParams(confID string, params map[string]any) (string, error) {
	return "", nil
}

func (f *Fake) Close() error {
	return nil
}
