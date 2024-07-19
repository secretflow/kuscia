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
	"os"
	"os/exec"
	"path/filepath"
	"time"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type nodeExporterModule struct {
	moduleRuntimeBase
	runMode        pkgcom.RunModeType
	rootDir        string
	nodeExportPort string
}

func NewNodeExporter(i *ModuleRuntimeConfigs) (Module, error) {
	readyURI := fmt.Sprintf("http://127.0.0.1:%s", i.NodeExportPort)
	return &nodeExporterModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "nodeexporter",
			readyTimeout: 60 * time.Second,
			rdz: readyz.NewHTTPReadyZ(readyURI, 200, func(body []byte) error {
				return nil
			}),
		},
		runMode:        i.RunMode,
		rootDir:        i.RootDir,
		nodeExportPort: i.NodeExportPort,
	}, nil
}

func (exporter *nodeExporterModule) Run(ctx context.Context) error {
	var args []string
	args = append(args, "--web.listen-address", ":"+exporter.nodeExportPort)
	disabledCollectors := getDisabledCollectors()
	args = append(args, disabledCollectors...)
	sp := supervisor.NewSupervisor("node_exporter", nil, -1)

	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.Command(filepath.Join(exporter.rootDir, "bin/node_exporter"), args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		return &ModuleCMD{
			cmd:   cmd,
			score: &nodeExporterOOMScore,
		}
	})
}

func getDisabledCollectors() []string {
	var disabledCollectors []string
	disabledCollectors = append(disabledCollectors, "--no-collector.bcache",
		"--no-collector.bonding",
		"--no-collector.btrfs",
		"--no-collector.conntrack",
		"--no-collector.cpufreq",
		"--no-collector.dmi",
		"--no-collector.edac",
		"--no-collector.entropy",
		"--no-collector.fibrechannel",
		"--no-collector.hwmon",
		"--no-collector.infiniband",
		"--no-collector.ipvs",
		"--no-collector.mdadm",
		"--no-collector.netclass",
		"--no-collector.nfs",
		"--no-collector.nfsd",
		"--no-collector.nvme",
		"--no-collector.os",
		"--no-collector.powersupplyclass",
		"--no-collector.pressure",
		"--no-collector.rapl",
		"--no-collector.schedstat",
		"--no-collector.selinux",
		"--no-collector.softnet",
		"--no-collector.stat",
		"--no-collector.tapestats",
		"--no-collector.textfile",
		"--no-collector.thermal_zone",
		"--no-collector.time",
		"--no-collector.timex",
		"--no-collector.udp_queues",
		"--no-collector.uname",
		"--no-collector.vmstat",
		"--no-collector.xfs",
		"--no-collector.zfs",
		"--no-collector.arp",
		"--no-collector.cpu_vulnerabilities",
		"--no-collector.cpu.guest",
		"--web.disable-exporter-metrics",
		"--collector.filesystem.fs-types-exclude=^(autofs|binfmt_misc|bpf|cgroup2?|configfs|debugfs|devpts|devtmpfs|fusectl|hugetlbfs|iso9660|mqueue|nsfs|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|selinuxfs|squashfs|sysfs|tracefs|tmpfs)$",
		"--collector.filesystem.mount-points-exclude=^/(dev|proc|sys|var|run|boot|/lib/docker/.+|var/lib/kubelet/.+)($|/)")
	return disabledCollectors
}
