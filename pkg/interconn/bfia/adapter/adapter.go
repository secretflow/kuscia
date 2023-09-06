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
	"encoding/json"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

const version = "2.0.0"

// InterConnJobInfo defines interconn job info.
type InterConnJobInfo struct {
	DAG    *interconn.DAG    `json:"dag"`
	Config *interconn.Config `json:"config"`
	JodID  string            `json:"job_id"`
	FlowID string            `json:"flow_id"`
}

// JobParamsResources defines resources of job params.
type JobParamsResources struct {
	Disk   float64 `json:"disk"`
	Memory float64 `json:"memory"`
	CPU    float64 `json:"cpu"`
}

// GenerateInterConnJobInfoFrom generates interconn job info from kuscia job.
func GenerateInterConnJobInfoFrom(kusciaJob *kusciaapisv1alpha1.KusciaJob, appImageLister kuscialistersv1alpha1.AppImageLister) (*InterConnJobInfo, error) {
	jobInfo := &InterConnJobInfo{
		DAG:    &interconn.DAG{},
		Config: &interconn.Config{},
	}
	// componentName:value
	taskParamsCommonAttr := make(map[string]interface{})
	// index:componentName:value
	taskParamsHostAttr := make(map[string]interface{})
	// index:componentName:value
	taskParamsGuestAttr := make(map[string]interface{})
	// index:componentName:value
	taskParamsArbiterAttr := make(map[string]interface{})

	for _, task := range kusciaJob.Spec.Tasks {
		taskInputConfig := &interconn.TaskInputConfig{}
		err := json.Unmarshal([]byte(task.TaskInputConfig), taskInputConfig)
		if err != nil {
			return nil, fmt.Errorf("unmarshal task %v taskInputConfig failed, %v", task.Alias, err)
		}

		aimg, err := appImageLister.Get(task.AppImage)
		if err != nil {
			return nil, fmt.Errorf("failed to get appimage %v, %v", task.AppImage, err)
		}

		imageVersion := aimg.Spec.Image.Tag
		if imageVersion == "" {
			imageVersion = "latest"
		}

		cpt := &interconn.Component{
			Code:       task.AppImage,
			Name:       taskInputConfig.Name,
			ModuleName: taskInputConfig.ModuleName,
			Version:    aimg.Spec.Image.Tag,
			Input:      taskInputConfig.Input,
			Output:     taskInputConfig.Output,
		}

		jobInfo.DAG.Components = append(jobInfo.DAG.Components, cpt)

		if taskInputConfig.Initiator != nil && jobInfo.Config.Initiator == nil {
			jobInfo.Config.Initiator = &interconn.ConfigInitiator{
				Role:   taskInputConfig.Initiator.Role,
				NodeId: taskInputConfig.Initiator.NodeId,
			}
		}

		if taskInputConfig.Role != nil && jobInfo.Config.Role == nil {
			jobInfo.Config.Role = &interconn.ConfigRole{
				Arbiter: taskInputConfig.Role.Arbiter,
				Host:    taskInputConfig.Role.Host,
				Guest:   taskInputConfig.Role.Guest,
			}
		}

		if jobInfo.Config.JobParams == nil {
			jobParams, err := buildJobParams(taskInputConfig.Role)
			if err != nil {
				return nil, fmt.Errorf("build job params failed, %v", err)
			}
			jobInfo.Config.JobParams = jobParams
		}

		parseTaskParamsFromTaskInputConfig(taskInputConfig, taskParamsCommonAttr, taskParamsHostAttr, taskParamsGuestAttr, taskParamsArbiterAttr)
	}

	if err := buildInterConnJobConfig(jobInfo, taskParamsCommonAttr, taskParamsHostAttr, taskParamsGuestAttr, taskParamsArbiterAttr); err != nil {
		return nil, err
	}

	jobInfo.DAG.Version = version
	jobInfo.FlowID = kusciaJob.Spec.FlowID
	jobInfo.JodID = kusciaJob.Name
	return jobInfo, nil
}

