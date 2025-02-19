// Copyright 2023 Ant Group Co., Ltd.

/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Modified by Ant Group in 2023.

package framework

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/kubelet/events"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util/queue"
	"k8s.io/kubernetes/pkg/kubelet/util/sliceutils"
	"k8s.io/utils/clock"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/kri"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	pkgpod "github.com/secretflow/kuscia/pkg/agent/pod"
	"github.com/secretflow/kuscia/pkg/agent/status"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (

	// Period for performing global cleanup tasks.
	housekeepingPeriod = time.Second * 5

	// Duration at which housekeeping failed to satisfy the invariant that
	// housekeeping should be fast to avoid blocking pod config (while
	// housekeeping is running no new pods are started or deleted).
	housekeepingWarningDuration = time.Second * 15

	// backOffPeriod is the period to back off when pod syncing results in an
	// error. It is also used as the base period for the exponential backoff
	// container restarts and image pulls.
	backOffPeriod = time.Second * 10
)

// SourcesReady tracks the set of configured sources seen by the agent.
type SourcesReady interface {
	// AllReady returns true if the currently configured sources have all been seen.
	AllReady() bool
}

// SyncHandler is an interface implemented by agent
type SyncHandler interface {
	HandlePodAdditions(pods []*corev1.Pod)
	HandlePodUpdates(pods []*corev1.Pod)
	HandlePodRemoves(pods []*corev1.Pod)
	HandlePodReconcile(pods []*corev1.Pod)
	HandlePodSyncs(pods []*corev1.Pod)
	HandlePodSyncByUID(uid types.UID)
	HandlePodCleanups(ctx context.Context) error
}

// PodsController is the controller implementation for Pod resources.
type PodsController struct {
	// chReady is a channel which will be closed once the runner is fully up and running.
	// this channel will never be closed if there is an error on startup.
	chReady    chan struct{}
	chStopping chan struct{}
	chStopped  chan struct{}

	chUpdates <-chan kubetypes.PodUpdate

	namespace   string
	nodeName    string
	nodeIPs     []net.IP
	nodeGetter  NodeGetter
	registryCfg *config.RegistryCfg

	provider kri.PodProvider

	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	recorder record.EventRecorder

	// podWorkers handle syncing Pods in response to events.
	podWorkers PodWorkers

	// sourcesReady records the sources seen by the kubelet, it is thread-safe.
	sourcesReady SourcesReady

	// podManager is a facade that abstracts away the various sources of pods
	// this Kubelet services.
	podManager pkgpod.Manager

	// clock is an interface that provides time related functionality in a way that makes it
	// easy to test the code.
	clock clock.WithTicker

	// Syncs pods statuses with apiserver; also used as a cache of statuses.
	statusManager status.Manager

	// A queue used to trigger pod workers.
	workQueue queue.WorkQueue

	// reasonCache caches the failure reason of the last creation of all containers, which is
	// used for generating ContainerStatus.
	reasonCache *kri.ReasonCache
}

// PodsControllerConfig is used to configure a new PodsController.
type PodsControllerConfig struct {
	Namespace string
	NodeName  string
	NodeIP    string

	ConfigCh     <-chan kubetypes.PodUpdate
	FrameworkCfg *config.FrameworkCfg
	RegistryCfg  *config.RegistryCfg

	KubeClient clientset.Interface

	NodeGetter NodeGetter

	EventRecorder record.EventRecorder

	SourcesReady SourcesReady
}

