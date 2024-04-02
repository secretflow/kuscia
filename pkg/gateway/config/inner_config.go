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

package config

import (
	"fmt"
	"os"

	restclient "k8s.io/client-go/rest"

	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func LoadMasterProxyConfig(masterConfig *kusciaconfig.MasterConfig) (*MasterConfig, error) {
	proxyCluster, err := LoadClusterConfig(masterConfig.TLSConfig, masterConfig.Endpoint)
	if err != nil {
		return nil, err
	}
	return &MasterConfig{
		Master:      false,
		MasterProxy: proxyCluster,
	}, nil
}

func LoadMasterConfig(masterConfig *kusciaconfig.MasterConfig, kubeConfig *restclient.Config) (*MasterConfig, error) {
	apiCert, err := LoadTLSCertByKubeConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	protocol, host, port, path, err := utils.ParseURL(kubeConfig.Host)
	if err != nil {
		return nil, err
	}

	var storageCluster *ClusterConfig
	if masterConfig.KusciaStorage != nil {
		storageCluster, err = LoadClusterConfig(masterConfig.KusciaStorage.TLSConfig,
			masterConfig.KusciaStorage.Endpoint)
		if err != nil {
			return nil, err
		}
	}

	var kusciaAPICluster *ClusterConfig
	if masterConfig.KusciaAPI != nil {
		kusciaAPICluster, err = LoadClusterConfig(masterConfig.KusciaAPI.TLSConfig, masterConfig.KusciaAPI.Endpoint)
		if err != nil {
			return nil, err
		}
	}

	return &MasterConfig{
		Master: true,
		APIServer: &ClusterConfig{
			Host:     host,
			Port:     port,
			Path:     path,
			Protocol: protocol,
			TLSCert:  apiCert,
		},
		KusciaStorage: storageCluster,
		KusciaAPI:     kusciaAPICluster,
		APIWhitelist:  masterConfig.APIWhitelist,
	}, nil
}

func LoadInterConnClusterConfig(transportConfig, schedulerConfig *kusciaconfig.ServiceConfig) (*InterConnClusterConfig, error) {
	var transServiceConfig *ClusterConfig
	var schedulerServiceConfig *ClusterConfig
	var err error

	if transportConfig != nil {
		transServiceConfig, err = LoadServiceConfig(transportConfig)
		if err != nil {
			return nil, err
		}
	}

	if schedulerConfig != nil {
		schedulerServiceConfig, err = LoadServiceConfig(schedulerConfig)
		if err != nil {
			return nil, err
		}
	}

	return &InterConnClusterConfig{
		TransportConfig: transServiceConfig,
		SchedulerConfig: schedulerServiceConfig,
	}, nil
}

func LoadServiceConfig(config *kusciaconfig.ServiceConfig) (*ClusterConfig, error) {
	return LoadClusterConfig(config.TLSConfig, config.Endpoint)
}

func LoadClusterConfig(config *kusciaconfig.TLSConfig, endpoint string) (*ClusterConfig, error) {
	cert, err := LoadTLSCertByTLSConfig(config)
	if err != nil {
		return nil, err
	}

	protocol, host, port, path, err := utils.ParseURL(endpoint)
	if err != nil {
		return nil, err
	}

	return &ClusterConfig{
		Host:     host,
		Port:     port,
		Path:     path,
		Protocol: protocol,
		TLSCert:  cert,
	}, nil
}

func LoadTLSCertByTLSConfig(config *kusciaconfig.TLSConfig) (*xds.TLSCert, error) {
	if config == nil || !config.EnableTLS {
		return nil, nil
	}

	cert := ""
	if config.CertData != "" {
		cert = config.CertData
	} else {
		if config.CertFile != "" {
			certBytes, err := os.ReadFile(config.CertFile)
			if err != nil {
				nlog.Errorf("LoadTLSCertByTLSConfig read CertFile failed")
				return nil, err
			}
			cert = string(certBytes)
		}
	}

	key := ""
	if config.KeyData != "" {
		key = config.KeyData
	} else {
		if config.KeyFile != "" {
			keyBytes, err := os.ReadFile(config.KeyFile)
			if err != nil {
				return nil, err
			}
			key = string(keyBytes)
		}

	}

	ca := ""
	if config.CAData != "" {
		ca = config.CAData
	} else {
		if config.CAFile != "" {
			data, err := os.ReadFile(config.CAFile)
			if err != nil {
				return nil, fmt.Errorf("invalid ca file: %s, detail: %v", config.CAFile, err)
			}
			ca = string(data)
		}
	}

	result := &xds.TLSCert{
		CertData: cert,
		KeyData:  key,
		CAData:   ca,
	}

	return result, nil
}

func LoadTLSCertByKubeConfig(config *restclient.Config) (*xds.TLSCert, error) {
	if len(config.CAData) == 0 {
		return nil, nil
	}

	// ignore CertData and KeyData, as we use token auth for apiserver
	return &xds.TLSCert{
		CAData: string(config.CAData),
	}, nil
}
