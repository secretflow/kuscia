package controllers

import (
	"context"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/labels"
)

type GC interface {
	GarbageCollectKusciajob(ctx context.Context)
	DeleteAllUnusedResourceKusciajob(ctx context.Context, kusciaClient kusciaclientset.Interface, name string, namespace string)
}

type GCController struct {
	ctx             context.Context
	kusciaClient    kusciaclientset.Interface
	kusciaJobLister kuscialistersv1alpha1.KusciaJobLister
}

func NewGCController(ctx context.Context, config ControllerConfig) (GC, error) {
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, time.Minute)
	kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	return &GCController{
		ctx:             ctx,
		kusciaClient:    kusciaClient,
		kusciaJobLister: kusciaJobInformer.Lister(),
	}, nil
}

func (dgc *GCController) DeleteAllUnusedResourceKusciajob(ctx context.Context, kusciaClient kusciaclientset.Interface, name string, namespace string) {
	dgc.kusciaClient.KusciaV1alpha1().KusciaJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (dgc *GCController) GarbageCollectKusciajob(ctx context.Context) {
	kusciaJobListers, _ := dgc.kusciaJobLister.List(labels.Everything())

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, kusciajob := range kusciaJobListers {
				if kusciajob.Status.CompletionTime != nil {
					kusciajobTime := kusciajob.Status.CompletionTime.Time
					nowTimestamp := time.Now()
					difference := nowTimestamp.Sub(kusciajobTime)
					if math.Abs(difference.Seconds()) > 59 {
						dgc.DeleteAllUnusedResourceKusciajob(ctx, dgc.kusciaClient, kusciajob.Name, kusciajob.Namespace)
					}
				}
			}
		}
	}
}
