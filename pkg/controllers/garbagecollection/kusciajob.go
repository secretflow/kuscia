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
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"time"
)

const (
	GCcontrollerName  = "KusciajobGC_controller"
	batchSize         = 100
	defaultGCDuration = 30 * 24 * time.Hour
)

type KusciajobGCController struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	kusciaClient          kusciaclientset.Interface
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	kubeInformerFactory   kubeinformers.SharedInformerFactory
	kusciaJobLister       kuscialistersv1alpha1.KusciaJobLister
	kusciaTaskSynced      cache.InformerSynced
	kusciaJobSynced       cache.InformerSynced
	namespaceSynced       cache.InformerSynced
	kusciajobGCDuration   time.Duration
}

func NewKusciajobGCController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()

	gcController := &KusciajobGCController{
		kusciaClient:          kusciaClient,
		kusciaInformerFactory: kusciaInformerFactory,
		kubeInformerFactory:   kubeInformerFactory,
		kusciaJobLister:       kusciaJobInformer.Lister(),
		kusciaTaskSynced:      kusciaTaskInformer.Informer().HasSynced,
		kusciaJobSynced:       kusciaJobInformer.Informer().HasSynced,
		namespaceSynced:       namespaceInformer.Informer().HasSynced,
		kusciajobGCDuration:   defaultGCDuration,
	}
	gcController.ctx, gcController.cancel = context.WithCancel(ctx)
	return gcController
}

func (kgc *KusciajobGCController) Run(flag int) error {
	nlog.Info("Starting KusciaJobGC controller")
	kgc.kusciaInformerFactory.Start(kgc.ctx.Done())
	kgc.kubeInformerFactory.Start(kgc.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for %v", kgc.Name())
	if ok := cache.WaitForCacheSync(kgc.ctx.Done(), kgc.kusciaTaskSynced, kgc.kusciaJobSynced, kgc.namespaceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Info("Starting GCworkers")
	if flag == 1 {
		kgc.GarbageCollectKusciajob(kgc.ctx, time.NewTicker(2*time.Second))
	} else {
		kgc.GarbageCollectKusciajob(kgc.ctx, nil)
	}
	return nil
}

func (kgc *KusciajobGCController) Stop() {
	if kgc.cancel != nil {
		kgc.cancel()
		kgc.cancel = nil
	}
}

func (kgc *KusciajobGCController) Name() string {
	return GCcontrollerName
}

func (kgc *KusciajobGCController) GarbageCollectKusciajob(ctx context.Context, ticker *time.Ticker) {
	nlog.Infof("kusciajobGCDuration is %v", kgc.kusciajobGCDuration)
	if ticker == nil {
		ticker = time.NewTicker(10 * time.Minute)
	}
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			kusciaJobListers, _ := kgc.kusciaJobLister.KusciaJobs(common.KusciaCrossDomain).List(labels.Everything())
			kusciaJobClient := kgc.kusciaClient.KusciaV1alpha1().KusciaJobs(common.KusciaCrossDomain)
			for i, kusciajob := range kusciaJobListers {
				if kusciajob.Status.CompletionTime != nil {
					kusciajobTime := kusciajob.Status.CompletionTime.Time
					durationTime := time.Since(kusciajobTime)
					if durationTime >= kgc.kusciajobGCDuration {
						kusciaJobClient.Delete(ctx, kusciajob.Name, metav1.DeleteOptions{})
						nlog.Infof("Delete outdated kusciajob %v (Outdated duration %v)", kusciajob.Name, durationTime)

					}
				}
				if (i+1)%batchSize == 0 {
					nlog.Info("KusciajobGC Sleeping for 5 second...")
					time.Sleep(5 * time.Second)
				}
			}
		}
	}
}
