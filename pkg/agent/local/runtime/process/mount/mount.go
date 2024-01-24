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
	"path/filepath"
)

type Mounter interface {
	MountIfNotMounted(device, target, mType, options string) error
	Mount(device, target, mType, options string) error
	UmountRoot() error
}

func NewSysMounter(rootfs string) (Mounter, error) {
	absRootfs, err := filepath.Abs(rootfs)
	if err != nil {
		return nil, err
	}
	return &sysMounter{rootfs: absRootfs}, nil
}

func NewSymlinkMounter(rootfs string) (Mounter, error) {
	absRootfs, err := filepath.Abs(rootfs)
	if err != nil {
		return nil, err
	}
	return &symlinkMounter{rootfs: absRootfs}, nil
}
