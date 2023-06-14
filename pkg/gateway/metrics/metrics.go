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

package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	metricsMonitorPeriod = 30 * time.Second
)

var (
	gatewayCPUStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu",
			Help: "gateway cpu status",
		},
		[]string{"stat"},
	)
	gatewayMemStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mem",
			Help: "gateway memory status",
		},
		[]string{"stat"},
	)
	gatewayIOStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "io",
			Help: "gateway io status",
		},
		[]string{"stat"},
	)
)

func MonitorRuntimeMetrics(stopCh <-chan struct{}) {
	if err := updateMetrics(); err != nil {
		nlog.Errorf("update prometheus metrics error: %v", err)
	}
	for {
		select {
		case <-time.After(metricsMonitorPeriod):
			if err := updateMetrics(); err != nil {
				nlog.Errorf("update prometheus metrics error: %v", err)
			}
		case <-stopCh:
			return
		}
	}
}

func updateMetrics() error {
	if cpustat, err := load.Avg(); err == nil {
		gatewayCPUStatus.WithLabelValues("num").Set(float64(runtime.NumCPU()))
		gatewayCPUStatus.WithLabelValues("load1").Set(cpustat.Load1)
		gatewayCPUStatus.WithLabelValues("load5").Set(cpustat.Load5)
		gatewayCPUStatus.WithLabelValues("load15").Set(cpustat.Load15)
	}

	if memstat, err := mem.VirtualMemory(); err == nil {
		gatewayMemStatus.WithLabelValues("total").Set(float64(memstat.Total))
		gatewayMemStatus.WithLabelValues("used").Set(float64(memstat.Used))
		gatewayMemStatus.WithLabelValues("free").Set(float64(memstat.Free))
		gatewayMemStatus.WithLabelValues("shared").Set(float64(memstat.Shared))
		gatewayMemStatus.WithLabelValues("buff.cache").Set(float64(memstat.Buffers + memstat.Cached))
		gatewayMemStatus.WithLabelValues("available").Set(float64(memstat.Available))
	}

	if iostats, err := net.IOCounters(true); err == nil {
		for _, iostat := range iostats {
			if iostat.Name != "eth0" && iostat.Name != "en0" {
				continue
			}

			gatewayIOStatus.WithLabelValues("send.bytes").Set(float64(iostat.BytesSent))
			gatewayIOStatus.WithLabelValues("send.packets").Set(float64(iostat.PacketsSent))
			gatewayIOStatus.WithLabelValues("recv.bytes").Set(float64(iostat.BytesRecv))
			gatewayIOStatus.WithLabelValues("recv.packets").Set(float64(iostat.PacketsRecv))
		}
	}
	return nil
}
