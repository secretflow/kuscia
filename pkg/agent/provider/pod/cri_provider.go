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

package pod

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	remote "k8s.io/cri-client/pkg"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/kubelet/logs"
	"k8s.io/kubernetes/pkg/kubelet/network/dns"
	"k8s.io/kubernetes/pkg/kubelet/util"
	"k8s.io/kubernetes/pkg/volume/validation"
	"k8s.io/utils/clock"
	netutils "k8s.io/utils/net"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/framework"
	"github.com/secretflow/kuscia/pkg/agent/framework/net"
	"github.com/secretflow/kuscia/pkg/agent/images"
	"github.com/secretflow/kuscia/pkg/agent/kri"
	"github.com/secretflow/kuscia/pkg/agent/kuberuntime"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/pleg"
	"github.com/secretflow/kuscia/pkg/agent/prober"
	proberesults "github.com/secretflow/kuscia/pkg/agent/prober/results"
	"github.com/secretflow/kuscia/pkg/agent/resource"
	"github.com/secretflow/kuscia/pkg/agent/status"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	// backOffPeriod is the period to back off when pod syncing results in an
	// error. It is also used as the base period for the exponential backoff
	// container restarts and image pulls.
	backOffPeriod = time.Second * 10

	// maxContainerBackOff is the max backoff period, exported for the e2e test
	maxContainerBackOff = 300 * time.Second

	plegChannelCapacity = 1000
	plegRelistPeriod    = time.Second * 1

	// ContainerGCPeriod is the period for performing container garbage collection.
	ContainerGCPeriod = time.Minute

	defaultVariableDirName = "var"
	defaultPodsDirName     = "pods"

	defaultVolumesDirName    = "volumes"
	defaultContainersDirName = "containers"
	defaultStorageDirName    = "storage"
)

// ProviderConfig is the config passed to initialize a registered provider.
type CRIProviderDependence struct {
	Namespace       string
	NodeIP          string
	RootDirectory   string
	StdoutDirectory string
	AllowPrivileged bool

	NodeName        string
	EventRecorder   record.EventRecorder
	ResourceManager *resource.KubeResourceManager

	PodStateProvider framework.PodStateProvider
	PodSyncHandler   framework.SyncHandler
	StatusManager    status.Manager

	Runtime        string
	CRIProviderCfg *config.CRIProviderCfg
	RegistryCfg    *config.RegistryCfg
	CpuAvailable   k8sresource.Quantity
	MemAvailable   k8sresource.Quantity
}

// CRIProvider implements the kubelet interface and stores pods in memory.
type CRIProvider struct {
	internalapi.ImageManagerService

	// static info
	nodeIPs        []net.IP
	ns             string
	rootDirectory  string
	registryConfig *config.RegistryCfg

	// k8s resource
	eventRecorder   record.EventRecorder
	resourceManager *resource.KubeResourceManager

	podCache pkgcontainer.Cache

	containerRuntime pkgcontainer.Runtime

	backOff *flowcontrol.Backoff

	statusManager status.Manager

	pleg pleg.PodLifecycleEventGenerator

	// dnsConfigurer is used for setting up DNS resolver configuration when launching pods.
	dnsConfigurer *dns.Configurer

	volumeManager *resource.VolumeManager
	// Handles container probing.
	probeManager prober.Manager
	// Manages container health check results.
	livenessManager  proberesults.Manager
	readinessManager proberesults.Manager
	startupManager   proberesults.Manager

	runtimeCache pkgcontainer.RuntimeCache
	// Manager for container logs.
	containerLogManager logs.ContainerLogManager

	podStateProvider framework.PodStateProvider

	podSyncHandler framework.SyncHandler

	chStopping   chan struct{}
	chStopped    chan struct{}
	cpuAvailable k8sresource.Quantity
	memAvailable k8sresource.Quantity
}

