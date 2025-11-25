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
	"fmt"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/compatibility"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/leaderelection"
	basecompatibility "k8s.io/component-base/compatibility"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/configz"
	"k8s.io/component-base/logs"
	"k8s.io/component-base/metrics"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	schedulerserverconfig "k8s.io/kubernetes/cmd/kube-scheduler/app/config"
	"k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/pkg/scheduler"
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

func NewScheduler(i *ModuleRuntimeConfigs) (Module, error) {
	componentGlobalsRegistry := compatibility.DefaultComponentGlobalsRegistry
	// make sure DefaultKubeComponent is registered in the DefaultComponentGlobalsRegistry.
	if componentGlobalsRegistry.EffectiveVersionFor(basecompatibility.DefaultKubeComponent) == nil {
		featureGate := utilfeature.DefaultMutableFeatureGate
		effectiveVersion := compatibility.DefaultBuildEffectiveVersion()
		utilruntime.Must(componentGlobalsRegistry.Register(basecompatibility.DefaultKubeComponent, effectiveVersion, featureGate))
	}
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
		Metrics:                  metrics.NewOptions(),
		Logs:                     logs.NewOptions(),
		ComponentGlobalsRegistry: componentGlobalsRegistry,
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
	}, nil
}

func (s *schedulerModule) Run(ctx context.Context) error {
	configPathTmpl := filepath.Join(s.rootDir, pkgcom.ConfPrefix, "scheduler-config.yaml.tmpl")
	configPath := filepath.Join(s.rootDir, pkgcom.ConfPrefix, "scheduler-config.yaml")
	if err := common.RenderConfig(configPathTmpl, configPath, s); err != nil {
		return err
	}
	// Configz registration.
	cz, err := configz.New("componentconfig")
	if err != nil {
		return fmt.Errorf("unable to register configz: %s", err)
	}

	s.opts.ConfigFile = configPath

	for {
		cc, sched, err := app.Setup(ctx, s.opts,
			app.WithPlugin(kusciascheduling.Name, kusciascheduling.New),
			app.WithPlugin(queuesort.Name, queuesort.New))
		if err != nil {
			nlog.Error(err)
			return err
		}

		cz.Set(cc)
		nlog.Infof("Start to run scheduler...")
		err = s.startScheduler(ctx, cc, sched)

		if err != nil {
			nlog.Warnf("Schedule run failed with: %s", err.Error())

			// fix me: "finished without leader elect"/"lost lease" is copied from app.Run function, may changed
			if err.Error() != "finished without leader elect" && err.Error() != "lost lease" {
				return err
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(3 * time.Second):
			}
		} else {
			// can't reach here(because s.startScheduler will never return nil)
			nlog.Infof("schedule run success")
			return nil
		}
	}
}

func (s *schedulerModule) WaitReady(ctx context.Context) error {
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
	}
	return ctx.Err()
}

func (s *schedulerModule) Name() string {
	return "kusciascheduler"
}

func (s *schedulerModule) startScheduler(ctx context.Context, cc *schedulerserverconfig.CompletedConfig, sched *scheduler.Scheduler) error {
	// Start events processing pipeline.
	cc.EventBroadcaster.StartRecordingToSink(ctx.Done())
	defer cc.EventBroadcaster.Shutdown()

	waitingForLeader := make(chan struct{})

	// Start all informers.
	cc.InformerFactory.Start(ctx.Done())
	// DynInformerFactory can be nil in tests.
	if cc.DynInformerFactory != nil {
		cc.DynInformerFactory.Start(ctx.Done())
	}

	// Wait for all caches to sync before scheduling.
	cc.InformerFactory.WaitForCacheSync(ctx.Done())
	// DynInformerFactory can be nil in tests.
	if cc.DynInformerFactory != nil {
		cc.DynInformerFactory.WaitForCacheSync(ctx.Done())
	}

	// If leader election is enabled, runCommand via LeaderElector until done and exit.
	if cc.LeaderElection != nil {
		cc.LeaderElection.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				close(waitingForLeader)
				sched.Run(ctx)
			},
			OnStoppedLeading: func() {
				select {
				case <-ctx.Done():
					// We were asked to terminate.
					nlog.Info("Requested to terminate, exiting")
				default:
					// We lost the lock.
					nlog.Warn("Leaderelection lost")
				}
			},
		}
		leaderElector, err := leaderelection.NewLeaderElector(*cc.LeaderElection)
		if err != nil {
			return fmt.Errorf("couldn't create leader elector: %v", err)
		}

		leaderElector.Run(ctx)

		return fmt.Errorf("lost lease")
	}

	// Leader election is disabled, so runCommand inline until done.
	close(waitingForLeader)
	sched.Run(ctx)
	return fmt.Errorf("finished without leader elect")
}
