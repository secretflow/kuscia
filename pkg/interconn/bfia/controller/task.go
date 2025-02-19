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

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	pkgcommon "github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

const (
	envConfigTaskID                    = "config.task_id"
	envConfigInstIDFormat              = "config.inst_id.%s.%d"
	envConfigNodeIDFormat              = "config.node_id.%s.%d"
	envConfigSelfRole                  = "config.self_role"
	envConfigSessionID                 = "config.session_id"
	envConfigTraceID                   = "config.trace_id"
	envConfigToken                     = "config.token"
	envRuntimeComponentName            = "runtime.component.name"
	envRuntimeComponentParameterFormat = "runtime.component.parameter.%s"
	envRuntimeComponentInputFormat     = "runtime.component.input.%s"
	envRuntimeComponentOutputFormat    = "runtime.component.output.%s"

	specKeyComponentNameFormat       = "component.%d.name"
	specKeyComponentInputNameFormat  = "component.%d.input.%d.name"
	specKeyComponentOutputNameFormat = "component.%d.output.%d.name"

	roleHost    = "host"
	roleGuest   = "guest"
	roleArbiter = "arbiter"

	taskParamNamespace = "namespace"
	taskParamsName     = "name"
)

// handleAddedOrDeletedKusciaTask handles added or deleted kuscia task.
func (c *Controller) handleAddedOrDeletedKusciaTask(obj interface{}) {
	task, ok := obj.(*kusciaapisv1alpha1.KusciaTask)
	if !ok {
		nlog.Warnf("Object %#v is not a KusciaTask", obj)
		return
	}

	if utilsres.SelfClusterAsInitiator(c.nsLister, task.Spec.Initiator, task.Annotations) {
		c.ktStatusSyncQueue.AddAfter(task.Name, taskStatusSyncInterval)
	}

	c.ktQueue.Add(task.Name)
}

// runTaskWorker runs a worker thread that just dequeues task, processes task, and marks them done.
func (c *Controller) runTaskWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, kusciaTaskQueueName, c.ktQueue, c.syncTaskHandler, maxRetries) {
	}
}

// syncTaskHandler deals with one pvcKey off the queue.  It returns false when it's time to quit.
func (c *Controller) syncTaskHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split task key %v, %v, skip processing it", key, err)
		return nil
	}

	rawKt, err := c.ktLister.KusciaTasks(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Kuscia task %v maybe deleted, skip to handle it", key)
			return nil
		}
		return err
	}

	if rawKt.Labels == nil || rawKt.Labels[pkgcommon.LabelTaskUnschedulable] != pkgcommon.True {
		nlog.Debugf("Kuscia task %v is schedulable, skip to handle it", key)
		return nil
	}

	if rawKt.DeletionTimestamp != nil {
		nlog.Infof("Kuscia task %v is terminating, skip to handle it", key)
		return nil
	}

	if rawKt.Status.CompletionTime != nil {
		nlog.Infof("Kuscia task %s is finished, skip to handle it", key)
		return nil
	}

	task := rawKt.DeepCopy()

	for i := range task.Spec.Parties {
		if injectErr := c.injectPartyTaskEnv(&task.Spec.Parties[i], task); injectErr != nil {
			return fmt.Errorf("failed to inject task %v env for party %v:%v, %v",
				task.Name, task.Spec.Parties[i].DomainID, task.Spec.Parties[i].Role, injectErr)
		}
	}

	task.Labels[pkgcommon.LabelTaskUnschedulable] = pkgcommon.False

	_, err = c.kusciaClient.KusciaV1alpha1().KusciaTasks(task.Namespace).Update(ctx, task, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update task %v, %v", task, err)
	}

	return nil
}

