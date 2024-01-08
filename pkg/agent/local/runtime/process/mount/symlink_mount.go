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

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type symlinkMounter struct {
	rootfs string
}

func (m *symlinkMounter) MountIfNotMounted(device, target, mType, options string) error {
	target = filepath.Join(m.rootfs, target)

	if paths.CheckFileOrDirExist(target) {
		return nil
	}

	nlog.Infof("Mount device=%q target=%q mType=%q options=%q", device, target, mType, options)
	return paths.Link(device, target, true)
}

func (m *symlinkMounter) Mount(device, target, mType, options string) error {
	target = filepath.Join(m.rootfs, target)
	nlog.Infof("Mount device=%q target=%q mType=%q options=%q", device, target, mType, options)

	return paths.Link(device, target, true)
}

func (m *symlinkMounter) UmountRoot() error {
	return nil
}