// GenerateKusciaJobFrom generates kuscia job from interconn job info.
func GenerateKusciaJobFrom(jobInfo *InterConnJobInfo) (*kusciaapisv1alpha1.KusciaJob, error) {
	if jobInfo == nil || jobInfo.Config == nil || jobInfo.DAG == nil {
		return nil, fmt.Errorf("dag and config of interconn job info can't be empty")
	}

	kusciaJob := &kusciaapisv1alpha1.KusciaJob{}
	kusciaJob.Name = jobInfo.JodID
	kusciaJob.Labels = map[string]string{
		common.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnBFIA),
	}
	kusciaJob.Spec.Stage = kusciaapisv1alpha1.JobCreateStage
	kusciaJob.Spec.FlowID = jobInfo.FlowID
	taskMaxParallelism := 2
	kusciaJob.Spec.MaxParallelism = &taskMaxParallelism
	kusciaJob.Spec.ScheduleMode = kusciaapisv1alpha1.KusciaJobScheduleModeStrict

	if jobInfo.Config.Initiator == nil || jobInfo.Config.Initiator.NodeId == "" {
		return nil, fmt.Errorf("interconn job config initiator can't be empty")
	}
	kusciaJob.Spec.Initiator = jobInfo.Config.Initiator.NodeId

	deps := buildKusciaJobDependenciesFrom(jobInfo)
	tolerable := false
	var jobTasks []kusciaapisv1alpha1.KusciaTaskTemplate
	for idx := range jobInfo.DAG.Components {
		cpt := jobInfo.DAG.Components[idx]
		if err := validateComponentCode(cpt.Code); err != nil {
			return nil, fmt.Errorf("validate appImage failed, %v", err)
		}

		taskInputConfig, parties, err := buildTaskInputConfigAndPartiesFrom(cpt, jobInfo.Config)
		if err != nil {
			return nil, fmt.Errorf("generate taskInputConfig failed, %v", err)
		}

		jobTasks = append(jobTasks, kusciaapisv1alpha1.KusciaTaskTemplate{
			Alias:           cpt.Name,
			Dependencies:    deps[cpt.Name],
			Tolerable:       &tolerable,
			AppImage:        cpt.Code,
			TaskInputConfig: taskInputConfig,
			Parties:         parties,
		})
	}

	kusciaJob.Spec.Tasks = jobTasks
	return kusciaJob, nil
}

// buildInterConnJobParamsResources builds resources for job params.
func buildInterConnJobParamsResources(appImageLister kuscialistersv1alpha1.AppImageLister, role, appName string) (*JobParamsResources, error) {
	appImage, err := appImageLister.Get(appName)
	if err != nil {
		return nil, fmt.Errorf("failed to get appimage %v, %v", appImage, err)
	}

	calculateResource := func(request corev1.ResourceList) (float64, float64, float64) {
		if request == nil {
			return -1, -1, -1
		}

		var (
			disk   float64
			memory float64
			cpu    float64
		)
		if request.Storage() != nil {
			disk += request.Storage().AsApproximateFloat64()
		}
		if request.Memory() != nil {
			memory += request.Memory().AsApproximateFloat64()
		}
		if request.Memory() != nil {
			cpu += request.Cpu().AsApproximateFloat64()
		}
		return disk, memory, cpu
	}

	var (
		resourceDisk   float64 = -1
		resourceMemory float64 = -1
		resourceCPU    float64 = -1
	)

	foundTpl := false
	for _, deployTpl := range appImage.Spec.DeployTemplates {
		if deployTpl.Role == role {
			foundTpl = true
			for _, container := range deployTpl.Spec.Containers {
				resourceDisk, resourceMemory, resourceCPU = calculateResource(container.Resources.Requests)
			}
			break
		}
	}

	if !foundTpl && len(appImage.Spec.DeployTemplates) > 0 {
		for _, container := range appImage.Spec.DeployTemplates[0].Spec.Containers {
			resourceDisk, resourceMemory, resourceCPU = calculateResource(container.Resources.Requests)
		}
	}

	return &JobParamsResources{
		Disk:   resourceDisk,
		Memory: resourceMemory,
		CPU:    resourceCPU,
	}, nil
}

