// Copyright 2024 Ant Group Co., Ltd.
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

package netstat

import (
	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"golang.org/x/net/context"
)

type ConnectionTask struct {
	client *client.Client
	output *TaskOutput
}

func NewConnectionTask(client *client.Client) Task {
	task := &ConnectionTask{
		client: client,
		output: new(TaskOutput),
	}
	task.output.Name = task.Name()
	return task
}

func (t *ConnectionTask) Run(ctx context.Context) {
	nlog.Infof("Run %v task", t.Name())
	defer nlog.Infof("Task %v done, output: %+v", t.Name(), t.output)
	t.output.DetectedValue = "N/A"
	// healthy check
	if _, err := t.client.Healthy(ctx); err != nil {
		t.output.Result = common.Fail
		t.output.Information = err.Error()
		return
	}
	t.output.Result = common.Pass
}

func (t *ConnectionTask) Output() *TaskOutput {
	return t.output
}

func (t *ConnectionTask) Name() string {
	return "CONNECTION"
}
