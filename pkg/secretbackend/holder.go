package secretbackend

import "sync"

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