func (c *Controller) injectPartyTaskEnv(party *kusciaapisv1alpha1.PartyInfo, task *kusciaapisv1alpha1.KusciaTask) error {
	if len(party.Template.Spec.Containers) == 0 {
		party.Template.Spec.Containers = append(party.Template.Spec.Containers, kusciaapisv1alpha1.Container{})
	}

	taskInputConfig := &interconn.TaskInputConfig{}
	if err := json.Unmarshal([]byte(task.Spec.TaskInputConfig), taskInputConfig); err != nil {
		return fmt.Errorf("failed to unmarsh task input config, detail-> %v", err)
	}

	envs := []corev1.EnvVar{
		{
			Name:  envConfigTaskID,
			Value: task.Name,
		},
		{
			Name:  envConfigSessionID,
			Value: "session_" + task.Name,
		},
		{
			Name:  envConfigTraceID,
			Value: "trace_" + task.Name,
		},
		{
			Name:  envConfigToken,
			Value: "token_" + task.Name,
		},
		{
			Name:  envRuntimeComponentName,
			Value: taskInputConfig.ModuleName,
		},
	}

	envs = append(envs, makeIDEnv(party.DomainID, roleHost, taskInputConfig.Role.Host)...)
	envs = append(envs, makeIDEnv(party.DomainID, roleGuest, taskInputConfig.Role.Guest)...)
	envs = append(envs, makeIDEnv(party.DomainID, roleArbiter, taskInputConfig.Role.Arbiter)...)

	taskParams := map[string]string{}
	if taskInputConfig.TaskParams.Common != nil {
		for k, v := range taskInputConfig.TaskParams.Common.Fields {
			strValue, err := getStringFromValue(v)
			if err != nil {
				return err
			}
			taskParams[fmt.Sprintf(envRuntimeComponentParameterFormat, k)] = strValue
		}
	}

	partyRoleIndex, err := getPartyRoleIndex(party, taskInputConfig.Role)
	if err != nil {
		return err
	}

	if appendErr := appendPartyTaskParams(party, partyRoleIndex, taskInputConfig.TaskParams, taskParams); appendErr != nil {
		return appendErr
	}

	for k, v := range taskParams {
		envs = append(envs, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	ownerRef := metav1.GetControllerOf(task)
	if ownerRef == nil {
		return errors.New("failed to get task owner")
	}
	kusciaJob, err := c.kjLister.KusciaJobs(task.Namespace).Get(ownerRef.Name)
	if err != nil {
		return fmt.Errorf("failed to get KusciaJob %v", ownerRef.Name)
	}

	inputEnvs, err := c.makeComponentInputEnvs(kusciaJob, party, partyRoleIndex, taskInputConfig)
	if err != nil {
		return err
	}
	envs = append(envs, inputEnvs...)

	outputEnvs, err := c.makeComponentOutputEnvs(kusciaJob.Name, task.Name, party, partyRoleIndex, taskInputConfig)
	if err != nil {
		return err
	}
	envs = append(envs, outputEnvs...)

	party.Template.Spec.Containers[0].Env = append(party.Template.Spec.Containers[0].Env, envs...)

	return nil
}

func makeIDEnv(domainID, role string, parties []string) []corev1.EnvVar {
	envs := make([]corev1.EnvVar, 0)
	for i, partyID := range parties {
		if partyID == domainID {
			envs = append(envs, corev1.EnvVar{
				Name:  envConfigSelfRole,
				Value: fmt.Sprintf("%s.%d", role, i),
			})
		}

		instID := corev1.EnvVar{
			Name:  fmt.Sprintf(envConfigInstIDFormat, role, i),
			Value: partyID,
		}

		nodeID := corev1.EnvVar{
			Name:  fmt.Sprintf(envConfigNodeIDFormat, role, i),
			Value: partyID,
		}

		envs = append(envs, instID, nodeID)
	}
	return envs
}

func getPartyRoleIndex(party *kusciaapisv1alpha1.PartyInfo, roleConfig *interconn.ConfigRole) (int, error) {
	var parties []string

	switch party.Role {
	case roleHost:
		parties = roleConfig.Host
	case roleGuest:
		parties = roleConfig.Guest
	case roleArbiter:
		parties = roleConfig.Arbiter
	default:
		return 0, fmt.Errorf("unsupported role type %s", party.Role)
	}

	for i, v := range parties {
		if v == party.DomainID {
			return i, nil
		}
	}

	return 0, fmt.Errorf("not found party %v/%v in role config", party.DomainID, party.Role)
}

func appendPartyTaskParams(party *kusciaapisv1alpha1.PartyInfo, roleIndex int, allTaskParams *interconn.ConfigParams, partyTaskParams map[string]string) error {
	var (
		roleParams = map[string]*structpb.Value{}
	)

	switch party.Role {
	case roleHost:
		if allTaskParams.Host == nil {
			return nil
		}
		roleParams = allTaskParams.Host.Fields
	case roleGuest:
		if allTaskParams.Guest == nil {
			return nil
		}
		roleParams = allTaskParams.Guest.Fields
	case roleArbiter:
		if allTaskParams.Arbiter == nil {
			return nil
		}
		roleParams = allTaskParams.Arbiter.Fields
	default:
		return fmt.Errorf("unsupported role type %s", party.Role)
	}

	value, ok := roleParams[strconv.Itoa(roleIndex)]
	if !ok {
		return nil
	}

	partyPrivateParams := value.GetStructValue()
	if partyPrivateParams == nil {
		return errors.New("can not convert party private params to struct")
	}

	for k, v := range partyPrivateParams.Fields {
		strValue, err := getStringFromValue(v)
		if err != nil {
			return err
		}
		partyTaskParams[fmt.Sprintf(envRuntimeComponentParameterFormat, k)] = strValue
	}

	return nil
}

type componentIO struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (c *Controller) makeComponentInputEnvs(
	job *kusciaapisv1alpha1.KusciaJob,
	party *kusciaapisv1alpha1.PartyInfo,
	roleIndex int,
	taskInputConfig *interconn.TaskInputConfig,
) ([]corev1.EnvVar, error) {

	var envs []corev1.EnvVar

	// check if it dependents on predecessor tasks.
	if len(taskInputConfig.Input) == 0 {
		partyTaskParams, err := getPartyTaskParams(taskInputConfig.TaskParams, party.Role, roleIndex)
		if err != nil {
			return nil, err
		}

		namespace, ok := partyTaskParams[taskParamNamespace]
		if !ok {
			return nil, errors.New("not found namespace in party task params")
		}

		name, ok := partyTaskParams[taskParamsName]
		if !ok {
			return nil, errors.New("not found name in party task params")
		}

		namespaceStr, err := getStringFromValue(namespace)
		if err != nil {
			return nil, err
		}

		nameStr, err := getStringFromValue(name)
		if err != nil {
			return nil, err
		}

		input := &componentIO{namespaceStr, nameStr}

		env, err := c.makeComponentInputEnv(taskInputConfig.ModuleName, party.AppImageRef, 1, input)

		if err != nil {
			return nil, err
		}

		envs = append(envs, *env)
	} else {
		for i, inputItem := range taskInputConfig.Input {
			elements := strings.Split(inputItem.Key, ".")
			if len(elements) < 2 {
				return nil, fmt.Errorf("invalid input key %q", inputItem.Key)
			}
			depTaskName := elements[0]
			depOutputKey := elements[1]

			depTask, err := getTaskTemplateByName(job.Spec.Tasks, depTaskName)
			if err != nil {
				return nil, err
			}

			depTaskInputConfig := &interconn.TaskInputConfig{}
			if err = json.Unmarshal([]byte(depTask.TaskInputConfig), depTaskInputConfig); err != nil {
				return nil, err
			}

			input := &componentIO{
				fmt.Sprintf("%s-%s-%d", job.Name, party.Role, roleIndex),
				fmt.Sprintf("%s-%s", depTask.TaskID, depOutputKey),
			}

			env, err := c.makeComponentInputEnv(taskInputConfig.ModuleName, party.AppImageRef, i+1, input)
			if err != nil {
				return nil, err
			}

			envs = append(envs, *env)
		}
	}

	return envs, nil
}

func (c *Controller) makeComponentInputEnv(componentName string, appImageName string, inputIndex int, io *componentIO) (*corev1.EnvVar, error) {
	inputName, err := c.findTaskIOEnvNameFromImage(componentName, appImageName, inputIndex, ioTypeInput)
	if err != nil {
		return nil, err
	}

	envKey := fmt.Sprintf(envRuntimeComponentInputFormat, inputName)
	return c.makeComponentIOEnv(envKey, io)
}

func (c *Controller) makeComponentOutputEnvs(
	jobName string,
	taskID string,
	party *kusciaapisv1alpha1.PartyInfo,
	roleIndex int,
	taskInputConfig *interconn.TaskInputConfig,
) ([]corev1.EnvVar, error) {
	var envs []corev1.EnvVar

	for i, outputItem := range taskInputConfig.Output {
		output := &componentIO{
			fmt.Sprintf("%s-%s-%d", jobName, party.Role, roleIndex),
			fmt.Sprintf("%s-%s", taskID, outputItem.Key),
		}

		outputName, err := c.findTaskIOEnvNameFromImage(taskInputConfig.ModuleName, party.AppImageRef, i+1, ioTypeOutput)
		if err != nil {
			return nil, err
		}

		envKey := fmt.Sprintf(envRuntimeComponentOutputFormat, outputName)

		env, err := c.makeComponentIOEnv(envKey, output)
		if err != nil {
			return nil, err
		}

		envs = append(envs, *env)
	}

	return envs, nil
}

func (c *Controller) makeComponentIOEnv(envKey string, io *componentIO) (*corev1.EnvVar, error) {
	ioBytes, err := json.Marshal(io)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal componetIO %v, detail-> %v", envKey, err)
	}

	return &corev1.EnvVar{
		Name:  envKey,
		Value: string(ioBytes),
	}, nil
}

const (
	ioTypeInput  = "input"
	ioTypeOutput = "output"
)

func (c *Controller) findTaskIOEnvNameFromImage(componentName string, appImageName string, inputIndex int, ioType string) (string, error) {
	appImage, err := c.appImageLister.Get(appImageName)
	if err != nil {
		return "", fmt.Errorf("failed to get AppImage %v", appImageName)
	}

	if appImage.Annotations == nil {
		return "", fmt.Errorf("empty AppImage annotation")
	}

	componentSpecString, ok := appImage.Annotations[pkgcommon.ComponentSpecAnnotationKey]
	if !ok {
		return "", fmt.Errorf("not found component spec annotation in AppImage %v", appImageName)
	}

	componentSpec := map[string]string{}
	if err := json.Unmarshal([]byte(componentSpecString), &componentSpec); err != nil {
		return "", fmt.Errorf("failed to unmarshal component spec in AppImage %v, detail-> %v", appImageName, err)
	}

	for i := 1; ; i++ {
		componentNameInSpec, ok := componentSpec[fmt.Sprintf(specKeyComponentNameFormat, i)]
		if !ok {
			return "", fmt.Errorf("not found expected compoent %v in component spec of AppImage %v", componentName, appImageName)
		}

		if componentName != componentNameInSpec {
			continue
		}

		switch ioType {
		case ioTypeInput:
			specKeyInputName := fmt.Sprintf(specKeyComponentInputNameFormat, i, inputIndex)
			inputName, ok := componentSpec[specKeyInputName]
			if !ok {
				return "", fmt.Errorf("not found expected input name %v in component spec of AppImage %v", specKeyInputName, appImageName)
			}

			return inputName, nil
		case ioTypeOutput:
			specKeyOutputName := fmt.Sprintf(specKeyComponentOutputNameFormat, i, inputIndex)
			outputName, ok := componentSpec[specKeyOutputName]
			if !ok {
				return "", fmt.Errorf("not found expected output name %v in component spec of AppImage %v", specKeyOutputName, appImageName)
			}

			return outputName, nil
		}

	}
}

func getPartyTaskParams(taskParams *interconn.ConfigParams, role string, roleIndex int) (map[string]*structpb.Value, error) {
	var roleParams = map[string]*structpb.Value{}

	switch role {
	case roleHost:
		roleParams = taskParams.Host.Fields
	case roleGuest:
		roleParams = taskParams.Guest.Fields
	case roleArbiter:
		roleParams = taskParams.Arbiter.Fields
	default:
		return nil, fmt.Errorf("unsupported role type %s", role)
	}

	value, ok := roleParams[strconv.Itoa(roleIndex)]
	if !ok {
		return nil, nil
	}
	partyParams := value.GetStructValue()
	if partyParams == nil {
		return nil, errors.New("can not convert party params to struct")
	}

	return partyParams.Fields, nil
}

func getTaskTemplateByName(tasks []kusciaapisv1alpha1.KusciaTaskTemplate, name string) (*kusciaapisv1alpha1.KusciaTaskTemplate, error) {
	for i := range tasks {
		if tasks[i].Alias == name {
			return &tasks[i], nil
		}
	}

	return nil, fmt.Errorf("not found task by name %v", name)
}

// runTaskStatusSyncWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runTaskStatusSyncWorker(ctx context.Context) {
	for c.processTaskStatusSyncNextWorkItem(ctx) {
	}
}