// NewPodsController creates a new pod controller with the provided config.
func NewPodsController(cfg *PodsControllerConfig) (*PodsController, error) {
	pc := &PodsController{
		chReady:      make(chan struct{}),
		chStopping:   make(chan struct{}),
		chStopped:    make(chan struct{}),
		chUpdates:    cfg.ConfigCh,
		namespace:    cfg.Namespace,
		nodeName:     cfg.NodeName,
		nodeIPs:      []net.IP{net.ParseIP(cfg.NodeIP)},
		nodeGetter:   cfg.NodeGetter,
		registryCfg:  cfg.RegistryCfg,
		recorder:     cfg.EventRecorder,
		sourcesReady: cfg.SourcesReady,
		clock:        clock.RealClock{},
		reasonCache:  kri.NewReasonCache(),
	}

	mirrorPodClient := pkgpod.NewBasicMirrorClient(cfg.KubeClient, cfg.NodeGetter)
	podManager := pkgpod.NewBasicPodManager(mirrorPodClient)

	statusManager := status.NewManager(cfg.KubeClient, podManager, pc)

	pc.podManager = podManager
	pc.statusManager = statusManager
	pc.workQueue = queue.NewBasicWorkQueue(pc.clock)
	pc.podWorkers = newPodWorkers(
		pc.syncPod,
		pc.syncTerminatingPod,
		pc.syncTerminatedPod,
		pc.getPodStatus,
		pc.recorder,
		pc.workQueue,
		cfg.FrameworkCfg.SyncFrequency,
		backOffPeriod,
	)

	return pc, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers.  It will block until the
// context is cancelled, at which point it will shutdown the work queue and
// wait for workers to finish processing their current work items prior to
// returning.
//
// Once this returns, you should not re-use the controller.
func (pc *PodsController) Run(ctx context.Context) error {
	// Shutdowns are idempotent, so we can call it multiple times. This is in case we have to bail out early for some reason.
	defer func() {
		close(pc.chStopped)
		nlog.Info("Pods controller exited")
	}()

	nlog.Info("Starting Pods controller ...")

	go func() {
		if err := pc.provider.Start(ctx); err != nil {
			nlog.Fatalf("Failed to start pod provider: %v", err)
		}
	}()
	pc.statusManager.Start()

	close(pc.chReady)

	pc.syncLoop(ctx, pc, pc.chUpdates)

	nlog.Info("Shutting down pods provider ...")
	pc.provider.Stop()

	return nil
}

// Ready returns a channel which gets closed once the PodsController is ready to handle scheduled pods.
// This channel will never close if there is an error on startup.
// The status of this channel after shutdown is indeterminate.
func (pc *PodsController) Ready() <-chan struct{} {
	return pc.chReady
}

func (pc *PodsController) Stop() chan struct{} {
	close(pc.chStopping)
	return pc.chStopped
}

func (pc *PodsController) GetPodStateProvider() PodStateProvider {
	return pc.podWorkers
}

func (pc *PodsController) GetStatusManager() status.Manager {
	return pc.statusManager
}

func (pc *PodsController) RegisterProvider(provider kri.PodProvider) {
	pc.provider = provider
}

// syncLoopIteration reads from various channels and dispatches pods to the
// given handler.
func (pc *PodsController) syncLoopIteration(ctx context.Context, handler SyncHandler, configCh <-chan kubetypes.PodUpdate, housekeepingCh <-chan time.Time) bool {
	select {
	case <-pc.chStopping:
		return false
	case u, open := <-configCh:
		// Update from a config source; dispatch it to the right handler
		// callback.
		if !open {
			nlog.Errorf("Update channel is closed, exiting the sync loop")
			return false
		}

		switch u.Op {
		case kubetypes.ADD:
			nlog.Infof("SyncLoop ADD, source=%v, pods=%+v", u.Source, format.Pods(u.Pods))
			// After restarting, kubelet will get all existing pods through
			// ADD as if they are new pods. These pods will then go through the
			// admission process and *may* be rejected. This can be resolved
			// once we have checkpointing.
			handler.HandlePodAdditions(u.Pods)
		case kubetypes.UPDATE:
			nlog.Infof("SyncLoop UPDATE, source=%v, pods=%+v", u.Source, format.Pods(u.Pods))
			handler.HandlePodUpdates(u.Pods)
		case kubetypes.REMOVE:
			nlog.Infof("SyncLoop REMOVE, source=%v, pods=%+v", u.Source, format.Pods(u.Pods))
			handler.HandlePodRemoves(u.Pods)
		case kubetypes.RECONCILE:
			nlog.Infof("SyncLoop RECONCILE, source=%v, pods=%+v", u.Source, format.Pods(u.Pods))
			handler.HandlePodReconcile(u.Pods)
		case kubetypes.DELETE:
			nlog.Infof("SyncLoop DELETE, source=%v, pods=%+v", u.Source, format.Pods(u.Pods))
			// DELETE is treated as a UPDATE because of graceful deletion.
			handler.HandlePodUpdates(u.Pods)
		case kubetypes.SET:
			nlog.Info("Agent does not support snapshot update")
		default:
			nlog.Infof("Invalid operation type %q received", u.Op)
		}
	case <-housekeepingCh:
		if !pc.sourcesReady.AllReady() {
			// If the sources aren't ready, skip housekeeping, as we may accidentally delete pods from unready sources.
			nlog.Info("SyncLoop (housekeeping, skipped): sources aren't ready yet")
		} else {
			start := time.Now()
			if err := handler.HandlePodCleanups(ctx); err != nil {
				nlog.Errorf("Failed cleaning pods: %v", err)
			}
			duration := time.Since(start)
			if duration > housekeepingWarningDuration {
				nlog.Errorf("Housekeeping took longer than 15s, seconds=%v", duration.Seconds())
			}
		}
	}
	return true
}

// syncLoop is the main loop for processing changes. It watches for changes from
// three channels (file, apiserver, and http) and creates a union of them. For
// any new change seen, will run a sync against desired state and running state. If
// no changes are seen to the configuration, will synchronize the last known desired
// state every sync-frequency seconds. Never returns.
func (pc *PodsController) syncLoop(ctx context.Context, handler SyncHandler, updates <-chan kubetypes.PodUpdate) {
	nlog.Info("Starting agent main sync loop")

	housekeepingTicker := time.NewTicker(housekeepingPeriod)
	defer housekeepingTicker.Stop()

	for {
		if !pc.syncLoopIteration(ctx, handler, updates, housekeepingTicker.C) {
			break
		}
	}
}

func (pc *PodsController) handleMirrorPod(mirrorPod *corev1.Pod, start time.Time) {
	// Mirror pod ADD/UPDATE/DELETE operations are considered an UPDATE to the
	// corresponding static pod. Send update to the pod worker if the static
	// pod exists.
	if pod, ok := pc.podManager.GetPodByMirrorPod(mirrorPod); ok {
		pc.dispatchWork(pod, kubetypes.SyncPodUpdate, mirrorPod, start)
	}
}

// deletePod deletes the pod from the internal state of the kubelet by:
// 1.  stopping the associated pod worker asynchronously
// 2.  signaling to kill the pod by sending on the podKillingCh channel
//
// deletePod returns an error if not all sources are ready or the pod is not
// found in the runtime cache.
func (pc *PodsController) deletePod(pod *corev1.Pod) error {
	if pod == nil {
		return fmt.Errorf("deletePod does not allow nil pod")
	}
	if !pc.sourcesReady.AllReady() {
		// If the sources aren't ready, skip deletion, as we may accidentally delete pods
		// for sources that haven't reported yet.
		return fmt.Errorf("skipping delete because sources aren't ready yet")
	}
	nlog.Infof("Pod %q has been deleted and must be killed", format.Pod(pod))
	pc.podWorkers.UpdatePod(UpdatePodOptions{
		Pod:        pod,
		UpdateType: kubetypes.SyncPodKill,
	})
	// We leave the volume/directory cleanup to the periodic cleanup routine.
	return nil
}

// HandlePodAdditions is the callback in SyncHandler for pods being added from
// a config source.
func (pc *PodsController) HandlePodAdditions(pods []*corev1.Pod) {
	start := pc.clock.Now()
	sort.Sort(sliceutils.PodsByCreationTime(pods))
	for _, pod := range pods {
		// Always add the pod to the pod manager. Kubelet relies on the pod
		// manager as the source of truth for the desired state. If a pod does
		// not exist in the pod manager, it means that it has been deleted in
		// the apiserver and no action (other than cleanup) is required.
		pc.podManager.AddPod(pod)

		if kubetypes.IsMirrorPod(pod) {
			pc.handleMirrorPod(pod, start)
			continue
		}

		if !pc.podWorkers.IsPodTerminationRequested(pod.UID) {
			if err := hook.Execute(&hook.PodAdditionContext{
				Pod:         pod,
				PodProvider: pc.provider,
			}); err != nil {
				if tError, ok := err.(*hook.TerminateError); ok {
					pc.rejectPod(pod, tError.Reason, tError.Message)
				}
				nlog.Warnf("Failed to execute hook plugins for pod %q: %v", format.Pod(pod), err)
				continue
			}

		}

		mirrorPod, _ := pc.podManager.GetMirrorPodByPod(pod)
		pc.dispatchWork(pod, kubetypes.SyncPodCreate, mirrorPod, start)
	}
}

// HandlePodUpdates is the callback in the SyncHandler interface for pods
// being updated from a config source.
func (pc *PodsController) HandlePodUpdates(pods []*corev1.Pod) {
	start := pc.clock.Now()
	for _, pod := range pods {
		pc.podManager.UpdatePod(pod)
		if kubetypes.IsMirrorPod(pod) {
			pc.handleMirrorPod(pod, start)
			continue
		}
		mirrorPod, _ := pc.podManager.GetMirrorPodByPod(pod)
		pc.dispatchWork(pod, kubetypes.SyncPodUpdate, mirrorPod, start)
	}
}

// HandlePodRemoves is the callback in the SyncHandler interface for pods
// being removed from a config source.
func (pc *PodsController) HandlePodRemoves(pods []*corev1.Pod) {
	start := pc.clock.Now()
	for _, pod := range pods {
		pc.podManager.DeletePod(pod)
		if kubetypes.IsMirrorPod(pod) {
			pc.handleMirrorPod(pod, start)
			continue
		}
		// Deletion is allowed to fail because the periodic cleanup routine
		// will trigger deletion again.
		if err := pc.deletePod(pod); err != nil {
			nlog.Errorf("Failed to delete pod %q, %v", format.Pod(pod), err)
		}
	}
}

// HandlePodReconcile is the callback in the SyncHandler interface for pods
// that should be reconciled.
func (pc *PodsController) HandlePodReconcile(pods []*corev1.Pod) {
	start := pc.clock.Now()
	for _, pod := range pods {
		// Update the pod in pod manager, status manager will do periodically reconcile according
		// to the pod manager.
		pc.podManager.UpdatePod(pod)

		// Reconcile Pod "Ready" condition if necessary. Trigger sync pod for reconciliation.
		if status.NeedToReconcilePodReadiness(pod) {
			mirrorPod, _ := pc.podManager.GetMirrorPodByPod(pod)
			nlog.Infof("Dispatch work for pod %q because reconcile is needed", format.Pod(pod))
			pc.dispatchWork(pod, kubetypes.SyncPodSync, mirrorPod, start)
		}

		nlog.Debugf("Ignore pod %q reconcile", format.Pod(pod))
	}
}

// HandlePodSyncs is the callback in the syncHandler interface for pods
// that should be dispatched to pod workers for sync.
func (pc *PodsController) HandlePodSyncs(pods []*corev1.Pod) {
	start := pc.clock.Now()
	for _, pod := range pods {
		mirrorPod, _ := pc.podManager.GetMirrorPodByPod(pod)
		pc.dispatchWork(pod, kubetypes.SyncPodSync, mirrorPod, start)
	}
}

func (pc *PodsController) HandlePodSyncByUID(uid types.UID) {
	pod, ok := pc.podManager.GetPodByUID(uid)
	if !ok {
		nlog.Warnf("Not found pod by uid %q", uid)
		return
	}

	pc.HandlePodSyncs([]*corev1.Pod{pod})
}

// dispatchWork starts the asynchronous sync of the pod in a pod worker.
// If the pod has completed termination, dispatchWork will perform no action.
func (pc *PodsController) dispatchWork(pod *corev1.Pod, syncType kubetypes.SyncPodType, mirrorPod *corev1.Pod, start time.Time) {
	// Run the sync in an async worker.
	pc.podWorkers.UpdatePod(UpdatePodOptions{
		Pod:        pod.DeepCopy(),
		MirrorPod:  mirrorPod,
		UpdateType: syncType,
		StartTime:  start,
	})
}

// rejectPod records an event about the pod with the given reason and message,
// and updates the pod to the failed phase in the status manage.
func (pc *PodsController) rejectPod(pod *corev1.Pod, reason, message string) {
	pc.recorder.Eventf(pod, corev1.EventTypeWarning, reason, message)
	pc.statusManager.SetPodStatus(pod, corev1.PodStatus{
		Phase:   corev1.PodFailed,
		Reason:  reason,
		Message: message})
}

// constructPodImage construct the image with the configured repository if image does not contain repository information.
func (pc *PodsController) constructPodImage(pod *corev1.Pod) {
	if pc.registryCfg.Default.Repository == "" {
		return
	}

	trimRepo := strings.TrimRight(pc.registryCfg.Default.Repository, "/")
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		switch strings.Count(container.Image, "/") {
		case 1:
			oldImage := container.Image
			imageSlice := strings.Split(strings.Trim(container.Image, "/"), "/")
			container.Image = fmt.Sprintf("%v/%v", trimRepo, imageSlice[len(imageSlice)-1])
			nlog.Debugf("Replace image %q with %q, container=%v, pod=%v", oldImage, container.Image, container.Name, format.Pod(pod))
		case 0:
			oldImage := container.Image
			container.Image = fmt.Sprintf("%v/%v", trimRepo, container.Image)
			nlog.Debugf("Replace image %q with %q, container=%v, pod=%v", oldImage, container.Image, container.Name, format.Pod(pod))
		default:
		}
	}
}

