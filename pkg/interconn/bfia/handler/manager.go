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

package handler

import (
	"context"
	"fmt"
	"sync"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

const (
	kjQueueName = "interconn-bfia-resourcemanager-kusciajob-queue"
	ktQueueName = "interconn-bfia-resourcemanager-kusciatask-queue"
	workers     = 4
	maxRetries  = 3
)

// ResourcesManager manages kuscia resources.
type ResourcesManager struct {
	ctx            context.Context
	cancel         context.CancelFunc
	KusciaClient   kusciaclientset.Interface
	KjLister       kuscialistersv1alpha1.KusciaJobLister
	KtLister       kuscialistersv1alpha1.KusciaTaskLister
	AppImageLister kuscialistersv1alpha1.AppImageLister
	kjQueue        workqueue.RateLimitingInterface
	ktQueue        workqueue.RateLimitingInterface

	jtMutex     sync.RWMutex
	jobTaskInfo map[string]map[string]struct{}
	taskJobInfo map[string]string
}

// NewResourcesManager returns a resources manager instance.
func NewResourcesManager(ctx context.Context, kusciaClient kusciaclientset.Interface) (*ResourcesManager, error) {
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)
	kjInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaJobs()
	ktInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()
	appImageInformer := kusciaInformerFactory.Kuscia().V1alpha1().AppImages()
	m := &ResourcesManager{
		KusciaClient:   kusciaClient,
		KjLister:       kjInformer.Lister(),
		KtLister:       ktInformer.Lister(),
		AppImageLister: appImageInformer.Lister(),
		kjQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), kjQueueName),
		ktQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ktQueueName),
		jobTaskInfo:    make(map[string]map[string]struct{}),
		taskJobInfo:    make(map[string]string),
	}
	m.ctx, m.cancel = context.WithCancel(ctx)

	kjSynced := kjInformer.Informer().HasSynced
	ktSynced := ktInformer.Informer().HasSynced
	appImageSynced := appImageInformer.Informer().HasSynced

	kjInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    m.handleAddedOrDeletedKusciaJob,
			DeleteFunc: m.handleAddedOrDeletedKusciaJob,
		},
	})

	ktInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: resourceFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    m.handleAddedOrDeletedKusciaTask,
			DeleteFunc: m.handleAddedOrDeletedKusciaTask,
		},
	})

	kusciaInformerFactory.Start(m.ctx.Done())
	nlog.Info("Waiting for informer cache to sync for bfia resources manager")
	if ok := cache.WaitForCacheSync(m.ctx.Done(), kjSynced, ktSynced, appImageSynced); !ok {
		return nil, fmt.Errorf("failed to wait for caches to sync for bfia resources manager")
	}
	nlog.Info("Finish Waiting for informer cache to sync for bfia resources manager")
	return m, nil
}

// Run will start related workers.
func (m *ResourcesManager) Run(ctx context.Context) {
	defer func() {
		m.kjQueue.ShutDown()
		m.ktQueue.ShutDown()
	}()

	nlog.Infof("Starting %v workers to handle object for bfia resources manager", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(m.ctx, m.runJobWorker, time.Second)
		go wait.UntilWithContext(m.ctx, m.runTaskWorker, time.Second)
	}
	<-ctx.Done()
}

// handleDeletedKusciaTask handles added or deleted kuscia job.
func (m *ResourcesManager) handleAddedOrDeletedKusciaJob(obj interface{}) {
	_, ok := obj.(*kusciaapisv1alpha1.KusciaJob)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaJob", obj)
		return
	}
	queue.EnqueueObjectWithKey(obj, m.kjQueue)
}

// handleDeletedKusciaJob handles added or deleted kuscia task.
func (m *ResourcesManager) handleAddedOrDeletedKusciaTask(obj interface{}) {
	_, ok := obj.(*kusciaapisv1alpha1.KusciaTask)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaTask", obj)
		return
	}
	queue.EnqueueObjectWithKey(obj, m.ktQueue)
}

// runJobWorker is a long-running function that will read and process a event on the work queue.
func (m *ResourcesManager) runJobWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, kjQueueName, m.kjQueue, m.syncJobHandler, maxRetries) {
	}
}

// runTaskWorker is a long-running function that will read and process a event on the work queue.
func (m *ResourcesManager) runTaskWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, kjQueueName, m.ktQueue, m.syncTaskHandler, maxRetries) {
	}
}

