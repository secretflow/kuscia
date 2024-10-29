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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/armon/circbuf"
	"github.com/opencontainers/selinux/go-selinux"
	grpcstatus "google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	crierror "k8s.io/cri-api/pkg/errors"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/kubelet/cri/remote"
	"k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/pkg/kubelet/kuberuntime/logs"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/util/tail"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"

	"github.com/secretflow/kuscia/pkg/agent/utils/format"
)

var (
	// ErrCreateContainerConfig - failed to create container config
	ErrCreateContainerConfig = errors.New("CreateContainerConfigError")
	// ErrCreateContainer - failed to create container
	ErrCreateContainer = errors.New("CreateContainerError")
)

func CalcRestartCountByLogDir(path string) (int, error) {
	// if the path doesn't exist then it's not an error
	if _, err := os.Stat(path); err != nil {
		return 0, nil
	}
	restartCount := int(0)
	files, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, err
	}
	restartCountLogFileRegex := regexp.MustCompile(`(\d+).log(\..*)?`)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		matches := restartCountLogFileRegex.FindStringSubmatch(file.Name())
		if len(matches) == 0 {
			continue
		}
		count, err := strconv.Atoi(matches[1])
		if err != nil {
			return restartCount, err
		}
		count++
		if count > restartCount {
			restartCount = count
		}
	}
	return restartCount, nil
}

// makeUID returns a randomly generated string.
func makeUID() string {
	return fmt.Sprintf("%08x", rand.Uint32())
}

// recordContainerEvent should be used by the runtime manager for all container related events.
// it has sanity checks to ensure that we do not write events that can abuse our masters.
// in particular, it ensures that a containerID never appears in an event message as that
// is prone to causing a lot of distinct events that do not count well.
// it replaces any reference to a containerID with the containerName which is stable, and is what users know.
func (m *kubeGenericRuntimeManager) recordContainerEvent(pod *v1.Pod, container *v1.Container, containerID, eventType, reason, message string, args ...interface{}) {
	ref, err := pkgcontainer.GenerateContainerRef(pod, container)
	if err != nil {
		nlog.Errorf("Can't make a container ref, pod=%v, containerName=%v, detail-> %v", format.Pod(pod), container.Name, err)
		return
	}
	eventMessage := message
	if len(args) > 0 {
		eventMessage = fmt.Sprintf(message, args...)
	}
	// this is a hack, but often the error from the runtime includes the containerID
	// which kills our ability to deduplicate events.  this protection makes a huge
	// difference in the number of unique events
	if containerID != "" {
		eventMessage = strings.Replace(eventMessage, containerID, container.Name, -1)
	}
	m.recorder.Event(ref, eventType, reason, eventMessage)
}

// makeMounts generates container volume mounts for kubelet runtime v1.
func (m *kubeGenericRuntimeManager) makeMounts(opts *pkgcontainer.RunContainerOptions, container *v1.Container) []*runtimeapi.Mount {
	var volumeMounts []*runtimeapi.Mount

	for idx := range opts.Mounts {
		v := opts.Mounts[idx]
		selinuxRelabel := v.SELinuxRelabel && selinux.GetEnabled()
		mount := &runtimeapi.Mount{
			HostPath:       v.HostPath,
			ContainerPath:  v.ContainerPath,
			Readonly:       v.ReadOnly,
			SelinuxRelabel: selinuxRelabel,
			Propagation:    v.Propagation,
		}

		volumeMounts = append(volumeMounts, mount)
	}

	// The reason we create and mount the log file in here (not in kubelet) is because
	// the file's location depends on the ID of the container, and we need to create and
	// mount the file before actually starting the container.
	if m.agentRuntime == config.ContainerRuntime && opts.PodContainerDir != "" && len(container.TerminationMessagePath) != 0 {
		// Because the PodContainerDir contains pod uid and container name which is unique enough,
		// here we just add a random id to make the path unique for different instances
		// of the same container.
		cid := makeUID()
		containerLogPath := filepath.Join(opts.PodContainerDir, cid)
		fs, err := m.osInterface.Create(containerLogPath)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error on creating termination-log file [%s]: %v", containerLogPath, err))
		} else {
			fs.Close()

			// Chmod is needed because os.WriteFile() ends up calling
			// open(2) to create the file, so the final mode used is "mode &
			// ~umask". But we want to make sure the specified mode is used
			// in the file no matter what the umask is.
			if err := m.osInterface.Chmod(containerLogPath, 0666); err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to set termination-log file permissions [%s]: %v", containerLogPath, err))
			}

			// Volume Mounts fail on Windows if it is not of the form C:/
			containerLogPath = volumeutil.MakeAbsolutePath(goruntime.GOOS, containerLogPath)
			terminationMessagePath := volumeutil.MakeAbsolutePath(goruntime.GOOS, container.TerminationMessagePath)
			selinuxRelabel := selinux.GetEnabled()
			volumeMounts = append(volumeMounts, &runtimeapi.Mount{
				HostPath:       containerLogPath,
				ContainerPath:  terminationMessagePath,
				SelinuxRelabel: selinuxRelabel,
			})
		}
	}

	return volumeMounts
}

