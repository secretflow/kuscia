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
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/coredns"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
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
	serverType   = "dns"
	tmpDirPrefix = "var/tmp/"
)

type CorednsModule struct {
	rootDir         string
	namespace       string
	envoyIP         string
	readyChan       chan struct{}
	coreDNSInstance *coredns.KusciaCoreDNS
}

func NewCoreDNS(conf *confloader.KusciaConfig) Module {
	return &CorednsModule{
		rootDir:   conf.RootDir,
		namespace: conf.DomainID,
		envoyIP:   conf.EnvoyIP,
		readyChan: make(chan struct{}),
	}
}

func (s *CorednsModule) Run(ctx context.Context) error {
	defer close(s.readyChan)
	if err := prepareResolvConf(s.rootDir); err != nil {
		nlog.Errorf("Failed to prepare coredns resolv.conf, %v", err)
		return err
	}

	plugin.Register(
		"kuscia",
		func(c *caddy.Controller) error {
			e, err := coredns.KusciaParse(c, s.namespace, s.envoyIP)
			if err != nil {
				return plugin.Error("kuscia", err)
			}

			dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
				e.Next = next
				return e
			})
			s.coreDNSInstance = e
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

	s.readyChan <- struct{}{}
	// Twiddle your thumbs
	instance.Wait()
	return nil
}

func (s *CorednsModule) WaitReady(ctx context.Context) error {
	select {
	case <-s.readyChan:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *CorednsModule) Name() string {
	return "coredns"
}

func RunCoreDNS(ctx context.Context, cancel context.CancelFunc, conf *confloader.KusciaConfig) Module {
	m := NewCoreDNS(conf)
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

func (s *CorednsModule) StartControllers(ctx context.Context, kubeclient kubernetes.Interface) {
	s.coreDNSInstance.StartControllers(ctx, kubeclient)
}

func prepareResolvConf(rootDir string) error {
	nlog.Infof("Start preparing coredns resolv.conf, root dir %v", rootDir)
	hostIP, err := network.GetHostIP()
	if err != nil {
		return err
	}

	resolvConf := "/etc/resolv.conf"
	backupResolvConf := filepath.Join(rootDir, tmpDirPrefix, "resolv.conf")
	exist := paths.CheckFileExist(backupResolvConf)
	if !exist {
		if err = paths.CopyFile(resolvConf, backupResolvConf); err != nil {
			return err
		}

		if err = updateResolvConf(backupResolvConf, hostIP, false); err != nil {
			return err
		}
	}

	if err = updateResolvConf(resolvConf, hostIP, true); err != nil {
		return err
	}

	nlog.Info("Finish preparing coredns resolv.conf")
	return nil
}

func updateResolvConf(fileName, hostIP string, add bool) error {
	lines, err := getFileContent(fileName)
	if err != nil {
		return err
	}
	var finalContent []string
	content := fmt.Sprintf("nameserver %s", hostIP)
	switch add {
	// add specific content
	case true:
		if len(lines) == 1 && strings.Contains(lines[0], content) {
			return nil
		}
		finalContent = append(finalContent, content)
	// delete specific content
	default:
		for i := range lines {
			if !strings.Contains(lines[i], content) {
				finalContent = append(finalContent, lines[i])
			}
		}

		// Coredns server would check whether the nameserver exist in backup resolv.conf.
		// If not, the coredns server will crash.
		// So if finalContent doesn't include nameserver, skip updating the file.
		foundNameserver := false
		for _, c := range finalContent {
			if strings.Contains(c, "nameserver") {
				foundNameserver = true
				break
			}
		}
		if !foundNameserver {
			nlog.Warnf("Due to the backup resolv.conf content %v doesn't include nameserver after removing %q, skip updating it", lines, content)
			return nil
		}
	}

	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, c := range finalContent {
		if _, err = fmt.Fprintln(file, c); err != nil {
			return err
		}
	}
	return nil
}

func getFileContent(fileName string) ([]string, error) {
	var lines []string
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil, err
	}
	return lines, nil
}
