package controllers

import (
	"context"
	"time"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GC interface {
	GarbageCollectDomainDatas() error
	GarbageCollectKusciaJobs() error
	DeleteAllUnusedDomainDatas() error
	DeleteAllUnusedKusciaJobs() error
	AddKusciaJobs(name string, namespace string, runDieState bool) error
	AddDomainDatas(name string, namespace string, runDieState bool) error
	UpdateKusciaJobState(name string, namespace string, runDieState bool) error
	UpdateDomainDataState(name string, namespace string, runDieState bool) error
}

type realKusciaJobDomainDataGC struct {
	ctx                   context.Context
	KusciaJobsname        []string
	KusciaJobsnamespace   []string
	DomainDatasname       []string
	DomainDatasnamespace  []string
	runDieKusciaJobState  []bool
	runDieDomainDataState []bool
	kusciaClient          kusciaclientset.Interface
}
func NewKusciaJobDomaindataGC(ctx context.Context, kusciaClient kusciaclientset.Interface) (GC, error) {
	return &realKusciaJobDomainDataGC{
		ctx:          ctx,
		kusciaClient: kusciaClient,
	}, nil
}
func (dgc *realKusciaJobDomainDataGC) GarbageCollectDomainDatas() error {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if dgc.runDieDomainDataState[0] {
				dgc.DeleteAllUnusedDomainDatas()
			}
			//dgc.runDieDomainDataState[0] = true  // for test
		case <-time.After(1 * time.Minute):
			nlog.Infof("Finished checking after 1 minutes")
		}
	}
}
func (dgc *realKusciaJobDomainDataGC) GarbageCollectKusciaJobs() error {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if dgc.runDieKusciaJobState[0] {
				dgc.DeleteAllUnusedKusciaJobs()
			}
			//dgc.runDieKusciaJobState[0] = true  // for test
		case <-time.After(1 * time.Minute):
			nlog.Infof("Finished checking after 1 minutes")
		}
	}

	//return nil
}
func indicesOf(slice []string, item string) []int {
	var indices []int
	for i, v := range slice {
		if v == item {
			indices = append(indices, i)
		}
	}
	return indices
}
func intersectionInt(slice1, slice2 []int) []int {
	elementMap := make(map[int]bool)
	for _, v := range slice1 {
		elementMap[v] = true
	}
	var intersection []int
	for _, v := range slice2 {
		if elementMap[v] {
			intersection = append(intersection, v)
			delete(elementMap, v)
		}
	}

	return intersection
}
func (dgc *realKusciaJobDomainDataGC) UpdateKusciaJobState(name string, namespace string, runDieState bool) error {
	indexNamespace := indicesOf(dgc.KusciaJobsnamespace, namespace)
	indexName := indicesOf(dgc.KusciaJobsname, name)
	intersection := intersectionInt(indexNamespace, indexName)
	dgc.runDieKusciaJobState[intersection[0]] = runDieState
	return nil
}
func (dgc *realKusciaJobDomainDataGC) UpdateDomainDataState(name string, namespace string, runDieState bool) error {
	indexNamespace := indicesOf(dgc.DomainDatasnamespace, namespace)
	indexName := indicesOf(dgc.DomainDatasname, name)
	intersection := intersectionInt(indexNamespace, indexName)
	dgc.runDieDomainDataState[intersection[0]] = runDieState
	return nil
}
func (dgc *realKusciaJobDomainDataGC) DeleteAllUnusedKusciaJobs() error {
	nlog.Info("Attempting to delete unused KusciaJobs")

	nlog.Infof("KusciaJob(%s/%s), will deleted  dkc dkc", dgc.KusciaJobsnamespace[0], dgc.KusciaJobsname[0])
	dgc.kusciaClient.KusciaV1alpha1().KusciaJobs(dgc.KusciaJobsnamespace[0]).Delete(dgc.ctx, dgc.KusciaJobsname[0], metav1.DeleteOptions{})

	dgc.KusciaJobsnamespace = dgc.KusciaJobsnamespace[1:]
	dgc.KusciaJobsname = dgc.KusciaJobsname[1:]
	
	return nil
}
func (dgc *realKusciaJobDomainDataGC) DeleteAllUnusedDomainDatas() error {
	nlog.Info("Attempting to delete unused domaindatas")
	
	nlog.Infof("DomainData(%s/%s), will deleted  dgc dgc", dgc.DomainDatasnamespace[0], dgc.DomainDatasname[0])
	dgc.kusciaClient.KusciaV1alpha1().DomainDatas(dgc.DomainDatasnamespace[0]).Delete(dgc.ctx, dgc.DomainDatasname[0], metav1.DeleteOptions{})
	dgc.DomainDatasnamespace = dgc.DomainDatasnamespace[1:]
	dgc.DomainDatasname = dgc.DomainDatasname[1:]
	
	return nil
}
func (dgc *realKusciaJobDomainDataGC) AddKusciaJobs(name string, namespace string, runDieState bool) error {
	dgc.KusciaJobsname = append(dgc.KusciaJobsname, name)
	dgc.KusciaJobsnamespace = append(dgc.KusciaJobsnamespace, namespace)
	dgc.runDieKusciaJobState = append(dgc.runDieKusciaJobState, runDieState)
	return nil
}
func (dgc *realKusciaJobDomainDataGC) AddDomainDatas(name string, namespace string, runDieState bool) error {
	dgc.DomainDatasname = append(dgc.DomainDatasname, name)
	dgc.DomainDatasnamespace = append(dgc.DomainDatasnamespace, namespace)
	dgc.runDieDomainDataState = append(dgc.runDieDomainDataState, runDieState)
	return nil
}