func (m *kubeGenericRuntimeManager) calculateContainerSecurityContext(context *v1.SecurityContext) *runtimeapi.LinuxContainerSecurityContext {
	if context == nil {
		return nil
	}
	privileged := m.calculatePrivileged(context.Privileged)
	if privileged == nil {
		return nil
	}
	return &runtimeapi.LinuxContainerSecurityContext{
		Privileged: *privileged,
	}
}

// generateLinuxContainerConfig generates linux container config for kubelet runtime v1.
func (m *kubeGenericRuntimeManager) generateLinuxContainerConfig(container *v1.Container, pod *v1.Pod) *runtimeapi.LinuxContainerConfig {
	lc := &runtimeapi.LinuxContainerConfig{
		Resources: &runtimeapi.LinuxContainerResources{},
	}

	// set linux container resources
	lc.Resources = m.calculateLinuxResources(container.Resources.Requests.Cpu(), container.Resources.Limits.Cpu(), container.Resources.Limits.Memory())
	// set security context
	lc.SecurityContext = m.calculateContainerSecurityContext(container.SecurityContext)

	// TODO calculate OomScoreAdj ...

	return lc
}

// generateContainerConfig generates container config for kubelet runtime v1.
func (m *kubeGenericRuntimeManager) generateContainerConfig(container *v1.Container, pod *v1.Pod, restartCount int, podIP, imageRef string, podIPs []string) (*runtimeapi.ContainerConfig, func(), error) {
	opts, cleanupAction, err := m.runtimeHelper.GenerateRunContainerOptions(pod, container, podIP, podIPs)
	if err != nil {
		return nil, nil, err
	}

	logDir := BuildContainerLogsDirectory(m.podStdoutRootDirectory, pod.Namespace, pod.Name, pod.UID, container.Name)
	err = m.osInterface.MkdirAll(logDir, 0755)
	if err != nil {
		return nil, cleanupAction, fmt.Errorf("create container log directory for container [%s] failed, detail-> %v", container.Name, err)
	}
	containerLogsPath := BuildContainerLogsPath(container.Name, restartCount)
	restartCountUint32 := uint32(restartCount)
	config := &runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{
			Name:    container.Name,
			Attempt: restartCountUint32,
		},
		Image:       &runtimeapi.ImageSpec{Image: imageRef},
		Command:     container.Command,
		Args:        container.Args,
		WorkingDir:  container.WorkingDir,
		Labels:      newContainerLabels(container, pod),
		Annotations: newContainerAnnotations(container, pod, restartCount, opts),
		Mounts:      m.makeMounts(opts, container),
		LogPath:     containerLogsPath,
		Stdin:       container.Stdin,
		StdinOnce:   container.StdinOnce,
		Tty:         container.TTY,
	}

	// set platform specific configurations.
	config.Linux = m.generateLinuxContainerConfig(container, pod)

	// set environment variables
	envs := make([]*runtimeapi.KeyValue, len(opts.Envs))
	for idx := range opts.Envs {
		e := opts.Envs[idx]
		envs[idx] = &runtimeapi.KeyValue{
			Key:   e.Name,
			Value: e.Value,
		}
	}
	config.Envs = envs

	return config, cleanupAction, nil
}