// syncJobHandler handles kuscia job.
func (m *ResourcesManager) syncJobHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split job key %v, %v, skip processing it", key, err)
		return nil
	}

	_, err = m.KjLister.KusciaJobs(namespace).Get(name)
	if k8serrors.IsNotFound(err) {
		m.DeleteJobTaskInfoBy(name)
		return nil
	}

	if err != nil {
		return err
	}

	m.InsertJob(name)
	return nil
}

// syncTaskHandler handles kuscia task.
func (m *ResourcesManager) syncTaskHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split task key %v, %v, skip processing it", key, err)
		return nil
	}

	kt, err := m.KtLister.KusciaTasks(namespace).Get(name)
	if k8serrors.IsNotFound(err) {
		m.DeleteTaskJobInfoBy(name)
		return nil
	}

	if err != nil {
		return err
	}

	jobID := kt.Annotations[common.JobIDAnnotationKey]
	if jobID == "" {
		return nil
	}

	if err = m.InsertTask(jobID, name); err != nil {
		nlog.Warn(err)
		return err
	}
	return nil
}

// resourceFilter filters resources.
func resourceFilter(obj interface{}) bool {
	filter := func(obj interface{}) bool {
		switch t := obj.(type) {
		case *kusciaapisv1alpha1.KusciaJob:
			return filterKusciaJob(t)
		case *kusciaapisv1alpha1.KusciaTask:
			return filterKusciaTask(t)
		default:
			return false
		}
	}

	rs, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		return filter(rs.Obj)
	}

	return filter(obj)
}

// filterKusciaJob filter kuscia job.
func filterKusciaJob(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}

	if labels[common.LabelInterConnProtocolType] != string(kusciaapisv1alpha1.InterConnBFIA) {
		return false
	}

	return true
}

// filterKusciaTask filter kuscia task.
func filterKusciaTask(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels == nil || labels[common.LabelInterConnProtocolType] != string(kusciaapisv1alpha1.InterConnBFIA) {
		return false
	}

	annotations := obj.GetAnnotations()
	if annotations == nil || annotations[common.JobIDAnnotationKey] == "" {
		return false
	}
	return true
}

// InsertJob inserts job info.
func (m *ResourcesManager) InsertJob(jobID string) {
	m.jtMutex.Lock()
	defer m.jtMutex.Unlock()
	if _, ok := m.jobTaskInfo[jobID]; ok {
		return
	}
	m.jobTaskInfo[jobID] = make(map[string]struct{})
}

// InsertTask inserts task info.
func (m *ResourcesManager) InsertTask(jobID string, taskID string) error {
	m.jtMutex.Lock()
	defer m.jtMutex.Unlock()
	if _, ok := m.taskJobInfo[taskID]; ok {
		return nil
	}

	taskInfo, ok := m.jobTaskInfo[jobID]
	if !ok {
		return fmt.Errorf("failed to find job %v in cache, please make sure job has beed started", jobID)
	}
	taskInfo[taskID] = struct{}{}
	m.taskJobInfo[taskID] = jobID
	return nil
}

// IsJobExist checks if job exist.
func (m *ResourcesManager) IsJobExist(jobID string) bool {
	m.jtMutex.RLock()
	defer m.jtMutex.RUnlock()
	_, ok := m.jobTaskInfo[jobID]
	return ok
}

// IsTaskExist checks if task exist.
func (m *ResourcesManager) IsTaskExist(taskID string) bool {
	m.jtMutex.RLock()
	defer m.jtMutex.RUnlock()
	_, ok := m.taskJobInfo[taskID]
	return ok
}

// DeleteJobTaskInfoBy deletes job info by jobID.
func (m *ResourcesManager) DeleteJobTaskInfoBy(jobID string) {
	m.jtMutex.Lock()
	defer m.jtMutex.Unlock()
	taskInfo := m.jobTaskInfo[jobID]
	for taskID := range taskInfo {
		delete(m.taskJobInfo, taskID)
	}
	delete(m.jobTaskInfo, jobID)
}

// DeleteTaskJobInfoBy deletes task info by taskID.
func (m *ResourcesManager) DeleteTaskJobInfoBy(taskID string) {
	m.jtMutex.Lock()
	defer m.jtMutex.Unlock()
	jobID := m.taskJobInfo[taskID]
	taskInfo := m.jobTaskInfo[jobID]
	delete(taskInfo, taskID)
	delete(m.taskJobInfo, taskID)
}
