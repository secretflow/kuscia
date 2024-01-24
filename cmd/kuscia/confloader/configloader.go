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
	"fmt"

	"github.com/secretflow/kuscia/pkg/secretbackend"
)

const (
	ConfigLoaderTypeForSecretBackend = "secret-backend"
	ConfigLoaderTypeForRemote        = "remote"
)

// ConfigLoader handle config.
type ConfigLoader interface {
	Load(ctx context.Context, conf *KusciaConfig) error
}

// ConfigLoaderChain is chain to handle kuscia config.
type ConfigLoaderChain []ConfigLoader

// Load will load and adjust the kuscia config.
func (c *ConfigLoaderChain) Load(ctx context.Context, conf *KusciaConfig) error {
	var err error
	for _, l := range *c {
		err = l.Load(ctx, conf)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewConfigLoaderChain return a config loader chain to handle kuscia config.
func NewConfigLoaderChain(ctx context.Context, loaders []ConfigLoaderConfig, holder *secretbackend.Holder) (ConfigLoaderChain, error) {
	loaderChain := make([]ConfigLoader, 0)
	for _, lc := range loaders {
		switch lc.Type {
		case ConfigLoaderTypeForSecretBackend:
			secretBackendConfigLoader, err := NewSecretBackendConfigLoader(ctx, lc.SecretBackendParams, holder)
			if err != nil {
				return nil, err
			}
			loaderChain = append(loaderChain, secretBackendConfigLoader)
		case ConfigLoaderTypeForRemote:
			return nil, fmt.Errorf("not implemented")
		default:
			return nil, fmt.Errorf("not implemented")
		}
	}
	return loaderChain, nil
}
