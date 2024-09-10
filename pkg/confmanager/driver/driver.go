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
	"crypto/rsa"
	"fmt"

	"k8s.io/client-go/kubernetes"
)

type Driver interface {
	GetConfig(ctx context.Context, key string) (string, bool, error)
	SetConfig(ctx context.Context, data map[string]string) error
	ListConfig(ctx context.Context, keys []string) (map[string]string, error)
	DeleteConfig(ctx context.Context, keys []string) error
}

const (
	CRDDriverType = "crd"
)

type Config struct {
	Driver     string
	DomainID   string
	ConfigName string
	// enable cache means getting data in local cache with k8s configmap lister
	// otherwise will use kubeClient to get configmap
	// for kuscia config command, will disable cache
	DisableCache bool
	KubeClient   kubernetes.Interface
	DomainKey    *rsa.PrivateKey
}

func NewDriver(ctx context.Context, conf *Config) (Driver, error) {
	switch conf.Driver {
	case CRDDriverType:
		return NewCRDDriver(ctx, conf)
	default:
		return nil, fmt.Errorf("confManager doesn't support driver: %q", conf.Driver)
	}
}