// syncPod is the transaction script for the sync of a single pod (setting up)
// a pod. This method is reentrant and expected to converge a pod towards the
// desired state of the spec.
func (pc *PodsController) syncPod(ctx context.Context, updateType kubetypes.SyncPodType, pod, mirrorPod *corev1.Pod, podStatus *pkgcontainer.PodStatus) (isTerminal bool, err error) {
	nlog.Infof("Sync pod %q enter", format.Pod(pod))
	defer func() {
		nlog.Infof("Sync pod %q exit, isTerminal=%v", format.Pod(pod), isTerminal)
	}()

	// Generate final API pod status with pod and status manager status
	apiPodStatus := pc.generateAPIPodStatus(pod, podStatus)
	// The pod IP may be changed in generateAPIPodStatus if the pod is using host network. (See #24576)
	// TODO(random-liu): After writing pod spec into container labels, check whether pod is using host network, and
	// set pod IP to hostIP directly in runtime.GetPodStatus
	podStatus.IPs = make([]string, 0, len(apiPodStatus.PodIPs))
	for _, ipInfo := range apiPodStatus.PodIPs {
		podStatus.IPs = append(podStatus.IPs, ipInfo.IP)
	}
	if len(podStatus.IPs) == 0 && len(apiPodStatus.PodIP) > 0 {
		podStatus.IPs = []string{apiPodStatus.PodIP}
	}

	// If the pod is terminal, we don't need to continue to setup the pod
	if apiPodStatus.Phase == corev1.PodSucceeded || apiPodStatus.Phase == corev1.PodFailed {
		pc.statusManager.SetPodStatus(pod, apiPodStatus)
		isTerminal = true
		return isTerminal, nil
	}

	pc.statusManager.SetPodStatus(pod, apiPodStatus)

	// Create Mirror Pod for Static Pod if it doesn't already exist
	if kubetypes.IsStaticPod(pod) {
		deleted := false
		if mirrorPod != nil {
			if mirrorPod.DeletionTimestamp != nil || !pc.podManager.IsMirrorPodOf(mirrorPod, pod) {
				// The mirror pod is semantically different from the static pod. Remove
				// it. The mirror pod will get recreated later.
				podFullName := pkgcontainer.GetPodFullName(pod)
				var deleteErr error
				deleted, deleteErr = pc.podManager.DeleteMirrorPod(podFullName, &mirrorPod.ObjectMeta.UID)
				if deleted {
					nlog.Infof("Deleted mirror pod %q because it is outdated", format.Pod(mirrorPod))
				} else if deleteErr != nil {
					nlog.Errorf("Failed deleting mirror pod %q: %v", format.Pod(mirrorPod), deleteErr)
				}
			}
		}
		if mirrorPod == nil || deleted {
			node, getErr := pc.nodeGetter.GetNode()
			if getErr != nil || node.DeletionTimestamp != nil {
				nlog.Debugf("No need to create a mirror pod, since node has been removed from the cluster")
			} else {
				nlog.Debugf("Creating a mirror pod for static pod %q", format.Pod(pod))
				if createErr := pc.podManager.CreateMirrorPod(pod); createErr != nil {
					nlog.Errorf("Failed creating a mirror pod for %q: %v", format.Pod(pod), createErr)
				}
			}
		}
	}

	// Create a copied pod because the pod may be modified later.
	podCopy := pod.DeepCopy()
	pc.constructPodImage(podCopy)

	// Call the container runtime's SyncPod callback
	if err = pc.provider.SyncPod(ctx, podCopy, podStatus, pc.reasonCache); err != nil {
		nlog.Errorf("Failed to sync pod: %v", err)
		return false, err
	}

	return false, nil
}