// NewCRIProvider creates a new CRIProvider, which implements the PodNotifier interface
func NewCRIProvider(dep *CRIProviderDependence) (kri.PodProvider, error) {
	realOS := pkgcontainer.RealOS{}

	cp := &CRIProvider{
		nodeIPs:        []net.IP{net.IP(dep.NodeIP)},
		ns:             dep.Namespace,
		registryConfig: dep.RegistryCfg,
		rootDirectory:  dep.RootDirectory,

		eventRecorder:   dep.EventRecorder,
		resourceManager: dep.ResourceManager,

		podStateProvider: dep.PodStateProvider,
		podSyncHandler:   dep.PodSyncHandler,
		statusManager:    dep.StatusManager,
		cpuAvailable:     dep.CpuAvailable,
		memAvailable:     dep.MemAvailable,

		chStopping: make(chan struct{}),
		chStopped:  make(chan struct{}),

		podCache: pkgcontainer.NewCache(),
		backOff:  flowcontrol.NewBackOff(backOffPeriod, maxContainerBackOff),
	}

	cp.rootDirectory = dep.RootDirectory

	imageBackOff := flowcontrol.NewBackOff(backOffPeriod, maxContainerBackOff)

	var (
		remoteRuntimeService internalapi.RuntimeService
		remoteImageService   internalapi.ImageManagerService
		err                  error
	)

	switch dep.Runtime {
	case config.ContainerRuntime:
		remoteRuntimeService, err = remote.NewRemoteRuntimeService(dep.CRIProviderCfg.RemoteRuntimeEndpoint, dep.CRIProviderCfg.RuntimeRequestTimeout, nil, nil)
		if err != nil {
			return nil, err
		}
		remoteImageService, err = remote.NewRemoteImageService(dep.CRIProviderCfg.RemoteImageEndpoint,
			dep.CRIProviderCfg.RuntimeRequestTimeout, nil, nil)
		if err != nil {
			return nil, err
		}
	case config.ProcessRuntime:
		processRuntimeDep := &process.RuntimeDependence{
			HostIP:         dep.NodeIP,
			ImageRootDir:   dep.CRIProviderCfg.LocalRuntime.ImageRootDir,
			SandboxRootDir: dep.CRIProviderCfg.LocalRuntime.SandboxRootDir,
		}
		if !filepath.IsAbs(processRuntimeDep.ImageRootDir) {
			processRuntimeDep.ImageRootDir = path.Join(dep.RootDirectory, processRuntimeDep.ImageRootDir)
		}
		if !filepath.IsAbs(processRuntimeDep.SandboxRootDir) {
			processRuntimeDep.SandboxRootDir = path.Join(dep.RootDirectory, processRuntimeDep.SandboxRootDir)
		}
		processRuntime, newErr := process.NewRuntime(processRuntimeDep)
		if newErr != nil {
			return nil, newErr
		}
		remoteRuntimeService = processRuntime
		remoteImageService = processRuntime
	default:
		return nil, fmt.Errorf("unknown runtime: %s", dep.Runtime)
	}

	cp.ImageManagerService = remoteImageService

	// setup containerLogManager for CRI container runtime
	containerLogManager, err := logs.NewContainerLogManager(
		remoteRuntimeService,
		realOS,
		dep.CRIProviderCfg.ContainerLogMaxSize,
		dep.CRIProviderCfg.ContainerLogMaxFiles,
		1,
		metav1.Duration{Duration: 10 * time.Second},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize container log manager: %v", err)
	}
	cp.containerLogManager = containerLogManager

	cp.livenessManager = proberesults.NewManager()
	cp.readinessManager = proberesults.NewManager()
	cp.startupManager = proberesults.NewManager()
	cp.probeManager = prober.NewManager(cp.statusManager, cp.livenessManager, cp.readinessManager, cp.startupManager, cp.eventRecorder)

	podsStdoutDirectory := filepath.Join(dep.StdoutDirectory, defaultPodsDirName)
	cp.containerRuntime, err = kuberuntime.NewManager(
		dep.EventRecorder,
		cp.livenessManager,
		cp.readinessManager,
		cp.startupManager,
		dep.PodStateProvider,
		realOS,
		containerLogManager,
		cp,
		remoteRuntimeService,
		remoteImageService,
		imageBackOff,
		false,
		0,
		0,
		true,
		podsStdoutDirectory,
		dep.AllowPrivileged,
		dep.Runtime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kuberuntime manager")
	}

	cp.pleg = pleg.NewGenericPLEG(cp.containerRuntime, plegChannelCapacity, plegRelistPeriod, cp.podCache, clock.RealClock{})

	cp.volumeManager = resource.NewVolumeManager(cp.resourceManager, cp)

	// construct a node reference used for events
	nodeRef := &v1.ObjectReference{
		Kind:      "Node",
		Name:      dep.NodeName,
		UID:       types.UID(dep.NodeName),
		Namespace: dep.Namespace,
	}

	clusterDNS := make([]net.IP, 0, len(dep.CRIProviderCfg.ClusterDNS))
	for _, ipEntry := range dep.CRIProviderCfg.ClusterDNS {
		ip := netutils.ParseIPSloppy(ipEntry)
		if ip == nil {
			nlog.Errorf("Invalid clusterDNS IP: %+v", ipEntry)
		} else {
			clusterDNS = append(clusterDNS, ip)
		}
	}
	cp.dnsConfigurer = dns.NewConfigurer(
		dep.EventRecorder,
		nodeRef,
		cp.nodeIPs,
		clusterDNS,
		dep.CRIProviderCfg.ClusterDomain,
		dep.CRIProviderCfg.ResolverConfig)

	cp.runtimeCache, err = pkgcontainer.NewRuntimeCache(cp.containerRuntime)
	if err != nil {
		return nil, err
	}

	if err = paths.EnsurePath(cp.getPodsDir(), true); err != nil {
		return nil, err
	}
	if err = paths.EnsurePath(podsStdoutDirectory, true); err != nil {
		return nil, err
	}

	return cp, nil
}

