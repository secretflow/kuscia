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
	"strings"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	quota "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
)

// CreatingHandler is used to handle task resource group which phase is creating.
type CreatingHandler struct {
	kubeClient   kubernetes.Interface
	kusciaClient kusciaclientset.Interface
	podLister    listers.PodLister
	trLister     kuscialistersv1alpha1.TaskResourceLister
}

// NewCreatingHandler returns a CreatingHandler instance.
func NewCreatingHandler(deps *Dependencies) *CreatingHandler {
	return &CreatingHandler{
		kubeClient:   deps.KubeClient,
		kusciaClient: deps.KusciaClient,
		podLister:    deps.PodLister,
		trLister:     deps.TrLister,
	}
}

// Handle is used to perform the real logic.
func (h *CreatingHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (bool, error) {
	var err error
	creatingCond, _ := utilscommon.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesGroupCondCreating)
	defer func() {
		now := metav1.Now().Rfc3339Copy()
		trg.Status.LastTransitionTime = now
		creatingCond.LastTransitionTime = now
		if err == nil {
			trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseReserving
			creatingCond.Status = v1.ConditionTrue
			creatingCond.Reason = "Create party task resource"
		} else {
			creatingCond.Status = v1.ConditionFalse
			creatingCond.Reason = fmt.Sprintf("Failed to create task resource, %v", err.Error())
		}
	}()

	if err = h.createTaskResources(trg); err != nil {
		return true, err
	}

	return true, nil
}

// createTaskResource is used to create task resources.
func (h *CreatingHandler) createTaskResources(trg *kusciaapisv1alpha1.TaskResourceGroup) error {
	if len(trg.Spec.Parties) == 0 {
		return fmt.Errorf("parties in task resource group %v can't be empty", trg.Name)
	}

	var (
		err      error
		pod      *v1.Pod
		trs      []*kusciaapisv1alpha1.TaskResource
		latestTr *kusciaapisv1alpha1.TaskResource
		buildTr  *kusciaapisv1alpha1.TaskResource
	)

	for _, party := range trg.Spec.Parties {
		trs, err = h.trLister.TaskResources(party.DomainID).List(labels.SelectorFromSet(labels.Set{common.LabelTaskResourceGroup: trg.Name}))
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}

		latestTr = findPartyTaskResource(party, trs)

		if latestTr == nil {
			buildTr, err = h.buildTaskResource(&party, trg)
			if err != nil {
				return err
			}

			latestTr, err = h.kusciaClient.KusciaV1alpha1().TaskResources(party.DomainID).Create(context.Background(), buildTr, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create task resource %v/%v failed, %v", buildTr.Namespace, buildTr.Name, err)
			}
		}

		for _, p := range party.Pods {
			pod, err = h.podLister.Pods(party.DomainID).Get(p.Name)
			if err != nil {
				return err
			}

			if value, exist := pod.Labels[kusciaapisv1alpha1.LabelTaskResource]; exist && value == latestTr.Name {
				continue
			}

			podCopy := pod.DeepCopy()
			if podCopy.Labels == nil {
				podCopy.Labels = map[string]string{}
			}

			podCopy.Labels[kusciaapisv1alpha1.LabelTaskResource] = latestTr.Name
			oldExtractedPod := utilscommon.ExtractPodLabels(pod)
			newExtractedPod := utilscommon.ExtractPodLabels(podCopy)
			if err = utilscommon.PatchPod(context.Background(), h.kubeClient, oldExtractedPod, newExtractedPod); err != nil {
				return fmt.Errorf("patch pod %v/%v label failed, %v", pod.Namespace, pod.Name, err)
			}
		}
	}
	return nil
}

// buildTaskResource is used to build task resource.
func (h *CreatingHandler) buildTaskResource(party *kusciaapisv1alpha1.TaskResourceGroupParty, trg *kusciaapisv1alpha1.TaskResourceGroup) (*kusciaapisv1alpha1.TaskResource, error) {
	tPods, err := h.buildTaskResourcePods(party.DomainID, party.Pods)
	if err != nil {
		return nil, fmt.Errorf("build task resource %v/%v pods failed, %v", party.DomainID, trg.Name, err.Error())
	}

	now := metav1.Now()
	tr := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: party.DomainID,
			Name:      generateTaskResourceName(trg.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(trg, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("TaskResourceGroup")),
			},
			Labels: map[string]string{
				common.LabelTaskResourceGroup: trg.Name,
				common.LabelTaskInitiator:     trg.Spec.Initiator,
			},
		},
		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			MinReservedPods:         getMinReservedPods(party),
			ResourceReservedSeconds: trg.Spec.ResourceReservedSeconds,
			Initiator:               trg.Spec.Initiator,
			Pods:                    tPods,
		},
		Status: kusciaapisv1alpha1.TaskResourceStatus{
			Phase:              kusciaapisv1alpha1.TaskResourcePhaseReserving,
			StartTime:          now,
			LastTransitionTime: now,
			Conditions: []kusciaapisv1alpha1.TaskResourceCondition{{
				LastTransitionTime: now,
				Status:             v1.ConditionTrue,
				Reason:             "Create task resource from task resource group",
				Type:               kusciaapisv1alpha1.TaskResourceCondReserving,
			}},
		},
	}

	return tr, nil
}

// buildTaskResourcePods is used to build task resource pods info.
func (h *CreatingHandler) buildTaskResourcePods(namespace string, ps []kusciaapisv1alpha1.TaskResourceGroupPartyPod) ([]kusciaapisv1alpha1.TaskResourcePod, error) {
	tPods := make([]kusciaapisv1alpha1.TaskResourcePod, 0, len(ps))
	for _, p := range ps {
		pod, err := h.podLister.Pods(namespace).Get(p.Name)
		if err != nil {
			return nil, err
		}

		var resources v1.ResourceRequirements
		for _, container := range pod.Spec.Containers {
			resources.Requests = quota.Add(resources.Requests, container.Resources.Requests)
			resources.Limits = quota.Add(resources.Limits, container.Resources.Limits)
		}

		tPod := kusciaapisv1alpha1.TaskResourcePod{
			Name:      p.Name,
			Resources: resources,
		}

		tPods = append(tPods, tPod)
	}
	return tPods, nil
}

// findPartyTaskResource is used to find specific task resource.
func findPartyTaskResource(party kusciaapisv1alpha1.TaskResourceGroupParty, trs []*kusciaapisv1alpha1.TaskResource) *kusciaapisv1alpha1.TaskResource {
	var count int
	for i, tr := range trs {
		if len(tr.Spec.Pods) != len(party.Pods) {
			continue
		}

		count = 0
		for _, tp := range tr.Spec.Pods {
			for _, pp := range party.Pods {
				if pp.Name == tp.Name {
					count++
					if count == len(party.Pods) {
						return trs[i]
					}
				}
			}
		}
	}
	return nil
}

// getMinReservedPods is used to get min reserved pods value.
func getMinReservedPods(party *kusciaapisv1alpha1.TaskResourceGroupParty) int {
	if party.MinReservedPods > 0 && party.MinReservedPods <= len(party.Pods) {
		return party.MinReservedPods
	}
	return len(party.Pods)
}

// generateTaskResourceName is used to generate task resource name.
func generateTaskResourceName(prefix string) string {
	uid := strings.Split(string(uuid.NewUUID()), "-")
	return prefix + "-" + uid[len(uid)-1]
}
