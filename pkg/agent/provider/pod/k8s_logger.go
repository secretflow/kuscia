// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pod

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/kuberuntime"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/empty"
	"github.com/secretflow/kuscia/pkg/agent/utils/logutils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/kubelet/logs"

	"k8s.io/client-go/tools/cache"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type K8sLogManager struct {
	nodeName            string
	stdoutDirectory     string
	workers             sync.Map
	bkClient            clientset.Interface
	bkNamespace         string
	namespace           string
	kubeInformerFactory kubeinformers.SharedInformerFactory
	podSynced           cache.InformerSynced

	empty.Runtime
	runkPodLogManager logs.ContainerLogManager
}

func NewK8sLogManager(
	nodeName string,
	stdoutDirectory string,
	bkClient clientset.Interface,
	bkNamespace string,
	namespace string,
	kubeClient clientset.Interface,
	logMaxSize string,
	logMaxFiles int) (*K8sLogManager, error) {

	kl := &K8sLogManager{
		nodeName:        nodeName,
		stdoutDirectory: stdoutDirectory,
		bkClient:        bkClient,
		bkNamespace:     bkNamespace,
		namespace:       namespace,
	}

	kl.kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient, 0, kubeinformers.WithNamespace(namespace),
	)
	podInformer := kl.kubeInformerFactory.Core().V1().Pods().Informer()
	kl.podSynced = podInformer.HasSynced

	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			kl.onUpdate(obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			kl.onUpdate(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			kl.onDelete(obj)
		},
	})

	if err != nil {
		nlog.Errorf("Add pod event handler failed, err: %v", err)
		return nil, err
	}

	//
	kl.runkPodLogManager, err = logs.NewContainerLogManager(
		kl,
		pkgcontainer.RealOS{},
		logMaxSize,
		logMaxFiles,
	)
	if err != nil {
		nlog.Errorf("Create runk pod log manager failed, err: %v", err)
		return nil, err
	}

	return kl, nil
}

// list all containers that need to request log
func (kl *K8sLogManager) ListContainers(ctx context.Context, filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	// get containers from workers
	ret := make([]*runtimeapi.Container, 16)
	kl.workers.Range(func(key, value any) bool {
		ret = append(ret, value.(*K8sLogWorker).ListContainers(ctx)...)
		return true
	})
	return ret, nil
}

// return the container status that need to request log
func (kl *K8sLogManager) ContainerStatus(ctx context.Context, containerID string, verbose bool) (*runtimeapi.ContainerStatusResponse, error) {
	// return status with logpath will be enough
	ret := &runtimeapi.ContainerStatusResponse{
		Status: &runtimeapi.ContainerStatus{
			LogPath: containerID,
		},
	}
	return ret, nil
}

// call worker's logger to reopen log
func (kl *K8sLogManager) ReopenContainerLog(ctx context.Context, ContainerID string) (err error) {
	err = nil
	kl.workers.Range(func(key, value any) bool {
		if worker, ok := value.(*K8sLogWorker); ok {
			if worker.LoggerExists(ContainerID) {
				err = worker.ReopenContainerLog(ContainerID)
				return false
			}
		}
		return true
	})
	return err
}

