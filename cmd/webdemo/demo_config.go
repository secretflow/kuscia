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
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/asserts"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
)

type DemoConfig struct {
	framework.ConfigLoader
	// config
	TestTag string `name:"test-type" usage:"Test type" required:""`
	Author  string `name:"author" usage:"Print demo author" default:"ant"`
	Token   string `name:"token" usage:"Token" secret:"pwd"`
}

func NewDemoConfig() *DemoConfig {
	return &DemoConfig{
		ConfigLoader: &config.FlagEnvConfigLoader{Source: config.SourceFlag},
	}
}

func (c *DemoConfig) Validate(errs *errorcode.Errs) {
	errs.AppendErr(asserts.NotEmpty(c.TestTag, "DataStoreType is empty"))
}

func (c *DemoConfig) Print() {
	nlog.Infof("TestTag=%v, Autho=%v, Token=%v", c.TestTag, c.Author, c.Token)
}
