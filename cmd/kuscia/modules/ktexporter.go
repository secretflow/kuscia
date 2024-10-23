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
	"time"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/ktexporter"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ktExporterModule struct {
	runMode            pkgcom.RunModeType
	domainID           string
	rootDir            string
	metricUpdatePeriod uint
	ktExportPort       string
}

func NewKtExporter(i *Dependencies) Module {
	return &ktExporterModule{
		runMode:            i.RunMode,
		domainID:           i.DomainID,
		rootDir:            i.RootDir,
		metricUpdatePeriod: i.MetricUpdatePeriod,
		ktExportPort:       i.KtExportPort,
	}
}

func (exporter *ktExporterModule) Run(ctx context.Context) error {
	ktexporter.KtExporter(ctx, exporter.runMode, exporter.domainID, exporter.metricUpdatePeriod, exporter.ktExportPort)
	return nil
}

func (exporter *ktExporterModule) readyz(host string) error {
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

func (exporter *ktExporterModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	tickerReady := time.NewTicker(time.Second)
	defer tickerReady.Stop()
	for {
		select {
		case <-tickerReady.C:
			if nil == exporter.readyz("http://localhost:"+exporter.ktExportPort) {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			return fmt.Errorf("wait metric exporter ready timeout")
		}
	}
}

func (exporter *ktExporterModule) Name() string {
	return "ktexporter"
}

func RunKtExporterWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := NewShutdownHookEntry(500 * time.Millisecond)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "ktexporter",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	RunKtExporter(runCtx, cancel, conf, shutdownEntry)
}

func RunKtExporter(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) Module {
	m := NewKtExporter(conf)
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
		nlog.Fatalf("KtTransport wait ready failed: %v", err)
	}
	nlog.Info("Kt exporter is ready")
	return m
}
