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

package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type FateJobAdapter struct {
	TaskConfig   string
	ClusterIP    string
	KusciaTaskID string
	JobID        string
}

type fateComponents struct {
	Module string                         `json:"module"`
	Input  map[string]map[string][]string `json:"input,omitempty"`
	Output map[string][]string            `json:"output,omitempty"`
}

type taskConfInitiator struct {
	Role    string  `json:"role"`
	PartyID float64 `json:"party_id"`
}

type taskConfRole struct {
	Host    []int `json:"host,omitempty,omitempty"`
	Guest   []int `json:"guest,omitempty,omitempty"`
	Arbiter []int `json:"arbiter,omitempty,omitempty"`
}

type taskComponentRole struct {
	Guest map[string]map[string]map[string]any `json:"guest,omitempty"`
	Host  map[string]map[string]map[string]any `json:"host,omitempty"`
}

type taskComponentParameters struct {
	Common map[string]any    `json:"common"`
	Role   taskComponentRole `json:"role,omitempty"`
}

type taskRuntimeConf struct {
	DslVersion          any                     `json:"dsl_version"`
	Initiator           taskConfInitiator       `json:"initiator"`
	Role                taskConfRole            `json:"role"`
	ComponentParameters taskComponentParameters `json:"component_parameters"`
}

type taskDsl struct {
	Components map[string]fateComponents `json:"components"`
}

type fateTaskConfig struct {
	Dsl         taskDsl         `json:"dsl"`
	RuntimeConf taskRuntimeConf `json:"runtime_conf"`
}

type fateJobResData struct {
	FJobID string `json:"f_job_id"`
}

type fateJobQueryRes struct {
	Data []fateJobResData `json:"data"`
}

type fateJobSubmitRes struct {
	JobID string `json:"jobId"`
}

func requestFateJob(ip string, action string, param []byte) ([]byte, error) {
	request, err := http.Post(fmt.Sprintf("http://%s:9380/v1/job/%s", ip, action), "application/json", bytes.NewBuffer(param))
	if err != nil {
		return nil, err
	}

	defer request.Body.Close()

	return io.ReadAll(request.Body)
}

func (s *FateJobAdapter) CheckJobSucceeded() (bool, error) {
	param := map[string]string{
		"job_id": s.JobID,
		"status": "success",
	}
	paramJSON, _ := json.Marshal(param)
	respBytes, err := requestFateJob(s.ClusterIP, "query", paramJSON)

	var resData fateJobQueryRes
	err = json.Unmarshal(respBytes, &resData)
	if err != nil {
		return false, err
	}

	return len(resData.Data) != 0, err
}

func (s *FateJobAdapter) CheckJobRunning() (bool, error) {
	param := map[string]string{
		"job_id": s.JobID,
		"status": "running",
	}
	paramJSON, _ := json.Marshal(param)
	respBytes, err := requestFateJob(s.ClusterIP, "query", paramJSON)
	if err != nil {
		return false, err
	}

	var resData fateJobQueryRes
	err = json.Unmarshal(respBytes, &resData)
	if err != nil {
		return false, err
	}

	return len(resData.Data) != 0, nil
}

func (s *FateJobAdapter) SubmitJob() error {
	respBytes, err := requestFateJob(s.ClusterIP, "submit", []byte(s.TaskConfig))
	if err != nil {
		return err
	}

	var resData fateJobSubmitRes
	err = json.Unmarshal(respBytes, &resData)
	if err != nil || resData.JobID == "" {
		return errors.New("fate job submit fail")
	}

	s.JobID = resData.JobID

	return s.addNoteFateJob()
}

func (s *FateJobAdapter) addNoteFateJob() error {
	taskInputConfig := fateTaskConfig{}
	err := json.Unmarshal([]byte(s.TaskConfig), &taskInputConfig)
	if err != nil {
		return err
	}

	initiator := taskInputConfig.RuntimeConf.Initiator
	param := map[string]any{
		"job_id":   s.JobID,
		"notes":    s.KusciaTaskID,
		"role":     initiator.Role,
		"party_id": initiator.PartyID,
	}
	paramJSON, _ := json.Marshal(param)
	_, err = requestFateJob(s.ClusterIP, "update", []byte(paramJSON))

	return err
}