// syncTerminatingPod is expected to terminate all running containers in a pod. Once this method
// returns without error, the pod's local state can be safely cleaned up. If runningPod is passed,
// we perform no status updates.
func (pc *PodsController) syncTerminatingPod(ctx context.Context, pod *corev1.Pod, podStatus *pkgcontainer.PodStatus, runningPod *pkgcontainer.Pod, gracePeriod *int64, podStatusFn func(*corev1.PodStatus)) error {
	nlog.Infof("Sync terminating pod %q enter", format.Pod(pod))
	defer nlog.Infof("Sync terminating pod %q exit", format.Pod(pod))

	// when we receive a runtime only pod (runningPod != nil) we don't need to update the status
	// manager or refresh the status of the cache, because a successful killPod will ensure we do
	// not get invoked again
	if runningPod != nil {
		// we kill the pod with the specified grace period since this is a termination
		if gracePeriod != nil {
			nlog.Debugf("Pod %q terminating with grace period %q", format.Pod(pod), *gracePeriod)
		} else {
			nlog.Debugf("Pod %q terminating with grace period nil", format.Pod(pod))
		}
		if err := pc.provider.KillPod(ctx, pod, *runningPod, gracePeriod); err != nil {
			pc.recorder.Eventf(pod, corev1.EventTypeWarning, events.FailedToKillPod, "error killing pod: %v", err)
			// there was an error killing the pod, so we return that error directly
			utilruntime.HandleError(err)
			return err
		}
		nlog.Debugf("Pod %q termination stopped all running orphan containers", format.Pod(pod))
		return nil
	}

	apiPodStatus := pc.generateAPIPodStatus(pod, podStatus)
	if podStatusFn != nil {
		podStatusFn(&apiPodStatus)
	}
	pc.statusManager.SetPodStatus(pod, apiPodStatus)

	if gracePeriod != nil {
		nlog.Debugf("Pod %q terminating with grace period %q", format.Pod(pod), *gracePeriod)
	} else {
		nlog.Debugf("Pod %q terminating with grace period nil", format.Pod(pod))
	}

	p := pkgcontainer.ConvertPodStatusToRunningPod("", podStatus)
	if err := pc.provider.KillPod(ctx, pod, p, gracePeriod); err != nil {
		pc.recorder.Eventf(pod, corev1.EventTypeWarning, events.FailedToKillPod, "error killing pod: %v", err)
		// there was an error killing the pod, so we return that error directly
		utilruntime.HandleError(err)
		return err
	}

	// Guard against consistency issues in KillPod implementations by checking that there are no
	// running containers. This method is invoked infrequently so this is effectively free and can
	// catch race conditions introduced by callers updating pod status out of order.
	// TODO: have KillPod return the terminal status of stopped containers and write that into the
	//  cache immediately
	podStatus, err := pc.provider.GetPodStatus(ctx, pod)
	if err != nil {
		return fmt.Errorf("unable to read pod %q status prior to final pod termination, detail-> %v", format.Pod(pod), err)
	}
	var runningContainers []string
	for _, s := range podStatus.ContainerStatuses {
		if s.State == pkgcontainer.ContainerStateRunning {
			runningContainers = append(runningContainers, s.ID.String())
		}
	}

	if len(runningContainers) > 0 {
		return fmt.Errorf("detected running containers after a successful KillPod, CRI violation: %v", runningContainers)
	}

	// we have successfully stopped all containers, the pod is terminating, our status is "done"
	nlog.Debugf("Pod %q termination stopped all running containers", format.Pod(pod))

	return nil
}

