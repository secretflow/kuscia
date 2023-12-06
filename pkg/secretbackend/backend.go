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

import "fmt"

// SecretDriver is actual secret manager system. It may be Vault, KMS, etc.
type SecretDriver interface {
	// Set store confId/value to actual Secret Backend, Secret Backend may convert confId/value to another form.
	Set(confID string, value string) error
	// Get lookup value for confId, Secret Backend may reconvert value if it converts value to another form when Set.
	Get(confID string) (string, error)
	// GetByParams lookup value for confId, Secret Backend may reconvert value if it converts value to another form when Set.
	GetByParams(confID string, params map[string]any) (string, error)
	// Close should free the SecretBackend resources.
	Close() error
}

type RegistryFactory func(config map[string]any) (SecretDriver, error)

var globalRegistry = registry{
	backendFactory: map[string]RegistryFactory{},
}

type registry struct {
	backendFactory map[string]RegistryFactory
}

// Register SecretBackend driver, should be call be driver when package import init, the name should be unique.
// Driver will be overwritten when register with same name, and the last registration will win.
func Register(name string, factory RegistryFactory) {
	globalRegistry.backendFactory[name] = factory
	return
}

// NewSecretBackendWith return a SecretBackend driver named params name. With a not-exist name, will return err.
func NewSecretBackendWith(name string, config map[string]any) (SecretDriver, error) {
	_, exist := globalRegistry.backendFactory[name]
	if !exist {
		return nil, fmt.Errorf("can't find secret driver with name=%s", name)
	}
	return globalRegistry.backendFactory[name](config)
}
