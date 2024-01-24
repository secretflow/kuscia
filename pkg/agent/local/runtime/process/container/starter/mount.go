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

package starter

import (
	"fmt"
	"path/filepath"

	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"

	mnt "github.com/secretflow/kuscia/pkg/agent/local/runtime/process/mount"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

func mountVolumes(rootfs string, mounter mnt.Mounter, mounts []*runtime.Mount) (retErr error) {
	for _, mount := range mounts {
		mountPoint := filepath.Join(rootfs, mount.ContainerPath)

		if paths.CheckFileExist(mount.HostPath) {
			if err := paths.Link(mount.HostPath, mountPoint, false); err != nil {
				return fmt.Errorf("failed to link path %q, detail-> %v", mountPoint, err)
			}
		} else {
			if err := paths.EnsureDirectory(mountPoint, true); err != nil {
				return err
			}
			if err := mounter.Mount(mount.HostPath, mount.ContainerPath, "none", "bind,rw"); err != nil {
				return fmt.Errorf("failed to mount path %q, detail-> %v", mount.ContainerPath, err)
			}
		}
	}

	return nil
}

func mountSystemFile(mounter mnt.Mounter) error {
	if err := mounter.MountIfNotMounted("proc", "/proc", "proc", ""); err != nil {
		return err
	}
	if err := mounter.MountIfNotMounted("/dev", "/dev", "none", "rbind"); err != nil {
		return err
	}
	return nil
}
