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

package kuberuntime

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type podsByID []*pkgcontainer.Pod

func (b podsByID) Len() int           { return len(b) }
func (b podsByID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b podsByID) Less(i, j int) bool { return b[i].ID < b[j].ID }

type containersByID []*pkgcontainer.Container

func (b containersByID) Len() int           { return len(b) }
func (b containersByID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b containersByID) Less(i, j int) bool { return b[i].ID.ID < b[j].ID.ID }

// Newest first.
type podSandboxByCreated []*runtimeapi.PodSandbox

func (p podSandboxByCreated) Len() int           { return len(p) }
func (p podSandboxByCreated) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p podSandboxByCreated) Less(i, j int) bool { return p[i].CreatedAt > p[j].CreatedAt }

type containerStatusByCreated []*pkgcontainer.Status

func (c containerStatusByCreated) Len() int           { return len(c) }
func (c containerStatusByCreated) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c containerStatusByCreated) Less(i, j int) bool { return c[i].CreatedAt.After(c[j].CreatedAt) }

// topkgcontainerState converts runtimeapi.ContainerState to pkgcontainer.State.
func topkgcontainerState(state runtimeapi.ContainerState) pkgcontainer.State {
	switch state {
	case runtimeapi.ContainerState_CONTAINER_CREATED:
		return pkgcontainer.ContainerStateCreated
	case runtimeapi.ContainerState_CONTAINER_RUNNING:
		return pkgcontainer.ContainerStateRunning
	case runtimeapi.ContainerState_CONTAINER_EXITED:
		return pkgcontainer.ContainerStateExited
	case runtimeapi.ContainerState_CONTAINER_UNKNOWN:
		return pkgcontainer.ContainerStateUnknown
	}

	return pkgcontainer.ContainerStateUnknown
}

// toRuntimeProtocol converts v1.Protocol to runtimeapi.Protocol.
func toRuntimeProtocol(protocol v1.Protocol) runtimeapi.Protocol {
	switch protocol {
	case v1.ProtocolTCP:
		return runtimeapi.Protocol_TCP
	case v1.ProtocolUDP:
		return runtimeapi.Protocol_UDP
	case v1.ProtocolSCTP:
		return runtimeapi.Protocol_SCTP
	}

	nlog.Warnf("Unknown protocol %q, defaulting to TCP", protocol)
	return runtimeapi.Protocol_TCP
}

// topkgcontainer converts runtimeapi.Container to pkgcontainer.Container.
func (m *kubeGenericRuntimeManager) topkgcontainer(c *runtimeapi.Container) (*pkgcontainer.Container, error) {
	if c == nil || c.Id == "" || c.Image == nil {
		return nil, fmt.Errorf("unable to convert a nil pointer to a runtime container")
	}

	annotatedInfo := getContainerInfoFromAnnotations(c.Annotations)
	return &pkgcontainer.Container{
		ID:      pkgcontainer.CtrID{Type: m.runtimeName, ID: c.Id},
		Name:    c.GetMetadata().GetName(),
		ImageID: c.ImageRef,
		Image:   c.Image.Image,
		Hash:    annotatedInfo.Hash,
		State:   topkgcontainerState(c.State),
	}, nil
}

// sandboxTopkgcontainer converts runtimeapi.PodSandbox to pkgcontainer.Container.
// This is only needed because we need to return sandboxes as if they were
// pkgcontainer.Containers to avoid substantial changes to PLEG.
// TODO: Remove this once it becomes obsolete.
func (m *kubeGenericRuntimeManager) sandboxTopkgcontainer(s *runtimeapi.PodSandbox) (*pkgcontainer.Container, error) {
	if s == nil || s.Id == "" {
		return nil, fmt.Errorf("unable to convert a nil pointer to a runtime container")
	}

	return &pkgcontainer.Container{
		ID:    pkgcontainer.CtrID{Type: m.runtimeName, ID: s.Id},
		State: pkgcontainer.SandboxToContainerState(s.State),
	}, nil
}

// getImageUser gets uid or user name that will run the command(s) from image. The function
// guarantees that only one of them is set.
func (m *kubeGenericRuntimeManager) getImageUser(ctx context.Context, image string) (*int64, string, error) {
	resp, err := m.imageService.ImageStatus(ctx, &runtimeapi.ImageSpec{Image: image}, false)
	if err != nil {
		return nil, "", err
	}
	imageStatus := resp.GetImage()

	if imageStatus != nil {
		if imageStatus.Uid != nil {
			return &imageStatus.GetUid().Value, "", nil
		}

		if imageStatus.Username != "" {
			return nil, imageStatus.Username, nil
		}
	}

	// If non of them is set, treat it as root.
	return new(int64), "", nil
}

