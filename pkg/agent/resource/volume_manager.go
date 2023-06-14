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

package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilstrings "k8s.io/utils/strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	hostPathPluginName  = "kubernetes.io/hostpath"
	configMapPluginName = "kubernetes.io/configmap"
	secretPluginName    = "kubernetes.io/secret"
)

type VolumeHelper interface {
	GetPodVolumesDir(podUID types.UID) string
}

type VolumeInfo struct {
	HostPath       string
	ReadOnly       bool
	Managed        bool
	SELinuxRelabel bool
}

type VolumeMap map[string]VolumeInfo

// VolumeManager Volume has an explicit lifetime - the same as the Pod that encloses it.
// Consequently, a volume outlives any Containers that run within the Pod,
// and data is preserved across Container restarts.
// Of course, when a Pod ceases to exist, the volume will cease to exist, too.
type VolumeManager struct {
	ResourceManager *KubeResourceManager
	volumeHelper    VolumeHelper

	podVolumeMap     map[types.UID]VolumeMap
	podVolumeMapLock sync.RWMutex
}

func NewVolumeManager(rm *KubeResourceManager, volumeHelper VolumeHelper) *VolumeManager {
	vs := &VolumeManager{
		ResourceManager: rm,
		podVolumeMap:    make(map[types.UID]VolumeMap),
		volumeHelper:    volumeHelper,
	}

	return vs
}

// getPath returns the full path to the directory which represents the
// named volume under the named plugin for specified pod.  This directory may not
// exist if the pod does not exist.
func (vm *VolumeManager) getPath(podUID types.UID, pluginName string, volumeName string) string {
	return filepath.Join(vm.volumeHelper.GetPodVolumesDir(podUID), utilstrings.EscapeQualifiedName(pluginName), volumeName)
}

func (vm *VolumeManager) MountVolumesForPod(pod *v1.Pod) error {
	volumeMap := make(VolumeMap)

	for _, volume := range pod.Spec.Volumes {
		var err error
		var volumeInfo *VolumeInfo
		switch {
		case volume.HostPath != nil:
			volumeInfo, err = vm.mountHostPath(volume.HostPath)
		case volume.ConfigMap != nil:
			volumeInfo, err = vm.mountConfigMap(pod, volume.ConfigMap)
		case volume.Secret != nil:
			volumeInfo, err = vm.mountSecret(pod, volume.Secret)
		default:
			err = fmt.Errorf("volume source %s not supported", volume.Name)
		}

		if err != nil {
			return fmt.Errorf("error mount volume %s, detail-> %s", volume.Name, err.Error())
		}

		volumeMap[volume.Name] = *volumeInfo
	}

	nlog.Infof("Mount volumes %+v for pod %q succeed", volumeMap, pod.Name)

	vm.podVolumeMapLock.Lock()
	defer vm.podVolumeMapLock.Unlock()
	vm.podVolumeMap[pod.UID] = volumeMap

	return nil
}

func (vm *VolumeManager) UnmountVolumesForPod(podUID types.UID) {
	vm.podVolumeMapLock.Lock()
	defer vm.podVolumeMapLock.Unlock()
	delete(vm.podVolumeMap, podUID)
}

func (vm *VolumeManager) GetMountedVolumesForPod(podUID types.UID) VolumeMap {
	vm.podVolumeMapLock.RLock()
	defer vm.podVolumeMapLock.RUnlock()

	return vm.podVolumeMap[podUID]
}

func (vm *VolumeManager) mountHostPath(hostPath *v1.HostPathVolumeSource) (*VolumeInfo, error) {
	hostPathType := v1.HostPathUnset
	if hostPath.Type != nil {
		hostPathType = *hostPath.Type
	}

	var err error
	switch hostPathType {
	case v1.HostPathUnset:
		err = paths.EnsurePath(hostPath.Path, true)
	case v1.HostPathDirectoryOrCreate:
		err = paths.EnsureDirectory(hostPath.Path, true)
	case v1.HostPathDirectory:
		err = paths.EnsureDirectory(hostPath.Path, false)
	case v1.HostPathFileOrCreate:
		err = paths.EnsureFile(hostPath.Path, true)
	case v1.HostPathFile:
		err = paths.EnsureFile(hostPath.Path, false)
	case v1.HostPathSocket:
		err = paths.EnsureFile(hostPath.Path, false)
	case v1.HostPathCharDev:
		err = fmt.Errorf("mount char device not supported now")
	case v1.HostPathBlockDev:
		err = fmt.Errorf("mount block device not supported now")
	default:
		err = fmt.Errorf("unknown mount type %s", hostPathType)
	}

	if err != nil {
		return nil, fmt.Errorf("error mount hostPath, type=%s, hostPath=%s, detail-> %s",
			hostPathType, hostPath.Path, err)
	}

	return &VolumeInfo{
		HostPath:       hostPath.Path,
		ReadOnly:       false,
		Managed:        false,
		SELinuxRelabel: false,
	}, nil
}

