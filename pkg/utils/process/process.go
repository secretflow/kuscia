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
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	oomScoreAdjMax = 1000
	oomScoreAdjMin = -1000
)

// CheckExists check whether process exists by name
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

// SetOOMScore sets the oom score for the provided pid.
func SetOOMScore(pid, score int) error {
	if score > oomScoreAdjMax || score < oomScoreAdjMin {
		return fmt.Errorf("invalid score %v, oom score must be between %d and %d", score, oomScoreAdjMin, oomScoreAdjMax)
	}

	path := fmt.Sprintf("/proc/%d/oom_score_adj", pid)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.WriteString(strconv.Itoa(score)); err != nil {
		return err
	}

	nlog.Infof("Set pid[%v] oom score adj to %v", pid, score)

	return nil
}
