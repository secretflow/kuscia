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

package handler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	statusUpdateRetries = 3
)

// PendingHandler is used to handle task resource group which phase is pending.
type PendingHandler struct {
	kusciaClient kusciaclientset.Interface
}

// NewPendingHandler returns a PendingHandler instance.
func NewPendingHandler(deps *Dependencies) *PendingHandler {
	return &PendingHandler{
		kusciaClient: deps.KusciaClient,
	}
}

// Handle is used to perform the real logic.
func (h *PendingHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (bool, error) {
	now := metav1.Now().Rfc3339Copy()
	trg.Status.StartTime = &now
	trg.Status.LastTransitionTime = &now

	validatedCond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourceGroupValidated)
	utilsres.SetTaskResourceGroupCondition(&now, validatedCond, v1.ConditionTrue, "")
	if err := validate(trg); err != nil {
		trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseFailed
		utilsres.SetTaskResourceGroupCondition(&now, validatedCond, v1.ConditionFalse,
			fmt.Sprintf("Validate task resource group failed, %v", err.Error()))
		return true, nil
	}

	needUpdate, err := setTaskResourceName(h.kusciaClient, trg)
	if err != nil {
		generatedCond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourceNameGenerated)
		nlog.Error(err)
		utilsres.SetTaskResourceGroupCondition(&now, generatedCond, v1.ConditionFalse, fmt.Sprintf("Generate task resource name failed, %v", err.Error()))
		return true, err
	}

	if needUpdate {
		generatedCond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourceNameGenerated)
		utilsres.SetTaskResourceGroupCondition(&now, generatedCond, v1.ConditionTrue, "")
	}

	trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseCreating
	return true, nil
}

// validate is used to validate task resource group.
func validate(trg *kusciaapisv1alpha1.TaskResourceGroup) error {
	totalParty := len(trg.Spec.Parties) + len(trg.Spec.OutOfControlledParties)
	if trg.Spec.MinReservedMembers > totalParty {
		return fmt.Errorf("task resource group %v minReservedMembers can't be greater than party counts", trg.Name)
	}

	if trg.Spec.Initiator == "" {
		return fmt.Errorf("task resource group initiator %v can't be empty", trg.Spec.Initiator)
	}

	if totalParty == 0 {
		return fmt.Errorf("parties in task resource group %v can't be empty", trg.Name)
	}

	return nil
}

// setTaskResourceName sets task resource name.
func setTaskResourceName(kusciaClient kusciaclientset.Interface, trg *kusciaapisv1alpha1.TaskResourceGroup) (needUpdate bool, err error) {
	for idx, party := range trg.Spec.Parties {
		if party.TaskResourceName == "" {
			needUpdate = true
			trg.Spec.Parties[idx].TaskResourceName = generateTaskResourceName(trg.Name)
		}
	}

	for idx, party := range trg.Spec.OutOfControlledParties {
		if party.TaskResourceName == "" {
			needUpdate = true
			trg.Spec.OutOfControlledParties[idx].TaskResourceName = generateTaskResourceName(trg.Name)
		}
	}

	if !needUpdate {
		return false, nil
	}

	for i, copyTrg := 0, trg.DeepCopy(); ; i++ {
		nlog.Infof("Start updating task resource group %q spec parties info", copyTrg.Name)
		_, err = kusciaClient.KusciaV1alpha1().TaskResourceGroups().Update(context.Background(), copyTrg, metav1.UpdateOptions{})
		if err == nil {
			nlog.Infof("Finish updating task resource group %q spec parties info", copyTrg.Name)
			return true, nil
		}

		nlog.Warnf("Failed to update task resource group %q spec parties info, %v", copyTrg.Name, err)
		if i >= statusUpdateRetries {
			break
		}

		if copyTrg, err = kusciaClient.KusciaV1alpha1().TaskResourceGroups().Get(context.Background(), copyTrg.Name, metav1.GetOptions{}); err != nil {
			return false, err
		}

		if reflect.DeepEqual(copyTrg.Spec.Parties, trg.Spec.Parties) &&
			reflect.DeepEqual(copyTrg.Spec.OutOfControlledParties, trg.Spec.OutOfControlledParties) {
			nlog.Infof("Task resource group %v spec parties info is already updated, skip to update it", copyTrg.Name)
			return false, nil
		}

		copyTrg.Spec.Parties = trg.Spec.Parties
		copyTrg.Spec.OutOfControlledParties = trg.Spec.OutOfControlledParties
	}

	return false, err
}

// generateTaskResourceName is used to generate task resource name.
func generateTaskResourceName(prefix string) string {
	uid := strings.Split(string(uuid.NewUUID()), "-")
	return prefix + "-" + uid[len(uid)-1]
}