// syncTerminatedPod cleans up a pod that has terminated (has no running containers).
// The invocations in this call are expected to tear down what PodResourcesAreReclaimed checks (which
// gates pod deletion). When this method exits the pod is expected to be ready for cleanup.
// TODO: make this method take a context and exit early
func (pc *PodsController) syncTerminatedPod(ctx context.Context, pod *corev1.Pod, podStatus *pkgcontainer.PodStatus) error {
	nlog.Infof("Sync terminated pod %q enter", format.Pod(pod))
	defer nlog.Infof("Sync terminated pod %q exit", format.Pod(pod))

	// generate the final status of the pod
	// TODO: should we simply fold this into TerminatePod? that would give a single pod update
	apiPodStatus := pc.generateAPIPodStatus(pod, podStatus)
	pc.statusManager.SetPodStatus(pod, apiPodStatus)

	if err := pc.provider.DeletePod(ctx, pod); err != nil {
		nlog.Errorf("Provider failed to cleanup pod %q: %v", format.Pod(pod), err)
	}

	// mark the final pod status
	pc.statusManager.TerminatePod(pod)
	nlog.Debugf("Pod %q is terminated and will need no more status updates", format.Pod(pod))

	return nil
}

func (pc *PodsController) getPodStatus(ctx context.Context, pod *corev1.Pod, minTime time.Time) (*pkgcontainer.PodStatus, error) {
	return pc.provider.GetPodStatus(ctx, pod)
}