// buildJobParams builds job params.
func buildJobParams(role *interconn.ConfigRole) (*interconn.ConfigParams, error) {
	jobParams := &interconn.ConfigParams{}
	roleMap := make(map[string]interface{})
	emptyStruct, _ := structpb.NewStruct(roleMap)
	for idx := range role.Arbiter {
		a, err := structpb.NewStruct(roleMap)
		if err != nil {
			return nil, err
		}
		a.Fields[strconv.Itoa(idx)] = structpb.NewStructValue(emptyStruct)
		jobParams.Arbiter = a
	}

	for idx := range role.Host {
		h, err := structpb.NewStruct(roleMap)
		if err != nil {
			return nil, err
		}
		h.Fields[strconv.Itoa(idx)] = structpb.NewStructValue(emptyStruct)
		jobParams.Host = h
	}

	for idx := range role.Guest {
		g, err := structpb.NewStruct(roleMap)
		if err != nil {
			return nil, err
		}
		g.Fields[strconv.Itoa(idx)] = structpb.NewStructValue(emptyStruct)
		jobParams.Guest = g
	}

	return jobParams, nil
}

// parseTaskParamsFromTaskInputConfig parses task params from taskInputConfig.
func parseTaskParamsFromTaskInputConfig(taskInputConfig *interconn.TaskInputConfig, commonAttr, hostAttr, guestAttr, arbiterAttr map[string]interface{}) {
	if taskInputConfig.TaskParams.Common != nil {
		commonAttr[taskInputConfig.Name] = taskInputConfig.TaskParams.Common.AsMap()
	}

	setAttr := func(name string, input, output map[string]interface{}) {
		for index, inputAttr := range input {
			outputAttr, exist := output[index]
			if !exist {
				output[index] = map[string]interface{}{
					name: inputAttr,
				}
			} else {
				attrMap := outputAttr.(map[string]interface{})
				attrMap[name] = inputAttr
				output[index] = attrMap
			}
		}
	}

	if taskInputConfig.TaskParams.Host != nil {
		setAttr(taskInputConfig.Name, taskInputConfig.TaskParams.Host.AsMap(), hostAttr)
	}

	if taskInputConfig.TaskParams.Guest != nil {
		setAttr(taskInputConfig.Name, taskInputConfig.TaskParams.Guest.AsMap(), guestAttr)
	}

	if taskInputConfig.TaskParams.Arbiter != nil {
		setAttr(taskInputConfig.Name, taskInputConfig.TaskParams.Arbiter.AsMap(), arbiterAttr)
	}
}

// buildInterConnJobConfig builds interconn job config.
func buildInterConnJobConfig(jobSpec *InterConnJobInfo, commonAttr, hostAttr, guestAttr, arbiterAttr map[string]interface{}) error {
	taskParams := &interconn.ConfigParams{}
	jobSpec.Config.Version = version
	if commonAttr != nil {
		common, err := structpb.NewStruct(commonAttr)
		if err != nil {
			return fmt.Errorf("new struct for task_params common atrribute failed, %v", err)
		}
		taskParams.Common = common
	}

	if hostAttr != nil {
		host, err := structpb.NewStruct(hostAttr)
		if err != nil {
			return fmt.Errorf("new struct for task_params host atrribute failed, %v", err)
		}
		taskParams.Host = host
	}

	if guestAttr != nil {
		guest, err := structpb.NewStruct(guestAttr)
		if err != nil {
			return fmt.Errorf("new struct for task_params guest atrribute failed, %v", err)
		}
		taskParams.Guest = guest
	}

	if arbiterAttr != nil {
		arbiter, err := structpb.NewStruct(arbiterAttr)
		if err != nil {
			return fmt.Errorf("new struct for task_params arbiter atrribute failed, %v", err)
		}
		taskParams.Arbiter = arbiter
	}
	jobSpec.Config.TaskParams = taskParams
	return nil
}

