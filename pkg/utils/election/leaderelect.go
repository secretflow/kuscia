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

package election

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// Concurrency-safe
type Elector interface {
	IsLeader() bool
	GetLeader() string
	MyIdentity() string
	Run(ctx context.Context)
}

var (
	newMutex   sync.Mutex
	electorMap map[string]Elector = make(map[string]Elector)
)

// Use namespace/name as key to save elector, same namespace/name will use same elector.
// All the callbacks will be called. The smallest healthCheckTimeout will be used.
func NewElector(kubeClient kubernetes.Interface, name string, opt ...Option) Elector {
	options := defaultOptions()
	options.KubeClient = kubeClient
	options.Name = name
	for _, o := range opt {
		o(options)
	}

	if err := checkOption(options); err != nil {
		nlog.Fatal(err)
		return nil
	}

	// if elector is already exist, we only need to add onStartedLeading/onStoppedLeading/onNewLeader
	// and if leader is already running, we need to cancel, and rerun
	newMutex.Lock()
	defer newMutex.Unlock()
	var e *k8sElector
	electorKeyName := fmt.Sprintf("%s/%s", options.Namespace, options.Name)
	if electorMap[electorKeyName] != nil {
		e = electorMap[electorKeyName].(*k8sElector)
		if options.KubeClient != e.kubeClient {
			nlog.Fatalf("KubeClient should be the same.")
		}
		if options.LockType != e.lockType {
			nlog.Fatalf("LockType should be the same.")
		}
		e.updateOptions(options)
	} else {
		e = &k8sElector{
			lockType:             options.LockType,
			kubeClient:           options.KubeClient,
			namespace:            options.Namespace,
			name:                 options.Name,
			leaseDuration:        15 * time.Second,
			renewDuration:        5 * time.Second,
			retryPeriod:          3 * time.Second,
			identity:             genIdentity(),
			healthCheckerTimeout: options.HealthCheckerTimeout,
			options:              []*Options{options},
			modifyMutex:          &sync.Mutex{},
			cancelCond:           sync.NewCond(&sync.Mutex{}),
		}
		electorMap[electorKeyName] = e
	}

	return e
}

type k8sElector struct {
	options       []*Options
	leaderID      atomic.String
	isLeader      atomic.Bool
	leaderElector *leaderelection.LeaderElector
	//
	identity             string
	lockType             string
	kubeClient           kubernetes.Interface
	namespace            string
	name                 string
	leaseDuration        time.Duration
	renewDuration        time.Duration
	retryPeriod          time.Duration
	healthCheckerTimeout time.Duration
	//
	cancelCond  *sync.Cond
	modifyMutex *sync.Mutex
	cancelFunc  context.CancelFunc
	rebuildFlag atomic.Bool
	runFlag     atomic.Bool
}

func (e *k8sElector) updateOptions(options *Options) {
	e.modifyMutex.Lock()
	defer e.modifyMutex.Unlock()
	e.options = append(e.options, options)
	if options.HealthCheckerTimeout < e.healthCheckerTimeout {
		e.healthCheckerTimeout = options.HealthCheckerTimeout
	}
}

func (e *k8sElector) buildLeaderElector() (*leaderelection.LeaderElector, error) {
	e.modifyMutex.Lock()
	defer e.modifyMutex.Unlock()
	rl, err := resourcelock.New(
		e.lockType,
		e.namespace,
		e.name,
		e.kubeClient.CoreV1(),
		e.kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: e.identity,
		},
	)
	if err != nil {
		return nil, err
	}

	var healthChecker *leaderelection.HealthzAdaptor
	if e.healthCheckerTimeout != 0 {
		healthChecker = leaderelection.NewLeaderHealthzAdaptor(e.healthCheckerTimeout)
	}

	lec := leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: e.leaseDuration,
		RenewDeadline: e.renewDuration,
		RetryPeriod:   e.retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: e.onStartedLeading,
			OnStoppedLeading: e.onStoppedLeading,
			OnNewLeader:      e.onNewLeader,
		},
		WatchDog: healthChecker,
		Name:     e.name,
	}

	leaderElector, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		return nil, err
	}

	return leaderElector, nil
}

func (e *k8sElector) IsLeader() bool {
	return e.isLeader.Load()
}

func (e *k8sElector) GetLeader() string {
	return e.leaderID.Load()
}

func (e *k8sElector) MyIdentity() string {
	return e.identity
}

func (e *k8sElector) onNewLeader(identity string) {
	e.leaderID.Store(identity)
	for _, o := range e.options {
		if o.OnNewLeader != nil {
			o.OnNewLeader(identity)
		}
	}
}

func (e *k8sElector) onStartedLeading(ctx context.Context) {
	e.isLeader.Store(true)
	for _, o := range e.options {
		if o.OnStartedLeading != nil {
			o.OnStartedLeading(ctx)
		}
	}
}

func (e *k8sElector) onStoppedLeading() {
	e.isLeader.Store(false)
	for _, o := range e.options {
		if o.OnStoppedLeading != nil {
			o.OnStoppedLeading()
		}
	}
}

func (e *k8sElector) innerRun(ctx context.Context) {
	reSelectLeaderDuration := 3 * time.Second
	for {
		nlog.Infof("Start leader elector innerRun.")
		// build before
		leaderElector, err := e.buildLeaderElector()
		if err != nil {
			nlog.Errorf("Build leader elector failed: %s", err.Error())
			// wait for reSelectLeaderDuration, and rebuild
			time.Sleep(reSelectLeaderDuration)
			continue
		}
		e.leaderElector = leaderElector
		// safely create cancelFunc
		e.cancelCond.L.Lock()
		innerCtx, cancelFunc := context.WithCancel(ctx)
		e.cancelFunc = cancelFunc
		// other goroutine can call cancelFunc from now on.
		e.cancelCond.Signal()
		e.cancelCond.L.Unlock()
		e.leaderElector.Run(innerCtx)
		nlog.Infof("Old leader %v stopped. sleep %v and retry to select new leader...", e.MyIdentity(), reSelectLeaderDuration)
		select {
		case <-innerCtx.Done():
			if e.rebuildFlag.CompareAndSwap(true, false) {
				nlog.Infof("Elector rebuild is required.")
				continue
			}
			nlog.Info("Context done, so no need to wait")
			return
		case <-time.After(reSelectLeaderDuration):
			continue
		}
	}
}

func (e *k8sElector) Run(ctx context.Context) {
	if e.runFlag.CompareAndSwap(false, true) {
		// first invoke
		e.innerRun(ctx)
	} else {
		// other invoke
		e.rebuildFlag.Store(true)
		e.cancelCond.L.Lock()
		for e.cancelFunc == nil {
			e.cancelCond.Wait()
		}
		e.cancelFunc()
		e.cancelFunc = nil
		e.cancelCond.L.Unlock()
	}
	// not first time invoke need to call calcenFunc
	<-ctx.Done()
}
