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
	"sync"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// Holder is container that help you manager your secret driver.
type Holder struct {
	drivers sync.Map
}

func NewHolder() *Holder {
	return &Holder{}
}

func (h *Holder) Add(name string, secretBackend SecretDriver) {
	h.drivers.Store(name, secretBackend)
}

func (h *Holder) Init(name string, driverName string, config map[string]any) error {
	backend, err := NewSecretBackendWith(driverName, config)
	if err != nil {
		return err
	}
	h.drivers.Store(name, backend)
	return nil
}

func (h *Holder) Get(name string) SecretDriver {
	value, _ := h.drivers.Load(name)
	return value.(SecretDriver)
}

func (h *Holder) Close(name string) {
	value, exist := h.drivers.Load(name)
	if exist {
		err := value.(SecretDriver).Close()
		if err != nil {
			nlog.Errorf("Close secret backend name=%s failed: %s", name, err)
		}
		h.drivers.Delete(name)
	}
}

func (h *Holder) CloseAll() {
	h.drivers.Range(func(key, value any) bool {
		err := value.(SecretDriver).Close()
		if err != nil {
			nlog.Errorf("Close secret backend name=%s failed: %s", key, err)
		}
		return true
	})
	h.drivers = sync.Map{}
}
