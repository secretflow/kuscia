// Copyright 2024 Ant Group Co., Ltd.
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

package garbagecollection

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	GCControllerName  = "kuscia-job-gc-controller"
	batchSize         = 100
	defaultGCDuration = 30 * 24 * time.Hour
)

type KusciaJobGCController struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	kusciaClient          kusciaclientset.Interface
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaJobLister       kuscialistersv1alpha1.KusciaJobLister
	kusciaTaskSynced      cache.InformerSynced
	kusciaJobSynced       cache.InformerSynced
	namespaceSynced       cache.InformerSynced
	kusciaJobGCDuration   time.Duration
}

func NewKusciaJobGCController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()

	gcController := &KusciaJobGCController{
		kusciaClient:          kusciaClient,
		kusciaInformerFactory: kusciaInformerFactory,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaJobLister:       kusciaJobInformer.Lister(),
		kusciaTaskSynced:      kusciaTaskInformer.Informer().HasSynced,
		kusciaJobSynced:       kusciaJobInformer.Informer().HasSynced,
		namespaceSynced:       namespaceInformer.Informer().HasSynced,
		kusciaJobGCDuration:   defaultGCDuration,
	}
	gcController.ctx, gcController.cancel = context.WithCancel(ctx)
	return gcController
}

func (kgc *KusciaJobGCController) Run(flag int) error {
	nlog.Info("Starting KusciaJobGC controller")
	kgc.kusciaInformerFactory.Start(kgc.ctx.Done())
	kgc.kubeInformerFactory.Start(kgc.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for `%s`", kgc.Name())
	if ok := cache.WaitForCacheSync(kgc.ctx.Done(), kgc.kusciaTaskSynced, kgc.kusciaJobSynced, kgc.namespaceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Info("Starting GC workers")
	if flag == 1 {
		kgc.GarbageCollectKusciaJob(kgc.ctx, time.NewTicker(2*time.Second))
	} else {
		kgc.GarbageCollectKusciaJob(kgc.ctx, nil)
	}
	return nil
}

func (kgc *KusciaJobGCController) Stop() {
	if kgc.cancel != nil {
		kgc.cancel()
		kgc.cancel = nil
	}
}

func (kgc *KusciaJobGCController) Name() string {
	return GCControllerName
}

func (kgc *KusciaJobGCController) GarbageCollectKusciaJob(ctx context.Context, ticker *time.Ticker) {
	nlog.Infof("KusciaJob GC Duration is %v", kgc.kusciaJobGCDuration)
	if ticker == nil {
		ticker = time.NewTicker(10 * time.Minute)
	}
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			kusciaJobs, _ := kgc.kusciaJobLister.KusciaJobs(common.KusciaCrossDomain).List(labels.Everything())
			kusciaJobClient := kgc.kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain)
			for i, kusciaJob := range kusciaJobs {
				if kusciaJob.Status.CompletionTime != nil {
					kusciaJobTime := kusciaJob.Status.CompletionTime.Time
					durationTime := time.Since(kusciaJobTime)
					if durationTime >= kgc.kusciaJobGCDuration {
						err := kusciaJobClient.Delete(ctx, kusciaJob.Name, metav1.DeleteOptions{})
						if err != nil {
							nlog.Errorf("Delete outdated kusciaJob `%s` error: %v", kusciaJob.Name, err)
							continue
						}
						nlog.Infof("Delete outdated kusciaJob `%s` (Outdated duration %v)", kusciaJob.Name, durationTime)

					}
				}
				if (i+1)%batchSize == 0 {
					nlog.Info("Kuscia Job GC Sleeping for 5 second...")
					time.Sleep(5 * time.Second)
				}
			}
		}
	}
}