func (vm *VolumeManager) writeFile(path string, data []byte, mode int32) error {
	dirName := filepath.Dir(path)
	if err := paths.EnsureDirectory(dirName, true); err != nil {
		nlog.Errorf("Read or Create dir fail, dir=%s, detail-> %s", dirName, err.Error())
		return err
	}

	if err := os.WriteFile(path, data, os.FileMode(mode)); err != nil {
		nlog.Errorf("Write file fail, path=%s, mode=%v, detail-> %s", path, mode, err.Error())
		return err
	}

	nlog.Infof("Mount (dump) file success, path=%v, mode=%v, size=%v", path, mode, len(data))
	return nil
}

// mountLiteralVolume dumps content in literalMap to file
func (vm *VolumeManager) mountLiteralVolume(targetPath string,
	defaultMode *int32, items []v1.KeyToPath, optional *bool, literalMap map[string][]byte) error {
	// config maps are always mounted readonly, keep same with k8s
	fileMode := int32(0644) // k8s default
	if defaultMode != nil {
		fileMode = *defaultMode
	}

	// dump config map data to file
	if len(items) == 0 {
		// If Items unspecified, each key-value pair in the Data field of the referenced
		// ConfigMap will be projected into the volume as a file whose name is the
		// key and content is the value.
		for k, v := range literalMap {
			err := vm.writeFile(filepath.Join(targetPath, k), v, fileMode)
			if err != nil {
				return fmt.Errorf("write data to file fail, key=%s, detail-> %s", k, err.Error())
			}
		}
	} else {
		// If specified, the listed keys will be
		// projected into the specified paths, and unlisted keys will not be
		// present. If a key is specified which is not present in the ConfigMap,
		// the volume setup will error unless it is marked optional.
		for _, item := range items {
			if data, exist := literalMap[item.Key]; exist {
				mode := fileMode
				if item.Mode != nil {
					mode = *item.Mode
				}

				err := vm.writeFile(filepath.Join(targetPath, item.Path), data, mode)
				if err != nil {
					return fmt.Errorf("write data to file failed, key=%s, path=%s, detail-> %s", item.Key, item.Path, err.Error())
				}
			} else {
				// ref to a nonexistent key
				if optional != nil && *optional {
					continue
				} else {
					return fmt.Errorf("volumeMount ref to a nonexistent key %s", item.Key)
				}
			}
		}
	}

	return nil
}

func (vm *VolumeManager) mountConfigMap(pod *v1.Pod, volume *v1.ConfigMapVolumeSource) (*VolumeInfo, error) {
	cmap, err := vm.ResourceManager.GetConfigMap(volume.Name)
	if err != nil {
		return nil, err
	}

	// build source map
	dataMap := make(map[string][]byte)
	for k, v := range cmap.Data {
		if _, exist := dataMap[k]; exist {
			return nil, fmt.Errorf("duplicate data key %s in config map %s", k, volume.Name)
		}
		dataMap[k] = []byte(v)
	}
	// The keys stored in BinaryData must not overlap with the ones in the Data field
	for k, v := range cmap.BinaryData {
		if _, exist := dataMap[k]; exist {
			return nil, fmt.Errorf("duplicate binary data key %s in config map %s", k, volume.Name)
		}
		dataMap[k] = v
	}

	hostPath := vm.getPath(pod.UID, configMapPluginName, volume.Name)

	if err = vm.mountLiteralVolume(hostPath, volume.DefaultMode, volume.Items, volume.Optional, dataMap); err != nil {
		return nil, fmt.Errorf("mount configmap volume %q failed, detail-> %v", volume.Name, err)
	}

	return &VolumeInfo{
		HostPath:       hostPath,
		ReadOnly:       true,
		Managed:        true,
		SELinuxRelabel: true,
	}, nil
}

func (vm *VolumeManager) mountSecret(pod *v1.Pod, volume *v1.SecretVolumeSource) (*VolumeInfo, error) {
	secret, err := vm.ResourceManager.GetSecret(volume.SecretName)
	if err != nil {
		return nil, err
	}

	// build source map
	dataMap := make(map[string][]byte)
	for k, v := range secret.Data {
		if _, exist := dataMap[k]; exist {
			return nil, fmt.Errorf("duplicate data key %s in secret %s", k, volume.SecretName)
		}
		dataMap[k] = v // the value is auto decoded by master
	}
	// Ignore secret.StringData, it is provided as a write-only convenience method.
	// and will never output when reading from the API.

	hostPath := vm.getPath(pod.UID, secretPluginName, volume.SecretName)

	if err = vm.mountLiteralVolume(hostPath, volume.DefaultMode, volume.Items, volume.Optional, dataMap); err != nil {
		return nil, fmt.Errorf("mount secret volume %q failed, detail-> %v", volume.SecretName, err)
	}

	return &VolumeInfo{
		HostPath:       hostPath,
		ReadOnly:       true,
		Managed:        true,
		SELinuxRelabel: true,
	}, nil
}