// Start cri provider
func (cp *CRIProvider) Start(ctx context.Context) error {
	nlog.Info("Starting CRI provider ...")

	cp.pleg.Start(ctx.Done())

	cp.startGarbageCollection()

	cp.containerLogManager.Start()

	cp.syncLoopIteration()

	close(cp.chStopped)

	return nil
}

// Stop cri provider
func (cp *CRIProvider) Stop() {
	start := time.Now()

	close(cp.chStopping)

	<-cp.chStopped
	nlog.Infof("CRIProvider stopped, time elapsed for stopping: %v", time.Now().Sub(start))
}

// syncLoopIteration reads from various channels and dispatches pods to the
// given handler.
//
// The different channels are handled as follows:
//
//   - plegCh: update the runtime cache; sync pod
//   - health manager: sync pods that have failed or in which one or more
//     containers have failed health checks
func (cp *CRIProvider) syncLoopIteration() {
	for {
		select {
		case <-cp.chStopping:
			return
		case e := <-cp.pleg.Watch():
			// PLEG event for a pod; sync it.
			nlog.Infof("Receive pleg event: %+v", e)

			if e.Type != pleg.ContainerRemoved {
				cp.podSyncHandler.HandlePodSyncByUID(e.ID)
			}
		case update := <-cp.livenessManager.Updates():
			nlog.Infof("Receive liveness manager event: %+v", update)

			if update.Result == proberesults.Failure {
				cp.podSyncHandler.HandlePodSyncByUID(update.PodUID)
			}
		case update := <-cp.readinessManager.Updates():
			nlog.Infof("Receive readiness manager event: %+v", update)

			ready := update.Result == proberesults.Success
			cp.statusManager.SetContainerReadiness(update.PodUID, update.ContainerID, ready)

			cp.podSyncHandler.HandlePodSyncByUID(update.PodUID)
		case update := <-cp.startupManager.Updates():
			nlog.Infof("Receive startup manager event: %+v", update)

			started := update.Result == proberesults.Success
			cp.statusManager.SetContainerStartup(update.PodUID, update.ContainerID, started)

			cp.podSyncHandler.HandlePodSyncByUID(update.PodUID)
		}
	}
}

// getRootDir returns the full path to the directory under which kubelet can
// store data.  These functions are useful to pass interfaces to other modules
// that may need to know where to write data without getting a whole kubelet
// instance.
func (cp *CRIProvider) getRootDir() string {
	return cp.rootDirectory
}

// getPodsDir returns the full path to the directory under which pod
// directories are created.
func (cp *CRIProvider) getPodsDir() string {
	return filepath.Join(cp.getRootDir(), defaultVariableDirName, defaultPodsDirName)
}

