// Copyright 2024 Ant Group Co., Ltd.
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

package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	listers "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

const (
	// default data size threshold is 3.5MB
	dataSizeThreshold = 3.5 * 1024 * 1024
	maxKeyLen         = 256
)

var once sync.Once
var crdDriver *CRDDriver

type CRDDriver struct {
	Ctx context.Context
	*Config
	ConfigMapLister listers.ConfigMapLister
}

func NewCRDDriver(ctx context.Context, conf *Config) (Driver, error) {
	var err error
	once.Do(func() {
		crdDriver, err = newCRDDriver(ctx, conf)
	})
	return crdDriver, err
}

func newCRDDriver(ctx context.Context, conf *Config) (*CRDDriver, error) {
	nlog.Info("Start initializing cm crd driver")
	driver := &CRDDriver{
		Ctx:    ctx,
		Config: conf,
	}

	if conf.KubeClient == nil {
		return nil, fmt.Errorf("kubeclient can't be empty for cm crd driver")
	}

	if !conf.DisableCache {
		nlog.Info("Start initializing cm configmap informer and waiting for cache sync")
		kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(conf.KubeClient, 0, informers.WithNamespace(conf.DomainID))
		configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
		driver.ConfigMapLister = configMapInformer.Lister()
		kubeInformerFactory.Start(ctx.Done())
		kubeInformerFactory.WaitForCacheSync(ctx.Done())
		nlog.Info("Finish initializing cm configmap informer and syncing cache")
	}
	nlog.Info("Finish initializing cm crd driver")
	return driver, nil
}

func (d *CRDDriver) GetConfig(ctx context.Context, key string) (string, bool, error) {
	cm, err := d.getConfigMap(ctx)
	if err != nil {
		return "", false, err
	}

	if len(cm.Data) == 0 {
		return "", false, nil
	}

	encValue, ok := cm.Data[key]
	if !ok {
		return "", false, nil
	}
	if encValue == "" {
		return "", true, nil
	}

	value, err := tls.DecryptOAEP(d.DomainKey, encValue)
	if err != nil {
		return "", true, err
	}

	return string(value), true, nil
}

func (d *CRDDriver) SetConfig(ctx context.Context, data map[string]string) error {
	if len(data) == 0 {
		return nil
	}

	cm, err := d.getConfigMap(ctx)
	if err != nil {
		return err
	}

	newCM := cm.DeepCopy()
	if newCM.Data == nil {
		newCM.Data = make(map[string]string)
	}

	if err = checkDataSize(newCM.Data); err != nil {
		return err
	}

	for key, value := range data {
		if len(key) > maxKeyLen {
			return fmt.Errorf("key[%v] length is %v bytes and exceed the max length %v bytes", key, len(key), maxKeyLen)
		}
		encValue, err := tls.EncryptOAEP(&d.DomainKey.PublicKey, []byte(value))
		if err != nil {
			return err
		}
		newCM.Data[key] = encValue
	}

	return resources.PatchConfigMap(ctx, d.KubeClient, cm, newCM)
}

func (d *CRDDriver) ListConfig(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	cm, err := d.getConfigMap(ctx)
	if err != nil {
		return nil, err
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	result := make(map[string]string)
	for _, key := range keys {
		encValue := cm.Data[key]
		if encValue != "" {
			value, err := tls.DecryptOAEP(d.DomainKey, encValue)
			if err != nil {
				return nil, err
			}
			result[key] = string(value)
		} else {
			result[key] = encValue
		}
	}
	return result, nil
}

func (d *CRDDriver) DeleteConfig(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	cm, err := d.getConfigMap(ctx)
	if err != nil {
		return err
	}

	if len(cm.Data) == 0 {
		return nil
	}

	newCM := cm.DeepCopy()
	for _, key := range keys {
		delete(newCM.Data, key)
	}

	return resources.PatchConfigMap(ctx, d.KubeClient, cm, newCM)
}

func (d *CRDDriver) getConfigMap(ctx context.Context) (*v1.ConfigMap, error) {
	if !d.Config.DisableCache {
		return d.ConfigMapLister.ConfigMaps(d.DomainID).Get(d.ConfigName)
	}
	return d.KubeClient.CoreV1().ConfigMaps(d.DomainID).Get(ctx, d.ConfigName, metav1.GetOptions{})
}

func checkDataSize(data map[string]string) error {
	v, err := json.Marshal(data)
	if err != nil {
		nlog.Warnf("Failed to marshal domain config data, %v, skip checking data size", err.Error())
		return nil
	}
	dataSize := len(v)
	nlog.Infof("Domain config data size is %v bytes now", dataSize)
	if dataSize > dataSizeThreshold {
		return fmt.Errorf("domain config data size is %v bytes and exceed the threshold %v bytes, "+
			"forbid creating or updating config", dataSize, dataSizeThreshold)
	}
	return nil
}
