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
	"errors"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/record"
)

type Options struct {
	Identity      string
	KubeClient    kubernetes.Interface
	EventRecorder record.EventRecorder
	Namespace     string
	Name          string
	LeaseDuration time.Duration
	RenewDuration time.Duration
	RetryPeriod   time.Duration
	HealthChecker *leaderelection.HealthzAdaptor

	// OnStartedLeading is called when a LeaderElector client starts leading
	OnStartedLeading func(context.Context)
	// OnStoppedLeading is called when a LeaderElector client stops leading
	OnStoppedLeading func()
	// OnNewLeader is called when the client observes a leader that is
	// not the previously observed leader. This includes the first observed
	// leader when the client starts.
	OnNewLeader func(identity string)
}

func defaultOptions() *Options {
	return &Options{
		Identity:      genIdentity(),
		KubeClient:    nil,
		EventRecorder: nil,
		Namespace:     "kube-system",
		Name:          "",
		LeaseDuration: 15 * time.Second,
		RenewDuration: 5 * time.Second,
		RetryPeriod:   3 * time.Second,
		HealthChecker: nil,
	}
}

func checkOption(opt *Options) error {
	if opt.KubeClient == nil {
		return errors.New("k8s client is nil")
	}

	if opt.Namespace == "" {
		return errors.New("election namespace is nil")
	}

	if opt.Name == "" {
		return errors.New("election name is nil")
	}

	return nil
}

// genIdentity generates unique id for leader election.
func genIdentity() string {
	host, _ := os.Hostname()
	return host + "_" + string(uuid.NewUUID())
}

type Option func(*Options)

func WithHealthChecker(healthChecker *leaderelection.HealthzAdaptor) Option {
	return func(o *Options) {
		o.HealthChecker = healthChecker
	}
}

func WithOnStartedLeading(onStartedLeading func(context.Context)) Option {
	return func(o *Options) {
		o.OnStartedLeading = onStartedLeading
	}
}

func WithOnStoppedLeading(onStartedLeading func()) Option {
	return func(o *Options) {
		o.OnStoppedLeading = onStartedLeading
	}
}

func WithOnNewLeader(onNewLeader func(string)) Option {
	return func(o *Options) {
		o.OnNewLeader = onNewLeader
	}
}
