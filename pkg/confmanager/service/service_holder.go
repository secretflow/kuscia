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

package service

import (
	"sync"

	cmconfig "github.com/secretflow/kuscia/pkg/confmanager/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type HolderExporter interface {
	ConfigurationService() IConfigurationService
	CertificateService() ICertificateService
	Ready() bool
}

var Exporter HolderExporter = &serviceHolderInstance

type Holder struct {
	certificateService   ICertificateService
	configurationService IConfigurationService

	ready  bool
	rwLock *sync.RWMutex
}

var serviceHolderInstance = Holder{
	ready:  false,
	rwLock: &sync.RWMutex{},
}

func InitServiceHolder(config *cmconfig.ConfManagerConfig) error {
	var err error
	serviceHolderInstance.rwLock.Lock()
	defer serviceHolderInstance.rwLock.Unlock()
	serviceHolderInstance.certificateService, err = NewCertificateService(CertConfig{
		CertValue:  config.DomainCertValue,
		PrivateKey: config.DomainKey,
	})
	if err != nil {
		nlog.Fatalf("Failed to init certificate service: %v", err)
		return err
	}
	serviceHolderInstance.configurationService, err = NewConfigurationService(config.BackendDriver, config.EnableConfAuth)
	if err != nil {
		nlog.Fatalf("Failed to init configuration service: %v", err)
		return err
	}
	serviceHolderInstance.ready = true
	return nil
}

// ConfigurationService exports ConfigurationService instance.
func (s *Holder) ConfigurationService() IConfigurationService {
	s.rwLock.RLock()
	defer s.rwLock.RUnlock()
	return serviceHolderInstance.configurationService
}

// CertificateService exports CertificateService instance.
func (s *Holder) CertificateService() ICertificateService {
	s.rwLock.RLock()
	defer s.rwLock.RUnlock()
	return serviceHolderInstance.certificateService
}

// Ready means whether holder available.
func (s *Holder) Ready() bool {
	s.rwLock.RLock()
	defer s.rwLock.RUnlock()
	return s.ready
}