// startContainer starts a container and returns a message indicates why it is failed on error.
// It starts the container through the following steps:
// * pull the image
// * create the container
// * start the container
// * run the post start lifecycle hooks (if applicable)
func (m *kubeGenericRuntimeManager) startContainer(ctx context.Context, podSandboxID string, podSandboxConfig *runtimeapi.PodSandboxConfig, spec *startSpec, pod *v1.Pod, podStatus *pkgcontainer.PodStatus, auth *credentialprovider.AuthConfig, podIP string, podIPs []string) (string, error) {
	container := spec.container

	// Step 1: pull the image.
	imageRef, msg, err := m.imagePuller.EnsureImageExists(ctx, pod, container, auth, podSandboxConfig)
	if err != nil {
		s, _ := grpcstatus.FromError(err)
		m.recordContainerEvent(pod, container, "", v1.EventTypeWarning, events.FailedToCreateContainer, "Error: %v", s.Message())
		return msg, err
	}

	// Step 2: create the container.
	// For a new container, the RestartCount should be 0
	restartCount := 0
	containerStatus := podStatus.FindContainerStatusByName(container.Name)
	if containerStatus != nil {
		restartCount = containerStatus.RestartCount + 1
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
		logDir := BuildContainerLogsDirectory(m.podStdoutRootDirectory, pod.Namespace, pod.Name, pod.UID, container.Name)
		restartCount, err = CalcRestartCountByLogDir(logDir)
		if err != nil {
			nlog.Warnf("Log directory %q exists but could not calculate restartCount: %v", logDir, err)
		}
	}

	containerConfig, cleanupAction, err := m.generateContainerConfig(container, pod, restartCount, podIP, imageRef, podIPs)
	if cleanupAction != nil {
		defer cleanupAction()
	}
	if err != nil {
		s, _ := grpcstatus.FromError(err)
		m.recordContainerEvent(pod, container, "", v1.EventTypeWarning, events.FailedToCreateContainer, "Error: %v", s.Message())
		return s.Message(), ErrCreateContainerConfig
	}

	containerID, err := m.runtimeService.CreateContainer(ctx, podSandboxID, containerConfig, podSandboxConfig)
	if err != nil {
		s, _ := grpcstatus.FromError(err)
		m.recordContainerEvent(pod, container, containerID, v1.EventTypeWarning, events.FailedToCreateContainer, "Error: %v", s.Message())
		return s.Message(), ErrCreateContainer
	}
	m.recordContainerEvent(pod, container, containerID, v1.EventTypeNormal, events.CreatedContainer, fmt.Sprintf("Created container %s", container.Name))

	// Step 3: start the container.
	err = m.runtimeService.StartContainer(ctx, containerID)
	if err != nil {
		s, _ := grpcstatus.FromError(err)
		m.recordContainerEvent(pod, container, containerID, v1.EventTypeWarning, events.FailedToStartContainer, "Error: %v", s.Message())
		return s.Message(), pkgcontainer.ErrRunContainer
	}
	m.recordContainerEvent(pod, container, containerID, v1.EventTypeNormal, events.StartedContainer, fmt.Sprintf("Started container %s", container.Name))

	return "", nil
}

// restoreSpecsFromContainerLabels restores all information needed for killing a container. In some
// case we may not have pod and container spec when killing a container, e.g. pod is deleted during
// kubelet restart.
// To solve this problem, we've already written necessary information into container labels. Here we
// just need to retrieve them from container labels and restore the specs.
// just pass the needed function not create the fake object.
func (m *kubeGenericRuntimeManager) restoreSpecsFromContainerLabels(ctx context.Context, containerID pkgcontainer.CtrID) (*v1.Pod, *v1.Container, error) {
	var pod *v1.Pod
	var container *v1.Container
	resp, err := m.runtimeService.ContainerStatus(ctx, containerID.ID, false)
	if err != nil {
		return nil, nil, err
	}
	s := resp.GetStatus()
	if s == nil {
		return nil, nil, remote.ErrContainerStatusNil
	}

	l := getContainerInfoFromLabels(s.Labels)
	a := getContainerInfoFromAnnotations(s.Annotations)
	// Notice that the followings are not full spec. The container killing code should not use
	// un-restored fields.
	pod = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:                        l.PodUID,
			Name:                       l.PodName,
			Namespace:                  l.PodNamespace,
			DeletionGracePeriodSeconds: a.PodDeletionGracePeriod,
		},
		Spec: v1.PodSpec{
			TerminationGracePeriodSeconds: a.PodTerminationGracePeriod,
		},
	}
	container = &v1.Container{
		Name:                   l.ContainerName,
		Ports:                  a.ContainerPorts,
		TerminationMessagePath: a.TerminationMessagePath,
	}
	if a.PreStopHandler != nil {
		container.Lifecycle = &v1.Lifecycle{
			PreStop: a.PreStopHandler,
		}
	}
	return pod, container, nil
}

