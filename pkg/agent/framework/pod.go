/*
Copyright 2016 The Kubernetes Authors.

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
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilnet "k8s.io/utils/net"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"

	"github.com/secretflow/kuscia/pkg/agent/status"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// Container state reason list
const (
	PodInitializing   = "PodInitializing"
	ContainerCreating = "ContainerCreating"
)

// getPhase returns the phase of a pod given its container info.
func getPhase(spec *corev1.PodSpec, info []corev1.ContainerStatus) corev1.PodPhase {
	pendingInitialization := 0
	failedInitialization := 0
	for _, container := range spec.InitContainers {
		containerStatus, ok := podutil.GetContainerStatus(info, container.Name)
		if !ok {
			pendingInitialization++
			continue
		}

		switch {
		case containerStatus.State.Running != nil:
			pendingInitialization++
		case containerStatus.State.Terminated != nil:
			if containerStatus.State.Terminated.ExitCode != 0 {
				failedInitialization++
			}
		case containerStatus.State.Waiting != nil:
			if containerStatus.LastTerminationState.Terminated != nil {
				if containerStatus.LastTerminationState.Terminated.ExitCode != 0 {
					failedInitialization++
				}
			} else {
				pendingInitialization++
			}
		default:
			pendingInitialization++
		}
	}

	unknown := 0
	running := 0
	waiting := 0
	stopped := 0
	succeeded := 0
	for _, container := range spec.Containers {
		containerStatus, ok := podutil.GetContainerStatus(info, container.Name)
		if !ok {
			unknown++
			continue
		}

		switch {
		case containerStatus.State.Running != nil:
			running++
		case containerStatus.State.Terminated != nil:
			stopped++
			if containerStatus.State.Terminated.ExitCode == 0 {
				succeeded++
			}
		case containerStatus.State.Waiting != nil:
			if containerStatus.LastTerminationState.Terminated != nil {
				stopped++
			} else {
				waiting++
			}
		default:
			unknown++
		}
	}

	if failedInitialization > 0 && spec.RestartPolicy == corev1.RestartPolicyNever {
		return corev1.PodFailed
	}

	switch {
	case pendingInitialization > 0:
		fallthrough
	case waiting > 0:
		// One or more containers has not been started
		return corev1.PodPending
	case running > 0 && unknown == 0:
		// All containers have been started, and at least
		// one container is running
		return corev1.PodRunning
	case running == 0 && stopped > 0 && unknown == 0:
		// All containers are terminated
		if spec.RestartPolicy == corev1.RestartPolicyAlways {
			// All containers are in the process of restarting
			return corev1.PodRunning
		}
		if stopped == succeeded {
			// RestartPolicy is not Always, and all
			// containers are terminated in success
			return corev1.PodSucceeded
		}
		if spec.RestartPolicy == corev1.RestartPolicyNever {
			// RestartPolicy is Never, and all containers are
			// terminated with at least one in failure
			return corev1.PodFailed
		}
		// RestartPolicy is OnFailure, and at least one in failure
		// and in the process of restarting
		return corev1.PodRunning
	default:
		return corev1.PodPending
	}
}

// sortPodIPs return the PodIPs sorted and truncated by the cluster IP family preference.
// The runtime pod status may have an arbitrary number of IPs, in an arbitrary order.
// PodIPs are obtained by: func (m *kubeGenericRuntimeManager) determinePodSandboxIPs()
// Pick out the first returned IP of the same IP family as the node IP
// first, followed by the first IP of the opposite IP family (if any)
// and use them for the Pod.Status.PodIPs and the Downward API environment variables
func (pc *PodsController) sortPodIPs(podIPs []string) []string {
	ips := make([]string, 0, 2)
	var validPrimaryIP, validSecondaryIP func(ip string) bool
	if len(pc.nodeIPs) == 0 || utilnet.IsIPv4(pc.nodeIPs[0]) {
		validPrimaryIP = utilnet.IsIPv4String
		validSecondaryIP = utilnet.IsIPv6String
	} else {
		validPrimaryIP = utilnet.IsIPv6String
		validSecondaryIP = utilnet.IsIPv4String
	}
	for _, ip := range podIPs {
		if validPrimaryIP(ip) {
			ips = append(ips, ip)
			break
		}
	}
	for _, ip := range podIPs {
		if validSecondaryIP(ip) {
			ips = append(ips, ip)
			break
		}
	}
	return ips
}

// convertStatusToAPIStatus initialize an api PodStatus for the given pod from
// the given internal pod status and the previous state of the pod from the API.
// It is purely transformative and does not alter the kubelet state at all.
func (pc *PodsController) convertStatusToAPIStatus(pod *corev1.Pod, podStatus *pkgcontainer.PodStatus, oldPodStatus corev1.PodStatus) *corev1.PodStatus {
	var apiPodStatus corev1.PodStatus

	// copy pod status IPs to avoid race conditions with PodStatus #102806
	podIPs := make([]string, len(podStatus.IPs))
	for j, ip := range podStatus.IPs {
		podIPs[j] = ip
	}

	// make podIPs order match node IP family preference #97979
	podIPs = pc.sortPodIPs(podIPs)
	for _, ip := range podIPs {
		apiPodStatus.PodIPs = append(apiPodStatus.PodIPs, corev1.PodIP{IP: ip})
	}
	if len(apiPodStatus.PodIPs) > 0 {
		apiPodStatus.PodIP = apiPodStatus.PodIPs[0].IP
	}

	apiPodStatus.ContainerStatuses = pc.convertToAPIContainerStatuses(
		pod, podStatus,
		oldPodStatus.ContainerStatuses,
		pod.Spec.Containers,
		len(pod.Spec.InitContainers) > 0,
		false,
	)
	apiPodStatus.InitContainerStatuses = pc.convertToAPIContainerStatuses(
		pod, podStatus,
		oldPodStatus.InitContainerStatuses,
		pod.Spec.InitContainers,
		len(pod.Spec.InitContainers) > 0,
		true,
	)

	return &apiPodStatus
}

// convertToAPIContainerStatuses converts the given internal container
// statuses into API container statuses.
func (pc *PodsController) convertToAPIContainerStatuses(
	pod *corev1.Pod,
	podStatus *pkgcontainer.PodStatus,
	previousStatus []corev1.ContainerStatus,
	containers []corev1.Container,
	hasInitContainers,
	isInitContainer bool) []corev1.ContainerStatus {
	convertContainerStatus := func(cs *pkgcontainer.Status, oldStatus *corev1.ContainerStatus) *corev1.ContainerStatus {
		cid := cs.ID.String()
		ctrStatus := &corev1.ContainerStatus{
			Name:         cs.Name,
			RestartCount: int32(cs.RestartCount),
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cid,
		}
		switch {
		case cs.State == pkgcontainer.ContainerStateRunning:
			ctrStatus.State.Running = &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(cs.StartedAt)}
		case cs.State == pkgcontainer.ContainerStateCreated:
			// containers that are created but not running are "waiting to be running"
			ctrStatus.State.Waiting = &corev1.ContainerStateWaiting{}
		case cs.State == pkgcontainer.ContainerStateExited:
			ctrStatus.State.Terminated = &corev1.ContainerStateTerminated{
				ExitCode:    int32(cs.ExitCode),
				Reason:      cs.Reason,
				Message:     cs.Message,
				StartedAt:   metav1.NewTime(cs.StartedAt),
				FinishedAt:  metav1.NewTime(cs.FinishedAt),
				ContainerID: cid,
			}

		case cs.State == pkgcontainer.ContainerStateUnknown &&
			oldStatus != nil && // we have an old status
			oldStatus.State.Running != nil: // our previous status was running
			// if this happens, then we know that this container was previously running and isn't anymore (assuming the CRI isn't failing to return running containers).
			// you can imagine this happening in cases where a container failed and the kubelet didn't ask about it in time to see the result.
			// in this case, the container should not to into waiting state immediately because that can make cases like runonce pods actually run
			// twice. "container never ran" is different than "container ran and failed".  This is handled differently in the kubelet
			// and it is handled differently in higher order logic like crashloop detection and handling
			ctrStatus.State.Terminated = &corev1.ContainerStateTerminated{
				Reason:   "ContainerStatusUnknown",
				Message:  "The container could not be located when the pod was terminated",
				ExitCode: 137, // this code indicates an error
			}
			// the restart count normally comes from the CRI (see near the top of this method), but since this is being added explicitly
			// for the case where the CRI did not return a status, we need to manually increment the restart count to be accurate.
			ctrStatus.RestartCount = oldStatus.RestartCount + 1

		default:
			// this collapses any unknown state to container waiting.  If any container is waiting, then the pod status moves to pending even if it is running.
			// if I'm reading this correctly, then any failure to read status on any container results in the entire pod going pending even if the containers
			// are actually running.
			// see https://github.com/kubernetes/kubernetes/blob/5d1b3e26af73dde33ecb6a3e69fb5876ceab192f/pkg/kubelet/kuberuntime/kuberuntime_container.go#L497 to
			// https://github.com/kubernetes/kubernetes/blob/8976e3620f8963e72084971d9d4decbd026bf49f/pkg/kubelet/kuberuntime/helpers.go#L58-L71
			// and interpreted here https://github.com/kubernetes/kubernetes/blob/b27e78f590a0d43e4a23ca3b2bf1739ca4c6e109/pkg/kubelet/kubelet_pods.go#L1434-L1439
			ctrStatus.State.Waiting = &corev1.ContainerStateWaiting{}
		}
		return ctrStatus
	}

	// Fetch old containers statuses from old pod status.
	oldStatuses := make(map[string]corev1.ContainerStatus, len(containers))
	for _, preStatus := range previousStatus {
		oldStatuses[preStatus.Name] = preStatus
	}

	// Set all container statuses to default waiting state
	statuses := make(map[string]*corev1.ContainerStatus, len(containers))
	defaultWaitingState := corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: ContainerCreating}}
	if hasInitContainers {
		defaultWaitingState = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: PodInitializing}}
	}

	for _, container := range containers {
		crtStatus := &corev1.ContainerStatus{
			Name:  container.Name,
			Image: container.Image,
			State: defaultWaitingState,
		}
		oldStatus, found := oldStatuses[container.Name]
		if found {
			if oldStatus.State.Terminated != nil {
				crtStatus = &oldStatus
			} else {
				// Apply some values from the old statuses as the default values.
				crtStatus.RestartCount = oldStatus.RestartCount
				crtStatus.LastTerminationState = oldStatus.LastTerminationState
			}
		}
		statuses[container.Name] = crtStatus
	}

	for _, container := range containers {
		found := false
		for _, cStatus := range podStatus.ContainerStatuses {
			if container.Name == cStatus.Name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		// if no container is found, then assuming it should be waiting seems plausible, but the status code requires
		// that a previous termination be present.  If we're offline long enough or something removed the container, then
		// the previous termination may not be present.  This next code block ensures that if the container was previously running
		// then when that container status disappears, we can infer that it terminated even if we don't know the status code.
		// By setting the lasttermination state we are able to leave the container status waiting and present more accurate
		// data via the API.

		oldStatus, ok := oldStatuses[container.Name]
		if !ok {
			continue
		}
		if oldStatus.State.Terminated != nil {
			// if the old container status was terminated, the lasttermination status is correct
			continue
		}
		if oldStatus.State.Running == nil {
			// if the old container status isn't running, then waiting is an appropriate status and we have nothing to do
			continue
		}

		// If we're here, we know the pod was previously running, but doesn't have a terminated status. We will check now to
		// see if it's in a pending state.
		crtStatus := statuses[container.Name]
		// If the status we're about to write indicates the default, the Waiting status will force this pod back into Pending.
		// That isn't true, we know the pod was previously running.
		isDefaultWaitingStatus := crtStatus.State.Waiting != nil && crtStatus.State.Waiting.Reason == ContainerCreating
		if hasInitContainers {
			isDefaultWaitingStatus = crtStatus.State.Waiting != nil && crtStatus.State.Waiting.Reason == PodInitializing
		}
		if !isDefaultWaitingStatus {
			// the status was written, don't override
			continue
		}
		if crtStatus.LastTerminationState.Terminated != nil {
			// if we already have a termination state, nothing to do
			continue
		}

		// setting this value ensures that we show as stopped here, not as waiting:
		// https://github.com/kubernetes/kubernetes/blob/90c9f7b3e198e82a756a68ffeac978a00d606e55/pkg/kubelet/kubelet_pods.go#L1440-L1445
		// This prevents the pod from becoming pending
		crtStatus.LastTerminationState.Terminated = &corev1.ContainerStateTerminated{
			Reason:   "ContainerStatusUnknown",
			Message:  "The container could not be located when the pod was deleted.  The container used to be Running",
			ExitCode: 137,
		}

		// If the pod was not deleted, then it's been restarted. Increment restart count.
		if pod.DeletionTimestamp == nil {
			crtStatus.RestartCount++
		}

		statuses[container.Name] = crtStatus
	}

	// Copy the slice before sorting it
	containerStatusesCopy := make([]*pkgcontainer.Status, len(podStatus.ContainerStatuses))
	copy(containerStatusesCopy, podStatus.ContainerStatuses)

	// Make the latest container status comes first.
	sort.Sort(sort.Reverse(pkgcontainer.SortContainerStatusesByCreationTime(containerStatusesCopy)))
	// Set container statuses according to the statuses seen in pod status
	containerSeen := map[string]int{}
	for _, cStatus := range containerStatusesCopy {
		cName := cStatus.Name
		if _, ok := statuses[cName]; !ok {
			// This would also ignore the infra container.
			continue
		}
		if containerSeen[cName] >= 2 {
			continue
		}
		var oldStatusPtr *corev1.ContainerStatus
		if oldStatus, ok := oldStatuses[cName]; ok {
			oldStatusPtr = &oldStatus
		}
		crtStatus := convertContainerStatus(cStatus, oldStatusPtr)
		if containerSeen[cName] == 0 {
			statuses[cName] = crtStatus
		} else {
			statuses[cName].LastTerminationState = crtStatus.State
		}
		containerSeen[cName] = containerSeen[cName] + 1
	}

	// Handle the containers failed to be started, which should be in Waiting state.
	for _, container := range containers {
		if isInitContainer {
			// If the init container is terminated with exit code 0, it won't be restarted.
			// TODO(random-liu): Handle this in a cleaner way.
			s := podStatus.FindContainerStatusByName(container.Name)
			if s != nil && s.State == pkgcontainer.ContainerStateExited && s.ExitCode == 0 {
				continue
			}
		}
		// If a container should be restarted in next sync pod, it is *Waiting*.
		if !pkgcontainer.ShouldContainerBeRestarted(&container, pod, podStatus) {
			continue
		}
		crtStatus := statuses[container.Name]
		reason, ok := pc.reasonCache.Get(pod.UID, container.Name)
		if !ok {
			// In fact, we could also apply Waiting state here, but it is less informative,
			// and the container will be restarted soon, so we prefer the original state here.
			// Note that with the current implementation of ShouldContainerBeRestarted the original state here
			// could be:
			//   * Waiting: There is no associated historical container and start failure reason record.
			//   * Terminated: The container is terminated.
			continue
		}
		if crtStatus.State.Terminated != nil {
			crtStatus.LastTerminationState = crtStatus.State
		}
		crtStatus.State = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Reason:  reason.Err.Error(),
				Message: reason.Message,
			},
		}
		statuses[container.Name] = crtStatus
	}

	// Sort the container statuses since clients of this interface expect the list
	// of containers in a pod has a deterministic order.
	if isInitContainer {
		return kubetypes.SortStatusesOfInitContainers(pod, statuses)
	}
	containerStatuses := make([]corev1.ContainerStatus, 0, len(statuses))
	for _, sts := range statuses {
		containerStatuses = append(containerStatuses, *sts)
	}

	sort.Sort(kubetypes.SortedContainerStatuses(containerStatuses))
	return containerStatuses
}

// generateAPIPodStatus creates the final API pod status for a pod, given the
// internal pod status. This method should only be called from within sync*Pod methods.
func (pc *PodsController) generateAPIPodStatus(pod *corev1.Pod, podStatus *pkgcontainer.PodStatus) corev1.PodStatus {
	// use the previous pod status, or the api status, as the basis for this pod
	oldPodStatus, found := pc.statusManager.GetPodStatus(pod.UID)
	if !found {
		oldPodStatus = pod.Status
	}
	s := pc.convertStatusToAPIStatus(pod, podStatus, oldPodStatus)

	// calculate the next phase and preserve reason
	allStatus := append(append([]corev1.ContainerStatus{}, s.ContainerStatuses...), s.InitContainerStatuses...)
	s.Phase = getPhase(&pod.Spec, allStatus)
	nlog.Debugf("Got phase for pod %q, oldPhase=%v, phase=%v", format.Pod(pod), oldPodStatus.Phase, s.Phase)

	// Perform a three-way merge between the statuses from the status manager,
	// runtime, and generated status to ensure terminal status is correctly set.
	if s.Phase != corev1.PodFailed && s.Phase != corev1.PodSucceeded {
		switch {
		case oldPodStatus.Phase == corev1.PodFailed || oldPodStatus.Phase == corev1.PodSucceeded:
			nlog.Debugf("Status manager phase was terminal, updating phase to match, pod=%v, oldPhase=%v", format.Pod(pod), oldPodStatus.Phase)
			s.Phase = oldPodStatus.Phase
		case pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded:
			nlog.Debugf("API phase was terminal, updating phase to match, pod=%v, phase=%v", format.Pod(pod), pod.Status.Phase)
			s.Phase = pod.Status.Phase
		}
	}

	if s.Phase == oldPodStatus.Phase {
		// preserve the reason and message which is associated with the phase
		s.Reason = oldPodStatus.Reason
		s.Message = oldPodStatus.Message
		if len(s.Reason) == 0 {
			s.Reason = pod.Status.Reason
		}
		if len(s.Message) == 0 {
			s.Message = pod.Status.Message
		}
	}

	// pods are not allowed to transition out of terminal phases
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		// API server shows terminal phase; transitions are not allowed
		if s.Phase != pod.Status.Phase {
			nlog.Warnf("Pod %q attempted illegal phase transition, originalStatusPhase=%v, apiStatusPhase=%v, apiStatus=%v",
				format.Pod(pod), pod.Status.Phase, s.Phase, s)
			// Force back to phase from the API server
			s.Phase = pod.Status.Phase
		}
	}

	// ensure the probe managers have up to date status for containers
	pc.provider.RefreshPodStatus(pod, s)

	// preserve all conditions not owned by the kubelet
	s.Conditions = make([]corev1.PodCondition, 0, len(pod.Status.Conditions)+1)
	for _, c := range pod.Status.Conditions {
		if !kubetypes.PodConditionByKubelet(c.Type) {
			s.Conditions = append(s.Conditions, c)
		}
	}
	// set all Kubelet-owned conditions
	s.Conditions = append(s.Conditions, status.GeneratePodInitializedCondition(&pod.Spec, s.InitContainerStatuses, s.Phase))
	s.Conditions = append(s.Conditions, status.GeneratePodReadyCondition(&pod.Spec, s.Conditions, s.ContainerStatuses, s.Phase))
	s.Conditions = append(s.Conditions, status.GenerateContainersReadyCondition(&pod.Spec, s.ContainerStatuses, s.Phase))
	s.Conditions = append(s.Conditions, corev1.PodCondition{
		Type:   corev1.PodScheduled,
		Status: corev1.ConditionTrue,
	})

	// set HostIP and initialize PodIP/PodIPs for host network pods
	hostIPs := pc.nodeIPs
	s.HostIP = hostIPs[0].String()
	// HostNetwork Pods inherit the node IPs as PodIPs. They are immutable once set,
	// other than that if the node becomes dual-stack, we add the secondary IP.
	if pkgcontainer.IsHostNetworkPod(pod) {
		// Primary IP is not set
		if s.PodIP == "" {
			s.PodIP = hostIPs[0].String()
			s.PodIPs = []corev1.PodIP{{IP: s.PodIP}}
		}
		// Secondary IP is not set #105320
		if len(hostIPs) == 2 && len(s.PodIPs) == 1 {
			s.PodIPs = append(s.PodIPs, corev1.PodIP{IP: hostIPs[1].String()})
		}
	}

	return *s
}

// removeOrphanedPodStatuses removes obsolete entries in podStatus where
// the pod is no longer considered bound to this node.
func (pc *PodsController) removeOrphanedPodStatuses(pods []*corev1.Pod, mirrorPods []*corev1.Pod) {
	podUIDs := make(map[types.UID]bool)
	for _, pod := range pods {
		podUIDs[pod.UID] = true
	}
	for _, pod := range mirrorPods {
		podUIDs[pod.UID] = true
	}
	pc.statusManager.RemoveOrphanedStatuses(podUIDs)
}

// deleteOrphanedMirrorPods checks whether pod killer has done with orphaned mirror pod.
// If pod killing is done, podManager.DeleteMirrorPod() is called to delete mirror pod
// from the API server
func (pc *PodsController) deleteOrphanedMirrorPods() {
	mirrorPods := pc.podManager.GetOrphanedMirrorPodNames()
	for _, podFullname := range mirrorPods {
		if !pc.podWorkers.IsPodForMirrorPodTerminatingByFullName(podFullname) {
			_, err := pc.podManager.DeleteMirrorPod(podFullname, nil)
			if err != nil {
				nlog.Errorf("Encountered error when deleting mirror pod %q: %v", podFullname, err)
			} else {
				nlog.Infof("Deleted pod %v", podFullname)
			}
		}
	}
}

// isAdmittedPodTerminal returns true if the provided config source pod is in
// a terminal phase, or if the Kubelet has already indicated the pod has reached
// a terminal phase but the config source has not accepted it yet. This method
// should only be used within the pod configuration loops that notify the pod
// worker, other components should treat the pod worker as authoritative.
func (pc *PodsController) isAdmittedPodTerminal(pod *corev1.Pod) bool {
	// pods are considered inactive if the config source has observed a
	// terminal phase (if the Kubelet recorded that the pod reached a terminal
	// phase the pod should never be restarted)
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return true
	}
	// a pod that has been marked terminal within the Kubelet is considered
	// inactive (may have been rejected by Kubelet admission)
	if sts, ok := pc.statusManager.GetPodStatus(pod.UID); ok {
		if sts.Phase == corev1.PodSucceeded || sts.Phase == corev1.PodFailed {
			return true
		}
	}
	return false
}

// HandlePodCleanups performs a series of cleanup work, including terminating
// pod workers, killing unwanted pods, and removing orphaned volumes/pod
// directories. No config changes are sent to pod workers while this method
// is executing which means no new pods can appear.
// NOTE: This function is executed by the main sync loop, so it
// should not contain any blocking calls.
func (pc *PodsController) HandlePodCleanups(ctx context.Context) error {
	allPods, mirrorPods := pc.podManager.GetPodsAndMirrorPods()
	// Pod phase progresses monotonically. Once a pod has reached a final state,
	// it should never leave regardless of the restart policy. The statuses
	// of such pods should not be changed, and there is no need to sync them.
	// TODO: the logic here does not handle two cases:
	//   1. If the containers were removed immediately after they died, kubelet
	//      may fail to generate correct statuses, let alone filtering correctly.
	//   2. If kubelet restarted before writing the terminated status for a pod
	//      to the apiserver, it could still restart the terminated pod (even
	//      though the pod was not considered terminated by the apiserver).
	// These two conditions could be alleviated by checkpointing kubelet.

	// Stop the workers for terminated pods not in the config source
	workingPods := pc.podWorkers.SyncKnownPods(allPods)

	allPodsByUID := make(map[types.UID]*corev1.Pod)
	for _, pod := range allPods {
		allPodsByUID[pod.UID] = pod
	}

	// Identify the set of pods that have workers, which should be all pods
	// from config that are not terminated, as well as any terminating pods
	// that have already been removed from config. Pods that are terminating
	// will be added to possiblyRunningPods, to prevent overly aggressive
	// cleanup of pod cgroups.
	runningPods := make(map[types.UID]sets.Empty)
	possiblyRunningPods := make(map[types.UID]sets.Empty)
	restartablePods := make(map[types.UID]sets.Empty)
	for uid, sync := range workingPods {
		switch sync {
		case SyncPod:
			runningPods[uid] = struct{}{}
			possiblyRunningPods[uid] = struct{}{}
		case TerminatingPod:
			possiblyRunningPods[uid] = struct{}{}
		case TerminatedAndRecreatedPod:
			restartablePods[uid] = struct{}{}
		}
	}

	runningRuntimePods, err := pc.provider.GetPods(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get pods, detail-> %v", err)
	}
	for _, runningPod := range runningRuntimePods {
		switch workerState, ok := workingPods[runningPod.ID]; {
		case ok && workerState == SyncPod, ok && workerState == TerminatingPod:
			// if the pod worker is already in charge of this pod, we don't need to do anything
			continue
		default:
			// If the pod isn't in the set that should be running and isn't already terminating, terminate
			// now. This termination is aggressive because all known pods should already be in a known state
			// (i.e. a removed static pod should already be terminating), so these are pods that were
			// orphaned due to kubelet restart or bugs. Since housekeeping blocks other config changes, we
			// know that another pod wasn't started in the background so we are safe to terminate the
			// unknown pods.
			if _, ok := allPodsByUID[runningPod.ID]; !ok {
				nlog.Infof("Clean up orphaned pod containers, podUID=%v", runningPod.ID)
				one := int64(1)
				pc.podWorkers.UpdatePod(UpdatePodOptions{
					UpdateType: kubetypes.SyncPodKill,
					RunningPod: runningPod,
					KillPodOptions: &KillPodOptions{
						PodTerminationGracePeriodSecondsOverride: &one,
					},
				})
			}
		}
	}

	// Remove orphaned pod statuses not in the total list of known config pods
	pc.removeOrphanedPodStatuses(allPods, mirrorPods)

	// Trigger provider to clean up pods
	err = pc.provider.CleanupPods(ctx, allPods, runningRuntimePods, possiblyRunningPods)
	if err != nil {
		nlog.Errorf("Failed cleaning up orphaned pod directories: %v", err)
	}

	// Remove any orphaned mirror pods (mirror pods are tracked by name via the
	// pod worker)
	pc.deleteOrphanedMirrorPods()

	// If two pods with the same UID are observed in rapid succession, we need to
	// resynchronize the pod worker after the first pod completes and decide whether
	// to restart the pod. This happens last to avoid confusing the desired state
	// in other components and to increase the likelihood transient OS failures during
	// container start are mitigated. In general only static pods will ever reuse UIDs
	// since the apiserver uses randomly generated UUIDv4 UIDs with a very low
	// probability of collision.
	for uid := range restartablePods {
		pod, ok := allPodsByUID[uid]
		if !ok {
			continue
		}
		if pc.isAdmittedPodTerminal(pod) {
			nlog.Infof("Pod %q is restartable after termination due to UID reuse, but pod phase is terminal", format.Pod(pod))
			continue
		}
		start := pc.clock.Now()
		mirrorPod, _ := pc.podManager.GetMirrorPodByPod(pod)
		nlog.Infof("Pod %q is restartable after termination due to UID %q reuse", format.Pod(pod), pod.UID)
		pc.dispatchWork(pod, kubetypes.SyncPodCreate, mirrorPod, start)
	}

	return nil
}

func countRunningContainerStatus(status corev1.PodStatus) int {
	var runningContainers int
	for _, c := range status.InitContainerStatuses {
		if c.State.Running != nil {
			runningContainers++
		}
	}
	for _, c := range status.ContainerStatuses {
		if c.State.Running != nil {
			runningContainers++
		}
	}
	for _, c := range status.EphemeralContainerStatuses {
		if c.State.Running != nil {
			runningContainers++
		}
	}
	return runningContainers
}

// PodResourcesAreReclaimed returns true if all required node-level resources that a pod was consuming have
// been reclaimed by the kubelet.  Reclaiming resources is a prerequisite to deleting a pod from the API server.
func (pc *PodsController) PodResourcesAreReclaimed(pod *corev1.Pod, status corev1.PodStatus) bool {
	if pc.podWorkers.CouldHaveRunningContainers(pod.UID) {
		// We shouldn't delete pods that still have running containers
		nlog.Infof("Pod %q is terminated, but some containers are still running", format.Pod(pod))
		return false
	}
	if count := countRunningContainerStatus(status); count > 0 {
		// We shouldn't delete pods until the reported pod status contains no more running containers (the previous
		// check ensures no more status can be generated, this check verifies we have seen enough of the status)
		nlog.Infof("Pod %q is terminated, but some container status has not yet been reported, running=%v", format.Pod(pod), count)
		return false
	}

	nlog.Infof("Pod %q is terminated and all resources are reclaimed", format.Pod(pod))

	return true
}

// PodCouldHaveRunningContainers returns true if the pod with the given UID could still have running
// containers. This returns false if the pod has not yet been started or the pod is unknown.
func (pc *PodsController) PodCouldHaveRunningContainers(pod *corev1.Pod) bool {
	return pc.podWorkers.CouldHaveRunningContainers(pod.UID)
}
