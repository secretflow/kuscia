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
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type Options struct {
	HealthCheckerTimeout time.Duration

	// OnStartedLeading is called when a LeaderElector client starts leading
	OnStartedLeading func(context.Context)
	// OnStoppedLeading is called when a LeaderElector client stops leading
	OnStoppedLeading func()
	// OnNewLeader is called when the client observes a leader that is
	// not the previously observed leader. This includes the first observed
	// leader when the client starts.
	OnNewLeader func(identity string)

	Name       string
	LockType   string
	Namespace  string
	KubeClient kubernetes.Interface
}

func defaultOptions() *Options {
	return &Options{
		HealthCheckerTimeout: 0,
		Namespace:            "kube-system",
		LockType:             resourcelock.LeasesResourceLock,
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

func WithHealthChecker(healthCheckTimeout time.Duration) Option {
	return func(o *Options) {
		o.HealthCheckerTimeout = healthCheckTimeout
	}
}

// onStartedLeading MUST be non-blocking
func WithOnStartedLeading(onStartedLeading func(context.Context)) Option {
	return func(o *Options) {
		o.OnStartedLeading = onStartedLeading
	}
}

// onStoppedLeading MUST be non-blocking
func WithOnStoppedLeading(onStoppedLeading func()) Option {
	return func(o *Options) {
		o.OnStoppedLeading = onStoppedLeading
	}
}

// onNewLeader MUST be non-blocking
func WithOnNewLeader(onNewLeader func(string)) Option {
	return func(o *Options) {
		o.OnNewLeader = onNewLeader
	}
}

func WithLockType(lockType string) Option {
	return func(o *Options) {
		o.LockType = lockType
	}
}

func WithNamespace(namespace string) Option {
	return func(o *Options) {
		o.Namespace = namespace
	}
}
