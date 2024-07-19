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

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/ssexporter"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
)

type ssExporterModule struct {
	moduleRuntimeBase
	runMode            pkgcom.RunModeType
	domainID           string
	rootDir            string
	metricUpdatePeriod uint
	ssExportPort       string
}

func NewSsExporter(i *ModuleRuntimeConfigs) (Module, error) {
	readyURI := fmt.Sprintf("http://127.0.0.1:%s", i.SsExportPort)
	return &ssExporterModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "ssexporter",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewHTTPReadyZ(readyURI, 404, func(body []byte) error {
				return nil
			}),
		},
		runMode:            i.RunMode,
		domainID:           i.DomainID,
		rootDir:            i.RootDir,
		metricUpdatePeriod: i.MetricUpdatePeriod,
		ssExportPort:       i.SsExportPort,
	}, nil
}

func (exporter *ssExporterModule) Run(ctx context.Context) error {
	return ssexporter.SsExporter(ctx, exporter.runMode, exporter.domainID, exporter.metricUpdatePeriod, exporter.ssExportPort)
}
