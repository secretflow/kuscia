package controllers

import (
	"context"
	"time"
    "fmt"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
    "k8s.io/client-go/tools/cache"
    "github.com/secretflow/kuscia/pkg/utils/nlog"
    kubeinformers "k8s.io/client-go/informers"
    "github.com/secretflow/kuscia/pkg/common"
)

const (
	GCcontrollerName        = "GC_controller"
)

type GCController struct {
  ctx                     context.Context
  cancel                  context.CancelFunc
  kusciaClient            kusciaclientset.Interface
  kusciaInformerFactory   kusciainformers.SharedInformerFactory
  kubeInformerFactory     kubeinformers.SharedInformerFactory
  kusciaJobLister         kuscialistersv1alpha1.KusciaJobLister
  kusciaTaskSynced cache.InformerSynced
  kusciaJobSynced  cache.InformerSynced
  namespaceSynced cache.InformerSynced
  deleteDDL               float64
}

func NewGCController(ctx context.Context, config ControllerConfig) IController {
  kubeClient := config.KubeClient
  kusciaClient := config.KusciaClient
  kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, time.Minute)
  kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute)
  
  kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
  kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
  namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
  
  gcController:= &GCController{
	kusciaClient:            kusciaClient,
    kusciaInformerFactory:   kusciaInformerFactory,
    kubeInformerFactory:     kubeInformerFactory,
	kusciaJobLister:         kusciaJobInformer.Lister(),
    kusciaTaskSynced:      kusciaTaskInformer.Informer().HasSynced,
    kusciaJobSynced:       kusciaJobInformer.Informer().HasSynced,
    namespaceSynced:       namespaceInformer.Informer().HasSynced,
	}
  gcController.ctx, gcController.cancel = context.WithCancel(ctx)
  gcController.deleteDDL = 3 //Can be set
  return gcController
}

func (kgc *GCController) Run(workers int) error {
  nlog.Info("Starting KusciaJobGC controller")
  kgc.kusciaInformerFactory.Start(kgc.ctx.Done())
  kgc.kubeInformerFactory.Start(kgc.ctx.Done())
 
  nlog.Infof("Waiting for informer cache to sync for %v", kgc.Name())
  if ok := cache.WaitForCacheSync(kgc.ctx.Done(), kgc.kusciaTaskSynced, kgc.kusciaJobSynced, kgc.namespaceSynced); !ok {
	return fmt.Errorf("failed to wait for caches to sync")
  }
 
  nlog.Info("Starting GCworkers")
  kgc.GarbageCollectKusciajob(kgc.ctx)
  return nil
}

func (kgc *GCController) Stop() {
  if kgc.cancel != nil {
		kgc.cancel()
		kgc.cancel = nil
 }
}

func (kgc *GCController) Name() string {
	return GCcontrollerName
}

func (kgc *GCController) DeleteAllUnusedResourceKusciajob(ctx context.Context, name string, namespace string) {
	kgc.kusciaClient.KusciaV1alpha1().KusciaJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}                  

func (kgc *GCController) GarbageCollectKusciajob(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
  
	for {
		select {
		case <-ticker.C:
        kusciaJobListers, _ := kgc.kusciaJobLister.KusciaJobs(common.KusciaCrossDomain).List(labels.Everything())
			for _, kusciajob := range kusciaJobListers {
				if kusciajob.Status.CompletionTime != nil {
					kusciajobTime := kusciajob.Status.CompletionTime.Time
					difference := time.Since(kusciajobTime)
					if difference.Hours()/24 >= kgc.deleteDDL {
						kgc.DeleteAllUnusedResourceKusciajob(ctx, kusciajob.Name, kusciajob.Namespace)
            nlog.Infof("Delete outdated kusciajob %v (Outdated duration %v days)",kusciajob.Name,difference.Hours()/24)
            
					}
				}
			}
		}
	}
}
