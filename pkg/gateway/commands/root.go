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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	kubeinformers "k8s.io/client-go/informers"

	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/gateway/clusters"
	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/gateway/controller"
	"github.com/secretflow/kuscia/pkg/gateway/metrics"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/signals"
)

const (
	gatewayName     = "gateway"
	concurrentSyncs = 8
)

var (
	ReadyChan = make(chan struct{})
)

func NewCommand(opts *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:          gatewayName,
		Long:         gatewayName + ` set route rules for ingress and egress netflow, and Responsible for authentication.`,
		Version:      meta.KusciaVersionString(),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			zLogWriter, err := zlogwriter.New(opts.logCfg)
			if err != nil {
				return err
			}
			nlog.Setup(nlog.SetWriter(zLogWriter))
			conf := opts.Config()
			err = conf.CheckConfig()
			if err != nil {
				return err
			}
			// create clientset
			clients, err := createClientSets(conf)
			if err != nil {
				return fmt.Errorf("failed to create clientSets, detail-> %v", err)
			}
			err = Run(signals.NewKusciaContextWithStopCh(signals.SetupSignalHandler()), conf, clients)
			if err != nil {
				nlog.Fatalf("failed to start gateway, %v", err)
			}
			return err
		},
	}

	return cmd
}

func Run(ctx context.Context, gwConfig *config.GatewayConfig, clients *kubeconfig.KubeClients) error {
	// parse private key
	priKeyData, err := os.ReadFile(gwConfig.DomainKeyFile)
	if err != nil {
		return err
	}
	prikey, err := utils.ParsePKCS1PrivateKeyData(priKeyData)
	if err != nil {
		return fmt.Errorf("failed to add scheme, detail-> %v", err)
	}

	// start xds server and envoy
	err = StartXds(gwConfig)
	if err != nil {
		return fmt.Errorf("start xds server fail with err: %v", err)
	}

	// add master Clusters
	masterConfig, err := config.LoadMasterConfig(gwConfig.MasterConfig, clients.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load masterConfig, detail-> %v", err)
	}

	err = clusters.AddMasterClusters(ctx, gwConfig.Namespace, masterConfig)
	if err != nil {
		return fmt.Errorf("add master clusters fail, detail-> %v", err)
	}

	// create informer factory
	defaultResync := time.Duration(gwConfig.ResyncPeriod) * time.Second
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(clients.KubeClient, defaultResync,
		kubeinformers.WithNamespace(gwConfig.Namespace))
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(clients.KusciaClient, defaultResync,
		informers.WithNamespace(gwConfig.Namespace))

	// start GatewayController
	gwc, err := controller.NewGatewayController(gwConfig.Namespace, prikey, clients.KusciaClient, kusciaInformerFactory.Kuscia().V1alpha1().Gateways())
	if err != nil {
		return fmt.Errorf("failed to new gateway controller, detail-> %v", err)
	}
	go gwc.Run(concurrentSyncs, ctx.Done())

	// start endpoints controller
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()

	clientCert, err := config.LoadTLSCertByTLSConfig(gwConfig.InnerClientTLS)
	if err != nil {
		return fmt.Errorf("load innerClientTLS fail, detail-> %v", err)
	}
	ec, err := controller.NewEndpointsController(serviceInformer, endpointsInformer, gwConfig.WhiteListFile,
		clientCert)
	if err != nil {
		return fmt.Errorf("failed to new endpoints controller, detail-> %v", err)
	}
	go ec.Run(concurrentSyncs, ctx.Done())

	// start DomainRoute controller
	drInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainRoutes()
	drConfig := &controller.DomainRouteConfig{
		Namespace:     gwConfig.Namespace,
		Prikey:        prikey,
		PrikeyData:    priKeyData,
		HandshakePort: gwConfig.HandshakePort,
	}
	drc := controller.NewDomainRouteController(drConfig, clients.KubeClient, clients.KusciaClient, drInformer)
	go drc.Run(concurrentSyncs*2, ctx.Done())

	// start runtime metrics collector
	go metrics.MonitorRuntimeMetrics(ctx.Done())

	// start cluster metrics collector
	envoyStatsEndpoint := fmt.Sprintf("http://127.0.0.1:%d", gwConfig.EnvoyAdminPort)
	mc := metrics.NewClusterMetricsCollector(serviceInformer.Lister(), endpointsInformer.Lister(),
		drInformer.Lister(), gwc, envoyStatsEndpoint)
	go mc.MonitorClusterMetrics(ctx.Done())

	// Notice that there is no need to run Start methods in a separate goroutine.
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(ctx.Done())
	kusciaInformerFactory.Start(ctx.Done())
	nlog.Info("Gateway running")
	close(ReadyChan)
	<-ctx.Done()
	nlog.Info("Gateway shutdown")
	return nil
}

func StartXds(gwConfig *config.GatewayConfig) error {
	// set route idle timeout
	xds.IdleTimeout = gwConfig.IdleTimeout

	xds.NewXdsServer(gwConfig.XDSPort, gwConfig.GetEnvoyNodeID())

	externalCert, err := config.LoadTLSCertByTLSConfig(gwConfig.ExternalTLS)
	if err != nil {
		return err
	}
	internalCert, err := config.LoadTLSCertByTLSConfig(gwConfig.InnerServerTLS)
	if err != nil {
		return err
	}

	xdsConfig := &xds.InitConfig{
		Basedir:      gwConfig.ConfBasedir,
		XDSPort:      gwConfig.XDSPort,
		ExternalPort: gwConfig.ExternalPort,
		ExternalCert: externalCert,
		InternalCert: internalCert,
		Logdir:       filepath.Join(gwConfig.RootDir, "var/logs/envoy/"),
	}

	xds.InitSnapshot(gwConfig.Namespace, utils.GetHostname(), xdsConfig)
	return nil
}

func createClientSets(config *config.GatewayConfig) (*kubeconfig.KubeClients, error) {
	masterURL := ""
	if config.MasterConfig.APIServer.KubeConfig == "" {
		masterURL = clusters.DomainAPIServer
		nlog.Infof("apiserver url is %s", masterURL)
	}

	return kubeconfig.CreateClientSetsFromKubeconfig(config.MasterConfig.APIServer.KubeConfig, masterURL)
}
