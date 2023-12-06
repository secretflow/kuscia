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
	"context"
	"reflect"
	"testing"

	"github.com/secretflow/kuscia/pkg/secretbackend"
	_ "github.com/secretflow/kuscia/pkg/secretbackend/mem"
)

func TestConfigLoaderChain_Load(t *testing.T) {
	type args struct {
		ctx    context.Context
		conf   *KusciaConfig
		holder *secretbackend.Holder
	}
	tests := []struct {
		name        string
		args        args
		want        string
		wantNewErr  bool
		wantLoadErr bool
	}{
		{
			name: "new config loader by chain",
			args: args{
				ctx: context.Background(),
				conf: &KusciaConfig{
					SecretBackends: []SecretBackendConfig{
						{
							Name:   "mem1",
							Driver: "mem",
							Params: map[string]any{
								"preset": map[string]string{
									"secretKeyDomainKeyData": "secretValueDomainKeyData",
								},
							},
						},
					},
					ConfLoaders: []ConfigLoaderConfig{
						{
							Type: ConfigLoaderTypeForSecretBackend,
							SecretBackendParams: SecretBackendParams{
								Backend: "mem1",
								InterestedKeys: []SecretBackendInterestedKey{
									{
										Key: "domainKeyData",
									},
								},
							},
						},
					},
					DomainKeyData: "secretKeyDomainKeyData",
				},
			},
			wantNewErr:  false,
			wantLoadErr: false,
			want:        "secretValueDomainKeyData",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			holder := secretbackend.NewHolder()
			for _, sbc := range tt.args.conf.SecretBackends {
				if err := holder.Init(sbc.Name, sbc.Driver, sbc.Params); err != nil {
					t.Errorf("init secret backend = %s should success", sbc.Name)
				}
			}
			got, err := NewConfigLoaderChain(tt.args.ctx, tt.args.conf.ConfLoaders, holder)
			if (err != nil) != tt.wantNewErr {
				t.Errorf("NewConfigLoaderChain() error = %v, wantErr %v", err, tt.wantNewErr)
				return
			}
			if err := got.Load(tt.args.ctx, tt.args.conf); (err != nil) != tt.wantLoadErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantLoadErr)
				return
			}
			if !reflect.DeepEqual(tt.args.conf.DomainKeyData, tt.want) {
				t.Errorf("Load() got = %v, want %v", tt.args.conf.DomainKeyData, tt.want)
			}
		})
	}
}
