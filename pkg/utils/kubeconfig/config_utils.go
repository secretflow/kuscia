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

// TODO delete the file

package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	serverSuffix = "kubernetes"
)

func BuildKusciaClientConfig() (*restclient.Config, error) {
	home, _ := homedir.Dir()
	kubeconfigPath := filepath.Join(home, ".kuscia", "template")
	kusciaConfig, err := getConfigFromEnv()
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			AuthInfo:    clientcmdapi.AuthInfo{Token: kusciaConfig.token},
			ClusterInfo: clientcmdapi.Cluster{Server: kusciaConfig.centralGateway + "/" + serverSuffix},
		}).ClientConfig()
	if err != nil {
		return config, err
	}
	config.BearerToken = kusciaConfig.token
	return config, err
}

type KusciaConfig struct {
	centralGateway string
	token          string
}

func getConfigFromEnv() (*KusciaConfig, error) {
	centralGateway := os.Getenv("CENTRAL_GATEWAY")
	token := os.Getenv("TOKEN")
	if centralGateway == "" || token == "" {
		return nil, fmt.Errorf("can't get centralGateway or token from env")
	}

	kusciaConfig := KusciaConfig{
		centralGateway: centralGateway,
		token:          token,
	}
	return &kusciaConfig, nil
}
