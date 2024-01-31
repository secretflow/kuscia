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
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type nodeExporterModule struct {
	runMode    pkgcom.RunModeType
	rootDir    string
	exportPort string
}

func NewNodeExporter(i *Dependencies) Module {
	return &nodeExporterModule{
		runMode:    i.RunMode,
		rootDir:    i.RootDir,
		exportPort: ":9100",
	}
}

func (exporter *nodeExporterModule) Run(ctx context.Context) error {
	var args []string
	if exporter.runMode == "master" {
		exporter.exportPort = ":9091"
		args = append(args, "--web.listen-address")
		args = append(args, exporter.exportPort)
	}
	sp := supervisor.NewSupervisor("node_exporter", nil, -1)
	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.CommandContext(ctx, filepath.Join(exporter.rootDir, "bin/node_exporter"), args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		return cmd
	})
}

func (exporter *nodeExporterModule) readyz(host string) error {
	cl := http.Client{}
	req, err := http.NewRequest(http.MethodGet, host, nil)
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
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		nlog.Error("ReadAll fail")
		return err
	}
	return nil
}

func (exporter *nodeExporterModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	tickerReady := time.NewTicker(time.Second)
	defer tickerReady.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerReady.C:
			if nil == exporter.readyz("http://127.0.0.1"+exporter.exportPort) {
				return nil
			}
		case <-ticker.C:
			return fmt.Errorf("wait node_exporter ready timeout")
		}
	}

}

func (exporter *nodeExporterModule) Name() string {
	return "nodeexporter"
}

func RunNodeExporter(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewNodeExporter(conf)
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
		nlog.Info("Node_exporter is ready")
	}
	return m
}
