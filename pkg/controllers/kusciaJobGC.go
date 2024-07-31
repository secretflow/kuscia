package controllers

import (
	"context"
	"fmt"
	"github.com/secretflow/kuscia/pkg/common"
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
	GCcontrollerName = "KusciajobGC_controller"
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
}

func NewKusciajobGCController(ctx context.Context, config ControllerConfig) IController {
	kubeClient := config.KubeClient
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, time.Minute)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute)

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
	}
	gcController.ctx, gcController.cancel = context.WithCancel(ctx)
	return gcController
}

func (kgc *KusciajobGCController) Run(workers int) error {
	nlog.Info("Starting KusciaJobGC controller")
	kgc.kusciaInformerFactory.Start(kgc.ctx.Done())
	kgc.kubeInformerFactory.Start(kgc.ctx.Done())

	nlog.Infof("Waiting for informer cache to sync for %v", kgc.Name())
	if ok := cache.WaitForCacheSync(kgc.ctx.Done(), kgc.kusciaTaskSynced, kgc.kusciaJobSynced, kgc.namespaceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	nlog.Info("Starting GCworkers")
	kgc.GarbageCollectKusciajob(kgc.ctx, 3*24*time.Hour)
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

func (kgc *KusciajobGCController) GarbageCollectKusciajob(ctx context.Context, deleteDDL time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			kusciaJobListers, _ := kgc.kusciaJobLister.KusciaJobs(common.KusciaCrossDomain).List(labels.Everything())
			batchSize := 1000
			for i, kusciajob := range kusciaJobListers {
				if kusciajob.Status.CompletionTime != nil {
					kusciajobTime := kusciajob.Status.CompletionTime.Time
					difference := time.Since(kusciajobTime)
					if difference >= deleteDDL {
						kgc.kusciaClient.KusciaV1alpha1().KusciaJobs(kusciajob.Namespace).Delete(ctx, kusciajob.Name, metav1.DeleteOptions{})
						nlog.Infof("Delete outdated kusciajob %v (Outdated duration %v days)", kusciajob.Name, difference.Hours()/24)

					}
				}
				if (i+1)%batchSize == 0 {
					nlog.Info("KusciajobGC Sleeping for 1 second...")
					time.Sleep(1 * time.Second)
				}
			}
		}
	}
}