// isInitContainerFailed returns true under the following conditions:
// 1. container has exited and exitcode is not zero.
// 2. container is in unknown state.
// 3. container gets OOMKilled.
func isInitContainerFailed(status *pkgcontainer.Status) bool {
	// When oomkilled occurs, init container should be considered as a failure.
	if status.Reason == "OOMKilled" {
		return true
	}

	if status.State == pkgcontainer.ContainerStateExited && status.ExitCode != 0 {
		return true
	}

	if status.State == pkgcontainer.ContainerStateUnknown {
		return true
	}

	return false
}

// getStableKey generates a key (string) to uniquely identify a
// (pod, container) tuple. The key should include the content of the
// container, so that any change to the container generates a new key.
func getStableKey(pod *v1.Pod, container *v1.Container) string {
	hash := strconv.FormatUint(pkgcontainer.HashContainer(container), 16)
	return fmt.Sprintf("%s_%s_%s_%s_%s", pod.Name, pod.Namespace, string(pod.UID), container.Name, hash)
}

// logPathDelimiter is the delimiter used in the log path.
const logPathDelimiter = "_"

// buildContainerLogsPath builds log path for container relative to pod logs directory.
func BuildContainerLogsPath(containerName string, restartCount int) string {
	return filepath.Join(containerName, fmt.Sprintf("%d.log", restartCount))
}

// BuildContainerLogsDirectory builds absolute log directory path for a container in pod.
func BuildContainerLogsDirectory(rootDirectory, podNamespace, podName string, podUID types.UID, containerName string) string {
	return filepath.Join(BuildPodLogsDirectory(rootDirectory, podNamespace, podName, podUID), containerName)
}

// BuildPodLogsDirectory builds absolute log directory path for a pod sandbox.
func BuildPodLogsDirectory(rootDirectory, podNamespace, podName string, podUID types.UID) string {
	return filepath.Join(rootDirectory, strings.Join([]string{podNamespace, podName,
		string(podUID)}, logPathDelimiter))
}

// parsePodUIDFromLogsDirectory parses pod logs directory name and returns the pod UID.
// It supports both the old pod log directory /var/log/pods/UID, and the new pod log
// directory /var/log/pods/NAMESPACE_NAME_UID.
func parsePodUIDFromLogsDirectory(name string) types.UID {
	parts := strings.Split(name, logPathDelimiter)
	return types.UID(parts[len(parts)-1])
}

func ipcNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil && pod.Spec.HostIPC {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func networkNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil && pod.Spec.HostNetwork {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func pidNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil {
		if pod.Spec.HostPID {
			return runtimeapi.NamespaceMode_NODE
		}
		if pod.Spec.ShareProcessNamespace != nil && *pod.Spec.ShareProcessNamespace {
			return runtimeapi.NamespaceMode_POD
		}
	}
	// Note that PID does not default to the zero value for v1.Pod
	return runtimeapi.NamespaceMode_CONTAINER
}

// namespacesForPod returns the runtimeapi.NamespaceOption for a given pod.
// An empty or nil pod can be used to get the namespace defaults for v1.Pod.
func namespacesForPod(pod *v1.Pod) *runtimeapi.NamespaceOption {
	return &runtimeapi.NamespaceOption{
		Ipc:     ipcNamespaceForPod(pod),
		Network: networkNamespaceForPod(pod),
		Pid:     pidNamespaceForPod(pod),
	}
}

const (
	milliCPUToCPU = 1000

	// 100000 is equivalent to 100ms
	quotaPeriod    = 100000
	minQuotaPeriod = 1000
)

// milliCPUToQuota converts milliCPU to CFS quota and period values
func milliCPUToQuota(milliCPU int64, period int64) (quota int64) {
	// CFS quota is measured in two values:
	//  - cfs_period_us=100ms (the amount of time to measure usage across)
	//  - cfs_quota=20ms (the amount of cpu time allowed to be used across a period)
	// so in the above example, you are limited to 20% of a single CPU
	// for multi-cpu environments, you just scale equivalent amounts
	// see https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt for details
	if milliCPU == 0 {
		return
	}

	// we then convert your milliCPU to a value normalized over a period
	quota = (milliCPU * period) / milliCPUToCPU

	// quota needs to be a minimum of 1ms.
	if quota < minQuotaPeriod {
		quota = minQuotaPeriod
	}

	return
}

// MilliCPUToShares converts the milliCPU to CFS shares.
func MilliCPUToShares(milliCPU int64) uint64 {
	if milliCPU == 0 {
		// Docker converts zero milliCPU to unset, which maps to kernel default
		// for unset: 1024. Return 2 here to really match kernel default for
		// zero milliCPU.
		return MinShares
	}
	// Conceptually (milliCPU / milliCPUToCPU) * sharesPerCPU, but factored to improve rounding.
	shares := (milliCPU * SharesPerCPU) / MilliCPUToCPU
	if shares < MinShares {
		return MinShares
	}
	if shares > MaxShares {
		return MaxShares
	}
	return uint64(shares)
}
