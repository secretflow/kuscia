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

package modules

import (
	"context"
	"errors"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	adapter "github.com/secretflow/kuscia/thirdparty/fate/pkg/adapter"
)

type fateJobModule struct {
	adapter *adapter.FateJobAdapter
}

type FateJobConfig struct {
	TaskConfig     map[string]string
	ClusterAddress string
}

func NewJob(i *FateJobConfig) (Module, error) {
	fateJobModule := &fateJobModule{
		adapter: &adapter.FateJobAdapter{
			ClusterIP:    i.ClusterAddress,
			KusciaTaskID: i.TaskConfig["task_id"],
		},
	}

	err := fateJobModule.adapter.HandleTaskConfig(i.TaskConfig["task_input_config"])

	return fateJobModule, err
}

func (s *fateJobModule) readyz() (bool, error) {
	succeeded, err := s.adapter.CheckJobSucceeded()
	if err != nil {
		return false, err
	} else if succeeded {
		nlog.Infof("fate job: %s is succeeded!", s.adapter.JobID)

		return true, nil
	}

	running, err := s.adapter.CheckJobRunning()
	if err != nil {
		return false, err
	} else if running {
		nlog.Infof("fate job: %s is running", s.adapter.JobID)

		return false, nil
	}

	nlog.Errorf("fate job: %s is failed!", s.adapter.JobID)

	return false, errors.New("fate job fail")
}

func (s *fateJobModule) Run(ctx context.Context) error {
	return s.adapter.SubmitJob()

}

func (s *fateJobModule) WaitReady(ctx context.Context) error {
	tickerReady := time.NewTicker(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerReady.C:
			done, err := s.readyz()
			if err != nil {
				return err
			} else if done {
				return nil
			}
		}
	}
}

func (s *fateJobModule) Name() string {
	return "job"
}

func RunJob(ctx context.Context, cancel context.CancelFunc, conf *FateJobConfig) {
	m, err := NewJob(conf)
	if err != nil {
		nlog.Fatal(err)
	}
	go func() {
		if err := m.Run(ctx); err != nil {
			nlog.Fatal(err)
			cancel()
		}
	}()
	if err = m.WaitReady(ctx); err != nil {
		nlog.Fatal(err)
	} else {
		nlog.Info("fate job is done")
	}
	cancel()
}
