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
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

const (
	defaultTTL = 5
)

var (
	errKeyNotFound = errors.New("key not found")
	errKeyInvalid  = errors.New("key invalid, must contain at least 3 fields")
)

// KusciaCoreDNS is a plugin talk to kubernetes.
type KusciaCoreDNS struct {
	Next plugin.Handler
	Fall fall.F

	Zones     []string
	Namespace string
	EnvoyIP   string

	Upstream *upstream.Upstream
	Cache    *cache.Cache

	counter uint64
}

// Services implements the ServiceBackend interface.
func (e *KusciaCoreDNS) Services(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (services []msg.Service, err error) {
	services, err = e.Records(ctx, state, exact)
	if err != nil {
		return
	}

	services = msg.Group(services)
	return
}

// Reverse implements the ServiceBackend interface.
func (e *KusciaCoreDNS) Reverse(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (services []msg.Service, err error) {
	return e.Services(ctx, state, exact, opt)
}

// Lookup implements the ServiceBackend interface.
func (e *KusciaCoreDNS) Lookup(ctx context.Context, state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return e.Upstream.Lookup(ctx, state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (e *KusciaCoreDNS) IsNameError(err error) bool {
	return err == errKeyNotFound
}

// Records looks up records in cache. If exact is true, it will lookup just this
// name. This is used when find matches when completing SRV lookups for instance.
func (e *KusciaCoreDNS) Records(ctx context.Context, state request.Request, exact bool) ([]msg.Service, error) {
	name := state.Name()
	if len(name) == 0 {
		return nil, errKeyNotFound
	}
	name = name[:len(name)-1] // remove closing dot

	var item interface{}
	found := false

	// FQDN: foo.bar.ns.svc
	if strings.HasSuffix(name, ".svc") {
		fields := strings.Split(name, ".")

		n := len(fields)
		if n < 3 {
			return nil, errKeyInvalid
		}

		if fields[n-2] == e.Namespace {
			item, found = e.Cache.Get(strings.TrimSuffix(name, ".svc"))
		} else {
			item = []string{e.EnvoyIP}
			found = true
		}
	} else {
		item, found = e.Cache.Get(name)
	}

	if !found {
		return nil, errKeyNotFound
	}

	endpoints, ok := item.([]string)
	if !ok || len(endpoints) == 0 {
		return nil, errKeyNotFound
	}

	// DNS Round-Robin
	counter := atomic.AddUint64(&e.counter, 1)
	services := []msg.Service{{Host: endpoints[counter%uint64(len(endpoints))], TTL: defaultTTL}}
	if what, _ := services[0].HostType(); what != dns.TypeCNAME {
		return services, nil
	}

	endpoints = e.resolveHost(services[0].Host)
	if len(endpoints) == 0 {
		return nil, errKeyNotFound
	}

	return []msg.Service{{Host: endpoints[counter%uint64(len(endpoints))], TTL: defaultTTL}}, nil
}

func (e *KusciaCoreDNS) resolveHost(host string) []string {
	// lookup in local cache
	cachedAddrsIPv4, found := e.Cache.Get(host)
	if found {
		items, ok := cachedAddrsIPv4.([]string)
		if !ok {
			return nil
		}
		return items
	}

	// lookup using the local resolver
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil
	}

	addrsIPv4 := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip.To4() != nil {
			addrsIPv4 = append(addrsIPv4, addr)
		}
	}

	// update local cache
	if len(addrsIPv4) != 0 {
		e.Cache.Set(host, addrsIPv4, defaultTTL*time.Second)
	}

	return addrsIPv4
}
