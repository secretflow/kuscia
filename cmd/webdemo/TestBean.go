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

package main

import (
	"context"
	"strconv"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/asserts"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
)

type TestBean struct {
	framework.ConfigLoader
	// config
	DataStoreType string `name:"data-store-type" usage:"Data Store Type" required:""`
	RPCPort       int    `name:"rpc-port" usage:"rpc-port" default:"8118"`

	DataStoreRPCPort string
	DemoConfig       *DemoConfig
}

func NewTestBean(c *DemoConfig) *TestBean {
	return &TestBean{
		ConfigLoader: &config.FlagEnvConfigLoader{Source: config.SourceFlag},
		DemoConfig:   c,
	}
}

func (b *TestBean) Validate(errs *errorcode.Errs) {
	errs.AppendErr(asserts.NotEmpty(b.DataStoreType, "DataStoreType is empty"))
	errs.AppendErr(asserts.IsTrue(b.RPCPort > 0, "RpcPort is invalid"))
}

func (b *TestBean) Init(e framework.ConfBeanRegistry) error {
	b.DataStoreRPCPort = b.DataStoreType + strconv.Itoa(b.RPCPort)
	// TODO Init ...
	nlog.Info("Init: " + b.DataStoreRPCPort)
	return nil
}

func (b *TestBean) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	nlog.Info("Start" + b.DataStoreRPCPort)
	b.DemoConfig.Print()
	return nil
}
