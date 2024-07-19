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

package container

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"

	st "github.com/secretflow/kuscia/pkg/agent/local/runtime/process/container/starter"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/errdefs"
	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/utils/cgroup"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/process"
	runenv "github.com/secretflow/kuscia/pkg/utils/runtime"
)

const (
	// errorStartReason is the exit reason when fails to start container.
	errorStartReason = "StartError"
	// errorStartExitCode is the exit code when fails to start container.
	// 128 is the same with Docker's behavior.
	errorStartExitCode = 128

	completeExitReason = "Completed"
	// errorExitReason is the exit reason when container exits with code non-zero.
	errorExitReason = "Error"

	defaultMaxLogFileSizeMB = 512
	defaultMaxLogAgeInDays  = 15
	defaultMaxLogFiles      = 10

	resolvConfPath = "/etc/resolv.conf"
)

type Metadata struct {
	// ID is the container id.
	ID string
	// Name is the container name.
	Name string
	// SandboxID is the sandbox id the container belongs to.
	SandboxID string
	// Config is the CRI container config.
	// NOTE(random-liu): Resource limits are updatable, the source
	// of truth for resource limits are in containerd.
	Config *runtime.ContainerConfig
	// ImageManifest is the manifest of image used by the container.
	ImageManifest *kii.Manifest
	// LogPath is the container log path.
	LogPath string
	// StopSignal is the system call signal that will be sent to the container to exit.
	StopSignal string
	// RooDirt is the root directory of the container
	RootDir string
}

// Container contains all resources associated with the container. All methods to
// mutate the internal state are thread-safe.
type Container struct {
	// Metadata is the metadata of the container, it is **immutable** after created.
	Metadata
	bundle *layout.ContainerBundle
	status Status
	sync.RWMutex
	imageStore store.Store
	imageName  *kii.ImageName
	starter    st.Starter
}

// Opts sets specific information to newly created Container.
type Opts func(*Container) error

// NewContainer creates an internally used container type.
func NewContainer(config *runtime.ContainerConfig, logDirectory, sandboxID string, sandboxBundle *layout.Bundle, imageStore store.Store) (*Container, error) {
	metadata := config.GetMetadata()
	if metadata == nil {
		return nil, errors.New("container config must include metadata")
	}

	cid := common.GenerateID(16)
	name := makeContainerName(metadata, cid)

	imageName, err := kii.NewImageName(config.Image.Image)
	if err != nil {
		return nil, err
	}
	manifest, err := imageStore.GetImageManifest(imageName)
	if err != nil {
		return nil, err
	}

	containerBundle := sandboxBundle.GetContainerBundle(name)

	c := &Container{
		Metadata: Metadata{
			ID:            cid,
			Name:          name,
			SandboxID:     sandboxID,
			Config:        config,
			ImageManifest: manifest,
			LogPath:       filepath.Join(logDirectory, config.LogPath),
			StopSignal:    manifest.Config.StopSignal,
		},
		bundle:     containerBundle,
		imageStore: imageStore,
		imageName:  imageName,
	}

	return c, nil
}

// makeContainerName generates container name from sandbox and container metadata.
// The name generated is unique as long as the sandbox container combination is
// unique.
func makeContainerName(c *runtime.ContainerMetadata, containerID string) string {
	return strings.Join([]string{
		c.Name,                       // 0: container name
		containerID,                  // 1: container uid
		fmt.Sprintf("%d", c.Attempt), // 2: attempt number of creating the container
	}, "_")
}

func (c *Container) Create(mountType kii.MountType) error {
	c.Lock()
	defer c.Unlock()

	if err := c.imageStore.MountImage(c.imageName, mountType, c.bundle.GetFsWorkingDirPath(), c.bundle.GetOciRootfsPath()); err != nil {
		return err
	}

	if err := paths.CopyFile(resolvConfPath, filepath.Join(c.bundle.GetOciRootfsPath(), resolvConfPath)); err != nil {
		return fmt.Errorf("failed to copy resolv.conf, detail-> %v", err)
	}

	c.status.CreatedAt = time.Now().UnixNano()

	return nil
}

