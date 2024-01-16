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

package process

import (
	"strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/shirou/gopsutil/v3/process"
)

// CheckProcessExists check whether process exists by name
func CheckExists(processName string) bool {
	// currently running processes.
	processes, err := process.Processes()

	if err != nil {
		nlog.Errorf("CheckProcessExists: get process failed %s", err.Error())
		return false
	}
	isExist := false
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			nlog.Warnf("CheckProcessExists: get process name failed %s", err.Error())
			continue
		}

		if strings.EqualFold(name, processName) {
			isExist = true
			break
		}
	}

	return isExist
}