func (s *FateJobAdapter) queryDependantFateJob(node string) (string, error) {
	param := map[string]string{
		"description": node,
	}

	paramJSON, _ := json.Marshal(param)
	respBytes, err := requestFateJob(s.ClusterIP, "query", paramJSON)
	if err != nil {
		return "", err
	}

	var resData fateJobQueryRes
	err = json.Unmarshal(respBytes, &resData)
	if err != nil {
		return "", err
	}

	nlog.Info(resData)

	jobID := ""
	if len(resData.Data) != 0 {
		jobID = resData.Data[0].FJobID
	}

	return jobID, nil
}

func (s *FateJobAdapter) HandleTaskConfig(config string) error {
	taskInputConfig := fateTaskConfig{}
	err := json.Unmarshal([]byte(config), &taskInputConfig)
	if err != nil {
		return err
	}

	readerOps := map[string]fateComponents{}
	ops := taskInputConfig.Dsl.Components

	if len(taskInputConfig.RuntimeConf.Role.Guest) != 0 && taskInputConfig.RuntimeConf.ComponentParameters.Role.Guest == nil {
		opParam := map[string]map[string]map[string]any{}

		for index := range taskInputConfig.RuntimeConf.Role.Guest {
			opParam[fmt.Sprint(index)] = map[string]map[string]any{}
		}

		taskInputConfig.RuntimeConf.ComponentParameters.Role.Guest = opParam
	}
	if len(taskInputConfig.RuntimeConf.Role.Host) != 0 && taskInputConfig.RuntimeConf.ComponentParameters.Role.Host == nil {

		opParam := map[string]map[string]map[string]any{}
		for index := range taskInputConfig.RuntimeConf.Role.Host {
			opParam[fmt.Sprint(index)] = map[string]map[string]any{}
		}

		taskInputConfig.RuntimeConf.ComponentParameters.Role.Host = opParam
	}

	for _, op := range ops {
		if op.Input != nil {
			for _, inputPorts := range op.Input {
				for _, inputInfos := range inputPorts {
					for _, inputInfo := range inputInfos {
						inputInfoPart := strings.Split(inputInfo, ".")

						if _, hasOp := ops[inputInfoPart[0]]; len(inputInfoPart) == 2 && !hasOp {
							fmt.Println(inputInfoPart[1])
							readerOpName := inputInfoPart[0]
							readerOps[readerOpName] = fateComponents{
								Module: "Reader",
								Output: map[string][]string{
									"data": {"data"},
								},
							}
							jobID, jobErr := s.queryDependantFateJob(readerOpName)
							err = jobErr
							if err != nil || jobID == "" {
								return errors.New("handle dependant job fail")
							}
							nlog.Infof("task %s dependant fate job: %s", s.KusciaTaskID, jobID)

							readerOpParam := map[string]any{
								"table": map[string]string{
									"job_id":         jobID,
									"component_name": readerOpName,
									"data_name":      inputInfoPart[1],
								},
							}
							for index := range taskInputConfig.RuntimeConf.Role.Guest {
								taskInputConfig.RuntimeConf.ComponentParameters.Role.Guest[fmt.Sprint(index)][readerOpName] = readerOpParam
							}
							for index := range taskInputConfig.RuntimeConf.Role.Host {
								taskInputConfig.RuntimeConf.ComponentParameters.Role.Host[fmt.Sprint(index)][readerOpName] = readerOpParam
							}
						}
					}

				}
			}
		}
	}

	for opName, op := range readerOps {
		taskInputConfig.Dsl.Components[opName] = op
	}

	taskInputConfigJSON, err := json.Marshal(taskInputConfig)
	if err != nil {
		return err
	}

	s.TaskConfig = string(taskInputConfigJSON)

	nlog.Infof("fate task config %s", s.TaskConfig)

	return nil
}
