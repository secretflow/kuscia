package controllers

import (
	"context"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
)

type GC interface {
	GarbageCollectDomaindata()
  GarbageCollectKusciajob()
	DeleteAllUnusedResourceDomaindata()
  DeleteAllUnusedResourceKusciajob()
}

type GCInfo struct {
	name string

	namespace string

	runTime time.Time
}

type GCController struct {
	ctx                   context.Context
	kusciaClient          kusciaclientset.Interface
	kusciaInformerFactory kusciainformers.SharedInformerFactory
	kusciaJobLister       kuscialistersv1alpha1.KusciaJobLister
	domaindataLister      kuscialistersv1alpha1.DomainDataLister
	gcInfo                GCInfo
}

func NewGCController(ctx context.Context, config ControllerConfig) (GC, error) {
	kusciaClient := config.KusciaClient
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, time.Minute)
	domaindataInformer := kusciaInformerFactory.Kuscia().V1alpha1().DomainDatas()
	kusciaJobInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()

	return &GCController{
		ctx:                   ctx,
		kusciaClient:          kusciaClient,
		kusciaInformerFactory: kusciaInformerFactory,
		kusciaJobLister:       kusciaJobInformer.Lister(),
		domaindataLister:      domaindataInformer.Lister(),
	}, nil
}

func (dgc *GCController) GarbageCollectDomaindata() {

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ddgs, _ := dgc.domaindataLister.List(labels.Everything())
			for _, ddg := range ddgs {
				dgc.gcInfo.name = ddg.Name
				dgc.gcInfo.namespace = ddg.Namespace
				break
			}
      //if dgc.gcInfo.runTime>60:
      dgc.DeleteAllUnusedResourceDomaindata()
		}
	}

}
func (dgc *GCController) GarbageCollectKusciajob() {

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ddgs, _ := dgc.kusciaJobLister.List(labels.Everything())
			for _, ddg := range ddgs {
				dgc.gcInfo.name = ddg.Name
				dgc.gcInfo.namespace = ddg.Namespace
				break
			}
      //if dgc.gcInfo.runTime>60:
      dgc.DeleteAllUnusedResourceKusciajob()
		}
	}

}
func (dgc *GCController) DeleteAllUnusedResourceKusciajob() {
	dgc.kusciaClient.KusciaV1alpha1().KusciaJobs(dgc.gcInfo.namespace).Delete(dgc.ctx, dgc.gcInfo.name, metav1.DeleteOptions{})
}
func (dgc *GCController) DeleteAllUnusedResourceDomaindata() {
	dgc.kusciaClient.KusciaV1alpha1().KusciaJobs(dgc.gcInfo.namespace).Delete(dgc.ctx, dgc.gcInfo.name, metav1.DeleteOptions{})
}