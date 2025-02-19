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

package kusciainit

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

func NewInitCommand(ctx context.Context) *cobra.Command {
	config := &InitConfig{}
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Init means init Kuscia config",
		Long:         `Init means init Kuscia config`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.DomainID == common.UnSupportedDomainID {
				nlog.Fatal("Input domain id can't be 'master', please choose another name")
			}
			kusciaConfig := config.convert2KusciaConfig()
			out, err := yaml.Marshal(kusciaConfig)
			if err != nil {
				nlog.Fatalf("Failed to convert to YAML format, err: %v", err)
			}

			fmt.Println(string(out))
			return nil
		},
	}
	cmd.Flags().StringVarP(&config.Mode, "mode", "", "", "Deploy Domain mode (Master, Lite, Autonomy), case insensitive")
	cmd.Flags().StringVarP(&config.DomainID, "domain", "d", "", "Domain ID, must follow DNS subdomain rules")
	cmd.Flags().StringVarP(&config.Runtime, "runtime", "r", "runc", "Domain runtime (runc, runk, runp), default runc")
	cmd.Flags().StringVarP(&config.DomainKeyFile, "domain-key-file", "f", "", "Load domain RSA private key file, none generate domain RSA key data")
	cmd.Flags().StringVarP(&config.LogLevel, "log-level", "l", "INFO", "Logging level (INFO, DEBUG, WARN) default INFO")
	cmd.Flags().StringVarP(&config.LiteDeployToken, "lite-deploy-token", "t", "", "The deploy token used by the lite connecting to the master")
	cmd.Flags().StringVarP(&config.MasterEndpoint, "master-endpoint", "m", "", "The master endpoint the lite connecting to")
	cmd.Flags().StringVarP(&config.DatastoreEndpoint, "datastore-endpoint", "e", "", "Database dsn connection string")
	cmd.Flags().StringVarP(&config.Protocol, "protocol", "p", "", "Protocol for KusciaAPI and gateway (NOTLS, TLS, MTLS)")
	cmd.Flags().BoolVarP(&config.EnableWorkloadApprove, "enable-workload-approve", "", false, "Approve configs for workloads")
	return cmd
}

type InitConfig struct {
	Mode                  string
	DomainID              string
	DomainKeyFile         string
	LogLevel              string
	LiteDeployToken       string
	MasterEndpoint        string
	Runtime               string
	DatastoreEndpoint     string
	Protocol              string
	EnableWorkloadApprove bool
}

func (config *InitConfig) convert2KusciaConfig() interface{} {
	var kusciaConfig interface{}
	if err := validateConfig(config); err != nil {
		nlog.Fatalf("Invalid config, err: %v", err)
	}

	mode := strings.ToLower(config.Mode)
	switch mode {
	case common.RunModeLite:
		kusciaConfig = config.convert2LiteKusciaConfig()
	case common.RunModeMaster:
		kusciaConfig = config.convert2MasterKusciaConfig()
	case common.RunModeAutonomy:
		kusciaConfig = config.convert2AutonomyKusciaConfig()
	default:
		nlog.Fatalf("Unsupported mode: %s", mode)
	}
	return kusciaConfig
}

func validateConfig(config *InitConfig) error {
	if len(config.DomainID) == 0 {
		return fmt.Errorf("invalid domain: empty not allowed")
	}

	err := resources.ValidateK8sName(config.DomainID, "DomainID")
	if err != nil {
		return fmt.Errorf("invalid domain: must conform to a regular expression `%s`", common.K3sRegex)
	}

	// validate runtime
	mode := strings.ToLower(config.Mode)
	if mode != common.RunModeMaster {
		if !isSupportedRuntime(config.Runtime) {
			return fmt.Errorf("unsupported runtime: %s", config.Runtime)
		}
	}

	if !isSupportedProtocol(config.Protocol) {
		return fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}

	return nil
}

func (config *InitConfig) convert2LiteKusciaConfig() confloader.LiteKusciaConfig {
	kusciaConfig := confloader.LiteKusciaConfig{
		CommonConfig:    config.convert2CommonConfig(),
		LiteDeployToken: config.LiteDeployToken,
		MasterEndpoint:  config.MasterEndpoint,
		Runtime:         config.Runtime,
	}
	return kusciaConfig
}

func (config *InitConfig) convert2MasterKusciaConfig() confloader.MasterKusciaConfig {
	kusciaConfig := confloader.MasterKusciaConfig{
		CommonConfig:      config.convert2CommonConfig(),
		DatastoreEndpoint: config.DatastoreEndpoint,
		AdvancedConfig:    config.convert2AdvancedConfig(),
	}
	return kusciaConfig
}

func (config *InitConfig) convert2AutonomyKusciaConfig() confloader.AutonomyKusciaConfig {
	kusciaConfig := confloader.AutonomyKusciaConfig{
		CommonConfig:      config.convert2CommonConfig(),
		Runtime:           config.Runtime,
		DatastoreEndpoint: config.DatastoreEndpoint,
		AdvancedConfig:    config.convert2AdvancedConfig(),
	}
	return kusciaConfig
}

func (config *InitConfig) convert2CommonConfig() confloader.CommonConfig {
	domainKeyData, err := loadDomainKeyData(config.DomainKeyFile)
	if err != nil {
		nlog.Fatalf("Init kuscia config err: %v", err)
	}
	commonConfig := confloader.CommonConfig{
		Mode:          config.Mode,
		DomainID:      config.DomainID,
		LogLevel:      config.LogLevel,
		DomainKeyData: domainKeyData,
		Protocol:      common.Protocol(config.Protocol),
	}
	return commonConfig
}

func (config *InitConfig) convert2AdvancedConfig() confloader.AdvancedConfig {
	return confloader.AdvancedConfig{
		EnableWorkloadApprove: config.EnableWorkloadApprove,
	}
}

func loadDomainKeyData(domainKeyFile string) (string, error) {
	var domainKeyData string
	var err error
	if domainKeyFile != "" {
		domainKeyData, err = tls.LoadKeyData(domainKeyFile)
		if err != nil {
			nlog.Errorf("Load domain key file error: %v", err)
			return "", err
		}
	} else {
		domainKeyData, err = tls.GenerateKeyData()
		if err != nil {
			nlog.Errorf("Generate domain key error: %v", err)
			return "", err
		}
	}
	return domainKeyData, nil
}

func isSupportedRuntime(runtime string) bool {
	switch runtime {
	case config.ContainerRuntime, config.K8sRuntime, config.ProcessRuntime:
		return true
	default:
		return false
	}
}

func isSupportedProtocol(protocol string) bool {
	if protocol == "" {
		return true
	}
	protocol = strings.ToUpper(protocol)
	switch common.Protocol(protocol) {
	case common.NOTLS, common.TLS, common.MTLS:
		return true
	default:
		return false
	}
}
