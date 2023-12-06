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
	"encoding/base64"
	"fmt"

	"github.com/secretflow/kuscia/pkg/secretbackend"
	// register driver
	_ "github.com/secretflow/kuscia/pkg/secretbackend/mem"
	_ "github.com/secretflow/kuscia/pkg/secretbackend/rfile"
)

type SecretBackendParams struct {
	Backend        string                       `yaml:"backend"`
	InterestedKeys []SecretBackendInterestedKey `yaml:"interestedKeys"`
}

type SecretBackendInterestedKey struct {
	Key          string `yaml:"key"`
	Base64Decode bool   `yaml:"base64Decode"`
}

// SecretBackendConfigLoader will decrypt interestedKeys of kuscia config.
type SecretBackendConfigLoader struct {
	params SecretBackendParams
	driver secretbackend.SecretDriver
}

func (s *SecretBackendConfigLoader) Load(ctx context.Context, conf *KusciaConfig) error {
	for _, ik := range s.params.InterestedKeys {
		secretKeyInterface, err := GetValue(conf, ik.Key, LookUpModeYamlTag)
		if err != nil {
			return err
		}
		secretKey, ok := secretKeyInterface.(string)
		if !ok {
			return fmt.Errorf("for secret backend config load, decrypted value must be string, key=%s", ik.Key)
		}
		confValue, err := s.driver.Get(secretKey)
		if err != nil {
			return err
		}
		if ik.Base64Decode {
			confValueDecode, err := base64.StdEncoding.DecodeString(confValue)
			if err != nil {
				return fmt.Errorf("for secret backend config load, key=%s base64 decode failed, err=%s", ik.Key, err)
			}
			confValue = string(confValueDecode)
		}
		if err := SetValue(conf, ik.Key, LookUpModeYamlTag, confValue); err != nil {
			return err
		}
	}
	return nil
}

// NewSecretBackendConfigLoader will return SecretBackendConfigLoader which be supported by secret backend driver.
func NewSecretBackendConfigLoader(ctx context.Context, params SecretBackendParams, holder *secretbackend.Holder) (ConfigLoader, error) {
	driver := holder.Get(params.Backend)
	if driver == nil {
		return nil, fmt.Errorf("can not find secret backend with name=%s", params.Backend)
	}
	return &SecretBackendConfigLoader{
		params: params,
		driver: driver,
	}, nil
}