// killContainer kills a container
func (m *kubeGenericRuntimeManager) killContainer(ctx context.Context, pod *v1.Pod, containerID pkgcontainer.CtrID, containerName string, message string, reason containerKillReason, gracePeriodOverride *int64) error {
	var containerSpec *v1.Container
	if pod != nil {
		if containerSpec = pkgcontainer.GetContainerSpec(pod, containerName); containerSpec == nil {
			return fmt.Errorf("failed to get containerSpec %q (id=%q) in pod %q when killing container for reason %q",
				containerName, containerID.String(), format.Pod(pod), message)
		}
	} else {
		// Restore necessary information if one of the specs is nil.
		restoredPod, restoredContainer, err := m.restoreSpecsFromContainerLabels(ctx, containerID)
		if err != nil {
			return err
		}
		pod, containerSpec = restoredPod, restoredContainer
	}

	// From this point, pod and container must be non-nil.
	gracePeriod := int64(minimumGracePeriodInSeconds)
	switch {
	case pod.DeletionGracePeriodSeconds != nil:
		gracePeriod = *pod.DeletionGracePeriodSeconds
	case pod.Spec.TerminationGracePeriodSeconds != nil:
		gracePeriod = *pod.Spec.TerminationGracePeriodSeconds

		switch reason {
		case reasonStartupProbe:
			if containerSpec.StartupProbe != nil && containerSpec.StartupProbe.TerminationGracePeriodSeconds != nil {
				gracePeriod = *containerSpec.StartupProbe.TerminationGracePeriodSeconds
			}
		case reasonLivenessProbe:
			if containerSpec.LivenessProbe != nil && containerSpec.LivenessProbe.TerminationGracePeriodSeconds != nil {
				gracePeriod = *containerSpec.LivenessProbe.TerminationGracePeriodSeconds
			}
		}
	}

	if len(message) == 0 {
		message = fmt.Sprintf("Stopping container %s", containerSpec.Name)
	}
	m.recordContainerEvent(pod, containerSpec, containerID.ID, v1.EventTypeNormal, events.KillingContainer, message)

	// always give containers a minimal shutdown window to avoid unnecessary SIGKILLs
	if gracePeriod < minimumGracePeriodInSeconds {
		gracePeriod = minimumGracePeriodInSeconds
	}
	if gracePeriodOverride != nil {
		gracePeriod = *gracePeriodOverride
		nlog.Debugf("Killing container with a grace period override, pod=%v, containerName=%v, containerID=%v, gracePeriod=%v", format.Pod(pod), containerName, containerID.String(), gracePeriod)
	} else {
		nlog.Debugf("Killing container with a grace period, pod=%v, containerName=%v, containerID=%v, gracePeriod=%v", format.Pod(pod), containerName, containerID.String(), gracePeriod)
	}

	err := m.runtimeService.StopContainer(ctx, containerID.ID, gracePeriod)
	if err != nil && !crierror.IsNotFound(err) {
		return fmt.Errorf("container %q (id=%q) termination failed with gracePeriod %q",
			containerName, containerID.String(), gracePeriod)
	}
	nlog.Infof("Container exited normally, pod=%v, containerName=%v, containerID=%v", format.Pod(pod), containerName, containerID.String())

	return nil
}

func topkgcontainerStatus(status *runtimeapi.ContainerStatus, runtimeName string) *pkgcontainer.Status {
	annotatedInfo := getContainerInfoFromAnnotations(status.Annotations)
	labeledInfo := getContainerInfoFromLabels(status.Labels)
	cStatus := &pkgcontainer.Status{
		ID: pkgcontainer.CtrID{
			Type: runtimeName,
			ID:   status.Id,
		},
		Name:         labeledInfo.ContainerName,
		Image:        status.Image.Image,
		ImageID:      status.ImageRef,
		Hash:         annotatedInfo.Hash,
		RestartCount: annotatedInfo.RestartCount,
		State:        topkgcontainerState(status.State),
		CreatedAt:    time.Unix(0, status.CreatedAt),
	}

	if status.State != runtimeapi.ContainerState_CONTAINER_CREATED {
		// If container is not in the created state, we have tried and
		// started the container. Set the StartedAt time.
		cStatus.StartedAt = time.Unix(0, status.StartedAt)
	}
	if status.State == runtimeapi.ContainerState_CONTAINER_EXITED {
		cStatus.Reason = status.Reason
		cStatus.Message = status.Message
		cStatus.ExitCode = int(status.ExitCode)
		cStatus.FinishedAt = time.Unix(0, status.FinishedAt)
	}
	return cStatus
}

