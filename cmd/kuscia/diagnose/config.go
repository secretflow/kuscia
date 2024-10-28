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

package diagnose

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type TaskConfig struct {
	TaskInputConfig string   `json:"task_input_config"`
	ServerPort      int      `json:"server_port"`
	Domains         []string `json:"domains"`
	Endpoints       []string `json:"endpoints"`
	SelfIdx         int      `json:"self_index"`
}

func ParseConfig(fileName string) (*netstat.NetworkJobConfig, error) {
	data, err := readFile(fileName)
	if err != nil {
		return nil, err
	}
	return parseConfig(data)
}

func readFile(fileName string) ([]byte, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		nlog.Errorf("Open config file error: %v\n", err)
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	return byteValue, nil
}

func parseConfig(data []byte) (*netstat.NetworkJobConfig, error) {
	taskConfig := new(TaskConfig)
	if err := json.Unmarshal(data, taskConfig); err != nil {
		nlog.Errorf("Json unmarshal error: %v", err)
		return nil, err
	}

	jobConfig := new(netstat.NetworkJobConfig)
	jobConfig.NetworkParam = new(netstat.NetworkParam)
	if err := json.Unmarshal([]byte(taskConfig.TaskInputConfig), jobConfig); err != nil {
		nlog.Errorf("Json unmarshal error: %v", err)
		return nil, err
	}
	jobConfig.SelfDomain = taskConfig.Domains[taskConfig.SelfIdx]
	jobConfig.SelfEndpoint = taskConfig.Endpoints[taskConfig.SelfIdx]
	if !jobConfig.Manual {
		jobConfig.PeerDomain = taskConfig.Domains[1-taskConfig.SelfIdx]
		jobConfig.PeerEndpoint = taskConfig.Endpoints[1-taskConfig.SelfIdx]
	}
	jobConfig.ServerPort = taskConfig.ServerPort

	return jobConfig, nil
}
