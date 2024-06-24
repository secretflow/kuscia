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

//nolint:dupl
package controllers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciascheme "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/scheme"
	"github.com/secretflow/kuscia/pkg/utils/election"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	// This is the timeout that determines the time beyond the lease expiry to be
	// allowed for timeout. Checks within the timeout period after the lease
	// expires will still return healthy.
	leaderHealthzAdaptorTimeout = time.Second * 20
)

func RunServer(ctx context.Context, opts *Options, clients *kubeconfig.KubeClients, controllerConstructions []ControllerConstruction) error {
	s := NewServer(opts, clients, controllerConstructions)

	if err := s.Run(ctx); err != nil {
		nlog.Errorf("Failed to run server: %v", err)
		return err
	}
	return nil
}

// server defines detailed info which used to run server.
type server struct {
	ctx                     context.Context
	mutex                   sync.Mutex
	options                 *Options
	eventRecorder           record.EventRecorder
	kubeClient              kubernetes.Interface
	kusciaClient            kusciaclientset.Interface
	extensionClient         apiextensionsclientset.Interface
	leaderElector           election.Elector
	electionChecker         *leaderelection.HealthzAdaptor
	controllers             []IController
	controllerConstructions []ControllerConstruction
}

func buildEventRecorder(kubeClient kubernetes.Interface, name string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(nlog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: name})
}

// NewServer returns a server instance.
func NewServer(opts *Options, clients *kubeconfig.KubeClients, controllerConstructions []ControllerConstruction) *server {
	s := &server{
		options:                 opts,
		eventRecorder:           buildEventRecorder(clients.KubeClient, opts.ControllerName),
		kubeClient:              clients.KubeClient,
		kusciaClient:            clients.KusciaClient,
		extensionClient:         clients.ExtensionsClient,
		electionChecker:         leaderelection.NewLeaderHealthzAdaptor(leaderHealthzAdaptorTimeout),
		controllerConstructions: controllerConstructions,
	}
	leaderElector := election.NewElector(
		s.kubeClient,
		s.options.ControllerName,
		election.WithHealthChecker(s.electionChecker),
		election.WithOnNewLeader(s.onNewLeader),
		election.WithOnStartedLeading(s.onStartedLeading),
		election.WithOnStoppedLeading(s.onStoppedLeading))
	if leaderElector == nil {
		nlog.Fatal("failed to new leader elector")
	}
	s.leaderElector = leaderElector
	return s
}

// Run is used to run server.
func (s *server) Run(ctx context.Context) error {
	s.ctx = ctx
	var crdNames []string
	crdNamesMap := map[string]bool{}
	for _, cc := range s.controllerConstructions {
		for _, name := range cc.CRDNames {
			if _, ok := crdNamesMap[name]; !ok {
				crdNames = append(crdNames, name)
				crdNamesMap[name] = true
			}
		}
	}
	if err := CheckCRDExists(ctx, s.extensionClient, crdNames); err != nil {
		return fmt.Errorf("check crd whether exist failed: %v", err.Error())
	}
	if err := kusciascheme.AddToScheme(scheme.Scheme); err != nil {
		return fmt.Errorf("failed to add scheme, detail-> %v", err)
	}

	nlog.Infof("Start running with %q identity", s.leaderElector.MyIdentity())

	s.runHealthCheckServer()
	s.leaderElector.Run(ctx)
	return nil
}

// onNewLeader is executed when leader is changed.
func (s *server) onNewLeader(identity string) {
	nlog.Info("On new leader")
	if s.leaderElector == nil {
		return
	}

	if identity == s.leaderElector.MyIdentity() {
		if s.controllersIsEmpty() {
			s.onStartedLeading(s.ctx)
		}
	}
	nlog.Infof("New leader has been elected: %s", identity)
}

// onStartedLeading is executed when leader started.
func (s *server) onStartedLeading(ctx context.Context) {
	nlog.Info("Start leading")
	if !s.controllersIsEmpty() {
		nlog.Info("Controllers already is running, skip initialized new controller")
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	config := ControllerConfig{
		RunMode:               s.options.RunMode,
		Namespace:             s.options.Namespace,
		RootDir:               s.options.RootDir,
		KubeClient:            s.kubeClient,
		KusciaClient:          s.kusciaClient,
		EventRecorder:         s.eventRecorder,
		EnableWorkloadApprove: s.options.EnableWorkloadApprove,
	}
	for _, cc := range s.controllerConstructions {
		controller := cc.NewControler(ctx, config)
		nlog.Infof("Run controller %v", controller.Name())
		go func(controller IController) {
			if err := controller.Run(s.options.Workers); err != nil {
				nlog.Fatalf("Error running controller %v, %v", controller.Name(), err)
			} else {
				nlog.Infof("Controller %v exit", controller.Name())
			}
		}(controller)
		s.controllers = append(s.controllers, controller)
	}
}

// onStoppedLeading is executed when leader stopped.
func (s *server) onStoppedLeading() {
	nlog.Warnf("Server %v Leading stopped, self identity: %v, leader identity: %v", s.Name(), s.leaderElector.MyIdentity(), s.leaderElector.GetLeader())
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for i := range s.controllers {
		s.controllers[i].Stop()
		s.controllers[i] = nil
	}
	s.controllers = nil
}

func (s *server) controllersIsEmpty() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.controllers == nil {
		return true
	}
	return false
}

// runHealthCheckServer runs health check server.
func (s *server) runHealthCheckServer() {
	var checks []healthz.HealthChecker
	checks = append(checks, s.electionChecker)

	mux := http.NewServeMux()
	healthz.InstallPathHandler(mux, "/healthz", checks...)
	mux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", s.options.HealthCheckPort),
		Handler: mux,
	}

	go func() {
		nlog.Infof("Start listening to %d for health check", s.options.HealthCheckPort)
		if err := httpServer.ListenAndServe(); err != nil {
			nlog.Fatalf("Error starting server for health check: %v", err)
		}
	}()
}

func (s *server) WaitReady(ctx context.Context) error {
	return ctx.Err()
}

func (s *server) Name() string {
	return s.options.ControllerName
}
