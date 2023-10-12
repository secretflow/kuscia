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

package kubeconfig

import (
	"fmt"
	"time"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
)

type KubeClients struct {
	KubeClient       kubernetes.Interface
	KusciaClient     kusciaclientset.Interface
	ExtensionsClient apiextensionsclientset.Interface
	Kubeconfig       *restclient.Config
}

func CreateClientSets(config *restclient.Config) (*KubeClients, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client set, detail-> %v", err)
	}

	kusciaClient, err := kusciaclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error building domain client set, detail-> %v", err)
	}
	extensionClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error building apiextensions kubernetes client set, detail-> %v", err)
	}
	return &KubeClients{
		KubeClient:       kubeClient,
		KusciaClient:     kusciaClient,
		ExtensionsClient: extensionClient,
		Kubeconfig:       config,
	}, nil
}

func CreateClientSetsFromToken(token, masterURL string) (*KubeClients, error) {
	kubeConfig, err := BuildClientConfigFromToken(token, masterURL)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client config from token, detail-> %v", err)
	}
	return CreateClientSets(kubeConfig)
}

func CreateClientSetsFromKubeconfig(kubeconfigPath, masterURL string) (*KubeClients, error) {
	return CreateClientSetsFromKubeconfigWithOptions(kubeconfigPath, masterURL, 500, 1000, 0)
}

func CreateClientSetsFromKubeconfigWithOptions(kubeconfigPath, masterURL string, QPS float32, Burst int, Timeout time.Duration) (*KubeClients, error) {
	kubeConfig, err := BuildClientConfigFromKubeconfig(kubeconfigPath, masterURL)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client config from token, detail-> %v", err)
	}
	if QPS != 0 {
		kubeConfig.QPS = QPS
	}
	if Burst != 0 {
		kubeConfig.Burst = Burst
	}
	if Timeout != 0 {
		kubeConfig.Timeout = Timeout
	}
	return CreateClientSets(kubeConfig)
}

func CreateKubeClientFromKubeconfigWithOptions(kubeconfigPath, masterURL string, QPS float32, Burst int, Timeout time.Duration) (kubernetes.Interface, error) {
	kubeConfig, err := BuildClientConfigFromKubeconfig(kubeconfigPath, masterURL)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client config from token, detail-> %v", err)
	}
	if QPS != 0 {
		kubeConfig.QPS = QPS
	}
	if Burst != 0 {
		kubeConfig.Burst = Burst
	}
	if Timeout != 0 {
		kubeConfig.Timeout = Timeout
	}
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client set, detail-> %v", err)
	}

	return kubeClient, nil
}

func BuildClientConfigFromKubeconfig(kubeconfigPath, masterURL string) (*restclient.Config, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build config from flags failed, detail-> %v", err)
	}

	return cfg, nil
}

func BuildClientConfigFromToken(token, masterURL string) (*restclient.Config, error) {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{
			AuthInfo:    clientcmdapi.AuthInfo{Token: token},
			ClusterInfo: clientcmdapi.Cluster{Server: masterURL, InsecureSkipTLSVerify: true},
		}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build config from token failed, detail-> %v", err)
	}
	cfg.BearerToken = token

	return cfg, nil
}