// getTerminationMessage looks on the filesystem for the provided termination message path, returning a limited
// amount of those bytes, or returns true if the logs should be checked.
func getTerminationMessage(status *runtimeapi.ContainerStatus, terminationMessagePath string, fallbackToLogs bool) (string, bool) {
	if len(terminationMessagePath) == 0 {
		return "", fallbackToLogs
	}
	// Volume Mounts fail on Windows if it is not of the form C:/
	terminationMessagePath = volumeutil.MakeAbsolutePath(goruntime.GOOS, terminationMessagePath)
	for _, mount := range status.Mounts {
		if mount.ContainerPath != terminationMessagePath {
			continue
		}
		path := mount.HostPath
		data, _, err := tail.ReadAtMost(path, pkgcontainer.MaxContainerTerminationMessageLength)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fallbackToLogs
			}
			return fmt.Sprintf("Error on reading termination log %s: %v", path, err), false
		}
		return string(data), fallbackToLogs && len(data) == 0
	}
	return "", fallbackToLogs
}

// ReadLogs read the container log and redirect into stdout and stderr.
// Note that containerID is only needed when following the log, or else
// just pass in empty string "".
func (m *kubeGenericRuntimeManager) ReadLogs(ctx context.Context, path, containerID string, apiOpts *v1.PodLogOptions, stdout, stderr io.Writer) error {
	// Convert v1.PodLogOptions into internal log options.
	opts := logs.NewLogOptions(apiOpts, time.Now())

	return logs.ReadLogs(ctx, path, containerID, opts, m.runtimeService, stdout, stderr)
}

// readLastStringFromContainerLogs attempts to read up to the max log length from the end of the CRI log represented
// by path. It reads up to max log lines.
func (m *kubeGenericRuntimeManager) readLastStringFromContainerLogs(path string) string {
	value := int64(pkgcontainer.MaxContainerTerminationMessageLogLines)
	buf, _ := circbuf.NewBuffer(pkgcontainer.MaxContainerTerminationMessageLogLength)
	if err := m.ReadLogs(context.Background(), path, "", &v1.PodLogOptions{TailLines: &value}, buf, buf); err != nil {
		nlog.Warnf("Error on reading termination message from logs: %v", err)
		return m.readGeneralLogs(path)
	}

	return buf.String()
}

func (m *kubeGenericRuntimeManager) readGeneralLogs(path string) string {
	notEmpty, fileSize := paths.CheckFileNotEmpty(path)
	if !notEmpty { // file is empty
		return ""
	}

	data, _, err := tail.ReadAtMost(path, pkgcontainer.MaxContainerTerminationMessageLogLength)
	if err != nil {
		message := fmt.Sprintf("Error on reading termination message from logs: %v", err)
		nlog.Warn(message)
		return message
	}

	startPos := len(data) - 1
	lines := 0
	for ; startPos >= 0 && lines < pkgcontainer.MaxContainerTerminationMessageLogLines; startPos-- {
		if data[startPos] == '\n' {
			lines++
		}
	}
	startPos++
	messageByte := int64(len(data) - startPos)
	if messageByte >= fileSize {
		return string(data[startPos:])
	}

	return fmt.Sprintf("... Ignore %v characters at the beginning ...\n", fileSize-messageByte) + string(data[startPos:])
}