// processJobStatusSyncNextWorkItem precess task status sync queue item.
func (c *Controller) processTaskStatusSyncNextWorkItem(ctx context.Context) bool {
	key, quit := c.ktStatusSyncQueue.Get()
	if quit {
		return false
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		nlog.Errorf("Failed to split job key %v, %v, skip processing it", key, err)
		return false
	}

	rawKt, err := c.ktLister.KusciaTasks(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Kuscia task %v maybe deleted, ignore it", key)
			c.ktStatusSyncQueue.Done(key)
			return true
		}
		nlog.Errorf("Failed to get kuscia task %v, %v", key, err)
		c.ktStatusSyncQueue.Done(key)
		return true
	}

	if rawKt.Status.Phase == kusciaapisv1alpha1.TaskFailed ||
		rawKt.Status.Phase == kusciaapisv1alpha1.TaskSucceeded {
		nlog.Infof("Kuscia task %v status is %v, skip query task status from other parties", key, rawKt.Status.Phase)
		c.ktStatusSyncQueue.Done(key)
		return true
	}

	defer func() {
		c.ktStatusSyncQueue.Done(key)
		c.ktStatusSyncQueue.AddAfter(key, taskStatusSyncInterval)
	}()

	cacheKey := getCacheKeyName(reqTypeQueryTaskStatusWithPoll, resourceTypeKusciaTask, rawKt.Name)
	if _, ok := c.inflightRequestCache.Get(cacheKey); ok {
		return true
	}

	_ = c.inflightRequestCache.Add(cacheKey, "", taskStatusSyncInterval)

	kt := rawKt.DeepCopy()
	now := metav1.Now().Rfc3339Copy()
	cond, _ := utilsres.GetKusciaTaskCondition(&kt.Status, kusciaapisv1alpha1.KusciaTaskCondStatusSynced, true)

	var wg sync.WaitGroup
	var errs errorcode.Errs
	var pts []kusciaapisv1alpha1.PartyTaskStatus
	for _, party := range kt.Spec.Parties {
		if !utilsres.IsOuterBFIAInterConnDomain(c.nsLister, party.DomainID) {
			continue
		}

		wg.Add(1)
		go func(party kusciaapisv1alpha1.PartyInfo) {
			resp, pollErr := c.bfiaClient.PollTaskStatus(ctx, c.getReqDomainIDFromKusciaTask(kt), buildHostFor(party.DomainID), kt.Name, party.Role)
			if pollErr != nil {
				errs.AppendErr(pollErr)
				pts = append(pts, kusciaapisv1alpha1.PartyTaskStatus{
					DomainID: party.DomainID,
					Role:     party.Role,
					Message:  pollErr.Error(),
					Phase:    kusciaapisv1alpha1.TaskFailed,
				})
			} else {
				icStatus := resp.Data.AsMap()["status"]
				taskPhase := common.InterConnTaskPhaseToKusciaTaskPhase[icStatus.(string)]
				if taskPhase == "" {
					taskPhase = kusciaapisv1alpha1.TaskPending
				}

				pts = append(pts, kusciaapisv1alpha1.PartyTaskStatus{
					DomainID: party.DomainID,
					Role:     party.Role,
					Phase:    taskPhase,
				})
			}
			defer wg.Done()
		}(party)

		go func() {
			wg.Wait()
			if len(errs) > 0 {
				err = fmt.Errorf("poll task status request failed, %v", errs.String())
				nlog.Error(err)
				utilsres.SetKusciaTaskCondition(now, cond, corev1.ConditionFalse, "ErrorPollTaskStatusRequest", err.Error())
			} else {
				if cond.Status != corev1.ConditionTrue {
					utilsres.SetKusciaTaskCondition(now, cond, corev1.ConditionTrue, "", "")
				}
			}

			utilsres.MergeKusciaTaskPartyTaskStatus(kt, pts)
			_ = utilsres.UpdateKusciaTaskStatusWithRetry(c.kusciaClient, rawKt, kt, statusUpdateRetries)
		}()
	}

	return true
}

func (c *Controller) getReqDomainIDFromKusciaTask(kt *kusciaapisv1alpha1.KusciaTask) string {
	for _, party := range kt.Spec.Parties {
		if !utilsres.IsOuterBFIAInterConnDomain(c.nsLister, party.DomainID) {
			return party.DomainID
		}
	}
	return ""
}

func getStringFromValue(value *structpb.Value) (string, error) {
	kind := value.GetKind()

	switch x := kind.(type) {
	case *structpb.Value_StringValue:
		return x.StringValue, nil
	case *structpb.Value_NumberValue:
		return strconv.FormatFloat(x.NumberValue, 'f', -1, 64), nil
	case *structpb.Value_BoolValue:
		return strconv.FormatBool(x.BoolValue), nil
	case *structpb.Value_StructValue:
		valueBytes, err := json.Marshal(x.StructValue)
		if err != nil {
			return "", err
		}
		return string(valueBytes), nil
	default:
		return "", fmt.Errorf("unsupported type of value %v", value.String())
	}
}
