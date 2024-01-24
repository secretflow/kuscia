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

package controllers

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	extensionfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/utils/election"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/signals"
)

type testcontroller struct {
	ctx       context.Context
	cancel    context.CancelFunc
	stoppedCh chan struct{}
}

func (t *testcontroller) Run(num int) error {
	wg := sync.WaitGroup{}
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func(ii int) {
			defer wg.Done()
			<-t.ctx.Done()
		}(i)
	}
	wg.Wait()
	close(t.stoppedCh)
	return nil
}

func (t *testcontroller) Stop() {
	t.cancel()
	<-t.stoppedCh
}

func (t *testcontroller) Name() string {
	return "testcontroller"
}

func testNewControllerFunc(ctx context.Context, config ControllerConfig) IController {
	t := &testcontroller{stoppedCh: make(chan struct{})}
	t.ctx, t.cancel = context.WithCancel(ctx)
	return t
}

func Test_server_run(t *testing.T) {
	opts := &Options{Workers: 3, HealthCheckPort: 28081, ControllerName: "test"}
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	extensionClient := extensionfake.NewSimpleClientset()
	stopCh := make(chan struct{})
	ctx := signals.NewKusciaContextWithStopCh(stopCh)
	s := NewServer(opts, &kubeconfig.KubeClients{KubeClient: kubeClient, KusciaClient: kusciaClient, ExtensionsClient: extensionClient}, []ControllerConstruction{{testNewControllerFunc, nil}})

	stoppedChan := make(chan struct{})

	go func() {
		assert.NoError(t, s.Run(ctx))
		close(stoppedChan)
	}()

	go func() {
		for i := 0; i < 20; i++ {
			time.Sleep(1 * time.Millisecond)
			if s.controllers != nil {
				break
			}
		}
		assert.True(t, s.controllers != nil)
		time.Sleep(1 * time.Second)
		close(stopCh)
	}()

	ticker := time.NewTicker(5 * time.Second)
	select {
	case <-stoppedChan:
		assert.True(t, s.controllers == nil)
	case <-ticker.C:
		t.Fatal("Timeout")
	}
}

func Test_server_restartLeading(t *testing.T) {
	opts := &Options{Workers: 3, ControllerName: "test"}

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "test"})

	s := &server{
		eventRecorder:           eventRecorder,
		kubeClient:              kubeClient,
		kusciaClient:            kusciaClient,
		options:                 opts,
		controllerConstructions: []ControllerConstruction{{testNewControllerFunc, nil}},
	}

	s.leaderElector = election.NewElector(
		s.kubeClient,
		s.options.ControllerName,
		election.WithHealthChecker(s.electionChecker),
		election.WithOnNewLeader(s.onNewLeader),
		election.WithOnStartedLeading(s.onStartedLeading),
		election.WithOnStoppedLeading(s.onStoppedLeading))

	goroutineNumBegin := runtime.NumGoroutine()
	go s.onStartedLeading(context.Background())
	time.Sleep(time.Second)
	goroutineNum := runtime.NumGoroutine()
	assert.True(t, goroutineNumBegin < goroutineNum)
	s.onStoppedLeading()
	time.Sleep(time.Second)
	assert.Equal(t, goroutineNumBegin, runtime.NumGoroutine())

	go s.onStartedLeading(context.Background())
	time.Sleep(time.Second)
	assert.Equal(t, goroutineNum, runtime.NumGoroutine())
	s.onStoppedLeading()
	time.Sleep(time.Second)
	assert.Equal(t, goroutineNumBegin, runtime.NumGoroutine())
}

func Test_server_NewLeading(t *testing.T) {
	opts := &Options{Workers: 3, ControllerName: "test"}

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "test"})

	s := &server{
		eventRecorder:           eventRecorder,
		kubeClient:              kubeClient,
		kusciaClient:            kusciaClient,
		options:                 opts,
		controllerConstructions: []ControllerConstruction{{testNewControllerFunc, nil}},
	}

	s.onNewLeader("test")
	leaderElector := election.NewElector(
		kubeClient,
		"test",
		election.WithHealthChecker(nil),
		election.WithOnNewLeader(s.onNewLeader),
		election.WithOnStartedLeading(s.onStartedLeading),
		election.WithOnStoppedLeading(s.onStoppedLeading))

	s.leaderElector = leaderElector
	s.onNewLeader("test")
}

func Test_server_runserver(t *testing.T) {
	opts := &Options{Workers: 3, HealthCheckPort: 38081, ControllerName: "test"}
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	extensionClient := extensionfake.NewSimpleClientset()
	stopCh := make(chan struct{})
	go func() {
		time.Sleep(1 * time.Second)
		close(stopCh)
	}()
	err := RunServer(
		signals.NewKusciaContextWithStopCh(stopCh),
		opts,
		&kubeconfig.KubeClients{
			KubeClient:       kubeClient,
			KusciaClient:     kusciaClient,
			ExtensionsClient: extensionClient,
		},
		[]ControllerConstruction{{testNewControllerFunc, nil}})
	assert.NoError(t, err)
}