// getPodContainerStatuses gets all containers' statuses for the pod.
func (m *kubeGenericRuntimeManager) getPodContainerStatuses(ctx context.Context, uid kubetypes.UID, name, namespace string) ([]*pkgcontainer.Status, error) {
	// Select all containers of the given pod.
	containers, err := m.runtimeService.ListContainers(ctx, &runtimeapi.ContainerFilter{
		LabelSelector: map[string]string{types.KubernetesPodUIDLabel: string(uid)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers, detail-> %v", err)
	}

	var statuses []*pkgcontainer.Status
	// TODO: optimization: set maximum number of containers per container name to examine.
	for _, c := range containers {
		resp, err := m.runtimeService.ContainerStatus(ctx, c.Id, false)
		// Between List (ListContainers) and check (ContainerStatus) another thread might remove a container, and that is normal.
		// The previous call (ListContainers) never fails due to a pod container not existing.
		// Therefore, this method should not either, but instead act as if the previous call failed,
		// which means the error should be ignored.
		if crierror.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get container %q status, detail-> %v", c.Id, err)
		}
		status := resp.GetStatus()
		if status == nil {
			return nil, remote.ErrContainerStatusNil
		}
		cStatus := topkgcontainerStatus(status, m.runtimeName)
		if status.State == runtimeapi.ContainerState_CONTAINER_EXITED {
			// Populate the termination message if needed.
			annotatedInfo := getContainerInfoFromAnnotations(status.Annotations)
			// If a container cannot even be started, it certainly does not have logs, so no need to fallbackToLogs.
			fallbackToLogs := annotatedInfo.TerminationMessagePolicy == v1.TerminationMessageFallbackToLogsOnError &&
				cStatus.ExitCode != 0 && cStatus.Reason != "ContainerCannotRun"
			tMessage, checkLogs := getTerminationMessage(status, annotatedInfo.TerminationMessagePath, fallbackToLogs)
			if checkLogs {
				tMessage = m.readLastStringFromContainerLogs(status.GetLogPath())
			}
			// Enrich the termination message written by the application is not empty
			if len(tMessage) != 0 {
				if len(cStatus.Message) != 0 {
					cStatus.Message += ": "
				}
				cStatus.Message += tMessage
			}
		}
		statuses = append(statuses, cStatus)
	}

	sort.Sort(containerStatusByCreated(statuses))
	return statuses, nil
}

// getContainers lists containers.
// The boolean parameter specifies whether returns all containers including
// those already exited and dead containers (used for garbage collection).
func (m *kubeGenericRuntimeManager) getContainers(ctx context.Context, allContainers bool) ([]*runtimeapi.Container, error) {
	filter := &runtimeapi.ContainerFilter{}
	if !allContainers {
		filter.State = &runtimeapi.ContainerStateValue{
			State: runtimeapi.ContainerState_CONTAINER_RUNNING,
		}
	}

	containers, err := m.runtimeService.ListContainers(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers, detail-> %v", err)
	}

	return containers, nil
}

// removeContainerLog removes the container log.
func (m *kubeGenericRuntimeManager) removeContainerLog(ctx context.Context, containerID string) error {
	// Use log manager to remove rotated logs.
	err := m.logManager.Clean(ctx, containerID)
	if err != nil {
		return err
	}

	resp, err := m.runtimeService.ContainerStatus(ctx, containerID, false)
	if err != nil {
		return fmt.Errorf("failed to get container status %q: %v", containerID, err)
	}
	status := resp.GetStatus()
	if status == nil {
		return remote.ErrContainerStatusNil
	}
	// Remove the legacy container log symlink.
	// TODO(random-liu): Remove this after cluster logging supports CRI container log path.
	labeledInfo := getContainerInfoFromLabels(status.Labels)
	legacySymlink := legacyLogSymlink(containerID, labeledInfo.ContainerName, labeledInfo.PodName,
		labeledInfo.PodNamespace)
	if err := m.osInterface.Remove(legacySymlink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove container %q log legacy symbolic link %q: %v",
			containerID, legacySymlink, err)
	}
	return nil
}

// removeContainer removes the container and the container logs.
// Notice that we remove the container logs first, so that container will not be removed if
// container logs are failed to be removed, and kubelet will retry this later. This guarantees
// that container logs to be removed with the container.
// Notice that we assume that the container should only be removed in non-running state, and
// it will not write container logs anymore in that state.
func (m *kubeGenericRuntimeManager) removeContainer(ctx context.Context, containerID string) error {
	nlog.Debugf("Removing container %q", containerID)

	// Keep container logs in kuscia.
	// Remove the container log.
	// TODO: Separate log and container lifecycle management.
	//if err := m.removeContainerLog(containerID); err != nil {
	//	return err
	//}

	// Remove the container.
	return m.runtimeService.RemoveContainer(ctx, containerID)
}