func (c *Container) Start() (retErr error) {
	c.Lock()
	defer c.Unlock()

	if err := c.canStart(); err != nil {
		return fmt.Errorf("container %q can not to be started: %v", c.ID, err)
	}
	defer func() {
		if retErr != nil {
			c.status.Pid = 0
			c.status.FinishedAt = time.Now().UnixNano()
			c.status.ExitCode = errorStartExitCode
			c.status.Reason = errorStartReason
			c.status.Message = retErr.Error()
		}
	}()

	starter, err := c.buildStarter()
	if err != nil {
		return fmt.Errorf("failed to build starter for container %q, detail-> %v", c.ID, err)
	}
	c.starter = starter

	if err := starter.Start(); err != nil {
		return fmt.Errorf("failed to start container %q, detail-> %v", c.ID, err)
	}

	c.status.StartedAt = time.Now().UnixNano()
	c.status.Pid = starter.Command().Process.Pid

	c.addCgroup(c.status.Pid)

	if runenv.Permission.HasSetOOMScorePermission() {
		process.SetOOMScore(c.status.Pid, 0)
	}

	go c.signalOnExit(starter)

	return nil
}

func (c *Container) buildStarter() (st.Starter, error) {
	stdMode := c.ImageManifest.Type == kii.ImageTypeStandard

	cmdLine := c.generateOriginalCmdLine()
	if len(cmdLine) == 0 {
		return nil, fmt.Errorf("container %q has no startup command or args", c.ID)
	}

	env := c.generateProcessEnv()

	if err := paths.EnsureFile(c.LogPath, true); err != nil {
		return nil, err
	}

	logFile := &lumberjack.Logger{
		Filename:   c.LogPath,
		MaxSize:    defaultMaxLogFileSizeMB,
		MaxBackups: defaultMaxLogFiles,
		MaxAge:     defaultMaxLogAgeInDays,
		LocalTime:  true,
	}

	workingDir := c.Config.WorkingDir
	if stdMode && workingDir == "" {
		workingDir = filepath.Join("/", c.ImageManifest.Config.WorkingDir)
	}

	initConfig := &st.InitConfig{
		CmdLine:         cmdLine,
		Env:             env,
		ContainerConfig: c.Config,
		Rootfs:          c.bundle.GetOciRootfsPath(),
		WorkingDir:      workingDir,
		LogFile:         logFile,
	}

	if stdMode {
		return st.NewProotStarter(initConfig)
	}

	return st.NewRawStarter(initConfig)
}

func (c *Container) addCgroup(pid int) {
	if !cgroup.HasPermission() || pid <= 0 {
		return
	}

	var (
		cpuQuota    *int64
		cpuPeriod   *uint64
		memoryLimit *int64
	)

	if c.Config != nil && c.Config.Linux != nil && c.Config.Linux.Resources != nil {
		if c.Config.Linux.Resources.CpuQuota > 0 {
			cpuQuota = &c.Config.Linux.Resources.CpuQuota
		}
		if c.Config.Linux.Resources.CpuPeriod > 0 {
			period := uint64(c.Config.Linux.Resources.CpuPeriod)
			cpuPeriod = &period
		}
		if c.Config.Linux.Resources.MemoryLimitInBytes > 0 {
			memoryLimit = &c.Config.Linux.Resources.MemoryLimitInBytes
		}
	}

	cgroupConfig := &cgroup.Config{
		Group:       fmt.Sprintf("%s/%s", cgroup.KusciaAppsGroup, c.ID),
		Pid:         uint64(pid),
		CPUQuota:    cpuQuota,
		CPUPeriod:   cpuPeriod,
		MemoryLimit: memoryLimit,
	}
	m, err := cgroup.NewManager(cgroupConfig)
	if err != nil {
		nlog.Warnf("New cgroup manager for container[%v] process[%v] failed, details -> %v, skip adding process into cgroup", c.Name, pid, err)
		return
	}

	if err = m.AddCgroup(); err != nil {
		nlog.Warnf("Add cgroup for container[%v] process[%v] failed, details -> %v, skip adding process into cgroup", c.Name, pid, err)
	}
}

