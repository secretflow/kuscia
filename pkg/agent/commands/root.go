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

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/secretflow/kuscia/pkg/agent/provider/node"
	"github.com/secretflow/kuscia/pkg/agent/provider/pod"
	"github.com/secretflow/kuscia/pkg/agent/resource"
	"github.com/secretflow/kuscia/pkg/agent/source"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

// NewCommand creates a new top-level command.
// This command is used to start the agent daemon
func NewCommand(ctx context.Context, opts *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent is a node instance in the kubernetes cluster.",
		Long: `Agent reuses some core capabilities of kubelet, such as node registration, pod management, 
CRI support, etc. In addition, agent has strengthened the security of pod and implemented various extension
functions through plug-ins. Supporting multiple runtimes is also a goal of agent.`,
		Version:      opts.AgentVersion,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			nlog.Infof("Agent is starting...")
			agentConfig, err := config.LoadAgentConfig(opts.AgentConfigFile)
			if err != nil {
				return err
			}
			nlog.Infof("Agent config=%+v", agentConfig)

			kubeClient, err := newKubeClient(&agentConfig.Source.Apiserver)
			if err != nil {
				nlog.Fatalf("Error loading kube config, detail-> %v", err)
			}
			agentConfig.Namespace = opts.Namespace
			agentConfig.NodeName = opts.NodeName
			agentConfig.NodeIP, err = network.GetHostIP()
			if err != nil {
				nlog.Fatalf("Get host IP fail: %v", err)
			}
			agentConfig.APIVersion = opts.APIVersion
			agentConfig.AgentVersion = opts.AgentVersion
			agentConfig.KeepNodeOnExit = opts.KeepNodeOnExit
			if err = RunRootCommand(ctx, agentConfig, kubeClient); err != nil {
				nlog.Fatal(err.Error())
			}
			return err
		},
	}

	installFlags(cmd.Flags(), opts)
	return cmd
}