// pruneInitContainersBeforeStart ensures that before we begin creating init
// containers, we have reduced the number of outstanding init containers still
// present. This reduces load on the container garbage collector by only
// preserving the most recent terminated init container.
func (m *kubeGenericRuntimeManager) pruneInitContainersBeforeStart(ctx context.Context, pod *v1.Pod, podStatus *pkgcontainer.PodStatus) {
	// only the last execution of each init container should be preserved, and only preserve it if it is in the
	// list of init containers to keep.
	initContainerNames := sets.NewString()
	for _, container := range pod.Spec.InitContainers {
		initContainerNames.Insert(container.Name)
	}
	for name := range initContainerNames {
		count := 0
		for _, status := range podStatus.ContainerStatuses {
			if status.Name != name ||
				(status.State != pkgcontainer.ContainerStateExited &&
					status.State != pkgcontainer.ContainerStateUnknown) {
				continue
			}
			// Remove init containers in unknown state. It should have
			// been stopped before pruneInitContainersBeforeStart is
			// called.
			count++
			// keep the first init container for this name
			if count == 1 {
				continue
			}
			// prune all other init containers that match this container name
			nlog.Debugf("Removing init container, containerName=%v, containerID=%v, count=%v", status.Name, status.ID.ID, count)
			if err := m.removeContainer(ctx, status.ID.ID); err != nil {
				nlog.Errorf("Failed to remove pod init container %q: %v; Skipping pod %q", status.Name, err, format.Pod(pod))
				continue
			}
		}
	}
}

// Remove all init containers. Note that this function does not check the state
// of the container because it assumes all init containers have been stopped
// before the call happens.
func (m *kubeGenericRuntimeManager) purgeInitContainers(ctx context.Context, pod *v1.Pod, podStatus *pkgcontainer.PodStatus) {
	initContainerNames := sets.NewString()
	for _, container := range pod.Spec.InitContainers {
		initContainerNames.Insert(container.Name)
	}
	for name := range initContainerNames {
		count := 0
		for _, status := range podStatus.ContainerStatuses {
			if status.Name != name {
				continue
			}
			count++
			// Purge all init containers that match this container name
			nlog.Debugf("Removing init container, containerName=%v, containerID=%v, count=%v", status.Name, status.ID.ID, count)
			if err := m.removeContainer(ctx, status.ID.ID); err != nil {
				utilruntime.HandleError(fmt.Errorf("failed to remove pod init container %q: %v; Skipping pod %q", status.Name, err, format.Pod(pod)))
				continue
			}
		}
	}
}

// findNextInitContainerToRun returns the status of the last failed container, the
// index of next init container to start, or done if there are no further init containers.
// Status is only returned if an init container is failed, in which case next will
// point to the current container.
func findNextInitContainerToRun(pod *v1.Pod, podStatus *pkgcontainer.PodStatus) (status *pkgcontainer.Status, next *v1.Container, done bool) {
	if len(pod.Spec.InitContainers) == 0 {
		return nil, nil, true
	}

	// If any of the main containers have status and are Running, then all init containers must
	// have been executed at some point in the past.  However, they could have been removed
	// from the container runtime now, and if we proceed, it would appear as if they
	// never ran and will re-execute improperly.
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		status := podStatus.FindContainerStatusByName(container.Name)
		if status != nil && status.State == pkgcontainer.ContainerStateRunning {
			return nil, nil, true
		}
	}

	// If there are failed containers, return the status of the last failed one.
	for i := len(pod.Spec.InitContainers) - 1; i >= 0; i-- {
		container := &pod.Spec.InitContainers[i]
		status := podStatus.FindContainerStatusByName(container.Name)
		if status != nil && isInitContainerFailed(status) {
			return status, container, false
		}
	}

	// There are no failed containers now.
	for i := len(pod.Spec.InitContainers) - 1; i >= 0; i-- {
		container := &pod.Spec.InitContainers[i]
		status := podStatus.FindContainerStatusByName(container.Name)
		if status == nil {
			continue
		}

		// container is still running, return not done.
		if status.State == pkgcontainer.ContainerStateRunning {
			return nil, nil, false
		}

		if status.State == pkgcontainer.ContainerStateExited {
			// all init containers successful
			if i == (len(pod.Spec.InitContainers) - 1) {
				return nil, nil, true
			}

			// all containers up to i successful, go to i+1
			return nil, &pod.Spec.InitContainers[i+1], false
		}
	}

	return nil, &pod.Spec.InitContainers[0], false
}
