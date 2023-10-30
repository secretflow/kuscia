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

package mem

import (
	"fmt"

	"github.com/secretflow/kuscia/pkg/secretbackend"

	"github.com/mitchellh/mapstructure"
)

type Mem struct {
	confs map[string]string
}

type Config struct {
	Preset map[string]string `mapstructure:"preset"`
}

func NewMem(configMap map[string]any) (secretbackend.SecretDriver, error) {
	config := Config{}
	if err := mapstructure.Decode(configMap, &config); err != nil {
		return nil, err
	}
	confs := map[string]string{}
	for k, v := range config.Preset {
		confs[k] = v
	}
	return &Mem{confs: confs}, nil
}

func (m *Mem) Set(confID string, value string) error {
	m.confs[confID] = value
	return nil
}

func (m *Mem) Get(confID string) (string, error) {
	return m.GetByParams(confID, nil)
}

func (m *Mem) GetByParams(confID string, params map[string]any) (string, error) {
	value, exist := m.confs[confID]
	if !exist {
		return "", fmt.Errorf("not exist")
	}
	return value, nil
}

func init() {
	secretbackend.Register("mem", NewMem)
}
