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
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
)

type NetworkMod struct {
	reporter *util.Reporter
	// NodeMod Mod
	cdrMod Mod
}

func NewNetworkMod(reporter *util.Reporter, kusciaAPIConn *grpc.ClientConn, config *DiagnoseConfig) Mod {
	return &NetworkMod{
		reporter: reporter,
		cdrMod:   NewDomainRouteMod(reporter, kusciaAPIConn, config),
	}
}

func (m *NetworkMod) Run(ctx context.Context) error {
	return m.cdrMod.Run(ctx)
}
