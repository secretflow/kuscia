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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type envoyModule struct {
	rootDir               string
	cluster               string
	id                    string
	commandLineConfigFile string
}

type EnvoyCommandLineConfig struct {
	Args []string `yaml:"args,omitempty"`
}

func (s *envoyModule) readyz(host string) error {
	cl := http.Client{}
	req, err := http.NewRequest(http.MethodGet, host+"/ready", nil)
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
		nlog.Error("Resp must has body")
		return fmt.Errorf("resp must has body")
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		nlog.Error("ReadAll fail")
		return err
	}

	if string(respBytes)[:len(respBytes)-1] != "LIVE" {
		return errors.New("not ready")
	}
	return nil
}

func getEnvoyCluster(domain string) string {
	return fmt.Sprintf("kuscia-gateway-%s", domain)
}

func NewEnvoy(i *Dependencies) Module {
	return &envoyModule{
		rootDir:               i.RootDir,
		cluster:               getEnvoyCluster(i.DomainID),
		id:                    fmt.Sprintf("%s-%s", getEnvoyCluster(i.DomainID), utils.GetHostname()),
		commandLineConfigFile: "envoy/command-line.yaml",
	}
}

func (s *envoyModule) Run(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Join(s.rootDir, common.LogPrefix, "envoy/"), 0755); err != nil {
		return err
	}
	deltaArgs, err := s.readCommandArgs()
	if err != nil {
		return err
	}

	args := []string{
		"-c",
		filepath.Join(s.rootDir, common.ConfPrefix, "envoy/envoy.yaml"),
		"--service-cluster",
		s.cluster,
		"--service-node",
		s.id,
		"--log-path",
		filepath.Join(s.rootDir, common.LogPrefix, "envoy/envoy.log"),
	}
	args = append(args, deltaArgs.Args...)
	sp := supervisor.NewSupervisor("envoy", nil, -1)
	go s.logRotate(ctx)
	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.CommandContext(ctx, filepath.Join(s.rootDir, "bin/envoy"), args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		return cmd
	})
}

func (s *envoyModule) logRotate(ctx context.Context) {
	for {
		t := time.Now()
		n := time.Date(t.Year(), t.Month(), t.Day(), 0, 1, 0, 0, t.Location())
		d := n.Sub(t)
		if d < 0 {
			n = n.Add(24 * time.Hour)
			d = n.Sub(t)
		}

		time.Sleep(d)

		cmd := exec.Command("logrotate", filepath.Join(s.rootDir, common.ConfPrefix, "logrotate.conf"))
		if err := cmd.Run(); err != nil {
			nlog.Errorf("Logrotate run error: %v", err)
		}
	}
}

func (s *envoyModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(60 * time.Second)
	tickerReady := time.NewTicker(time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerReady.C:
			if nil == s.readyz("http://127.0.0.1:10000") {
				return nil
			}
		case <-ticker.C:
			return fmt.Errorf("wait envoy ready timeout")
		}
	}
}

func (s *envoyModule) Name() string {
	return "envoy"
}

func (s *envoyModule) readCommandArgs() (*EnvoyCommandLineConfig, error) {
	configPath := filepath.Join(s.rootDir, common.ConfPrefix, s.commandLineConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config EnvoyCommandLineConfig
	err = yaml.Unmarshal(data, &config)
	return &config, err
}

func RunEnvoy(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewEnvoy(conf)
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
		nlog.Info("Envoy is ready")
	}
	return m
}