// buildKusciaJobDependenciesFrom builds kuscia job dependencies from interconn job info.
func buildKusciaJobDependenciesFrom(jobInfo *InterConnJobInfo) map[string][]string {
	deps := make(map[string][]string)
	for _, t := range jobInfo.DAG.Components {
		if len(t.Input) == 0 {
			continue
		}

		for _, i := range t.Input {
		loop:
			for _, tt := range jobInfo.DAG.Components {
				if t.Name == tt.Name && t.ModuleName == tt.ModuleName {
					continue
				}

				for _, o := range tt.Output {
					if i.Type == o.Type && i.Key == fmt.Sprintf("%s.%s", tt.Name, o.Key) {
						deps[t.Name] = append(deps[t.Name], tt.Name)
						break loop
					}
				}
			}
		}
	}
	return deps
}

// validateComponentCode validates component code.
func validateComponentCode(code string) error {
	if code == "" {
		return fmt.Errorf("component code can't be empty")
	}

	return nil
}

// buildTaskInputConfigAndPartiesFrom builds taskInputConfig and parties from interconn job component and config.
func buildTaskInputConfigAndPartiesFrom(cpt *interconn.Component, config *interconn.Config) (string, []kusciaapisv1alpha1.Party, error) {
	taskInputConfig := &interconn.TaskInputConfig{
		Name:       cpt.Name,
		ModuleName: cpt.ModuleName,
		Input:      cpt.Input,
		Output:     cpt.Output,
		Role:       config.Role,
		Initiator:  config.Initiator,
		TaskParams: &interconn.ConfigParams{},
	}

	var parties []kusciaapisv1alpha1.Party
	arbiter, partiesRet, err := parseTaskRoleParams(config.TaskParams.Arbiter, config.Role.Arbiter, cpt.Name, "arbiter")
	if err != nil {
		return "", nil, err
	}
	taskInputConfig.TaskParams.Arbiter = arbiter
	parties = append(parties, partiesRet...)

	host, partiesRet, err := parseTaskRoleParams(config.TaskParams.Host, config.Role.Host, cpt.Name, "host")
	if err != nil {
		return "", nil, err
	}
	taskInputConfig.TaskParams.Host = host
	parties = append(parties, partiesRet...)

	guest, partiesRet, err := parseTaskRoleParams(config.TaskParams.Guest, config.Role.Guest, cpt.Name, "guest")
	if err != nil {
		return "", nil, err
	}
	taskInputConfig.TaskParams.Guest = guest
	parties = append(parties, partiesRet...)

	commonTaskParams := make(map[string]interface{})
	if config.TaskParams.Common != nil {
		for cptName, content := range config.TaskParams.Common.Fields {
			if cptName == cpt.Name {
				for k, v := range content.GetStructValue().AsMap() {
					commonTaskParams[k] = v
				}
				break
			}
		}

		if len(commonTaskParams) > 0 {
			s, err := structpb.NewStruct(commonTaskParams)
			if err != nil {
				return "", nil, err
			}
			taskInputConfig.TaskParams.Common = s
		}
	}

	retS, err := json.Marshal(taskInputConfig)
	if err != nil {
		return "", nil, err
	}
	return string(retS), parties, nil
}

// parseTaskRoleParams parses task role params.
func parseTaskRoleParams(roleParams *structpb.Struct, roles []string, componentName, role string) (*structpb.Struct, []kusciaapisv1alpha1.Party, error) {
	if roleParams == nil {
		return nil, nil, nil
	}

	var parties []kusciaapisv1alpha1.Party
	paramsMap := make(map[string]interface{})
	for roleIndex, f := range roleParams.Fields {
		for cptName, content := range f.GetStructValue().AsMap() {
			if cptName == componentName {
				paramsMap[roleIndex] = content

				index, err := strconv.Atoi(roleIndex)
				if err != nil {
					return nil, nil, fmt.Errorf("role index should be number string format, %v", err)
				}

				if index+1 > len(roles) {
					return nil, nil, fmt.Errorf("role index %v should be less than role %v length", index+1, roles)
				}

				parties = append(parties, kusciaapisv1alpha1.Party{
					DomainID: roles[index],
					Role:     role,
				})
				break
			}
		}
	}

	if len(paramsMap) == 0 {
		return nil, nil, nil
	}

	s, err := structpb.NewStruct(paramsMap)
	if err != nil {
		return nil, nil, err
	}
	return s, parties, nil
}