func RunRootCommand(ctx context.Context, agentConfig *config.AgentConfig, kubeClient kubernetes.Interface) error {
	nlog.Infof("Run root command, Namespace=%v, node_name=%v, ip=%v", agentConfig.Namespace, agentConfig.NodeName, agentConfig.NodeIP)
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

	// Create another shared informer factory for Kubernetes secrets and configmaps (not subject to any selectors).
	scmInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, kubeinformers.WithNamespace(agentConfig.Namespace))
	eb := record.NewBroadcaster()
	eb.StartLogging(nlog.Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events(agentConfig.Namespace)})
	eventRecorder := eb.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: "Agent",
		Host:      agentConfig.NodeName,
	})

	rm := resource.NewResourceManager(
		scmInformerFactory.Core().V1().Secrets().Lister().Secrets(agentConfig.Namespace),
		scmInformerFactory.Core().V1().ConfigMaps().Lister().ConfigMaps(agentConfig.Namespace))
	podAuditor := node.NewPodsAuditor(agentConfig)

	// init nodeProvider
	nodeCfg := &node.GenericNodeConfig{
		Namespace:    agentConfig.Namespace,
		NodeName:     agentConfig.NodeName,
		NodeIP:       agentConfig.NodeIP,
		APIVersion:   agentConfig.APIVersion,
		AgentVersion: agentConfig.AgentVersion,
		AgentConfig:  agentConfig,
		PodsAuditor:  podAuditor,
	}
	nodeProvider, err := node.NewNodeProvider(nodeCfg)
	if err != nil {
		return fmt.Errorf("error setting up node provider, detail-> %v", err)
	}

	// init nodeController
	nodeController, err := framework.NewNodeController(nodeProvider,
		kubeClient.CoreV1().Nodes(),
		kubeClient.CoordinationV1().Leases(corev1.NamespaceNodeLease), agentConfig.KeepNodeOnExit)
	if err != nil {
		return fmt.Errorf("error setting up node controller, detail-> %v", err)
	}

	configCh := make(chan kubetypes.PodUpdate, 50)

	// init sourceManager
	sourceCfg := &source.InitConfig{
		Namespace:  agentConfig.Namespace,
		NodeName:   types.NodeName(agentConfig.NodeName),
		SourceCfg:  &agentConfig.Source,
		KubeClient: kubeClient,
		Updates:    configCh,
		Recorder:   eventRecorder,
	}
	sourceManager := source.NewManager(sourceCfg)

	// init podsController
	podsControllerConfig := &framework.PodsControllerConfig{
		Namespace:     agentConfig.Namespace,
		NodeName:      agentConfig.NodeName,
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
		return fmt.Errorf("error setting up pods controller, detail-> %v", err)
	}

	// init criProvider
	providerConfig := &framework.ProviderConfig{
		Namespace:        agentConfig.Namespace,
		NodeName:         agentConfig.NodeName,
		NodeIP:           agentConfig.NodeIP,
		RootDirectory:    agentConfig.RootDir,
		StdoutDirectory:  agentConfig.StdoutPath,
		AllowPrivileged:  agentConfig.AllowPrivileged,
		EventRecorder:    eventRecorder,
		ResourceManager:  rm,
		PodStateProvider: podsController.GetPodStateProvider(),
		PodSyncHandler:   podsController,
		StatusManager:    podsController.GetStatusManager(),

		CRIProviderCfg: &agentConfig.Provider.CRI,
		RegistryCfg:    &agentConfig.Registry,
	}

	criProvider, err := pod.NewCRIProvider(providerConfig)
	if err != nil {
		return fmt.Errorf("error setting up pods provider, detail-> %v", err)
	}

	// register provider
	podsController.RegisterProvider(criProvider)

	chStopKubeClient := make(chan struct{})
	go scmInformerFactory.Start(chStopKubeClient)

	var agentReadyMessage string

	chSourceManager := make(chan struct{})
	if err := sourceManager.Run(chSourceManager); err != nil {
		return fmt.Errorf("failed to run source manager, detail-> %v", err)
	}

	go func() {
		if err := nodeController.Run(ctx); err != nil {
			nlog.Fatalf("Failed to run node controller: %v", err)
		}
	}()
	<-nodeController.Ready()

	go func() {
		if err := podsController.Run(ctx); err != nil {
			nlog.Fatalf("Failed to run pods controller: %v", err)
		}
	}()
	<-podsController.Ready()
	nlog.Debugf("Agent core service started success")

	// check namespace exist and wait namespace ready
	for {
		_, err := kubeClient.CoreV1().Namespaces().Get(ctx, agentConfig.Namespace, metav1.GetOptions{})
		if err == nil {
			nlog.Info("Agent started")
			nodeController.NotifyAgentReady(ctx, true, "Agent is ready")
			break
		}

		if k8serrors.IsNotFound(err) {
			agentReadyMessage = fmt.Sprintf("Domain %q does not exist", agentConfig.Namespace)
		} else {
			agentReadyMessage = fmt.Sprintf("Get domain info fail, domain=%v, err=%v", agentConfig.Namespace, err)
		}
		nodeController.NotifyAgentReady(ctx, false, agentReadyMessage)
		nlog.Warn(agentReadyMessage)

		select {
		case <-ctx.Done():
			nlog.Infof("Stop watch kuscia domain, since agent is shutting down")
		case <-time.After(30 * time.Second):
			continue // continue for loop
		}

		break // context cancelled, agent is shutting down
	}
	close(ReadyChan)
	<-ctx.Done()
	<-podsController.Stop()
	<-nodeController.Stop()
	nlog.Info("Shutting down k8s-clients ...")
	close(chStopKubeClient)

	nlog.Info("Agent exited")
	return nlog.Sync()
}

func newKubeClient(cfg *config.ApiserverSourceCfg) (*kubernetes.Clientset, error) {
	clientConfig, err := kubeconfig.BuildClientConfigFromKubeconfig(cfg.KubeconfigFile, cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("error building client config, detail-> %v", err)
	}

	clientConfig.QPS = cfg.QPS
	clientConfig.Burst = cfg.Burst
	clientConfig.Timeout = cfg.Timeout
	return kubernetes.NewForConfig(clientConfig)
}
