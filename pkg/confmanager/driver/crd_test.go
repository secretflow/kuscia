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
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/utils/tls"
)

var (
	testKey   = "test-key"
	testValue = "test-value"
	testKey1  = "test-key1"
	testKey2  = "test-key2"
)

func makeNewCRDDriver(disableCache bool) (*CRDDriver, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	encValue, err := tls.EncryptOAEP(&privateKey.PublicKey, []byte(testValue))
	if err != nil {
		return nil, err
	}
	configmap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "alice",
			Name:      "domain-config",
		},
		Data: map[string]string{
			testKey1: "",
			testKey2: "v2",
			testKey:  encValue,
		},
	}
	kubeClient := kubefake.NewSimpleClientset(&configmap)

	return newCRDDriver(context.Background(), &Config{
		Driver:       CRDDriverType,
		DomainID:     "alice",
		ConfigName:   "domain-config",
		DisableCache: disableCache,
		KubeClient:   kubeClient,
		DomainKey:    privateKey,
	})
}

func TestNewCRDDriver(t *testing.T) {
	_, err := NewCRDDriver(context.Background(), &Config{})
	assert.Error(t, err)
	_, err = makeNewCRDDriver(true)
	assert.NoError(t, err)
	_, err = makeNewCRDDriver(false)
	assert.NoError(t, err)
}

func TestGetConfig(t *testing.T) {
	d, err := makeNewCRDDriver(true)
	assert.NoError(t, err)

	value, exist, err := d.GetConfig(d.Ctx, testKey)
	assert.Equal(t, testValue, value)
	assert.True(t, exist)
	assert.NoError(t, err)

	value, exist, err = d.GetConfig(d.Ctx, testKey1)
	assert.Equal(t, "", value)
	assert.True(t, exist)
	assert.NoError(t, err)

	value, exist, err = d.GetConfig(d.Ctx, testKey2)
	assert.Equal(t, "", value)
	assert.True(t, exist)
	assert.Error(t, err)

	value, exist, err = d.GetConfig(d.Ctx, "key3")
	assert.Equal(t, "", value)
	assert.False(t, exist)
	assert.NoError(t, err)

	d, err = makeNewCRDDriver(false)
	assert.NoError(t, err)

	d.Config.ConfigName = "config"
	value, exist, err = d.GetConfig(d.Ctx, testKey1)
	assert.Error(t, err)
}

func TestSetConfig(t *testing.T) {
	d, err := makeNewCRDDriver(true)
	assert.NoError(t, err)

	err = d.SetConfig(d.Ctx, map[string]string{testKey: "value"})
	assert.NoError(t, err)

	invalidKey := strings.Repeat("s", 257)
	err = d.SetConfig(d.Ctx, map[string]string{invalidKey: "value"})
	assert.Error(t, err)
}

func TestListConfig(t *testing.T) {
	d, err := makeNewCRDDriver(true)
	assert.NoError(t, err)

	v, err := d.ListConfig(d.Ctx, []string{testKey})
	assert.NoError(t, err)
	assert.Equal(t, testValue, v[testKey])

	v, err = d.ListConfig(d.Ctx, []string{testKey2})
	assert.Error(t, err)

	v, err = d.ListConfig(d.Ctx, []string{testKey1})
	assert.NoError(t, err)
	assert.Equal(t, "", v[testKey])
}

func TestDeleteConfig(t *testing.T) {
	d, err := makeNewCRDDriver(true)
	assert.NoError(t, err)

	err = d.DeleteConfig(d.Ctx, []string{testKey})
	assert.NoError(t, err)
}

func TestCheckDataSize(t *testing.T) {
	data := map[string]string{
		"key": "value",
	}

	got := checkDataSize(data)
	assert.NoError(t, got)
}
