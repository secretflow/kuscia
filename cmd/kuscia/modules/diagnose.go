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

package modules

import (
	"context"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/mods"
	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	SubCMDCustomResourceDestination = "crd"
	SubCMDClusterDomainRoute        = "cdr"
	SubCMDNetwork                   = "network"
	SubCMDAPP                       = "app"
)

type diagnoseModule struct {
	Reporter *util.Reporter
	Mod      mods.Mod
}

func NewDiagnose(config *mods.DiagnoseConfig) Module {
	m := new(diagnoseModule)
	m.Reporter = util.NewReporter(config.ReportFile)
	kusciaAPIConn, err := client.NewKusciaAPIConn()
	if err != nil {
		nlog.Fatalf("init kuscia api conn failed, %v", err)
	}
	switch config.Command {
	case SubCMDNetwork:
		m.Mod = mods.NewNetworkMod(m.Reporter, kusciaAPIConn, config)
	case SubCMDClusterDomainRoute:
		m.Mod = mods.NewDomainRouteMod(m.Reporter, kusciaAPIConn, config)
	default:
		nlog.Errorf("invalid support subcmd")
		return nil
	}
	return m
}

func (d *diagnoseModule) Run(ctx context.Context) error {
	defer func() {
		d.Reporter.Render()
		d.Reporter.Close()
	}()
	if err := d.Mod.Run(ctx); err != nil {
		return err
	}
	return nil
}

func (d *diagnoseModule) WaitReady(ctx context.Context) error {
	return nil
}

func (d *diagnoseModule) Name() string {
	return "diagnose"
}