func (kl *K8sLogManager) Start(ctx context.Context) error {
	nlog.Info("Starting k8s log manager")
	go kl.kubeInformerFactory.Start(ctx.Done())
	nlog.Info("Waiting for informer cache to sync")
	if !cache.WaitForCacheSync(ctx.Done(), kl.podSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	kl.runkPodLogManager.Start()
	<-ctx.Done()
	return nil
}

func (kl *K8sLogManager) onUpdate(obj interface{}) {
	pod, ok := validatePod(obj, kl.nodeName)
	if !ok {
		return
	}
	kl.SyncPodLog(context.Background(), pod)
}

func (kl *K8sLogManager) onDelete(obj interface{}) {
	pod, ok := validatePod(obj, kl.nodeName)
	if !ok {
		return
	}
	nlog.Infof("Receive pod %s:%s deletion event, stop logging...", pod.Name, pod.UID)
	kl.StopPodLog(context.Background(), pod)
}

func (kl *K8sLogManager) SyncPodLog(ctx context.Context, pod *v1.Pod) {
	nlog.Infof("Getting pod %s:%s, PodStatus: %v", pod.Name, pod.UID, pod.Status.Phase)
	// check pod status
	switch pod.Status.Phase {
	case v1.PodPending:
		return
	case v1.PodRunning:
		kl.StartPodLog(ctx, pod)
	case v1.PodSucceeded, v1.PodFailed, v1.PodUnknown:
		kl.StopPodLog(ctx, pod)
	}
}

func (kl *K8sLogManager) StartPodLog(ctx context.Context, pod *v1.Pod) {
	k8sLogWorker, ok := kl.WorkerExists(pod, true)
	if ok {
		nlog.Infof("Pod log worker %s:%s already registered, skip duplicate running", pod.Name, pod.UID)
		return
	}
	kl.RunWorker(ctx, k8sLogWorker)
}

func (kl *K8sLogManager) StopPodLog(ctx context.Context, pod *v1.Pod) {
	k8sLogWorker, ok := kl.WorkerExists(pod, false)
	if !ok {
		nlog.Infof("Pod log worker %s:%s not existed, try backup log", pod.Name, pod.UID)
		k8sLogWorker.BackupLog(ctx)
	}
	kl.CleanWorker(k8sLogWorker)
}

func (kl *K8sLogManager) WorkerExists(pod *v1.Pod, stream bool) (*K8sLogWorker, bool) {
	key := generateKey(pod.Name, pod.UID)
	val, ok := kl.workers.LoadOrStore(key, NewK8sLogWorker(kl.stdoutDirectory, pod, kl.bkClient, kl.bkNamespace, stream))
	k8sLogWorker := val.(*K8sLogWorker)
	return k8sLogWorker, ok
}

func (kl *K8sLogManager) RunWorker(ctx context.Context, k8sLogWorker *K8sLogWorker) {
	nlog.Infof("Run pod log worker %s:%s", k8sLogWorker.podName, k8sLogWorker.podUID)
	workerCtx, cancel := context.WithCancel(ctx)
	k8sLogWorker.cancel = cancel
	k8sLogWorker.Start(workerCtx)
}

func (kl *K8sLogManager) CleanWorker(k8sLogWorker *K8sLogWorker) {
	if k8sLogWorker.stream {
		nlog.Infof("Stop stream pod log %s:%s", k8sLogWorker.podName, k8sLogWorker.podUID)
		k8sLogWorker.Stop()
	}
	nlog.Infof("Clean pod log worker %s:%s", k8sLogWorker.podName, k8sLogWorker.podUID)
	kl.workers.Delete(generateKey(k8sLogWorker.podName, k8sLogWorker.podUID))
}

type K8sLogWorker struct {
	podName       string
	podUID        types.UID
	rootDirectory string
	namespace     string
	bkClient      clientset.Interface
	bkNamespace   string
	containers    []string
	stream        bool
	cancel        context.CancelFunc

	loggerMap sync.Map
}

func NewK8sLogWorker(rootDirectory string, pod *v1.Pod, bkClient clientset.Interface, bkNamespace string, stream bool) *K8sLogWorker {
	containers := make([]string, 0)
	for _, container := range pod.Spec.Containers {
		containers = append(containers, container.Name)
	}
	return &K8sLogWorker{
		podName:       pod.Name,
		podUID:        pod.UID,
		rootDirectory: rootDirectory,
		namespace:     pod.Namespace,
		bkClient:      bkClient,
		bkNamespace:   bkNamespace,
		containers:    containers,
		stream:        stream,
		loggerMap:     sync.Map{},
	}
}

func (kw *K8sLogWorker) ListContainers(ctx context.Context) []*runtimeapi.Container {
	// get backend pod
	pod, err := kw.bkClient.CoreV1().Pods(kw.bkNamespace).Get(ctx, kw.podName, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	ret := make([]*runtimeapi.Container, len(pod.Status.ContainerStatuses))

	for _, status := range pod.Status.ContainerStatuses {
		restartCount := kw.calcRestart(pod, kw.rootDirectory, kw.namespace, status.Name)
		containerLogPath := kuberuntime.BuildContainerLogsPath(kuberuntime.BuildContainerLogsDirectory(kw.rootDirectory, kw.namespace, kw.podName, kw.podUID, status.Name), restartCount)
		// use logPath as Id
		c := &runtimeapi.Container{
			Id: containerLogPath,
		}
		if status.State.Running != nil {
			c.State = runtimeapi.ContainerState_CONTAINER_RUNNING
		}
		ret = append(ret, c)
	}
	return ret
}

func (kw *K8sLogWorker) LoggerExists(logpath string) bool {
	_, ok := kw.loggerMap.Load(logpath)
	return ok
}

func (kw *K8sLogWorker) ReopenContainerLog(logpath string) error {
	v, ok := kw.loggerMap.Load(logpath)
	if !ok {
		return nil
	}
	return v.(*logutils.ReopenableLogger).ReopenFile()
}

func (kw *K8sLogWorker) Start(ctx context.Context) {
	// create pod log directory
	if err := kw.BuildLogDirectory(); err != nil {
		return
	}

	// start logging
	for _, container := range kw.containers {
		go kw.LogContainerStreamWithRetry(ctx, container)
	}
}

func (kw *K8sLogWorker) BackupLog(ctx context.Context) {
	// create pod log directory
	if err := kw.BuildLogDirectory(); err != nil {
		return
	}

	// get backend pod
	pod, err := kw.bkClient.CoreV1().Pods(kw.bkNamespace).Get(ctx, kw.podName, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Error getting pod %s: %v", kw.podName, err)
		return
	}

	// backup logs
	for _, container := range kw.containers {
		kw.LogContainer(ctx, pod, container)
	}
}

func (kw *K8sLogWorker) BuildLogDirectory() error {
	podLogDirectory := kuberuntime.BuildPodLogsDirectory(kw.rootDirectory, kw.namespace, kw.podName, kw.podUID)
	if err := paths.EnsureDirectory(podLogDirectory, true); err != nil {
		nlog.Errorf("Error creating pod %s log directory: %v", kw.podName, err)
		return err
	}
	nlog.Infof("Create pod log directory %s success", podLogDirectory)

	for _, container := range kw.containers {
		// create container log directory
		containerLogDir := kuberuntime.BuildContainerLogsDirectory(kw.rootDirectory, kw.namespace, kw.podName, kw.podUID, container)
		if err := paths.EnsureDirectory(containerLogDir, true); err != nil {
			nlog.Errorf("Error creating pod %s container %s log directory: %v", kw.podName, container, err)
			return err
		}
		nlog.Infof("Create pod container log directory %s success", containerLogDir)
	}
	return nil
}

func (kw *K8sLogWorker) Stop() {
	if kw.cancel != nil {
		kw.cancel()
	}
}

func (kw *K8sLogWorker) RequestLog(ctx context.Context, container string, follow bool) (io.ReadCloser, error) {
	nlog.Infof("Request GetLogs for %v/%v, follow: %v", kw.podName, container, follow)
	opts := &v1.PodLogOptions{
		Container: container,
		Follow:    follow,
	}
	req := kw.bkClient.CoreV1().Pods(kw.bkNamespace).GetLogs(kw.podName, opts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	return podLogs, nil
}

func (kw *K8sLogWorker) LogContainer(ctx context.Context, pod *v1.Pod, containerName string) {
	// check log file exist
	restartCount := kw.calcRestart(pod, kw.rootDirectory, kw.namespace, containerName)
	containerLogPath := kuberuntime.BuildContainerLogsPath(kuberuntime.BuildContainerLogsDirectory(kw.rootDirectory, kw.namespace, kw.podName, kw.podUID, containerName), restartCount)
	if paths.CheckFileExist(containerLogPath) {
		nlog.Infof("Container log path for %s/%s existed, skip backup logs", kw.podName, containerName)
		return
	}

	// log request
	podLogs, err := kw.RequestLog(ctx, containerName, kw.stream)
	if err != nil {
		nlog.Warnf("Error in opening stream for %s/%s, err: %v", kw.podName, containerName, err)
		return
	}
	defer podLogs.Close()

	// use reopanable logger to write to file
	file := logutils.NewReopenableLogger(containerLogPath)
	kw.loggerMap.Store(containerLogPath, file)
	defer file.Close()
	defer kw.loggerMap.Delete(containerLogPath)

	_, err = io.Copy(file, podLogs)
	if err != nil {
		nlog.Errorf("Error writing to file for %s/%s: %v", kw.podName, containerName, err)
	}
}

func (kw *K8sLogWorker) LogContainerStreamWithRetry(ctx context.Context, containerName string) {
	maxRetry := 3
	retryTime := 0
	for {
		select {
		case <-ctx.Done():
			nlog.Info("Worker received stop signal, return")
			return
		default:
			nlog.Infof("Start logging %s/%s, retry %d times", kw.podName, containerName, retryTime)
			err := kw.LogContainerStream(ctx, containerName)
			if err == nil {
				return
			}
			retryTime++
			if retryTime == maxRetry {
				nlog.Errorf("Log %s/%s reaches max try times, stop...", kw.podName, containerName)
				return
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (kw *K8sLogWorker) LogContainerStream(ctx context.Context, containerName string) error {
	// get backend pod
	pod, err := kw.bkClient.CoreV1().Pods(kw.bkNamespace).Get(ctx, kw.podName, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Error getting pod %s: %v", kw.podName, err)
		return err
	}

	// log request
	podLogs, err := kw.RequestLog(ctx, containerName, kw.stream)
	if err != nil {
		nlog.Warnf("Error in opening stream for %s/%s, err: %v", kw.podName, containerName, err)
		return err
	}
	defer podLogs.Close()

	// open file
	restartCount := kw.calcRestart(pod, kw.rootDirectory, kw.namespace, containerName)
	containerLogPath := kuberuntime.BuildContainerLogsPath(kuberuntime.BuildContainerLogsDirectory(kw.rootDirectory, kw.namespace, kw.podName, kw.podUID, containerName), restartCount)
	nlog.Infof("Open logging file %s for %v/%v", containerLogPath, kw.podName, containerName)
	// use reopanable logger to write to file
	file := logutils.NewReopenableLogger(containerLogPath)
	kw.loggerMap.Store(containerLogPath, file)

	defer file.Close()
	defer kw.loggerMap.Delete(containerLogPath)

	// write log
	buf := make([]byte, 1<<12) // 100KB
	for {
		cnt, err := podLogs.Read(buf)
		if err != nil {
			if err == io.EOF {
				nlog.Infof("Pod %s/%s log reach end, close files", kw.podName, containerName)
				return nil
			}
			nlog.Errorf("Error in copy from podLogs to buf for %s, err: %v", kw.podName, err)
			return err
		}
		if cnt == 0 {
			continue
		}
		if _, err := file.Write(buf[:cnt]); err != nil {
			nlog.Errorf("Error writing to file, err: %v", err)
			return err
		}
	}
}

// modified from kuberuntime_container.go
func (kw *K8sLogWorker) calcRestart(pod *v1.Pod, rootDirectory string, namespace string, containerName string) int {
	restartCount := 0
	containerStatus, ok := findContainerStatusByName(pod, containerName)
	if ok {
		restartCount = int(containerStatus.RestartCount) // don't add 1
	} else {
		// The container runtime keeps state on container statuses and
		// what the container restart count is. When nodes are rebooted
		// some container runtimes clear their state which causes the
		// restartCount to be reset to 0. This causes the logfile to
		// start at 0.log, which either overwrites or appends to the
		// already existing log.
		//
		// We are checking to see if the log directory exists, and find
		// the latest restartCount by checking the log name -
		// {restartCount}.log - and adding 1 to it.

		logDir := kuberuntime.BuildContainerLogsDirectory(rootDirectory, kw.namespace, kw.podName, kw.podUID, containerName)
		var err error
		restartCount, err = kuberuntime.CalcRestartCountByLogDir(logDir)
		if err != nil {
			nlog.Warnf("Log directory %q exists but could not calculate restartCount: %v", logDir, err)
		}
	}
	return restartCount

}

func findContainerStatusByName(pod *v1.Pod, containerName string) (v1.ContainerStatus, bool) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == containerName {
			return containerStatus, true
		}
	}
	return v1.ContainerStatus{}, false
}

func generateKey(name string, id types.UID) string {
	return fmt.Sprintf("%s:%s", name, id)
}

func validatePod(obj interface{}, nodeName string) (*v1.Pod, bool) {
	var (
		pod *v1.Pod
		ok  bool
	)

	if pod, ok = obj.(*v1.Pod); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Errorf("Error decoding object, invalid type %T", obj)
			return nil, false
		}
		pod, ok = tombstone.Obj.(*v1.Pod)
		if !ok {
			nlog.Errorf("Can't convert object to pod")
			return nil, false
		}
		nlog.Debugf("Recovered deleted object %q from tombstone", pod.GetName())
	}

	if pod.Spec.NodeName != nodeName {
		nlog.Debugf("Pod %s is not running on this node (%v != %v), skipping", pod.Name, pod.Spec.NodeName, nodeName)
		return nil, false
	}
	return pod, true
}
