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

package interconn

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/interconn/bfia"
	iccommon "github.com/secretflow/kuscia/pkg/interconn/common"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia"
	"github.com/secretflow/kuscia/pkg/utils/election"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	serverName = common.KusciaControllerManagerLeaseName
)

// Server defines detailed info which used to run Server.
type Server struct {
	ctx                     context.Context
	mutex                   sync.Mutex
	eventRecorder           record.EventRecorder
	kubeClient              kubernetes.Interface
	kusciaClient            kusciaclientset.Interface
	extensionClient         apiextensionsclientset.Interface
	bfiaServer              *bfia.Server
	kusciaServer            *kuscia.Server
	leaderElector           election.Elector
	controllers             []iccommon.IController
	controllerConstructions []iccommon.ControllerConstruction
}

// NewServer returns a Server instance.
func NewServer(ctx context.Context, clients *kubeconfig.KubeClients) (*Server, error) {
	s := &Server{
		ctx:             ctx,
		kubeClient:      clients.KubeClient,
		kusciaClient:    clients.KusciaClient,
		extensionClient: clients.ExtensionsClient,
	}

	bfiaServer, err := bfia.NewServer(ctx, clients)
	if err != nil {
		nlog.Fatalf("new bfia Server failed, %v", err.Error())
		return nil, err
	}
	s.bfiaServer = bfiaServer
	s.kusciaServer = kuscia.NewServer(clients)
	s.controllerConstructions = append(s.controllerConstructions, iccommon.ControllerConstruction{
		NewControler: s.bfiaServer.NewController,
	})
	s.controllerConstructions = append(s.controllerConstructions, iccommon.ControllerConstruction{
		NewControler: s.kusciaServer.NewController,
		CRDNames:     s.kusciaServer.CRDNames,
	})

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(nlog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: clients.KubeClient.CoreV1().Events("")})
	s.eventRecorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: serverName})

	leaderElector := election.NewElector(
		s.kubeClient,
		serverName,
		election.WithOnNewLeader(s.onNewLeader),
		election.WithOnStartedLeading(s.onStartedLeading),
		election.WithOnStoppedLeading(s.onStoppedLeading))
	if leaderElector == nil {
		nlog.Fatal("failed to new leader elector")
		return nil, err
	}
	s.leaderElector = leaderElector
	return s, nil
}

// onNewLeader is executed when leader is changed.
func (s *Server) onNewLeader(identity string) {
	if s.leaderElector == nil {
		return
	}

	if identity == s.leaderElector.MyIdentity() {
		nlog.Infof("Current node has been elected as the leader: %s", identity)
		return
	}
	nlog.Infof("New leader has been elected: %s", identity)
}

// onStartedLeading is executed when leader started.
func (s *Server) onStartedLeading(ctx context.Context) {
	nlog.Info("Start leading")
	if !s.controllerIsEmpty() {
		nlog.Info("Interconn controllers already is running, skip initializing new controller")
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, cc := range s.controllerConstructions {
		controller := cc.NewControler(ctx, s.kubeClient, s.kusciaClient, s.eventRecorder)
		nlog.Infof("Run controller %v ", controller.Name())
		go func(controller iccommon.IController) {
			if err := controller.Run(4); err != nil {
				nlog.Errorf("Error running controller %v: %v", controller.Name(), err)
			} else {
				nlog.Warnf("Controller %v exit", controller.Name())
			}
		}(controller)
		s.controllers = append(s.controllers, controller)
	}
}

// onStoppedLeading is executed when leader stopped.
func (s *Server) onStoppedLeading() {
	nlog.Warnf("Server %v leading stopped", serverName)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, c := range s.controllers {
		c.Stop()
	}
	s.controllers = nil
}

func (s *Server) controllerIsEmpty() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.controllers == nil
}

func (s *Server) WaitReady(ctx context.Context) error {
	return ctx.Err()
}

func (s *Server) Name() string {
	return serverName
}

// Run starts the inter connection service.
func (s *Server) Run(ctx context.Context) error {
	for _, cc := range s.controllerConstructions {
		if err := iccommon.CheckCRDExists(ctx, s.extensionClient, cc.CRDNames); err != nil {
			return fmt.Errorf("check crd whether exist failed, %v", err)
		}
	}

	go func() {
		if err := s.bfiaServer.Run(ctx); err != nil {
			nlog.Fatalf("Run bfia Server failed, %v", err)
		}
	}()

	s.leaderElector.Run(ctx)
	return nil
}
