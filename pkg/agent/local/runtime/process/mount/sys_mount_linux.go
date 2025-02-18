//go:build linux
// +build linux

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

package mount

import (
	"fmt"
	"path/filepath"
	"strings"

	sysmount "github.com/moby/sys/mount"
	"github.com/moby/sys/mountinfo"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type sysMounter struct {
	rootfs string
}

func (m *sysMounter) MountIfNotMounted(device, target, mType, options string) error {
	target = filepath.Join(m.rootfs, target)

	mounted, err := mountinfo.Mounted(target)
	if err != nil {
		return err
	}

	if !mounted {
		nlog.Infof("Mount device=%q target=%q mType=%q options=%q", device, target, mType, options)
		if err := sysmount.Mount(device, target, mType, options); err != nil {
			return err
		}
	}

	return nil
}

func (m *sysMounter) Mount(device, target, mType, options string) error {
	target = filepath.Join(m.rootfs, target)
	nlog.Infof("Mount device=%q target=%q mType=%q options=%q", device, target, mType, options)

	return sysmount.Mount(device, target, mType, options)
}

func (m *sysMounter) GetMountsRoot() ([]*mountinfo.Info, error) {
	mounts, err := mountinfo.GetMounts(func(info *mountinfo.Info) (skip, stop bool) {
		if !strings.HasPrefix(info.Mountpoint, m.rootfs) {
			return true, false
		}

		return false, false
	})
	if err != nil {
		return nil, err
	}

	return mounts, nil
}

func (m *sysMounter) UmountRoot() error {
	mounts, err := m.GetMountsRoot()
	if err != nil {
		return err
	}

	for _, mo := range mounts {
		mounted, mountErr := mountinfo.Mounted(mo.Mountpoint)
		if mountErr != nil {
			return fmt.Errorf("failed to check mounted, detail-> %v", mountErr)
		}
		if !mounted {
			continue
		}

		if unmountErr := sysmount.Unmount(mo.Mountpoint); unmountErr != nil {
			nlog.Errorf("Failed to umount %q: %v", mo.Mountpoint, unmountErr)
			continue
		}

		nlog.Infof("Umount %q successfully", mo.Mountpoint)
	}

	mounts, err = m.GetMountsRoot()
	if err != nil {
		return err
	}

	if len(mounts) > 0 {
		return fmt.Errorf("failed to clear all mounts, left=%v", len(mounts))
	}

	return nil
}
