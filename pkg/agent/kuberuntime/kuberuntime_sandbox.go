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
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	kubetypes "k8s.io/apimachinery/pkg/types"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	resourcehelper "k8s.io/kubernetes/pkg/api/v1/resource"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/utils/nlog"

	"github.com/secretflow/kuscia/pkg/agent/utils/format"

	"github.com/secretflow/kuscia/pkg/agent/framework/net"
)

const (
	// These limits are defined in the kernel:
	// https://github.com/torvalds/linux/blob/0bddd227f3dc55975e2b8dfa7fc6f959b062a2c7/kernel/sched/sched.h#L427-L428
	MinShares = 2
	MaxShares = 262144

	SharesPerCPU  = 1024
	MilliCPUToCPU = 1000

	// 100000 is equivalent to 100ms
	QuotaPeriod    = 100000
	MinQuotaPeriod = 1000
)

// generatePodSandboxLinuxConfig generates LinuxPodSandboxConfig from v1.Pod.
func (m *kubeGenericRuntimeManager) generatePodSandboxLinuxConfig(pod *v1.Pod) (*runtimeapi.LinuxPodSandboxConfig, error) {
	cgroupParent := m.runtimeHelper.GetPodCgroupParent(pod)
	lc := &runtimeapi.LinuxPodSandboxConfig{
		CgroupParent: cgroupParent,
		SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
			Seccomp: &runtimeapi.SecurityProfile{
				ProfileType: runtimeapi.SecurityProfile_RuntimeDefault,
			},
			Privileged: false,
		},
	}
	for _, p := range pod.Spec.Containers {
		if p.SecurityContext != nil {
			privilegedResult := m.calculatePrivileged(p.SecurityContext.Privileged)
			if privilegedResult != nil {
				lc.SecurityContext.Privileged = *privilegedResult
			}
		}
	}

	return lc, nil
}

// calculateLinuxResources will create the linuxContainerResources type based on the provided CPU and memory resource requests, limits
func (m *kubeGenericRuntimeManager) calculateLinuxResources(cpuRequest, cpuLimit, memoryLimit *resource.Quantity) *runtimeapi.LinuxContainerResources {
	resources := runtimeapi.LinuxContainerResources{}
	var cpuShares int64

	memLimit := memoryLimit.Value()

	// If request is not specified, but limit is, we want request to default to limit.
	// API server does this for new containers, but we repeat this logic in Kubelet
	// for containers running on existing Kubernetes clusters.
	if cpuRequest.IsZero() && !cpuLimit.IsZero() {
		cpuShares = int64(MilliCPUToShares(cpuLimit.MilliValue()))

	} else {
		// if cpuRequest.Amount is nil, then MilliCPUToShares will return the minimal number
		// of CPU shares.
		cpuShares = int64(MilliCPUToShares(cpuRequest.MilliValue()))
	}
	resources.CpuShares = cpuShares
	if memLimit != 0 {
		resources.MemoryLimitInBytes = memLimit
	}

	if m.cpuCFSQuota {
		// if cpuLimit.Amount is nil, then the appropriate default value is returned
		// to allow full usage of cpu resource.
		cpuPeriod := int64(quotaPeriod)
		cpuQuota := milliCPUToQuota(cpuLimit.MilliValue(), cpuPeriod)
		resources.CpuQuota = cpuQuota
		resources.CpuPeriod = cpuPeriod
	}

	return &resources
}

func (m *kubeGenericRuntimeManager) convertOverheadToLinuxResources(pod *v1.Pod) *runtimeapi.LinuxContainerResources {
	resources := &runtimeapi.LinuxContainerResources{}
	if pod.Spec.Overhead != nil {
		cpu := pod.Spec.Overhead.Cpu()
		memory := pod.Spec.Overhead.Memory()

		// For overhead, we do not differentiate between requests and limits. Treat this overhead
		// as "guaranteed", with requests == limits
		resources = m.calculateLinuxResources(cpu, cpu, memory)
	}

	return resources
}

