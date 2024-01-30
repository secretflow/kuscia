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
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type ssExporterModule struct {
	runMode            pkgcom.RunModeType
	rootDir            string
	metricUpdatePeriod uint
}

func NewSsExporter(i *Dependencies) Module {
	return &ssExporterModule{
		runMode:            i.RunMode,
		rootDir:            i.RootDir,
		metricUpdatePeriod: i.MetricUpdatePeriod,
	}
}

func (exporter *ssExporterModule) Run(ctx context.Context) error {
	ssexporter.SsExporter(ctx, exporter.runMode, exporter.metricUpdatePeriod)
	return nil
}

func (exporter *ssExporterModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	select {
	case <-ssexporter.ReadyChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return fmt.Errorf("wait metric exporter ready timeout")
	}
}

func (exporter *ssExporterModule) Name() string {
	return "ssexporter"
}

func RunSsExporter(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewSsExporter(conf)
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
		nlog.Info("Ss exporter is ready")
	}
	return m
}
