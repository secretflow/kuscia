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

package modules

import (
	"context"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/metrics"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/scheduler/kusciascheduling"
	"github.com/secretflow/kuscia/pkg/scheduler/queuesort"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type schedulerModule struct {
	rootDir    string
	Kubeconfig string
	opts       *options.Options
}

func NewScheduler(i *Dependencies) Module {
	o := &options.Options{
		SecureServing:  apiserveroptions.NewSecureServingOptions().WithLoopback(),
		Authentication: apiserveroptions.NewDelegatingAuthenticationOptions(),
		Authorization:  apiserveroptions.NewDelegatingAuthorizationOptions(),
		Deprecated: &options.DeprecatedOptions{
			PodMaxInUnschedulablePodsDuration: 5 * time.Minute,
		},
		LeaderElection: &componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
			RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
			RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
			ResourceLock:      "leases",
			ResourceName:      "kube-scheduler",
			ResourceNamespace: "kube-system",
		},
		Metrics: metrics.NewOptions(),
		Logs:    logs.NewOptions(),
	}

	o.Authentication.TolerateInClusterLookupFailure = true
	o.Authentication.RemoteKubeConfigFileOptional = true
	o.Authorization.RemoteKubeConfigFileOptional = true
	o.Authorization.RemoteKubeConfigFile = i.KubeconfigFile
	o.Authentication.RemoteKubeConfigFile = i.KubeconfigFile

	// Set the PairName but leave certificate directory blank to generate in-memory by default
	o.SecureServing.ServerCert.CertDirectory = ""
	o.SecureServing.ServerCert.PairName = "kube-scheduler"
	o.SecureServing.BindPort = kubeschedulerconfig.DefaultKubeSchedulerPort

	o.Master = i.ApiserverEndpoint

	return &schedulerModule{
		rootDir:    i.RootDir,
		Kubeconfig: i.KubeconfigFile,
		opts:       o,
	}
}

func (s *schedulerModule) Run(ctx context.Context) error {
	configPathTmpl := filepath.Join(s.rootDir, pkgcom.ConfPrefix, "scheduler-config.yaml.tmpl")
	configPath := filepath.Join(s.rootDir, pkgcom.ConfPrefix, "scheduler-config.yaml")
	if err := common.RenderConfig(configPathTmpl, configPath, s); err != nil {
		return err
	}

	s.opts.ConfigFile = configPath
	cc, sched, err := app.Setup(
		ctx,
		s.opts,
		app.WithPlugin(kusciascheduling.Name, kusciascheduling.New),
		app.WithPlugin(queuesort.Name, queuesort.New))
	if err != nil {
		nlog.Error(err)
		return err
	}
	return app.Run(ctx, cc, sched)
}

func (s *schedulerModule) WaitReady(ctx context.Context) error {
	return ctx.Err()
}

func (s *schedulerModule) Name() string {
	return "kusciascheduler"
}

func RunScheduler(ctx context.Context, cancel context.CancelFunc, conf *Dependencies) Module {
	m := NewScheduler(conf)
	go func() {
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Error(err)
		cancel()
	} else {
		nlog.Info("scheduler is ready")
	}

	return m
}
