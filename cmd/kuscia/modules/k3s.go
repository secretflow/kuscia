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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type k3sModule struct {
	rootDir        string
	kubeconfigFile string
	bindAddress    string
	listenPort     string
	dataDir        string

	enableAudit bool
}

func (s *k3sModule) readyz(host string) error {
	cl := http.Client{}
	caCertFile, err := os.ReadFile(filepath.Join(s.dataDir, "server/tls/server-ca.crt"))
	if err != nil {
		nlog.Error(err)
		return err
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertFile) {
		msg := "caCertFile format error"
		nlog.Error(msg)
		return fmt.Errorf("%s", msg)
	}
	certPEMBlock, err := os.ReadFile(filepath.Join(s.dataDir, "server/tls/client-admin.crt"))
	if err != nil {
		nlog.Error(err)
		return err
	}

	keyPEMBlock, err := os.ReadFile(filepath.Join(s.dataDir, "server/tls/client-admin.key"))
	if err != nil {
		nlog.Error(err)
		return err
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		nlog.Error(err)
		return err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:      caCertPool,
			Certificates: []tls.Certificate{cert},
		},
	}
	cl.Transport = tr
	req, err := http.NewRequest(http.MethodGet, host+"/readyz", nil)
	if err != nil {
		nlog.Errorf("NewRequest error:%s", err.Error())
		return err
	}
	resp, err := cl.Do(req)
	if err != nil {
		nlog.Errorf("Get ready err:%s", err.Error())
		return err
	}
	if resp == nil || resp.Body == nil {
		nlog.Error("resp must has body")
		return fmt.Errorf("resp must has body")
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		nlog.Error("ReadAll fail")
		return err
	}
	if string(respBytes) != "ok" {
		return errors.New("not ready")
	}
	return nil
}

func NewK3s(i *Dependencies) Module {
	return &k3sModule{
		rootDir:        i.RootDir,
		kubeconfigFile: i.KubeconfigFile,
		bindAddress:    "0.0.0.0",
		listenPort:     "6443",
		dataDir:        filepath.Join(i.RootDir, "var/k3s/"),
		enableAudit:    false,
	}
}

func (s *k3sModule) Run(ctx context.Context) error {

	args := []string{
		"server",
		"-v=5",
		"-d=" + s.dataDir,
		"-o=" + s.kubeconfigFile,
		"--disable-agent",
		"--bind-address=" + s.bindAddress,
		"--https-listen-port=" + s.listenPort,
		"--disable-cloud-controller",
		"--disable-network-policy",
		"--disable-scheduler",
		"--flannel-backend=none",
		"--disable=traefik",
		"--disable=coredns",
		"--disable=servicelb",
		"--disable=local-storage",
		"--disable=metrics-server",
	}
	if s.enableAudit {
		args = append(args,
			"--kube-apiserver-arg=audit-log-path="+filepath.Join(s.rootDir, LogPrefix, "k3s-audit.log"),
			"--kube-apiserver-arg=audit-policy-file="+filepath.Join(s.rootDir, ConfPrefix, "k3s/k3s-audit-policy.yaml"),
			"--kube-apiserver-arg=audit-log-maxbackup=10",
			"--kube-apiserver-arg=audit-log-maxsize=300",
		)
	}

	sp := supervisor.NewSupervisor("k3s", nil, -1)
	fout, err := os.OpenFile(filepath.Join(s.rootDir, LogPrefix, "k3s.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		nlog.Warnf("open k3s stdout logfile failed")
		return nil
	}
	defer fout.Close()
	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.CommandContext(ctx, filepath.Join(s.rootDir, "bin/k3s"), args...)
		cmd.Stderr = fout
		cmd.Stdout = fout
		return cmd
	})
}

func (s *k3sModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	tickerReady := time.NewTicker(time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerReady.C:
			if nil == s.readyz("https://127.0.0.1:"+s.listenPort) {
				return nil
			}
		case <-ticker.C:
			return fmt.Errorf("wait k3s ready timeout")
		}
	}
}

func (s *k3sModule) Name() string {
	return "k3s"
}

func RunK3s(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewK3s(conf)
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
		if err = applyCRD(conf); err != nil {
			nlog.Error(err)
			cancel()
		}

		nlog.Info("k3s is ready")
	}

	return m
}

func applyCRD(conf *Dependencies) error {
	dirPath := filepath.Join(conf.RootDir, "crds/v1alpha1")
	dirs, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		if dir.IsDir() {
			continue
		}
		cmd := exec.Command(filepath.Join(conf.RootDir, "bin/kubectl"), "--kubeconfig", conf.KubeconfigFile, "apply", "-f", filepath.Join(dirPath, dir.Name()))
		nlog.Infof("apply %s", filepath.Join(dirPath, dir.Name()))
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			nlog.Errorf("apply %s err:%s", filepath.Join(dirPath, dir.Name()), err.Error())
			return err
		}
	}
	return nil
}
