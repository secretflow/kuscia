// Copyright 2025 Ant Group Co., Ltd.
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
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	DomainDataGCControllerName  = "kuscia-domaindata-gc-controller"
	vendorSecretFlow            = "secretflow"
	defaultDomainDataGCDuration = 30 * 24 * time.Hour // 30 days
	kddBatchSize                = 100
)

type KusciaDomainDataGCController struct {
	ctx                        context.Context
	cancel                     context.CancelFunc
	kusciaClient               kusciaclientset.Interface
	kusciaInformerFactory      kusciainformers.SharedInformerFactory
	kubeInformerFactory        kubeinformers.SharedInformerFactory
	kusciaDomainDataLister     kuscialistersv1alpha1.DomainDataLister
	kusciaDomainDataSynced     cache.InformerSynced
	kusciaTaskSynced           cache.InformerSynced
	kusciaJobSynced            cache.InformerSynced
	kusciaDomainDataGCDuration time.Duration
}

func NewKusciaDomainDataGCController(ctx context.Context, config controllers.ControllerConfig) controllers.IController {
	// If kuscia domain data garbage collection is disabled, return nil controller
	if !config.KddGarbageCollectionEnabled {
		return &NilController{}
	}

	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	kusciaDomainDataInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	// Use configured duration, if 0 use default 720 hours (30 days)
	gcDuration := defaultDomainDataGCDuration
	if config.DomainDataGCDurationHours > 0 {
		gcDuration = time.Duration(config.DomainDataGCDurationHours) * time.Hour
	}

	gcController := &KusciaDomainDataGCController{
		kusciaClient:               kusciaClient,
		kusciaInformerFactory:      kusciaInformerFactory,
		kubeInformerFactory:        kubeInformerFactory,
		kusciaDomainDataLister:     kusciaDomainDataInformer.Lister(),
		kusciaTaskSynced:           kusciaTaskInformer.Informer().HasSynced,
		kusciaJobSynced:            kusciaJobInformer.Informer().HasSynced,
		kusciaDomainDataSynced:     kusciaDomainDataInformer.Informer().HasSynced,
		kusciaDomainDataGCDuration: gcDuration,
	}
	gcController.ctx, gcController.cancel = context.WithCancel(ctx)
	return gcController
}

func (kgc *KusciaDomainDataGCController) Run(flag int) error {
	nlog.Info("Starting KusciaDomainDataGC controller")
	kgc.kusciaInformerFactory.Start(kgc.ctx.Done())
	kgc.kubeInformerFactory.Start(kgc.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for `%s`", kgc.Name())
	if ok := cache.WaitForCacheSync(kgc.ctx.Done(), kgc.kusciaTaskSynced, kgc.kusciaJobSynced, kgc.kusciaDomainDataSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Info("Starting GC workers")
	if flag == 1 {
		kgc.GarbageCollectKusciaDomainData(kgc.ctx, time.NewTicker(2*time.Second))
	} else {
		kgc.GarbageCollectKusciaDomainData(kgc.ctx, nil)
	}
	return nil
}

func (kgc *KusciaDomainDataGCController) Stop() {
	if kgc.cancel != nil {
		kgc.cancel()
		kgc.cancel = nil
	}
}

func (kgc *KusciaDomainDataGCController) Name() string {
	return DomainDataGCControllerName
}

func (kgc *KusciaDomainDataGCController) GarbageCollectKusciaDomainData(ctx context.Context, ticker *time.Ticker) {
	nlog.Infof("KusciaDomainData GC Duration is %v", kgc.kusciaDomainDataGCDuration)
	if ticker == nil {
		ticker = time.NewTicker(10 * time.Minute)
	}
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// List all domain data resources in all namespaces
			kusciaDomainDatas, _ := kgc.kusciaDomainDataLister.List(labels.Everything())

			// For each namespace, we need to get the specific client
			namespaceDomainDataMap := make(map[string][]*kusciaapisv1alpha1.DomainData)

			// Group by namespace
			for _, domainData := range kusciaDomainDatas {
				namespace := domainData.Namespace
				if namespace == "" {
					namespace = common.KusciaCrossDomain
				}
				namespaceDomainDataMap[namespace] = append(namespaceDomainDataMap[namespace], domainData)
			}

			// Process each namespace's domain data
			for namespace, domainDatas := range namespaceDomainDataMap {
				kusciaDomainDataClient := kgc.kusciaClient.KusciaV1alpha1().DomainDatas(namespace)

				for i, domainData := range domainDatas {
					// Check if should be garbage collected based on label and creation time
					if kgc.shouldGarbageCollect(domainData) {
						err := kusciaDomainDataClient.Delete(ctx, domainData.Name, metav1.DeleteOptions{})
						if err != nil {
							nlog.Errorf("Delete outdated kusciaDomainData `%s` in namespace `%s` error: %v", domainData.Name, namespace, err)
							continue
						}
						nlog.Infof("Delete outdated kusciaDomainData `%s` in namespace `%s` (Age: %v)", domainData.Name, namespace, time.Since(domainData.CreationTimestamp.Time))
					}

					// Batch processing with sleep
					if (i+1)%kddBatchSize == 0 {
						nlog.Info("Kuscia DomainData GC Sleeping for 5 seconds...")
						time.Sleep(5 * time.Second)
					}
				}
			}
		}
	}
}

func (kgc *KusciaDomainDataGCController) shouldGarbageCollect(domainData *kusciaapisv1alpha1.DomainData) bool {
	// Check labels for vendor=secretflow
	labels := domainData.GetLabels()
	if labels == nil {
		return false
	}

	vendor, exists := labels[common.LabelDomainDataVendor]
	if !exists || vendor != vendorSecretFlow {
		return false
	}

	// Check creation time - older than 2 days
	creationTime := domainData.CreationTimestamp.Time
	age := time.Since(creationTime)
	if age < kgc.kusciaDomainDataGCDuration {
		return false
	}

	return true
}

// NilController is a dummy controller that does nothing when GC is disabled
type NilController struct{}

func (n *NilController) Run(flag int) error {
	return nil
}

func (n *NilController) Stop() {
	// No-op
}

func (n *NilController) Name() string {
	return DomainDataGCControllerName
}
