// Copyright 2024 Ant Group Co., Ltd.
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

package mods

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type DomainRouteMod struct {
	source      string
	destination string
	config      *netstat.NetworkParam
	reporter    *util.Reporter
	taskGroup   *netstat.TaskGroup
	crdMod      Mod
}

func NewDomainRouteMod(reporter *util.Reporter, kusciaAPIConn *grpc.ClientConn, config *DiagnoseConfig) Mod {
	items := []*CRDItem{
		{
			source:      config.Source,
			destination: config.Destination,
			typ:         common.CRDDomainRoute,
		},
	}
	if config.Bidirection {
		items = append(items, &CRDItem{
			source:      config.Destination,
			destination: config.Source,
			typ:         common.CRDDomainRoute,
		})
	}
	crdMod := NewCRDMod(items, reporter, kusciaAPIConn)

	// init
	svcName := fmt.Sprintf("diagnose.%s.svc", config.Destination)
	cli := client.NewDiagnoseClient(svcName)
	// net test
	taskGroup := netstat.NewTaskGroup(cli, config.NetworkParam)

	return &DomainRouteMod{
		source:      config.Source,
		destination: config.Destination,
		config:      config.NetworkParam,
		reporter:    reporter,
		taskGroup:   taskGroup,
		crdMod:      crdMod,
	}
}

func (m *DomainRouteMod) Run(ctx context.Context) error {
	nlog.Infof("diagnose <%s-%s> network statitsics", m.source, m.destination)
	// crd config diagnose
	if err := m.crdMod.Run(ctx); err != nil {
		return err
	}

	// run task
	results, err := m.taskGroup.Start(ctx)
	if err != nil {
		nlog.Errorf("Taskgroup failed, %v", err)
		return err
	}

	// display result
	table := m.reporter.NewTableWriter()
	table.SetTitle(fmt.Sprintf("NETWORK STATSTICS(%s-%s):", m.source, m.destination))
	table.AddHeader([]string{"NAME", "DETECTED VALUE", "THRESHOLD", "RESULT", "INFORMATION"})

	for _, taskOutput := range results {
		table.AddRow(taskOutput.ToStringArray())
	}
	return nil

}
