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

package mist

import (
	"github.com/secretflow/kuscia/pkg/secretbackend"
	"testing"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/asserts"
)

var (
	mistDriverParams = map[string]any{
		"app_name": "secretpad",
		"tenant":   "ALIPAY",
		"env":      "test",
	}
)

func newMistDriver() (secretbackend.SecretDriver, error) {
	backend, err := secretbackend.NewSecretBackendWith("mist", mistDriverParams)
	if err != nil {
		nlog.Errorf("New secret backend with name=%s failed: %s", "mist", err)
		return nil, err
	}
	return backend, nil
}

func TestNewMistSecretDriver(t *testing.T) {
	type args struct {
		configMap map[string]any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "init mist client should return no error",
			args:    args{configMap: mistDriverParams},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMistSecretDriver(tt.args.configMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMistSecretDriver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestMist_Set(t *testing.T) {
	type args struct {
		confID string
		value  string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "mist set should return unsupported error",
			args: args{
				confID: "abc",
				value:  "abc",
			},
			wantErr: true,
		},
	}
	driver, err := newMistDriver()
	_ = asserts.IsNil(err, "new mist driver error should be nil")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := driver.Set(tt.args.confID, tt.args.value); (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMist_Get(t *testing.T) {
	type fields struct {
		configMap map[string]any
	}
	type args struct {
		confID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name:   "mist get should for short secret value should success",
			fields: fields{configMap: mistDriverParams},
			args: args{
				confID: "a",
			},
			want:    "aa",
			wantErr: false,
		},
		{
			name:   "mist get should for long compose value should success",
			fields: fields{configMap: mistDriverParams},
			args: args{
				confID: "b",
			},
			want:    "b0b0b1b1",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMistSecretDriver(tt.fields.configMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMistSecretDriver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			m.(*Mist).setMistClient(&MockMistClient{
				store: map[string]string{
					"other_manual__a":        "aa",
					"other_manual__b":        "{\"_kuscia_multipart_\": 2}",
					"other_manual__b_part_0": "b0b0",
					"other_manual__b_part_1": "b1b1",
				},
			})
			got, err := m.Get(tt.args.confID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

type MockMistClient struct {
	store map[string]string
}

func (m *MockMistClient) GetSecretInfo(secretName string) (string, string, string, string, error) {
	return "", m.store[secretName], "", "", nil
}

func (m *MockMistClient) GetFactorSecretInfo(secretName string, factor string) (string, string, string, string, error) {
	return "", "", "", "", nil
}
