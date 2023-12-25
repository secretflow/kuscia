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

package coredns

import (
	"context"
	"fmt"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/patrickmn/go-cache"
	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	cleanupInterval   = 5 * time.Minute
	defaultSyncPeriod = 10 * time.Minute
	defaultExpiration = 30 * time.Minute
)

var localService = []string{"datamesh", "confmanager", "dataproxy"}

func KusciaParse(c *caddy.Controller, namespace, envoyIP string) (*KusciaCoreDNS, error) {
	etc := KusciaCoreDNS{
		EnvoyIP:   envoyIP,
		Namespace: namespace,
		Upstream:  upstream.New(),
		Cache:     cache.New(defaultExpiration, cleanupInterval),
	}

	etc.Cache.Set(fmt.Sprintf("transport.%s", etc.Namespace), []string{etc.EnvoyIP}, -1)
	hostIP, err := network.GetHostIP()
	if err != nil {
		nlog.Fatal(err)
	}

	for _, svc := range localService {
		etc.Cache.Set(svc, []string{hostIP}, -1)
	}
	if c.Next() {
		etc.Zones = c.RemainingArgs()
		if len(etc.Zones) == 0 {
			etc.Zones = make([]string, len(c.ServerBlockKeys))
			copy(etc.Zones, c.ServerBlockKeys)
		}
		for i, str := range etc.Zones {
			if zones := plugin.Host(str).NormalizeExact(); len(zones) > 0 {
				etc.Zones[i] = zones[0]
			}
		}

		for c.NextBlock() {
			switch c.Val() {
			case "fallthrough":
				etc.Fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				if c.Val() != "}" {
					return &KusciaCoreDNS{}, c.Errf("unknown property '%s'", c.Val())
				}
			}
		}

		return &etc, nil
	}
	return &KusciaCoreDNS{}, nil
}

func (e *KusciaCoreDNS) StartControllers(ctx context.Context, kubeclient kubernetes.Interface) error {
	err := e.Start(ctx, kubeclient)
	if err != nil {
		return fmt.Errorf("start coredns controller failed, %v", err)
	}
	return nil
}
