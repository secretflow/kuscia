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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/framework"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/agent/provider"
	"github.com/secretflow/kuscia/pkg/agent/resource"
	"github.com/secretflow/kuscia/pkg/agent/source"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

func RunRootCommand(ctx context.Context, agentConfig *config.AgentConfig, kubeClient kubernetes.Interface) error {
	nlog.Infof("Run root command, Namespace=%v", agentConfig.Namespace)
	if agentConfig.Namespace == "" {
		return fmt.Errorf("agent can not start with an empty domain id, you must restart agent with flag --namespace=DOMAIN_ID")
	}

	// load plugins
	pluginDependencies := &plugin.Dependencies{
		AgentConfig: agentConfig,
	}
	if err := plugin.Init(pluginDependencies); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// check namespace exist
	for {
		_, err := kubeClient.CoreV1().Pods(agentConfig.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
		if err == nil {
			break
		}

		nlog.Warnf("Failed to list resource in namespace %v from master: %v", agentConfig.Namespace, err)

		select {
		case <-ctx.Done():
			nlog.Infof("Stop watch kuscia domain, since agent is shutting down")
		case <-time.After(3 * time.Second):
			continue // continue for loop
		}

		break // context cancelled, agent is shutting down
	}

	// Create shared informer factory for Kubernetes pods, secrets and configmaps (not subject to any selectors).
	resourceInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, kubeinformers.WithNamespace(agentConfig.Namespace))

	rm := resource.NewResourceManager(
		kubeClient,
		agentConfig.Namespace,
		resourceInformerFactory.Core().V1().Pods().Lister().Pods(agentConfig.Namespace),
		resourceInformerFactory.Core().V1().Secrets().Lister().Secrets(agentConfig.Namespace),
		resourceInformerFactory.Core().V1().ConfigMaps().Lister().ConfigMaps(agentConfig.Namespace))

	// init provider factory
	providerFactory, err := provider.NewFactory(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to create provider factory, detail-> %v", err)
	}

	// init nodeProvider
	nodeProvider, err := providerFactory.BuildNodeProvider()
	if err != nil {
		return fmt.Errorf("failed to build node provider, detail-> %v", err)
	}

	// init nodeController
	nodeController, err := framework.NewNodeController(agentConfig.Namespace,
		nodeProvider,
		kubeClient.CoreV1().Nodes(),
		kubeClient.CoordinationV1().Leases(corev1.NamespaceNodeLease), &agentConfig.Node)
	if err != nil {
		return fmt.Errorf("failed to build node controller, detail-> %v", err)
	}

	go func() {
		if err := nodeController.Run(ctx); err != nil {
			nlog.Fatalf("Failed to run node controller: %v", err)
		}
	}()
	<-nodeController.Ready()

	node, err := nodeController.GetNode()
	if err != nil {
		return fmt.Errorf("failed to get node, detail-> %v", err)
	}

	// init event recorder
	eb := record.NewBroadcaster()
	eb.StartLogging(nlog.Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events(agentConfig.Namespace)})
	eventRecorder := eb.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: "Agent",
		Host:      node.Name,
	})

	// init sourceManager
	configCh := make(chan kubetypes.PodUpdate, 50)

	sourceCfg := &source.InitConfig{
		Namespace:  agentConfig.Namespace,
		NodeName:   types.NodeName(node.Name),
		SourceCfg:  &agentConfig.Source,
		KubeClient: kubeClient,
		Updates:    configCh,
		Recorder:   eventRecorder,
	}
	sourceManager := source.NewManager(sourceCfg)

	// init podsController
	podsControllerConfig := &framework.PodsControllerConfig{
		Namespace:     agentConfig.Namespace,
		NodeName:      node.Name,
		NodeIP:        agentConfig.NodeIP,
		ConfigCh:      configCh,
		FrameworkCfg:  &agentConfig.Framework,
		RegistryCfg:   &agentConfig.Registry,
		KubeClient:    kubeClient,
		NodeGetter:    nodeController,
		EventRecorder: eventRecorder,
		SourcesReady:  sourceManager,
	}

	podsController, err := framework.NewPodsController(podsControllerConfig)
	if err != nil {
		return fmt.Errorf("failed to build pods controller, detail-> %v", err)
	}

	// init pod provider
	podProvider, err := providerFactory.BuildPodProvider(node.Name, eventRecorder, rm, podsController)
	if err != nil {
		return fmt.Errorf("failed to build pod provider, detail-> %v", err)
	}

	// register provider
	podsController.RegisterProvider(podProvider)

	chStopKubeClient := make(chan struct{})
	go resourceInformerFactory.Start(chStopKubeClient)

	chSourceManager := make(chan struct{})
	if err := sourceManager.Run(chSourceManager); err != nil {
		return fmt.Errorf("failed to run source manager, detail-> %v", err)
	}

	go func() {
		if err := podsController.Run(ctx); err != nil {
			nlog.Fatalf("Failed to run pods controller: %v", err)
		}
	}()
	<-podsController.Ready()

	nlog.Info("Agent started")
	nodeController.NotifyAgentReady()
	close(ReadyChan)
	<-ctx.Done()
	<-podsController.Stop()
	nodeController.Stop()
	nlog.Info("Shutting down k8s-clients ...")
	close(chStopKubeClient)

	nlog.Info("Agent exited")
	return nlog.Sync()
}
