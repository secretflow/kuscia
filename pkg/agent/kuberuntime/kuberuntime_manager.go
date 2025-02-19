/*
Copyright 2014 The Kubernetes Authors.

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

//go:generate mockgen -source==kuberuntime_manager.go -destination=testing/runtime_mock.go -package=testing PodStatusProvider

package kuberuntime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/component-base/logs/logreduction"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	crierror "k8s.io/cri-api/pkg/errors"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/pkg/kubelet/logs"
	"k8s.io/kubernetes/pkg/kubelet/metrics"

	"github.com/secretflow/kuscia/pkg/agent/images"
	"github.com/secretflow/kuscia/pkg/utils/nlog"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"

	"github.com/secretflow/kuscia/pkg/agent/utils/format"

	proberesults "github.com/secretflow/kuscia/pkg/agent/prober/results"
)

const (
	// The api version of kubelet runtime api
	kubeRuntimeAPIVersion = "0.1.0"
	// A minimal shutdown window for avoiding unnecessary SIGKILLs
	minimumGracePeriodInSeconds = 2

	// How frequently to report identical errors
	identicalErrorDelay = 1 * time.Minute
)

var (
	// ErrVersionNotSupported is returned when the api version of runtime interface is not supported
	ErrVersionNotSupported = errors.New("runtime api version is not supported")
)

// podStateProvider can determine if none of the elements are necessary to retain (pod content)
// or if none of the runtime elements are necessary to retain (containers)
type podStateProvider interface {
	IsPodTerminationRequested(kubetypes.UID) bool
	ShouldPodContentBeRemoved(kubetypes.UID) bool
	ShouldPodRuntimeBeRemoved(kubetypes.UID) bool
}

// startSpec wraps the spec required to start a container, either a regular/init container
// or an ephemeral container. Ephemeral containers contain all the fields of regular/init
// containers, plus some additional fields. In both cases startSpec.container will be set.
type startSpec struct {
	container *v1.Container
}

func containerStartSpec(c *v1.Container) *startSpec {
	return &startSpec{container: c}
}

type kubeGenericRuntimeManager struct {
	runtimeName string

	agentRuntime string

	recorder record.EventRecorder

	osInterface pkgcontainer.OSInterface

	// If true, enforce container cpu limits with CFS quota support.
	cpuCFSQuota bool

	// gRPC service clients.
	runtimeService internalapi.RuntimeService
	imageService   internalapi.ImageManagerService

	// wrapped image puller.
	imagePuller images.ImageManager

	// RuntimeHelper that wraps kubelet to generate runtime container options.
	runtimeHelper pkgcontainer.RuntimeHelper

	// Health check results.
	livenessManager  proberesults.Manager
	readinessManager proberesults.Manager
	startupManager   proberesults.Manager

	// Container GC manager.
	containerGC *containerGC

	// Manage container logs.
	logManager logs.ContainerLogManager

	// PodState provider instance.
	podStateProvider podStateProvider

	// Cache last per-container error message to reduce log spam.
	logReduction *logreduction.LogReduction

	// The root directory for pod stdout logs.
	podStdoutRootDirectory string

	// allowPrivileged if true, securityContext.Privileged will work for container.
	allowPrivileged bool
}

func NewManager(recorder record.EventRecorder,
	livenessManager proberesults.Manager,
	readinessManager proberesults.Manager,
	startupManager proberesults.Manager,
	podStateProvider podStateProvider,
	osInterface pkgcontainer.OSInterface,
	logManager logs.ContainerLogManager,
	runtimeHelper pkgcontainer.RuntimeHelper,
	runtimeService internalapi.RuntimeService,
	imageService internalapi.ImageManagerService,
	imageBackOff *flowcontrol.Backoff,
	serializeImagePulls bool,
	imagePullQPS float32,
	imagePullBurst int,
	cpuCFSQuota bool,
	podStdoutRootDirectory string,
	allowPrivileged bool,
	agentRuntime string) (pkgcontainer.Runtime, error) {
	ctx := context.Background()
	m := &kubeGenericRuntimeManager{
		recorder:               recorder,
		livenessManager:        livenessManager,
		readinessManager:       readinessManager,
		startupManager:         startupManager,
		osInterface:            osInterface,
		runtimeHelper:          runtimeHelper,
		runtimeService:         runtimeService,
		imageService:           imageService,
		cpuCFSQuota:            cpuCFSQuota,
		logManager:             logManager,
		podStateProvider:       podStateProvider,
		logReduction:           logreduction.NewLogReduction(identicalErrorDelay),
		podStdoutRootDirectory: podStdoutRootDirectory,
		allowPrivileged:        allowPrivileged,
		agentRuntime:           agentRuntime,
	}

	typedVersion, err := m.getTypedVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("get runtime version failed, detail-> %v", err)
	}

	// Only matching kubeRuntimeAPIVersion is supported now
	// TODO: Runtime API machinery is under discussion at https://github.com/kubernetes/kubernetes/issues/28642
	if typedVersion.Version != kubeRuntimeAPIVersion {
		nlog.Errorf("This runtime api version is not supported, apiVersion=%v, supportedAPIVersion=%v", typedVersion.Version, kubeRuntimeAPIVersion)
		return nil, ErrVersionNotSupported
	}

	m.runtimeName = typedVersion.RuntimeName
	nlog.Infof("Container runtime initialized, containerRuntime=%v, version=%v, apiVersion=%v", typedVersion.RuntimeName, typedVersion.RuntimeVersion, typedVersion.RuntimeApiVersion)

	m.imagePuller = images.NewImageManager(
		pkgcontainer.FilterEventRecorder(recorder),
		m,
		imageBackOff,
		serializeImagePulls,
		imagePullQPS,
		imagePullBurst)

	m.containerGC = &containerGC{
		client:           runtimeService,
		manager:          m,
		podStateProvider: podStateProvider,
	}

	return m, nil
}

// Type returns the type of the container runtime.
func (m *kubeGenericRuntimeManager) Type() string {
	return m.runtimeName
}

func newRuntimeVersion(version string) (*utilversion.Version, error) {
	if ver, err := utilversion.ParseSemantic(version); err == nil {
		return ver, err
	}
	return utilversion.ParseGeneric(version)
}

func (m *kubeGenericRuntimeManager) getTypedVersion(ctx context.Context) (*runtimeapi.VersionResponse, error) {
	typedVersion, err := m.runtimeService.Version(ctx, kubeRuntimeAPIVersion)
	if err != nil {
		return nil, fmt.Errorf("get remote runtime typed version failed: %v", err)
	}
	return typedVersion, nil
}

// Version returns the version information of the container runtime.
func (m *kubeGenericRuntimeManager) Version(ctx context.Context) (pkgcontainer.Version, error) {
	typedVersion, err := m.getTypedVersion(ctx)
	if err != nil {
		return nil, err
	}

	return newRuntimeVersion(typedVersion.RuntimeVersion)
}

// If a container is still in backoff, the function will return a brief backoff error and
// a detailed error message.
func (m *kubeGenericRuntimeManager) doBackOff(pod *v1.Pod, container *v1.Container, podStatus *pkgcontainer.PodStatus, backOff *flowcontrol.Backoff) (bool, string, error) {
	var cStatus *pkgcontainer.Status
	for _, c := range podStatus.ContainerStatuses {
		if c.Name == container.Name && c.State == pkgcontainer.ContainerStateExited {
			cStatus = c
			break
		}
	}

	if cStatus == nil {
		return false, "", nil
	}

	nlog.Debugf("Checking backoff for container %q in pod %q", container.Name, format.Pod(pod))
	// Use the finished time of the latest exited container as the start point to calculate whether to do back-off.
	ts := cStatus.FinishedAt
	// backOff requires a unique key to identify the container.
	key := getStableKey(pod, container)
	if backOff.IsInBackOffSince(key, ts) {
		if containerRef, err := pkgcontainer.GenerateContainerRef(pod, container); err == nil {
			m.recorder.Eventf(containerRef, v1.EventTypeWarning, events.BackOffStartContainer, "Back-off restarting failed container")
		}
		err := fmt.Errorf("back-off %s restarting failed container=%s pod=%s", backOff.Get(key), container.Name, format.Pod(pod))
		nlog.Warnf("Back-off restarting failed container: %v", err)
		return true, err.Error(), pkgcontainer.ErrCrashLoopBackOff
	}

	backOff.Next(key, ts)
	return false, "", nil
}

// containerKillReason explains what killed a given container
type containerKillReason string

const (
	reasonStartupProbe  containerKillReason = "StartupProbe"
	reasonLivenessProbe containerKillReason = "LivenessProbe"
	reasonUnknown       containerKillReason = "Unknown"
)

// containerToKillInfo contains necessary information to kill a container.
type containerToKillInfo struct {
	// The spec of the container.
	container *v1.Container
	// The name of the container.
	name string
	// The message indicates why the container will be killed.
	message string
	// The reason is a clearer source of info on why a container will be killed
	// TODO: replace message with reason?
	reason containerKillReason
}

// podActions keeps information what to do for a pod.
type podActions struct {
	// Stop all running (regular, init and ephemeral) containers and the sandbox for the pod.
	KillPod bool
	// Whether need to create a new sandbox. If needed to kill pod and create
	// a new pod sandbox, all init containers need to be purged (i.e., removed).
	CreateSandbox bool
	// The id of existing sandbox. It is used for starting containers in ContainersToStart.
	SandboxID string
	// The attempt number of creating sandboxes for the pod.
	Attempt uint32

	// The next init container to start.
	NextInitContainerToStart *v1.Container
	// ContainersToStart keeps a list of indexes for the containers to start,
	// where the index is the index of the specific container in the pod spec (
	// pod.Spec.Containers.
	ContainersToStart []int
	// ContainersToKill keeps a map of containers that need to be killed, note that
	// the key is the container ID of the container, while
	// the value contains necessary information to kill a container.
	ContainersToKill map[pkgcontainer.CtrID]containerToKillInfo
	// EphemeralContainersToStart is a list of indexes for the ephemeral containers to start,
	// where the index is the index of the specific container in pod.Spec.EphemeralContainers.
	EphemeralContainersToStart []int
}

// podSandboxChanged checks whether the spec of the pod is changed and returns
// (changed, new attempt, original sandboxID if exist).
func (m *kubeGenericRuntimeManager) podSandboxChanged(pod *v1.Pod, podStatus *pkgcontainer.PodStatus) (bool, uint32, string) {
	if len(podStatus.SandboxStatuses) == 0 {
		nlog.Infof("No sandbox for pod %q can be found. Need to start a new one", format.Pod(pod))
		return true, 0, ""
	}

	readySandboxCount := 0
	for _, s := range podStatus.SandboxStatuses {
		if s.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			readySandboxCount++
		}
	}

	// Needs to create a new sandbox when readySandboxCount > 1 or the ready sandbox is not the latest one.
	sandboxStatus := podStatus.SandboxStatuses[0]
	if readySandboxCount > 1 {
		nlog.Infof("Multiple sandboxes are ready for Pod %q. Need to reconcile them", format.Pod(pod))

		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}
	if sandboxStatus.State != runtimeapi.PodSandboxState_SANDBOX_READY {
		nlog.Infof("No ready sandbox for pod %q can be found. Need to start a new one", format.Pod(pod))
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}

	// Needs to create a new sandbox when network namespace changed.
	if sandboxStatus.GetLinux().GetNamespaces().GetOptions().GetNetwork() != networkNamespaceForPod(pod) {
		nlog.Infof("Sandbox for pod %q has changed. Need to start a new one", format.Pod(pod))
		return true, sandboxStatus.Metadata.Attempt + 1, ""
	}

	// Needs to create a new sandbox when the sandbox does not have an IP address.
	if !pkgcontainer.IsHostNetworkPod(pod) && sandboxStatus.Network.Ip == "" {
		nlog.Infof("Sandbox for pod %q has no IP address. Need to start a new one", format.Pod(pod))
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}

	return false, sandboxStatus.Metadata.Attempt, sandboxStatus.Id
}

func containerChanged(container *v1.Container, containerStatus *pkgcontainer.Status) (uint64, uint64, bool) {
	expectedHash := pkgcontainer.HashContainer(container)
	return expectedHash, containerStatus.Hash, containerStatus.Hash != expectedHash
}

func shouldRestartOnFailure(pod *v1.Pod) bool {
	return pod.Spec.RestartPolicy != v1.RestartPolicyNever
}

func containerSucceeded(c *v1.Container, podStatus *pkgcontainer.PodStatus) bool {
	cStatus := podStatus.FindContainerStatusByName(c.Name)
	if cStatus == nil || cStatus.State == pkgcontainer.ContainerStateRunning {
		return false
	}
	return cStatus.ExitCode == 0
}

// computePodActions checks whether the pod spec has changed and returns the changes if true.
func (m *kubeGenericRuntimeManager) computePodActions(pod *v1.Pod, podStatus *pkgcontainer.PodStatus) podActions {
	createPodSandbox, attempt, sandboxID := m.podSandboxChanged(pod, podStatus)
	changes := podActions{
		KillPod:           createPodSandbox,
		CreateSandbox:     createPodSandbox,
		SandboxID:         sandboxID,
		Attempt:           attempt,
		ContainersToStart: []int{},
		ContainersToKill:  make(map[pkgcontainer.CtrID]containerToKillInfo),
	}

	// If we need to (re-)create the pod sandbox, everything will need to be
	// killed and recreated, and init containers should be purged.
	if createPodSandbox {
		if !shouldRestartOnFailure(pod) && attempt != 0 && len(podStatus.ContainerStatuses) != 0 {
			// Should not restart the pod, just return.
			// we should not create a sandbox for a pod if it is already done.
			// if all containers are done and should not be started, there is no need to create a new sandbox.
			// this stops confusing logs on pods whose containers all have exit codes, but we recreate a sandbox before terminating it.
			//
			// If ContainerStatuses is empty, we assume that we've never
			// successfully created any containers. In this case, we should
			// retry creating the sandbox.
			changes.CreateSandbox = false
			return changes
		}

		// Get the containers to start, excluding the ones that succeeded if RestartPolicy is OnFailure.
		var containersToStart []int
		for idx, c := range pod.Spec.Containers {
			if pod.Spec.RestartPolicy == v1.RestartPolicyOnFailure && containerSucceeded(&c, podStatus) {
				continue
			}
			containersToStart = append(containersToStart, idx)
		}
		// We should not create a sandbox for a Pod if initialization is done and there is no container to start.
		if len(containersToStart) == 0 {
			_, _, done := findNextInitContainerToRun(pod, podStatus)
			if done {
				changes.CreateSandbox = false
				return changes
			}
		}

		if len(pod.Spec.InitContainers) != 0 {
			// Pod has init containers, return the first one.
			changes.NextInitContainerToStart = &pod.Spec.InitContainers[0]
			return changes
		}
		changes.ContainersToStart = containersToStart
		return changes
	}

	// Check initialization progress.
	initLastStatus, next, done := findNextInitContainerToRun(pod, podStatus)
	if !done {
		if next != nil {
			initFailed := initLastStatus != nil && isInitContainerFailed(initLastStatus)
			if initFailed && !shouldRestartOnFailure(pod) {
				changes.KillPod = true
			} else {
				// Always try to stop containers in unknown state first.
				if initLastStatus != nil && initLastStatus.State == pkgcontainer.ContainerStateUnknown {
					changes.ContainersToKill[initLastStatus.ID] = containerToKillInfo{
						name:      next.Name,
						container: next,
						message: fmt.Sprintf("Init container is in %q state, try killing it before restart",
							initLastStatus.State),
						reason: reasonUnknown,
					}
				}
				changes.NextInitContainerToStart = next
			}
		}
		// Initialization failed or still in progress. Skip inspecting non-init
		// containers.
		return changes
	}

	// Number of running containers to keep.
	keepCount := 0
	// check the status of containers.
	for idx, container := range pod.Spec.Containers {
		containerStatus := podStatus.FindContainerStatusByName(container.Name)

		// If container does not exist, or is not running, check whether we
		// need to restart it.
		if containerStatus == nil || containerStatus.State != pkgcontainer.ContainerStateRunning {
			if pkgcontainer.ShouldContainerBeRestarted(&container, pod, podStatus) {
				nlog.Infof("Container %q of pod %q is not in the desired state and shall be started", container.Name, format.Pod(pod))
				changes.ContainersToStart = append(changes.ContainersToStart, idx)
				if containerStatus != nil && containerStatus.State == pkgcontainer.ContainerStateUnknown {
					// If container is in unknown state, we don't know whether it
					// is actually running or not, always try killing it before
					// restart to avoid having 2 running instances of the same container.
					changes.ContainersToKill[containerStatus.ID] = containerToKillInfo{
						name:      containerStatus.Name,
						container: &pod.Spec.Containers[idx],
						message: fmt.Sprintf("Container is in %q state, try killing it before restart",
							containerStatus.State),
						reason: reasonUnknown,
					}
				}
			}
			continue
		}
		// The container is running, but kill the container if any of the following condition is met.
		var message string
		var reason containerKillReason
		restart := shouldRestartOnFailure(pod)
		if _, _, changed := containerChanged(&container, containerStatus); changed {
			message = fmt.Sprintf("Container %s definition changed", container.Name)
			// Restart regardless of the restart policy because the container
			// spec changed.
			restart = true
		} else if liveness, found := m.livenessManager.Get(containerStatus.ID); found && liveness == proberesults.Failure {
			// If the container failed the liveness probe, we should kill it.
			message = fmt.Sprintf("Container %s failed liveness probe", container.Name)
			reason = reasonLivenessProbe
		} else if startup, found := m.startupManager.Get(containerStatus.ID); found && startup == proberesults.Failure {
			// If the container failed the startup probe, we should kill it.
			message = fmt.Sprintf("Container %s failed startup probe", container.Name)
			reason = reasonStartupProbe
		} else {
			// Keep the container.
			keepCount++
			continue
		}

		// We need to kill the container, but if we also want to restart the
		// container afterwards, make the intent clear in the message. Also do
		// not kill the entire pod since we expect container to be running eventually.
		if restart {
			message = fmt.Sprintf("%s, will be restarted", message)
			changes.ContainersToStart = append(changes.ContainersToStart, idx)
		}

		changes.ContainersToKill[containerStatus.ID] = containerToKillInfo{
			name:      containerStatus.Name,
			container: &pod.Spec.Containers[idx],
			message:   message,
			reason:    reason,
		}
		nlog.Infof("Message for Container %v(%v) of pod %q: %v", container.Name, containerStatus.ID, format.Pod(pod), message)
	}

	if keepCount == 0 && len(changes.ContainersToStart) == 0 {
		changes.KillPod = true
	}

	return changes
}

// calculatePrivileged will calculate final privileged value with privilegedFeatureGate.
func (m *kubeGenericRuntimeManager) calculatePrivileged(privileged *bool) *bool {
	if m.allowPrivileged && privileged != nil && *privileged == true {
		privilegedResult := true
		return &privilegedResult
	}

	return nil
}

func (m *kubeGenericRuntimeManager) SyncPod(ctx context.Context, pod *v1.Pod, podStatus *pkgcontainer.PodStatus, auth *credentialprovider.AuthConfig, backOff *flowcontrol.Backoff) (result pkgcontainer.PodSyncResult) {
	// Step 1: Compute sandbox and container changes.
	podContainerChanges := m.computePodActions(pod, podStatus)
	nlog.Infof("ComputePodActions got for pod %q: %v", format.Pod(pod), podContainerChanges)
	if podContainerChanges.CreateSandbox {
		refer, err := ref.GetReference(legacyscheme.Scheme, pod)
		if err != nil {
			nlog.Errorf("Couldn't make a ref to pod %q", format.Pod(pod))
		}
		if podContainerChanges.SandboxID != "" {
			m.recorder.Eventf(refer, v1.EventTypeNormal, events.SandboxChanged, "Pod sandbox changed, it will be killed and re-created.")
		} else {
			nlog.Debugf("SyncPod received new pod %q, will create a sandbox for it", format.Pod(pod))
		}
	}

	// Step 2: Kill the pod if the sandbox has changed.
	if podContainerChanges.KillPod {
		if podContainerChanges.CreateSandbox {
			nlog.Debugf("Stopping PodSandbox for pod %q, will start new one", format.Pod(pod))
		} else {
			nlog.Debugf("Stopping PodSandbox for pod %q, because all other containers are dead", format.Pod(pod))
		}

		killResult := m.killPodWithSyncResult(ctx, pod, pkgcontainer.ConvertPodStatusToRunningPod(m.runtimeName, podStatus), nil)
		result.AddPodSyncResult(killResult)
		if killResult.Error() != nil {
			nlog.Errorf("killPodWithSyncResult failed: %v", killResult.Error())
			return
		}

		if podContainerChanges.CreateSandbox {
			m.purgeInitContainers(ctx, pod, podStatus)
		}
	} else {
		// Step 3: kill any running containers in this pod which are not to keep.
		for containerID, containerInfo := range podContainerChanges.ContainersToKill {
			nlog.Infof("Killing unwanted container %v(%v) for pod %q", containerInfo.name, containerID, format.Pod(pod))
			killContainerResult := pkgcontainer.NewSyncResult(pkgcontainer.KillContainer, containerInfo.name)
			result.AddSyncResult(killContainerResult)
			if err := m.killContainer(ctx, pod, containerID, containerInfo.name, containerInfo.message, containerInfo.reason, nil); err != nil {
				killContainerResult.Fail(pkgcontainer.ErrKillContainer, err.Error())
				nlog.Errorf("Kill container %v(%v) for pod %q failed", containerInfo.name, containerID, format.Pod(pod))
				return
			}
		}
	}

	// Keep terminated init containers fairly aggressively controlled
	// This is an optimization because container removals are typically handled
	// by container garbage collector.
	m.pruneInitContainersBeforeStart(ctx, pod, podStatus)

	// We pass the value of the PRIMARY podIP and list of podIPs down to
	// generatePodSandboxConfig and generateContainerConfig, which in turn
	// passes it to various other functions, in order to facilitate functionality
	// that requires this value (hosts file and downward API) and avoid races determining
	// the pod IP in cases where a container requires restart but the
	// podIP isn't in the status manager yet. The list of podIPs is used to
	// generate the hosts file.
	//
	// We default to the IPs in the passed-in pod status, and overwrite them if the
	// sandbox needs to be (re)started.
	var podIPs []string
	if podStatus != nil {
		podIPs = podStatus.IPs
	}

	// Step 4: Create a sandbox for the pod if necessary.
	podSandboxID := podContainerChanges.SandboxID
	if podContainerChanges.CreateSandbox {
		var msg string
		var err error

		nlog.Debugf("Creating PodSandbox for pod %q", format.Pod(pod))
		metrics.StartedPodsTotal.Inc()
		createSandboxResult := pkgcontainer.NewSyncResult(pkgcontainer.CreatePodSandbox, format.Pod(pod))
		result.AddSyncResult(createSandboxResult)

		podSandboxID, msg, err = m.createPodSandbox(ctx, pod, podContainerChanges.Attempt)
		if err != nil {
			// createPodSandbox can return an error from CNI, CSI,
			// or CRI if the Pod has been deleted while the POD is
			// being created. If the pod has been deleted then it's
			// not a real error.
			//
			// SyncPod can still be running when we get here, which
			// means the PodWorker has not acked the deletion.
			if m.podStateProvider.IsPodTerminationRequested(pod.UID) {
				nlog.Debugf("Pod %q was deleted and sandbox failed to be created", format.Pod(pod))
				return
			}

			createSandboxResult.Fail(pkgcontainer.ErrCreatePodSandbox, msg)
			nlog.Errorf("CreatePodSandbox for pod %q failed", format.Pod(pod))
			refer, referr := reference.GetReference(legacyscheme.Scheme, pod)
			if referr != nil {
				nlog.Errorf("Couldn't make a ref to pod %q: %v", format.Pod(pod), referr)
			}
			m.recorder.Eventf(refer, v1.EventTypeWarning, events.FailedCreatePodSandBox, "Failed to create pod sandbox: %v", err)

			return
		}
		nlog.Debugf("Created PodSandbox %q for pod %q", podSandboxID, format.Pod(pod))

		resp, err := m.runtimeService.PodSandboxStatus(ctx, podSandboxID, false)
		if err != nil {
			refer, referr := reference.GetReference(legacyscheme.Scheme, pod)
			if referr != nil {
				nlog.Errorf("Couldn't make a ref to pod %q: %v", format.Pod(pod), referr)
			}
			m.recorder.Eventf(refer, v1.EventTypeWarning, events.FailedStatusPodSandBox, "Unable to get pod sandbox status: %v", err)

			nlog.Errorf("Failed to get pod sandbox status; Skipping pod %q", format.Pod(pod))
			result.Fail(err)
			return
		}
		if resp.GetStatus() == nil {
			result.Fail(errors.New("pod sandbox status is nil"))
			return
		}

		// If we ever allow updating a pod from non-host-network to
		// host-network, we may use a stale IP.
		if !pkgcontainer.IsHostNetworkPod(pod) {
			// Overwrite the podIPs passed in the pod status, since we just started the pod sandbox.
			podIPs = m.determinePodSandboxIPs(pod.Namespace, pod.Name, resp.GetStatus())
			nlog.Debugf("Determined the ip %q for pod %q after sandbox changed", podIPs, format.Pod(pod))
		}
	}

	// the start containers routines depend on pod ip(as in primary pod ip)
	// instead of trying to figure out if we have 0 < len(podIPs)
	// everytime, we short circuit it here
	podIP := ""
	if len(podIPs) != 0 {
		podIP = podIPs[0]
	}

	// Get podSandboxConfig for containers to start.
	configPodSandboxResult := pkgcontainer.NewSyncResult(pkgcontainer.ConfigPodSandbox, podSandboxID)
	result.AddSyncResult(configPodSandboxResult)
	podSandboxConfig, err := m.generatePodSandboxConfig(pod, 0)
	if err != nil {
		message := fmt.Sprintf("GeneratePodSandboxConfig for pod %q failed: %v", format.Pod(pod), err)
		nlog.Errorf("GeneratePodSandboxConfig for pod %q failed: %v", format.Pod(pod), err)
		configPodSandboxResult.Fail(pkgcontainer.ErrConfigPodSandbox, message)
		return
	}

	// Helper containing boilerplate common to starting all types of containers.
	// typeName is a description used to describe this type of container in log messages,
	// currently: "container"
	// metricLabel is the label used to describe this type of container in monitoring metrics.
	// currently: "container"
	start := func(ctx context.Context, typeName, metricLabel string, spec *startSpec) error {
		startContainerResult := pkgcontainer.NewSyncResult(pkgcontainer.StartContainer, spec.container.Name)
		result.AddSyncResult(startContainerResult)

		isInBackOff, msg, err := m.doBackOff(pod, spec.container, podStatus, backOff)
		if isInBackOff {
			startContainerResult.Fail(err, msg)
			nlog.Debugf("Backing Off restarting container %q in pod %q", spec.container.Name, format.Pod(pod))
			return err
		}

		nlog.Debugf("Creating container %q in pod %q", spec.container.Name, format.Pod(pod))
		// NOTE (aramase) podIPs are populated for single stack and dual stack clusters. Send only podIPs.
		if msg, err := m.startContainer(ctx, podSandboxID, podSandboxConfig, spec, pod, podStatus, auth, podIP, podIPs); err != nil {
			// startContainer() returns well-defined error codes that have reasonable cardinality for metrics and are
			// useful to cluster administrators to distinguish "server errors" from "user errors".
			metrics.StartedContainersErrorsTotal.WithLabelValues(metricLabel, err.Error()).Inc()
			startContainerResult.Fail(err, msg)
			nlog.Errorf("Container %q start failed in pod %q, containerMessage-> %v, err-> %v", spec.container.Name, format.Pod(pod), msg, err)
			return err
		}

		return nil
	}

	// Step 5: start the init container.
	if container := podContainerChanges.NextInitContainerToStart; container != nil {
		// Start the next init container.
		if err := start(ctx, "init container", metrics.InitContainer, containerStartSpec(container)); err != nil {
			return
		}
	}

	// Step 6: start containers in podContainerChanges.ContainersToStart.
	for _, idx := range podContainerChanges.ContainersToStart {
		_ = start(ctx, "container", metrics.Container, containerStartSpec(&pod.Spec.Containers[idx]))
	}
	return
}

// killContainersWithSyncResult kills all pod's containers with sync results.
func (m *kubeGenericRuntimeManager) killContainersWithSyncResult(ctx context.Context, pod *v1.Pod, runningPod pkgcontainer.Pod, gracePeriodOverride *int64) (syncResults []*pkgcontainer.SyncResult) {
	containerResults := make(chan *pkgcontainer.SyncResult, len(runningPod.Containers))
	wg := sync.WaitGroup{}

	wg.Add(len(runningPod.Containers))
	for _, container := range runningPod.Containers {
		go func(container *pkgcontainer.Container) {
			defer utilruntime.HandleCrash()
			defer wg.Done()

			killContainerResult := pkgcontainer.NewSyncResult(pkgcontainer.KillContainer, container.Name)
			if err := m.killContainer(ctx, pod, container.ID, container.Name, "", reasonUnknown, gracePeriodOverride); err != nil {
				killContainerResult.Fail(pkgcontainer.ErrKillContainer, err.Error())
				// Use runningPod for logging as the pod passed in could be *nil*.
				nlog.Errorf("Kill container %v(%v) in pod %q failed", container.Name, container.ID, format.PodDesc(runningPod.Name, runningPod.Namespace, runningPod.ID))
			}
			containerResults <- killContainerResult
		}(container)
	}
	wg.Wait()
	close(containerResults)

	for containerResult := range containerResults {
		syncResults = append(syncResults, containerResult)
	}
	return
}

// killPodWithSyncResult kills a runningPod and returns SyncResult.
// Note: The pod passed in could be *nil* when kubelet restarted.
func (m *kubeGenericRuntimeManager) killPodWithSyncResult(ctx context.Context, pod *v1.Pod, runningPod pkgcontainer.Pod, gracePeriodOverride *int64) (result pkgcontainer.PodSyncResult) {
	killContainerResults := m.killContainersWithSyncResult(ctx, pod, runningPod, gracePeriodOverride)
	for _, containerResult := range killContainerResults {
		result.AddSyncResult(containerResult)
	}

	// stop sandbox, the sandbox will be removed in GarbageCollect
	killSandboxResult := pkgcontainer.NewSyncResult(pkgcontainer.KillPodSandbox, runningPod.ID)
	result.AddSyncResult(killSandboxResult)
	// Stop all sandboxes belongs to same pod
	for _, podSandbox := range runningPod.Sandboxes {
		if err := m.runtimeService.StopPodSandbox(ctx, podSandbox.ID.ID); err != nil && !crierror.IsNotFound(err) {
			killSandboxResult.Fail(pkgcontainer.ErrKillPodSandbox, err.Error())
			nlog.Errorf("Failed to stop sandbox %q: %v", podSandbox.ID, err)
		}
	}

	return
}

// KillPod kills all the containers of a pod. Pod may be nil, running pod must not be.
// gracePeriodOverride if specified allows the caller to override the pod default grace period.
// only hard kill paths are allowed to specify a gracePeriodOverride in the kubelet in order to not corrupt user data.
// it is useful when doing SIGKILL for hard eviction scenarios, or max grace period during soft eviction scenarim.osInterface.
func (m *kubeGenericRuntimeManager) KillPod(ctx context.Context, pod *v1.Pod, runningPod pkgcontainer.Pod, gracePeriodOverride *int64) error {
	err := m.killPodWithSyncResult(ctx, pod, runningPod, gracePeriodOverride)
	return err.Error()
}

// GetPodStatus retrieves the status of the pod, including the
// information of all containers in the pod that are visible in Runtime.
func (m *kubeGenericRuntimeManager) GetPodStatus(ctx context.Context, uid kubetypes.UID, name, namespace string) (*pkgcontainer.PodStatus, error) {
	// Now we retain restart count of container as a container label. Each time a container
	// restarts, pod will read the restart count from the registered dead container, increment
	// it to get the new restart count, and then add a label with the new restart count on
	// the newly started container.
	// However, there are some limitations of this method:
	//	1. When all dead containers were garbage collected, the container status could
	//	not get the historical value and would be *inaccurate*. Fortunately, the chance
	//	is really slim.
	//	2. When working with old version containers which have no restart count label,
	//	we can only assume their restart count is 0.
	// Anyhow, we only promised "best-effort" restart count reporting, we can just ignore
	// these limitations now.
	// TODO: move this comment to SyncPod.
	podSandboxIDs, err := m.getSandboxIDByPodUID(ctx, uid, nil)
	if err != nil {
		return nil, err
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       uid,
		},
	}

	podFullName := format.Pod(pod)

	nlog.Debugf("Got sandbox IDs %q for pod %q", podSandboxIDs, format.Pod(pod))

	var sandboxStatuses []*runtimeapi.PodSandboxStatus
	var podIPs []string
	for idx, podSandboxID := range podSandboxIDs {
		resp, statusErr := m.runtimeService.PodSandboxStatus(ctx, podSandboxID, false)
		// Between List (getSandboxIDByPodUID) and check (PodSandboxStatus) another thread might remove a container, and that is normal.
		// The previous call (getSandboxIDByPodUID) never fails due to a pod sandbox not existing.
		// Therefore, this method should not either, but instead act as if the previous call failed,
		// which means the error should be ignored.
		if crierror.IsNotFound(statusErr) {
			continue
		}
		if statusErr != nil {
			return nil, fmt.Errorf("failed to get sandbox %q status for pod %q, detail-> %v", podSandboxID, format.Pod(pod), statusErr)
		}
		if resp.GetStatus() == nil {
			return nil, errors.New("pod sandbox status is nil")
		}
		sandboxStatuses = append(sandboxStatuses, resp.Status)
		// Only get pod IP from latest sandbox
		if idx == 0 && resp.Status.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			podIPs = m.determinePodSandboxIPs(namespace, name, resp.Status)
		}
	}

	// Get statuses of all containers visible in the pod.
	containerStatuses, err := m.getPodContainerStatuses(ctx, uid, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %q container statuses, detail-> %v", format.Pod(pod), err)
	}
	m.logReduction.ClearID(podFullName)

	return &pkgcontainer.PodStatus{
		ID:                uid,
		Name:              name,
		Namespace:         namespace,
		IPs:               podIPs,
		SandboxStatuses:   sandboxStatuses,
		ContainerStatuses: containerStatuses,
	}, nil
}

// GetPods returns a list of containers grouped by pods. The boolean parameter
// specifies whether the runtime returns all containers including those already
// exited and dead containers (used for garbage collection).
func (m *kubeGenericRuntimeManager) GetPods(ctx context.Context, all bool) ([]*pkgcontainer.Pod, error) {
	pods := make(map[kubetypes.UID]*pkgcontainer.Pod)
	sandboxes, err := m.getPodSandboxes(ctx, all)
	if err != nil {
		return nil, err
	}
	for i := range sandboxes {
		s := sandboxes[i]
		if s.Metadata == nil {
			nlog.Debugf("Sandbox %q does not have metadata", s)
			continue
		}
		podUID := kubetypes.UID(s.Metadata.Uid)
		if _, ok := pods[podUID]; !ok {
			pods[podUID] = &pkgcontainer.Pod{
				ID:        podUID,
				Name:      s.Metadata.Name,
				Namespace: s.Metadata.Namespace,
			}
		}
		p := pods[podUID]
		converted, convertErr := m.sandboxTopkgcontainer(s)
		if convertErr != nil {
			nlog.Warnf("Convert sandbox %q of pod %q failed: %v", s.Id, podUID, convertErr)
			continue
		}
		p.Sandboxes = append(p.Sandboxes, converted)
	}

	containers, err := m.getContainers(ctx, all)
	if err != nil {
		return nil, err
	}
	for i := range containers {
		c := containers[i]
		if c.Metadata == nil {
			nlog.Debugf("Container %q does not have metadata", c.Id)
			continue
		}

		labelledInfo := getContainerInfoFromLabels(c.Labels)
		pod, found := pods[labelledInfo.PodUID]
		if !found {
			pod = &pkgcontainer.Pod{
				ID:        labelledInfo.PodUID,
				Name:      labelledInfo.PodName,
				Namespace: labelledInfo.PodNamespace,
			}
			pods[labelledInfo.PodUID] = pod
		}

		converted, err := m.topkgcontainer(c)
		if err != nil {
			nlog.Debugf("Convert container %q of pod %q failed: %v", c.Id, labelledInfo.PodUID, err)
			continue
		}

		pod.Containers = append(pod.Containers, converted)
	}

	// Convert map to list.
	var result []*pkgcontainer.Pod
	for _, pod := range pods {
		result = append(result, pod)
	}

	return result, nil
}

// GarbageCollect removes dead containers using the specified container gc policy.
func (m *kubeGenericRuntimeManager) GarbageCollect(ctx context.Context, gcPolicy pkgcontainer.GCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	return m.containerGC.GarbageCollect(ctx, gcPolicy, m.podStdoutRootDirectory, allSourcesReady, evictNonDeletedPods)
}