func (m *kubeGenericRuntimeManager) calculateSandboxResources(pod *v1.Pod) *runtimeapi.LinuxContainerResources {
	req, lim := resourcehelper.PodRequestsAndLimitsWithoutOverhead(pod)
	return m.calculateLinuxResources(req.Cpu(), lim.Cpu(), lim.Memory())
}

func (m *kubeGenericRuntimeManager) applySandboxResources(pod *v1.Pod, config *runtimeapi.PodSandboxConfig) error {

	if config.Linux == nil {
		return nil
	}
	config.Linux.Resources = m.calculateSandboxResources(pod)
	config.Linux.Overhead = m.convertOverheadToLinuxResources(pod)

	return nil
}

// generatePodSandboxConfig generates pod sandbox config from v1.Pod.
func (m *kubeGenericRuntimeManager) generatePodSandboxConfig(pod *v1.Pod, attempt uint32) (*runtimeapi.PodSandboxConfig, error) {
	// TODO: deprecating podsandbox resource requirements in favor of the pod level cgroup
	// Refer https://github.com/kubernetes/kubernetes/issues/29871
	podUID := string(pod.UID)
	podSandboxConfig := &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Uid:       podUID,
			Attempt:   attempt,
		},
		Labels:      newPodLabels(pod),
		Annotations: newPodAnnotations(pod),
	}

	dnsConfig, err := m.runtimeHelper.GetPodDNS(pod)
	if err != nil {
		return nil, err
	}
	podSandboxConfig.DnsConfig = dnsConfig

	if !pkgcontainer.IsHostNetworkPod(pod) {
		// TODO: Add domain support in new runtime interface
		podHostname, podDomain, err := m.runtimeHelper.GeneratePodHostNameAndDomain(pod)
		if err != nil {
			return nil, err
		}
		podHostname, err = util.GetNodenameForKernel(podHostname, podDomain, pod.Spec.SetHostnameAsFQDN)
		if err != nil {
			return nil, err
		}
		podSandboxConfig.Hostname = podHostname
	}

	logDir := BuildPodLogsDirectory(m.podStdoutRootDirectory, pod.Namespace, pod.Name, pod.UID)
	podSandboxConfig.LogDirectory = logDir

	var portMappings []*runtimeapi.PortMapping
	for _, c := range pod.Spec.Containers {
		containerPortMappings := pkgcontainer.MakePortMappings(&c)

		for idx := range containerPortMappings {
			port := containerPortMappings[idx]
			hostPort := int32(port.HostPort)
			containerPort := int32(port.ContainerPort)
			protocol := toRuntimeProtocol(port.Protocol)
			portMappings = append(portMappings, &runtimeapi.PortMapping{
				HostIp:        port.HostIP,
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Protocol:      protocol,
			})
		}

	}
	if len(portMappings) > 0 {
		podSandboxConfig.PortMappings = portMappings
	}

	lc, err := m.generatePodSandboxLinuxConfig(pod)
	if err != nil {
		return nil, err
	}
	podSandboxConfig.Linux = lc

	// Update config to include overhead, sandbox level resources
	if err := m.applySandboxResources(pod, podSandboxConfig); err != nil {
		return nil, err
	}
	return podSandboxConfig, nil
}