func (c *Container) deleteCgroup(pid int) {
	if !cgroup.HasPermission() || pid <= 0 {
		return
	}

	cgroupConfig := &cgroup.Config{
		Group: fmt.Sprintf("%s/%s", cgroup.KusciaAppsGroup, c.ID),
		Pid:   uint64(pid),
	}
	m, err := cgroup.NewManager(cgroupConfig)
	if err != nil {
		nlog.Warnf("New cgroup manager for container[%v] process[%v] failed, details -> %v, skip deleting process from cgroup", c.Name, pid, err)
		return
	}

	for i := 0; i < 5; i++ {
		err = m.DeleteCgroup()
		if err == nil {
			return
		}
		nlog.Warnf("Delete cgroup for container[%v] process[%v] failed, details -> %v, max retry count[5], current retry count[%v]", c.Name, pid, err, i+1)
		time.Sleep(2 * time.Second)
	}
}

// The final execution command follows the following rules：
//  1. If you do not supply command or args for a Container, the defaults defined in the
//     Docker image are used.
//  2. If you supply a command but no args for a Container, only the supplied command is used.
//     The default EntryPoint and the default Cmd defined in the Docker image are ignored.
//  3. If you supply only args for a Container, the default Entrypoint defined in the Docker image
//     is run with the args that you supplied.
//  4. If you supply a command and args, the default Entrypoint and the default Cmd defined in
//     the Docker image are ignored. Your command is run with your args.
//
// Example:
// +------------------+-----------+-------------------+----------------+----------------+
// | Image Entrypoint | Image Cmd | Container command | Container args | Command run    |
// +------------------+-----------+-------------------+----------------+----------------+
// | [/ep-1]          | [foo bar] | <not set>         | <not set>      | [ep-1 foo bar] |
// +------------------+-----------+-------------------+----------------+----------------+
// | [/ep-1]          | [foo bar] | [/ep-2]           | <not set>      | [ep-2]         |
// +------------------+-----------+-------------------+----------------+----------------+
// | [/ep-1]          | [foo bar] | <not set>         | [zoo boo]      | [ep-1 zoo boo] |
// +------------------+-----------+-------------------+----------------+----------------+
// | [/ep-1]          | [foo bar] | [/ep-2]           | [zoo boo]      | [ep-2 zoo boo] |
// +------------------+-----------+-------------------+----------------+----------------+
func (c *Container) generateOriginalCmdLine() []string {
	var result []string

	if len(c.Config.Command) > 0 {
		result = append(result, c.Config.Command...)
		return append(result, c.Config.Args...)
	}

	// container command not set
	result = append(result, c.ImageManifest.Config.Entrypoint...)
	if len(c.Config.Args) > 0 {
		return append(result, c.Config.Args...)
	}

	return append(result, c.ImageManifest.Config.Cmd...)
}

func (c *Container) generateProcessEnv() []string {
	// priority 1 (lowest): inject os env
	envs := os.Environ()

	// priority 2: inject image env
	envs = append(envs, c.ImageManifest.Config.Env...)

	// priority 3 (highest）: inject pod env
	for _, item := range c.Config.Envs {
		envs = append(envs, fmt.Sprintf("%v=%v", strings.ToUpper(item.Key), item.Value))
	}
	return envs
}

func (c *Container) signalOnExit(starter st.Starter) {
	cmdErr := starter.Wait()
	nlog.Infof("Container %q got process (%d) exit signal, err=%v", c.ID, starter.Command().Process.Pid, cmdErr)

	c.Lock()
	defer c.Unlock()

	// if the main thread exit, kill all other process in same container
	if c.status.Pid > 0 {
		if err := syscall.Kill(-c.status.Pid, syscall.SIGKILL); err != nil {
			if !errors.Is(err, syscall.ESRCH) {
				nlog.Errorf("[container:%s] Failed to kill process group %d: %v", c.ID, c.status.Pid, err)
			}
			// all process already exit
		}
	}

	go c.deleteCgroup(c.status.Pid)

	if c.status.FinishedAt == 0 {
		c.status.Pid = 0
		c.status.FinishedAt = time.Now().UnixNano()
		c.status.ExitCode = starter.Command().ProcessState.ExitCode()
	}

	nlog.Infof("Container %q exited, state=%v", c.ID, starter.Command().ProcessState.String())
}

func (c *Container) canStart() error {
	// Return error if container is not in created state.
	if c.status.State() != runtime.ContainerState_CONTAINER_CREATED {
		return fmt.Errorf("container is in %s state", criContainerStateToString(c.status.State()))
	}

	return nil
}

