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

package commands

import (
	"os"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
)

type Opts struct {
	Kubeconfig        string
	Namespace         string
	GatewayConfigFile string
	ResyncPeriod      int
	IdleTimeout       int
	logCfg            *zlogwriter.LogConfig
}

// AddFlags adds flags for a specific server to the specified FlagSet.
func (o *Opts) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Kubeconfig, "kubeconfig", "", "Path to a kubeconfig.")
	fs.StringVar(&o.Namespace, "namespace", "default", "Namespace to list-watch services.")
	fs.StringVar(&o.GatewayConfigFile, "gateway-config", "conf/gateway.yaml", "Namespace to list-watch services.")
	fs.IntVar(&o.ResyncPeriod, "resync-period", 600, `Resync period of informers, in seconds`)
	fs.IntVar(&o.IdleTimeout, "idle-timeout", 60, "Specify a field of envoy routing config, namely "+
		"config.route.v3.RouteAction.idle_timeout, which bounds the amount of time the request’s stream may be idle.")

	o.logCfg = zlogwriter.InstallPFlags(fs)
}

func loadAndOverrideConfig(config *config.GatewayConfig, configPath string) *config.GatewayConfig {
	if configPath == "" {
		return config // no need to load config file
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		nlog.Errorf("read configfile %s failed, err: %s", configPath, err.Error())
		return config
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		nlog.Errorf("configfile %s format err: %s . will use default", configPath, err.Error())
		return config
	}

	return config
}

func (o *Opts) overWriteConfigByOpts(config *config.GatewayConfig) *config.GatewayConfig {
	if o.Kubeconfig != "" {
		if config.MasterConfig.APIServer != nil {
			config.MasterConfig.APIServer = &kusciaconfig.APIServerConfig{}
		}
		config.MasterConfig.APIServer.KubeConfig = o.Kubeconfig
	}

	if o.Namespace != "" {
		config.Namespace = o.Namespace
	}

	if o.IdleTimeout != 60 || config.IdleTimeout == 0 {
		config.IdleTimeout = o.IdleTimeout
	}
	return config
}

func (o *Opts) Config() *config.GatewayConfig {
	return loadAndOverrideConfig(o.overWriteConfigByOpts(config.DefaultStaticGatewayConfig()), o.GatewayConfigFile)
}
