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

package kusciaconfig

import (
	"fmt"
)

type TLSConfig struct {
	CertFile string `yaml:"certFile,omitempty"`
	CertData string `yaml:"certData,omitempty"`
	KeyFile  string `yaml:"keyFile,omitempty"`
	KeyData  string `yaml:"keyData,omitempty"`
	CAData   string `yaml:"caData,omitempty"`
	CAFile   string `yaml:"caFile,omitempty"`
}

func CheckTLSConfig(config *TLSConfig, name string) error {
	if config == nil {
		return nil
	}

	// for tls client may only need ServerCa
	if config.CAFile != "" {
		return nil
	}

	// for server or mtls client ,should specify keyFile and CertFile
	if config.KeyData == "" && config.KeyFile == "" {
		return fmt.Errorf("TLS(%s) need specify keyFile", name)
	}

	if config.CertData == "" && config.CertFile == "" {
		return fmt.Errorf("TLS(%s) need specify caFile", name)
	}
	return nil
}
