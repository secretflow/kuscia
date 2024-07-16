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
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/transport/server/http"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

const (
	transportBinPath    = "bin/transport"
	transportModuleName = "transport"
)

type transportModule struct {
	rootDir    string
	configPath string
	address    string
}

func NewTransport(i *Dependencies) Module {
	return &transportModule{
		rootDir:    i.RootDir,
		configPath: i.TransportConfigFile,
		address:    fmt.Sprintf("127.0.0.1:%d", i.TransportPort),
	}
}

func (t *transportModule) Run(ctx context.Context) error {
	return t.runAsGoroutine(ctx)
}

func (t *transportModule) runAsGoroutine(ctx context.Context) error {
	return http.Run(ctx, t.configPath)
}

func (t *transportModule) runAsSubProcess(ctx context.Context) error {
	LogDir := filepath.Join(t.rootDir, common.LogPrefix, fmt.Sprintf("%s/", transportModuleName))
	if err := os.MkdirAll(LogDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(LogDir, fmt.Sprintf("%s/%s.log", transportModuleName, transportModuleName))
	args := []string{
		"--transport-config=" + t.configPath,
		"--log.path=" + logPath,
	}

	sp := supervisor.NewSupervisor(transportModuleName, nil, -1)
	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.Command(filepath.Join(t.rootDir, transportBinPath), args...)
		cmd.Env = os.Environ()
		return &ModuleCMD{
			cmd:   cmd,
			score: &transportOOMScore,
		}
	})
}

func (t *transportModule) Name() string {
	return transportModuleName
}

func (t *transportModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	tickerReady := time.NewTicker(time.Second)
	defer tickerReady.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerReady.C:
			if nil == t.readyz(t.address) {
				return nil
			}
		case <-ticker.C:
			return fmt.Errorf("wait transport ready timeout")
		}
	}
}

func (t *transportModule) readyz(address string) error {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil || conn == nil {
		err := fmt.Errorf("transport start fail, err: %v", err)
		nlog.Warnf("%v", err)
		return err
	}
	conn.Close()
	return nil
}

func RunTransportWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := NewShutdownHookEntry(1 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:      "transport",
		DestroyCh: runCtx.Done(),
		DestroyFn: cancel,
	})
	RunTransport(runCtx, cancel, conf, shutdownEntry)
}

func RunTransport(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) Module {
	m := NewTransport(conf)
	go func() {
		defer func() {
			if shutdownEntry != nil {
				shutdownEntry.RunShutdown()
			}
		}()
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Fatalf("Transport wait ready failed: %v", err)
	}
	nlog.Info("transport is ready")
	return m
}

func GetTransportPort(configPath string) (int, error) {
	transConfig, err := config.LoadTransConfig(configPath)
	if err != nil {
		return 0, err
	}
	return transConfig.HTTPConfig.Port, nil
}