// createPodSandbox creates a pod sandbox and returns (podSandBoxID, message, error).
func (m *kubeGenericRuntimeManager) createPodSandbox(ctx context.Context, pod *v1.Pod, attempt uint32) (string, string, error) {
	podSandboxConfig, err := m.generatePodSandboxConfig(pod, attempt)
	if err != nil {
		message := fmt.Sprintf("Failed to generate sandbox config for pod %q: %v", format.Pod(pod), err)
		nlog.Errorf("Failed to generate sandbox config for pod %q", format.Pod(pod))
		return "", message, err
	}

	// Create pod logs directory
	err = m.osInterface.MkdirAll(podSandboxConfig.LogDirectory, 0755)
	if err != nil {
		message := fmt.Sprintf("Failed to create log directory for pod %q: %v", format.Pod(pod), err)
		nlog.Errorf("Failed to create log directory for pod %q", format.Pod(pod))
		return "", message, err
	}

	runtimeHandler := ""
	podSandBoxID, err := m.runtimeService.RunPodSandbox(ctx, podSandboxConfig, runtimeHandler)
	if err != nil {
		message := fmt.Sprintf("Failed to create sandbox for pod %q: %v", format.Pod(pod), err)
		nlog.Errorf("Failed to create sandbox for pod %q", format.Pod(pod))
		return "", message, err
	}

	return podSandBoxID, "", nil
}

// determinePodSandboxIP determines the IP addresses of the given pod sandbox.
func (m *kubeGenericRuntimeManager) determinePodSandboxIPs(podNamespace, podName string, podSandbox *runtimeapi.PodSandboxStatus) []string {
	var podIPs []string
	if podSandbox.Network == nil {
		nlog.Infof("Pod Sandbox status doesn't have network information, cannot report IPs, pod=%v/%v", podNamespace, podName)
		return podIPs
	}

	// ip could be an empty string if runtime is not responsible for the
	// IP (e.g., host networking).

	// pick primary IP
	if len(podSandbox.Network.Ip) != 0 {
		if net.ParseIP(podSandbox.Network.Ip) == nil {
			nlog.Infof("Pod Sandbox reported an unparseable primary IP %q, pod=%v/%v", podSandbox.Network.Ip, podNamespace, podName)
			return nil
		}
		podIPs = append(podIPs, podSandbox.Network.Ip)
	}

	// pick additional ips, if cri reported them
	for _, podIP := range podSandbox.Network.AdditionalIps {
		if nil == net.ParseIP(podIP.Ip) {
			nlog.Infof("Pod Sandbox reported an unparseable additional IP %q, pod=%v/%v", podIP.Ip, podNamespace, podName)
			return nil
		}
		podIPs = append(podIPs, podIP.Ip)
	}

	return podIPs
}

// getPodSandboxID gets the sandbox id by podUID and returns ([]sandboxID, error).
// Param state could be nil in order to get all sandboxes belonging to same pod.
func (m *kubeGenericRuntimeManager) getSandboxIDByPodUID(ctx context.Context, podUID kubetypes.UID, state *runtimeapi.PodSandboxState) ([]string, error) {
	filter := &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{types.KubernetesPodUIDLabel: string(podUID)},
	}
	if state != nil {
		filter.State = &runtimeapi.PodSandboxStateValue{
			State: *state,
		}
	}
	sandboxes, err := m.runtimeService.ListPodSandbox(ctx, filter)
	if err != nil {
		nlog.Errorf("Failed to list sandboxes for pod %q", podUID)
		return nil, err
	}

	if len(sandboxes) == 0 {
		return nil, nil
	}

	// Sort with newest first.
	sandboxIDs := make([]string, len(sandboxes))
	sort.Sort(podSandboxByCreated(sandboxes))
	for i, s := range sandboxes {
		sandboxIDs[i] = s.Id
	}

	return sandboxIDs, nil
}

// getSandboxes lists all (or just the running) sandboxes.
func (m *kubeGenericRuntimeManager) getPodSandboxes(ctx context.Context, all bool) ([]*runtimeapi.PodSandbox, error) {
	var filter *runtimeapi.PodSandboxFilter
	if !all {
		readyState := runtimeapi.PodSandboxState_SANDBOX_READY
		filter = &runtimeapi.PodSandboxFilter{
			State: &runtimeapi.PodSandboxStateValue{
				State: readyState,
			},
		}
	}

	resp, err := m.runtimeService.ListPodSandbox(ctx, filter)
	if err != nil {
		nlog.Errorf("Failed to list pod sandboxes")
		return nil, err
	}

	return resp, nil
}
