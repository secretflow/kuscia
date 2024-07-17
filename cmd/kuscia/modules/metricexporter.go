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
	"time"

	"github.com/secretflow/kuscia/pkg/metricexporter"
	"github.com/secretflow/kuscia/pkg/metricexporter/envoyexporter"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
)

type metricExporterModule struct {
	moduleRuntimeBase
	rootDir          string
	metricURLs       map[string]string
	nodeExportPort   string
	ssExportPort     string
	metricExportPort string
}

func NewMetricExporter(i *ModuleRuntimeConfigs) (Module, error) {
	readyURI := fmt.Sprintf("http://127.0.0.1:%s", i.MetricExportPort)
	return &metricExporterModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "metricexporter",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewHTTPReadyZ(readyURI, 404, func(body []byte) error {
				return nil
			}),
		},
		rootDir:          i.RootDir,
		nodeExportPort:   i.NodeExportPort,
		ssExportPort:     i.SsExportPort,
		metricExportPort: i.MetricExportPort,
		metricURLs: map[string]string{
			"node-exporter": "http://localhost:" + i.NodeExportPort + "/metrics",
			"envoy":         envoyexporter.GetEnvoyMetricURL(),
			"ss":            "http://localhost:" + i.SsExportPort + "/ssmetrics",
		},
	}, nil
}

func (exporter *metricExporterModule) Run(ctx context.Context) error {
	metricexporter.MetricExporter(ctx, exporter.metricURLs, exporter.metricExportPort)
	return nil
}
