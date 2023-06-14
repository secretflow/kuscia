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

package modules

import (
	"context"
	"os"
	"path/filepath"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/coredns"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var directives = []string{
	"metadata",
	"cancel",
	"reload",
	"nsid",
	"bufsize",
	"root",
	"bind",
	"debug",
	"ready",
	"health",
	"pprof",
	"prometheus",
	"errors",
	"log",
	"dnstap",
	"acl",
	"any",
	"chaos",
	"loadbalance",
	"cache",
	"rewrite",
	"dnssec",
	"autopath",
	"template",
	"transfer",
	"hosts",
	"route53",
	"file",
	"auto",
	"secondary",
	"kuscia",
	"loop",
	"forward",
	"erratic",
	"whoami",
	"on",
	"sign",
}

const (
	serverType = "dns"
)

type corednsModule struct {
	kubeclient kubernetes.Interface
	rootDir    string
	namespace  string
	envoyIP    string
}

func NewCoredns(i *Dependencies) Module {
	namespace := i.DomainID
	return &corednsModule{
		rootDir:    i.RootDir,
		namespace:  namespace,
		envoyIP:    i.EnvoyIP,
		kubeclient: i.Clients.KubeClient,
	}
}

func (s *corednsModule) Run(ctx context.Context) error {
	plugin.Register(
		"kuscia",
		func(c *caddy.Controller) error {
			e, err := coredns.KusciaParse(ctx, c, s.kubeclient, s.namespace, s.envoyIP)
			if err != nil {
				return plugin.Error("kuscia", err)
			}

			dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
				e.Next = next
				return e
			})

			return nil
		},
	)
	dnsserver.Directives = directives

	contents, err := os.ReadFile(filepath.Join(s.rootDir, ConfPrefix, "corefile"))
	if err != nil {
		return err
	}
	// Start your engines
	instance, err := caddy.Start(caddy.CaddyfileInput{
		Contents:       contents,
		ServerTypeName: serverType,
	})
	if err != nil {
		return err
	}

	// Twiddle your thumbs
	instance.Wait()
	return nil
}

func (s *corednsModule) WaitReady(ctx context.Context) error {
	return ctx.Err()
}

func (s *corednsModule) Name() string {
	return "coredns"
}

func RunCoreDNS(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewCoredns(conf)
	go func() {
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Error(err)
		cancel()
	} else {
		nlog.Info("coredns is ready")
	}

	return m
}
