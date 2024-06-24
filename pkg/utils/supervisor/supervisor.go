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

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var defaultRestartIntervalMS = []int{
	0, 0, 100, 100, 200, 200, 500, 500, 1000, 1000,
	2000, 2000, 2000, 2000, 5000, 5000,
}

// 3s
const defaultMinRunningTimeMS = 3000

type Cmd interface {
	Start() error
	Wait() error
	Pid() int
	SetOOMScore() error
}

type Supervisor struct {
	tag                  string
	restartIntervalMS    []int
	restartIntervalIndex int
	maxRestartCount      int
	minRunningTimeMS     int
}

func NewSupervisor(tag string, restartIntervalMS []int, maxRestartCount int) *Supervisor {
	if restartIntervalMS == nil {
		restartIntervalMS = defaultRestartIntervalMS
	}
	if maxRestartCount < 0 {
		maxRestartCount = math.MaxInt - 1
	}

	return &Supervisor{
		restartIntervalIndex: 0,
		restartIntervalMS:    restartIntervalMS,
		maxRestartCount:      maxRestartCount,
		tag:                  tag,
		minRunningTimeMS:     defaultMinRunningTimeMS,
	}
}

// TODO: use ctx to stop process and stop restart
func (s *Supervisor) Run(ctx context.Context, startup func(ctx context.Context) Cmd) error {
	if startup == nil {
		nlog.Errorf("[%s] input startup callback is nil", s.tag)
		return errors.New("input startup callback is nil")
	}
	nlog.Infof("[%s] start and watch subprocess", s.tag)

	isFirstRun := true
	for {
		isEveryTimeFailed := true
		s.restartIntervalIndex = 0
		for i := 0; i <= s.maxRestartCount; i++ {
			nlog.Infof("[%s] try to start new process", s.tag)
			if err := s.runProcess(ctx, startup(ctx)); err != nil {
				nlog.Warnf("[%s] run process failed with %v", s.tag, err)
				if isFirstRun {
					// if first time start process failed, exit at once
					return fmt.Errorf("startup process failed at first time, so stop at once, error: %v", err)
				}

				isFirstRun = false
				if s.restartIntervalIndex < len(s.restartIntervalMS)-1 {
					s.restartIntervalIndex++
				}
			} else { // process run success or longer than minRunningTimeMS
				isFirstRun = false
				isEveryTimeFailed = false
				s.restartIntervalIndex = 0
				break
			}

			// wait restart again
			intervalMS := s.restartIntervalMS[s.restartIntervalIndex]
			if intervalMS > 0 {
				nlog.Debugf("[%s] process exited, restart again after %d ms", s.tag, intervalMS)
				select {
				case <-time.After(time.Duration(intervalMS * int(time.Millisecond))):
				case <-ctx.Done():
					nlog.Warnf("[%s] context had done, no need to wait to restart", s.tag)
					return fmt.Errorf("context had done, no need to wait to restart")
				}
			}

		}

		if isEveryTimeFailed {
			nlog.Warnf("[%s] try to run process failed after %d times", s.tag, s.maxRestartCount+1)
			return fmt.Errorf("run process failed after %d times", s.maxRestartCount)
		}

	}
}

func (s *Supervisor) runProcess(ctx context.Context, cmd Cmd) error {
	stime := time.Now()
	if cmd == nil {
		return errors.New("create subprocess failed")
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start process(%d) failed with %v", cmd.Pid(), err)
	}

	if err := cmd.SetOOMScore(); err != nil {
		nlog.Warnf("Set process(%d) oom_score_adj failed, %v, skip setting it", cmd.Pid(), err)
	}

	err := cmd.Wait()
	if err != nil { // process exit failed
		nlog.Warnf("Process(%d) exit with error: %v", cmd.Pid(), err)
	} else {
		nlog.Infof("Process(%d) exit normally", cmd.Pid())
	}

	if dt := time.Since(stime); dt.Milliseconds() <= int64(s.minRunningTimeMS) {
		tmerr := fmt.Sprintf("process(%d) only existed %d ms, less than %d ms", cmd.Pid(), dt.Milliseconds(), s.minRunningTimeMS)
		if err != nil {
			return fmt.Errorf("%s, with error: %v", tmerr, err)
		}
		return errors.New(tmerr)
	}

	return nil
}