// getPodDir returns the full path to the per-pod directory for the pod with
// the given UID.
func (cp *CRIProvider) getPodDir(podUID types.UID) string {
	return filepath.Join(cp.getPodsDir(), string(podUID))
}

// getPodContainerDir returns the full path to the per-pod data directory under
// which container data is held for the specified pod.  This directory may not
// exist if the pod or container does not exist.
func (cp *CRIProvider) getPodContainerDir(podUID types.UID, ctrName string) string {
	return filepath.Join(cp.getPodDir(podUID), defaultContainersDirName, ctrName)
}

// GetPodVolumesDir returns the full path to the per-pod data directory under
// which volumes are created for the specified pod.  This directory may not
// exist if the pod does not exist.
func (cp *CRIProvider) GetPodVolumesDir(podUID types.UID) string {
	return filepath.Join(cp.getPodDir(podUID), defaultVolumesDirName)
}

func (cp *CRIProvider) GetStorageDir() string {
	return filepath.Join(cp.getRootDir(), defaultVariableDirName, defaultStorageDirName)
}

// makePodDataDirs creates the dirs for the pod datas.
func (cp *CRIProvider) makePodDataDirs(pod *v1.Pod) error {
	uid := pod.UID
	if err := os.MkdirAll(cp.getPodDir(uid), 0750); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.MkdirAll(cp.GetPodVolumesDir(uid), 0750); err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

// StartGarbageCollection starts garbage collection threads.
func (cp *CRIProvider) startGarbageCollection() {
	policy := pkgcontainer.GCPolicy{
		MinAge:             time.Minute,
		MaxContainers:      -1,
		MaxPerPodContainer: 1,
	}
	go wait.Until(func() {
		ctx := context.Background()
		if err := cp.containerRuntime.GarbageCollect(ctx, policy, true, false); err != nil {
			nlog.Errorf("Container garbage collection failed: %v", err)
		}
	}, ContainerGCPeriod, wait.NeverStop)
}

// makeMounts determines the mount points for the given container.
func (cp *CRIProvider) makeMounts(pod *v1.Pod, container *v1.Container, podVolumes resource.VolumeMap, envs []pkgcontainer.EnvVar) ([]pkgcontainer.Mount, error) {
	var mounts []pkgcontainer.Mount
	for _, mount := range container.VolumeMounts {
		vol, ok := podVolumes[mount.Name]
		if !ok {
			return nil, fmt.Errorf("cannot find volume %q to mount into container %q", mount.Name, container.Name)
		}

		if !filepath.IsAbs(mount.MountPath) {
			mount.MountPath = filepath.Join(container.WorkingDir, mount.MountPath)
		}

		hostPath := vol.HostPath

		if mount.SubPath != "" {
			if filepath.IsAbs(mount.SubPath) {
				return nil, fmt.Errorf("error SubPath `%s` must not be an absolute path", mount.SubPath)
			}

			if err := validation.ValidatePathNoBacksteps(mount.SubPath); err != nil {
				return nil, fmt.Errorf("unable to provision SubPath `%s`: %v", mount.SubPath, err)
			}

			hostPath = filepath.Join(vol.HostPath, mount.SubPath)

			if subPathExists, err := paths.CheckExists(paths.CheckSymlinkOnly, hostPath); err != nil {
				nlog.Errorf("Could not determine if subPath exists, will not attempt to change its permissions, path=%v", hostPath)
			} else if !subPathExists {
				// Create the sub path now because if it's auto-created later when referenced, it may have an
				// incorrect ownership and mode. For example, the sub path directory must have at least g+rwx
				// when the pod specifies an fsGroup, and if the directory is not created here, Docker will
				// later auto-create it with the incorrect mode 0750
				// Make extra care not to escape the volume!
				info, err := os.Stat(vol.HostPath)
				if err != nil {
					return nil, err
				}
				perm := info.Mode()

				if err := os.MkdirAll(hostPath, perm); err != nil {
					return nil, fmt.Errorf("failed to mkdir %q, detail-> %v", hostPath, err)
				}

				// chmod the sub path because umask may have prevented us from making the sub path with the same
				// permissions as the mounter path
				if err := os.Chmod(hostPath, perm); err != nil {
					return nil, err
				}
			}
		}

		if err := hook.Execute(&hook.MakeMountsContext{
			Pod:             pod,
			Container:       container,
			HostPath:        &hostPath,
			Mount:           &mount,
			Envs:            envs,
			PodVolumesDir:   cp.GetPodVolumesDir(pod.UID),
			ResourceManager: cp.resourceManager,
		}); err != nil {
			return nil, err
		}

		mounts = append(mounts, pkgcontainer.Mount{
			Name:           mount.Name,
			ContainerPath:  mount.MountPath,
			HostPath:       hostPath,
			ReadOnly:       mount.ReadOnly || vol.ReadOnly,
			SELinuxRelabel: vol.SELinuxRelabel,
		})
	}

	return mounts, nil
}

// GenerateRunContainerOptions generates the RunContainerOptions, which can be used by
// the container runtime to set parameters for launching a container.
func (cp *CRIProvider) GenerateRunContainerOptions(pod *v1.Pod, container *v1.Container, podIP string, podIPs []string) (*pkgcontainer.RunContainerOptions, func(), error) {
	opts := &pkgcontainer.RunContainerOptions{}
	if err := hook.Execute(&hook.GenerateContainerOptionContext{
		Pod:          pod,
		Container:    container,
		Opts:         opts,
		PodIPs:       podIPs,
		ContainerDir: cp.getPodContainerDir(pod.UID, container.Name),
		ImageService: cp.ImageManagerService,
		CpuAvailable: cp.cpuAvailable,
		MemAvailable: cp.memAvailable,
	}); err != nil {
		return nil, nil, err
	}

	// The value of hostname is the short host name and it is sent to makeMounts to create /etc/hosts file.
	hostname, hostDomainName, err := cp.GeneratePodHostNameAndDomain(pod)
	if err != nil {
		return nil, nil, err
	}
	// nodename will be equals to hostname if SetHostnameAsFQDN is nil or false. If SetHostnameFQDN
	// is true and hostDomainName is defined, nodename will be the FQDN (hostname.hostDomainName)
	nodename, err := util.GetNodenameForKernel(hostname, hostDomainName, pod.Spec.SetHostnameAsFQDN)
	if err != nil {
		return nil, nil, err
	}
	opts.Hostname = nodename

	envs, err := populateContainerEnvironment(context.Background(), pod, container, cp.resourceManager, cp.eventRecorder)
	if err != nil {
		return nil, nil, err
	}
	opts.Envs = append(opts.Envs, envs...)

	volumes := cp.volumeManager.GetMountedVolumesForPod(pod.UID)

	mounts, err := cp.makeMounts(pod, container, volumes, opts.Envs)
	if err != nil {
		return nil, nil, err
	}
	opts.Mounts = append(opts.Mounts, mounts...)

	opts.Mounts = append(opts.Mounts, pkgcontainer.Mount{
		Name:          "storage",
		ContainerPath: filepath.Join(cp.GetStorageDir()),
		HostPath:      filepath.Join(cp.GetStorageDir()),
	})

	// adding TerminationMessagePath on Windows is only allowed if ContainerD is used. Individual files cannot
	// be mounted as volumes using Docker for Windows.
	if len(container.TerminationMessagePath) != 0 {
		p := cp.getPodContainerDir(pod.UID, container.Name)
		if err := os.MkdirAll(p, 0750); err != nil {
			nlog.Errorf("Error on creating dir %q: %v", p, err)
		} else {
			opts.PodContainerDir = p
		}
	}

	nlog.Infof("Successfully generated the run options of container %q in pod %q", container.Name, format.Pod(pod))

	return opts, nil, nil
}

// GetPodDNS returns DNS settings for the pod.
// This function is defined in pkgcontainer.RuntimeHelper interface so we
// have to implement it.
func (cp *CRIProvider) GetPodDNS(pod *v1.Pod) (dnsConfig *runtimeapi.DNSConfig, err error) {
	return cp.dnsConfigurer.GetPodDNS(pod)
}

// GetPodCgroupParent gets pod cgroup parent from container manager.
func (cp *CRIProvider) GetPodCgroupParent(pod *v1.Pod) string {
	return ""
}

// GetPodDir returns the full path to the per-pod data directory for the
// specified pod. This directory may not exist if the pod does not exist.
func (cp *CRIProvider) GetPodDir(podUID types.UID) string {
	return cp.getPodDir(podUID)
}

// truncatePodHostnameIfNeeded truncates the pod hostname if it's longer than 63 chars.
func truncatePodHostnameIfNeeded(podName, hostname string) (string, error) {
	// Cap hostname at 63 chars (specification is 64bytes which is 63 chars and the null terminating char).
	const hostnameMaxLen = 63
	if len(hostname) <= hostnameMaxLen {
		return hostname, nil
	}
	truncated := hostname[:hostnameMaxLen]
	nlog.Warnf("Hostname for pod was too long, truncated it, podName=%v, hostnameMaxLen=%v, truncatedHostname%v", podName, hostnameMaxLen, truncated)
	// hostname should not end with '-' or '.'
	truncated = strings.TrimRight(truncated, "-.")
	if len(truncated) == 0 {
		// This should never happen.
		return "", fmt.Errorf("hostname for pod %q was invalid: %q", podName, hostname)
	}
	return truncated, nil
}

// GeneratePodHostNameAndDomain creates a hostname and domain name for a pod,
// given that pod's spec and annotations or returns an error.
func (cp *CRIProvider) GeneratePodHostNameAndDomain(pod *v1.Pod) (string, string, error) {
	clusterDomain := cp.dnsConfigurer.ClusterDomain

	hostname := pod.Name
	if len(pod.Spec.Hostname) > 0 {
		if msgs := utilvalidation.IsDNS1123Label(pod.Spec.Hostname); len(msgs) != 0 {
			return "", "", fmt.Errorf("pod Hostname %q is not a valid DNS label: %s", pod.Spec.Hostname, strings.Join(msgs, ";"))
		}
		hostname = pod.Spec.Hostname
	}

	hostname, err := truncatePodHostnameIfNeeded(pod.Name, hostname)
	if err != nil {
		return "", "", err
	}

	hostDomain := ""
	if len(pod.Spec.Subdomain) > 0 {
		if msgs := utilvalidation.IsDNS1123Label(pod.Spec.Subdomain); len(msgs) != 0 {
			return "", "", fmt.Errorf("pod Subdomain %q is not a valid DNS label: %s", pod.Spec.Subdomain, strings.Join(msgs, ";"))
		}
		hostDomain = fmt.Sprintf("%s.%s.svc.%s", pod.Spec.Subdomain, pod.Namespace, clusterDomain)
	}

	return hostname, hostDomain, nil
}

// GetExtraSupplementalGroupsForPod returns a list of the extra
// supplemental groups for the Pod. These extra supplemental groups come
// from annotations on persistent volumes that the pod depends on.
func (cp *CRIProvider) GetExtraSupplementalGroupsForPod(pod *v1.Pod) []int64 {
	//return kl.volumeManager.GetExtraSupplementalGroupsForPod(pod)
	return []int64{}
}

func getDockerConfig(secret *v1.Secret) (credentialprovider.DockerConfig, error) {
	// .DockerConfigJson example:
	// {
	//    "auths": {
	//        "DOCKER_REGISTRY_SERVER": {
	//            "username": "DOCKER_USER",
	//            "password": "DOCKER_PASSWORD",
	//            "auth": "RE9DS0VSX1VTRVI6RE9DS0VSX1BBU1NXT1JE"
	//        }
	//    }
	// }
	dockerCfgBytes, exists := secret.Data[v1.DockerConfigJsonKey]
	if (secret.Type == v1.SecretTypeDockerConfigJson) && exists && (len(dockerCfgBytes) > 0) {
		var dockerCfgJSON credentialprovider.DockerConfigJSON
		if err := json.Unmarshal(dockerCfgBytes, &dockerCfgJSON); err != nil {
			return nil, fmt.Errorf("parse %q fail, detail-> %v", v1.SecretTypeDockerConfigJson, err.Error())
		}
		return dockerCfgJSON.Auths, nil
	}

	dockerCfgBytes, exists = secret.Data[v1.DockerConfigKey]
	if (secret.Type == v1.SecretTypeDockercfg) && exists && (len(dockerCfgBytes) > 0) {
		var dockerCfg credentialprovider.DockerConfig
		if err := json.Unmarshal(dockerCfgBytes, &dockerCfg); err != nil {
			return nil, fmt.Errorf("parse %q fail, detail-> %v", v1.SecretTypeDockercfg, err.Error())
		}

		return dockerCfg, nil
	}

	return nil, fmt.Errorf("invalid image pull secret %q, no docker config info, secret.Type=%v", secret.Name, secret.Type)
}

func (cp *CRIProvider) getAuthFromSecret(repository, secretName string) (*credentialprovider.AuthConfig, error) {
	secret, err := cp.resourceManager.GetSecret(secretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %q, detail-> %v", secretName, err)
	}

	auths, err := getDockerConfig(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get docker config from secret %q, detail-> %v", secretName, err)
	}

	for server, entry := range auths {
		if repository == server {
			return &credentialprovider.AuthConfig{
				Username: entry.Username,
				Password: entry.Password,
			}, nil
		}
	}

	return nil, fmt.Errorf("failed to get matched authorization for repository %q", repository)
}

// getRegistryAuth gets authorization information for connecting to a registry.
func (cp *CRIProvider) getRegistryAuth() *credentialprovider.AuthConfig {
	if cp.registryConfig.Default.SecretName != "" {
		auth, err := cp.getAuthFromSecret(cp.registryConfig.Default.Repository, cp.registryConfig.Default.SecretName)
		if err == nil {
			return auth
		}

		nlog.Errorf("Unable to obtain authorization configuration from secret, ignoring and using local configuration, err-> %v", err)
	}

	return &credentialprovider.AuthConfig{
		Username:      cp.registryConfig.Default.Username,
		Password:      cp.registryConfig.Default.Password,
		Auth:          cp.registryConfig.Default.Auth,
		ServerAddress: cp.registryConfig.Default.ServerAddress,
		IdentityToken: cp.registryConfig.Default.IdentityToken,
		RegistryToken: cp.registryConfig.Default.RegistryToken,
	}

}

// SyncPod is the transaction script for the sync of a single pod (setting up)
// a pod. This method is reentrant and expected to converge a pod towards the
// desired state of the spec.
//
// Arguments:
//
// pod - the pod that is being set up
// podStatus - the most recent pod status observed for this pod which can
//
//	be used to determine the set of actions that should be taken during
//	this loop of syncPod
//
// reasonCache - caches the failure reason of the last creation of all containers
//
// The workflow is:
// * Create the data directories for the pod if they do not exist
// * Wait for volumes to attach/mount
// * Fetch the pull secrets for the pod
// * Call the container runtime's SyncPod callback
//
// If any step of this workflow errors, the error is returned, and is repeated
// on the next syncPod call.
//
// This operation writes all events that are dispatched in order to provide
// the most accurate information possible about an error situation to aid debugging.
// Callers should not write an event if this operation returns an error.
func (cp *CRIProvider) SyncPod(ctx context.Context, pod *v1.Pod, podStatus *pkgcontainer.PodStatus, reasonCache *kri.ReasonCache) error {
	nlog.Infof("CRIProvider start syncing pod %q", format.Pod(pod))

	// Make data directories for the pod
	if err := cp.makePodDataDirs(pod); err != nil {
		return fmt.Errorf("unable to make pod data directories for pod, detail-> %v", err)
	}

	if !cp.podStateProvider.IsPodTerminationRequested(pod.UID) {
		if err := cp.volumeManager.MountVolumesForPod(pod); err != nil {
			return fmt.Errorf("unable to mount volumes for pod %q: %v; skipping pod", format.Pod(pod), err)
		}
	}

	// Fetch the authorization information for the pod
	auth := cp.getRegistryAuth()

	// Ensure the pod is being probed
	cp.probeManager.AddPod(pod)

	// Call the container runtime's SyncPod callback
	result := cp.containerRuntime.SyncPod(ctx, pod, podStatus, auth, cp.backOff)
	reasonCache.Update(pod.UID, result)
	if err := result.Error(); err != nil {
		// Do not return error if the only failures were pods in backoff
		for _, r := range result.SyncResults {
			if r.Error != pkgcontainer.ErrCrashLoopBackOff && r.Error != images.ErrImagePullBackOff {
				// Do not record an event here, as we keep all event logging for sync pod failures
				// local to container runtime, so we get better errors.
				return err
			}
		}

		return nil
	}

	return nil
}

// KillPod instructs the container runtime to kill the pod. It also clears probe resource.
func (cp *CRIProvider) KillPod(ctx context.Context, pod *v1.Pod, runningPod pkgcontainer.Pod, gracePeriodOverride *int64) error {
	nlog.Infof("CRIProvider start killing pod %q", format.Pod(pod))

	cp.probeManager.StopLivenessAndStartup(pod)

	if err := cp.containerRuntime.KillPod(ctx, pod, runningPod, gracePeriodOverride); err != nil {
		return err
	}

	// Once the containers are stopped, we can stop probing for liveness and readiness.
	cp.probeManager.RemovePod(pod)

	return nil
}

func (cp *CRIProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	nlog.Infof("CRIProvider start deleting pod %q", format.Pod(pod))

	cp.volumeManager.UnmountVolumesForPod(pod.UID)

	return nil
}

// CleanupPods removes the root directory of pods that should not be
// running and that have no containers running.  Note that we roll up logs here since it runs in the main loop.
func (cp *CRIProvider) CleanupPods(ctx context.Context, pods []*v1.Pod, runningPods []*pkgcontainer.Pod, possiblyRunningPods map[types.UID]sets.Empty) error {
	cp.probeManager.CleanupPods(possiblyRunningPods)

	cp.backOff.GC()

	allPods := sets.NewString()
	for _, pod := range pods {
		allPods.Insert(string(pod.UID))
	}
	for _, pod := range runningPods {
		allPods.Insert(string(pod.ID))
	}

	found, err := cp.listPodsFromDisk()
	if err != nil {
		return err
	}

	var orphanRemovalErrors []error

	for _, uid := range found {
		if allPods.Has(string(uid)) {
			continue
		}

		podDir := cp.getPodDir(uid)
		if err = os.RemoveAll(podDir); err != nil {
			orphanRemovalErrors = append(orphanRemovalErrors, err)
		}
	}

	logSpew := func(errs []error) {
		if len(errs) > 0 {
			nlog.Errorf("There were many similar errors, turn up verbosity to see them, numErrs=%v, err=%v", len(errs), errs[0])
			for _, err := range errs {
				nlog.Debugf("Orphan pod: %v", err)
			}
		}
	}
	logSpew(orphanRemovalErrors)
	return utilerrors.NewAggregate(orphanRemovalErrors)

}

// GetPods instructs the container runtime to get pods.
func (cp *CRIProvider) GetPods(ctx context.Context, all bool) ([]*pkgcontainer.Pod, error) {
	return cp.containerRuntime.GetPods(ctx, all)
}

// UpdatePodStatus instructs the probeManager to update pod status.
func (cp *CRIProvider) RefreshPodStatus(pod *v1.Pod, podStatus *v1.PodStatus) {
	cp.probeManager.UpdatePodStatus(pod.UID, podStatus)
}

// GetPodStatus instructs the container runtime to get pod status.
func (cp *CRIProvider) GetPodStatus(ctx context.Context, pod *v1.Pod) (*pkgcontainer.PodStatus, error) {
	return cp.containerRuntime.GetPodStatus(ctx, pod.UID, pod.Name, pod.Namespace)
}

// listPodsFromDisk gets a list of pods that have data directories.
func (cp *CRIProvider) listPodsFromDisk() ([]types.UID, error) {
	podInfos, err := os.ReadDir(cp.getPodsDir())
	if err != nil {
		return nil, err
	}
	var pods []types.UID
	for i := range podInfos {
		if podInfos[i].IsDir() {
			pods = append(pods, types.UID(podInfos[i].Name()))
		}
	}
	return pods, nil
}
