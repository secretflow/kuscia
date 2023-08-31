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

	"github.com/secretflow/kuscia/pkg/gateway/clusters"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
)

func LoadMasterConfig(masterConfig *kusciaconfig.MasterConfig, kubeConfig *restclient.Config) (*clusters.MasterConfig, error) {
	isMaster := false
	if masterConfig.APIServer != nil {
		isMaster = true
	}

	if isMaster {
		apiCert, err := LoadTLSCertByKubeConfig(kubeConfig)
		if err != nil {
			return nil, err
		}

		protocol, host, port, err := utils.ParseURL(kubeConfig.Host)
		if err != nil {
			return nil, err
		}

		var storageCluster *clusters.ClusterConfig
		if masterConfig.KusciaStorage != nil {
			storageCluster, err = LoadClusterConfig(masterConfig.KusciaStorage.TLSConfig,
				masterConfig.KusciaStorage.Endpoint)
			if err != nil {
				return nil, err
			}
		}

		return &clusters.MasterConfig{
			Master: true,
			APIServer: &clusters.ClusterConfig{
				Host:     host,
				Port:     port,
				Protocol: protocol,
				TLSCert:  apiCert,
			},
			KusciaStorage: storageCluster,
			ApiWhitelist:  masterConfig.ApiWhitelist,
		}, nil
	}

	proxyCluster, err := LoadClusterConfig(masterConfig.TLSConfig, masterConfig.Endpoint)
	if err != nil {
		return nil, err
	}
	return &clusters.MasterConfig{
		Master:      false,
		MasterProxy: proxyCluster,
	}, nil
}

func LoadInterConnClusterConfig(transportConfig, schedulerConfig *kusciaconfig.ServiceConfig) (*clusters.
	InterConnClusterConfig, error) {
	var transServiceConfig *clusters.ClusterConfig
	var schedulerServiceConfig *clusters.ClusterConfig
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

	return &clusters.InterConnClusterConfig{
		TransportConfig: transServiceConfig,
		SchedulerConfig: schedulerServiceConfig,
	}, nil
}

func LoadServiceConfig(config *kusciaconfig.ServiceConfig) (*clusters.ClusterConfig, error) {
	return LoadClusterConfig(config.TLSConfig, config.Endpoint)
}

func LoadClusterConfig(config *kusciaconfig.TLSConfig, endpoint string) (*clusters.ClusterConfig, error) {
	cert, err := LoadTLSCertByTLSConfig(config)
	if err != nil {
		return nil, err
	}

	protocol, host, port, err := utils.ParseURL(endpoint)
	if err != nil {
		return nil, err
	}

	return &clusters.ClusterConfig{
		Host:     host,
		Port:     port,
		Protocol: protocol,
		TLSCert:  cert,
	}, nil
}

func LoadTLSCertByTLSConfig(config *kusciaconfig.TLSConfig) (*xds.TLSCert, error) {
	if config == nil {
		return nil, nil
	}

	cert := ""
	key := ""
	if config.CertFile != "" {
		certBytes, err := os.ReadFile(config.CertFile)
		if err != nil {
			return nil, err
		}

		keyBytes, err := os.ReadFile(config.KeyFile)
		if err != nil {
			return nil, err
		}

		cert = string(certBytes)
		key = string(keyBytes)
	}

	ca := ""
	if config.CAFile != "" {
		data, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("invalid ca file: %s, detail: %v", config.CAFile, err)
		}
		ca = string(data)
	}

	return &xds.TLSCert{
		CertData: cert,
		KeyData:  key,
		CAData:   ca,
	}, nil
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
