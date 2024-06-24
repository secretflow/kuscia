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

	"github.com/secretflow/kuscia/pkg/metricexporter"
	"github.com/secretflow/kuscia/pkg/metricexporter/envoyexporter"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type metricExporterModule struct {
	rootDir          string
	metricURLs       map[string]string
	nodeExportPort   string
	ssExportPort     string
	metricExportPort string
}

func NewMetricExporter(i *Dependencies) Module {
	return &metricExporterModule{
		rootDir:          i.RootDir,
		nodeExportPort:   i.NodeExportPort,
		ssExportPort:     i.SsExportPort,
		metricExportPort: i.MetricExportPort,
		metricURLs: map[string]string{
			"node-exporter": "http://localhost:" + i.NodeExportPort + "/metrics",
			"envoy":         envoyexporter.GetEnvoyMetricURL(),
			"ss":            "http://localhost:" + i.SsExportPort + "/ssmetrics",
		},
	}
}

func (exporter *metricExporterModule) Run(ctx context.Context) error {
	metricexporter.MetricExporter(ctx, exporter.metricURLs, exporter.metricExportPort)
	return nil
}

func (exporter *metricExporterModule) readyz(host string) error {
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

func (exporter *metricExporterModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	tickerReady := time.NewTicker(time.Second)
	defer tickerReady.Stop()
	for {
		select {
		case <-tickerReady.C:
			if nil == exporter.readyz("http://localhost:"+exporter.metricExportPort) {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			return fmt.Errorf("wait metric exporter ready timeout")
		}
	}
}

func (exporter *metricExporterModule) Name() string {
	return "metricexporter"
}

func RunMetricExporterWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := newShutdownHookEntry(1 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "metricexporter",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	RunMetricExporter(runCtx, cancel, conf, shutdownEntry)
}

func RunMetricExporter(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) Module {
	m := NewMetricExporter(conf)
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
		nlog.Fatalf("MetricTransport wait ready failed: %v", err)
	}
	nlog.Info("Metric exporter is ready")
	return m
}