func (c *Container) Stop() error {
	c.Lock()
	defer c.Unlock()

	if c.status.State() != runtime.ContainerState_CONTAINER_RUNNING {
		nlog.Infof("Container %q is in %s state, skip stopping", c.ID, criContainerStateToString(c.status.State()))
		return nil
	}

	if c.status.Pid > 0 {
		nlog.Infof("Killing container %v, pgid=%v", c.ID, c.status.Pid)

		if err := syscall.Kill(-c.status.Pid, syscall.SIGKILL); err != nil {
			if !errors.Is(err, syscall.ESRCH) {
				return fmt.Errorf("failed to kill process group %v: %v", c.status.Pid, err)
			}
		}
	} else {
		c.status.FinishedAt = time.Now().UnixNano()
	}

	return nil
}

func (c *Container) Release() error {
	c.Lock()
	defer c.Unlock()

	if err := c.canRelease(); err != nil {
		nlog.Errorf("Container %q can not to be released: %v", c.ID, err)
		return errdefs.ErrCanNotRemove
	}

	if c.starter != nil {
		if err := c.starter.Release(); err != nil {
			return fmt.Errorf("failed to release starter for container %q, detail-> %v", c.ID, err)
		}
	}

	if err := c.imageStore.UmountImage(c.bundle.GetFsWorkingDirPath()); err != nil {
		return fmt.Errorf("failed to umount image %q for container %q, detail-> %v", c.Config.Image.Image, c.ID, err)
	}

	if err := os.RemoveAll(c.bundle.GetRootDirectory()); err != nil {
		return fmt.Errorf("failed to remove container %q sandbox root directory, detail-> %v", c.ID, err)
	}

	c.status.Released = true

	return nil
}

func (c *Container) canRelease() error {
	// Do not release container if it's still running or unknown.
	if c.status.State() == runtime.ContainerState_CONTAINER_RUNNING {
		return errors.New("container is still running, to stop first")
	}
	if c.status.Released {
		return errors.New("container is already released")
	}
	return nil
}

func (c *Container) GetCRIStatus() *runtime.ContainerStatus {
	c.RLock()
	defer c.RUnlock()

	spec := c.Config.Image
	imageRef := c.ImageManifest.ID

	status := toCRIContainerStatus(c, spec, imageRef)

	return status
}

func (c *Container) GetStatus() Status {
	c.RLock()
	defer c.RUnlock()

	return c.status
}

// toCRIContainerStatus converts internal container object to CRI container status.
func toCRIContainerStatus(container *Container, spec *runtime.ImageSpec, imageRef string) *runtime.ContainerStatus {
	meta := container.Metadata
	status := container.status
	reason := status.Reason
	if status.State() == runtime.ContainerState_CONTAINER_EXITED && reason == "" {
		if status.ExitCode == 0 {
			reason = completeExitReason
		} else {
			reason = errorExitReason
		}
	}

	// If container is in the created state, not set started and finished unix timestamps
	var st, ft int64
	switch status.State() {
	case runtime.ContainerState_CONTAINER_RUNNING:
		// If container is in the running state, set started unix timestamps
		st = status.StartedAt
	case runtime.ContainerState_CONTAINER_EXITED, runtime.ContainerState_CONTAINER_UNKNOWN:
		st, ft = status.StartedAt, status.FinishedAt
	}

	return &runtime.ContainerStatus{
		Id:          meta.ID,
		Metadata:    meta.Config.GetMetadata(),
		State:       status.State(),
		CreatedAt:   status.CreatedAt,
		StartedAt:   st,
		FinishedAt:  ft,
		ExitCode:    int32(status.ExitCode),
		Image:       spec,
		ImageRef:    imageRef,
		Reason:      reason,
		Message:     status.Message,
		Labels:      meta.Config.GetLabels(),
		Annotations: meta.Config.GetAnnotations(),
		Mounts:      meta.Config.GetMounts(),
		LogPath:     meta.LogPath,
		Resources:   status.Resources,
	}
}

// criContainerStateToString formats CRI container state to string.
func criContainerStateToString(state runtime.ContainerState) string {
	return runtime.ContainerState_name[int32(state)]
}
