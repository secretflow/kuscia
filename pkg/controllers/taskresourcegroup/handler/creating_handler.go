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

//nolint:dupl
package handler

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	quota "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// CreatingHandler is used to handle task resource group which phase is creating.
type CreatingHandler struct {
	kubeClient      kubernetes.Interface
	kusciaClient    kusciaclientset.Interface
	namespaceLister listers.NamespaceLister
	podLister       listers.PodLister
	trLister        kuscialistersv1alpha1.TaskResourceLister
}

// NewCreatingHandler returns a CreatingHandler instance.
func NewCreatingHandler(deps *Dependencies) *CreatingHandler {
	return &CreatingHandler{
		kubeClient:      deps.KubeClient,
		kusciaClient:    deps.KusciaClient,
		namespaceLister: deps.NamespaceLister,
		podLister:       deps.PodLister,
		trLister:        deps.TrLister,
	}
}

// Handle is used to perform the real logic.
func (h *CreatingHandler) Handle(trg *kusciaapisv1alpha1.TaskResourceGroup) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	trg.Status.Phase = kusciaapisv1alpha1.TaskResourceGroupPhaseReserving
	trg.Status.LastTransitionTime = &now

	cond, _ := utilsres.GetTaskResourceGroupCondition(&trg.Status, kusciaapisv1alpha1.TaskResourcesCreated)
	if err = h.createTaskResources(trg); err != nil {
		needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionFalse, fmt.Sprintf("Failed to create task resource, %v", err.Error()))
		return needUpdate, err
	}

	needUpdate = utilsres.SetTaskResourceGroupCondition(&now, cond, v1.ConditionTrue, "")
	return needUpdate, nil
}

// createTaskResource is used to create task resources.
func (h *CreatingHandler) createTaskResources(trg *kusciaapisv1alpha1.TaskResourceGroup) error {
	interConnDomains := utilsres.GetInterConnParties(trg.Annotations)
	// Both initiator and partner need to create other's task resource in bfia protocol
	// The above creation is done by kuscia task resource controller in kuscia protocol
	if utilsres.SelfClusterAsInitiator(h.namespaceLister, trg.Spec.Initiator, trg.Annotations) || utilsres.IsBFIAResource(trg) {
		for _, party := range trg.Spec.OutOfControlledParties {
			_, err := h.trLister.TaskResources(party.DomainID).Get(party.TaskResourceName)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					buildTr, err := h.buildTaskResource(&party, trg, interConnDomains)
					if err != nil {
						return err
					}

					nlog.Infof("Create task resource %v of task resource group %v", party.TaskResourceName, trg.Name)
					_, err = h.kusciaClient.KusciaV1alpha1().TaskResources(party.DomainID).Create(context.Background(), buildTr, metav1.CreateOptions{})
					if err != nil && !k8serrors.IsAlreadyExists(err) {
						return fmt.Errorf("create task resource %v/%v failed, %v", buildTr.Namespace, buildTr.Name, err)
					}
				} else {
					return fmt.Errorf("failed to get task resource %v of task resource group %v", party.TaskResourceName, trg.Name)
				}
			}
		}
	}

	for _, party := range trg.Spec.Parties {
		tr, err := h.trLister.TaskResources(party.DomainID).Get(party.TaskResourceName)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				buildTr, err := h.buildTaskResource(&party, trg, interConnDomains)
				if err != nil {
					return err
				}

				nlog.Infof("Create task resource %v of task resource group %v", party.TaskResourceName, trg.Name)
				tr, err = h.kusciaClient.KusciaV1alpha1().TaskResources(party.DomainID).Create(context.Background(), buildTr, metav1.CreateOptions{})
				if err != nil && !k8serrors.IsAlreadyExists(err) {
					return fmt.Errorf("create task resource %v/%v failed, %v", buildTr.Namespace, buildTr.Name, err)
				}
			} else {
				return fmt.Errorf("failed to get task resource %v of task resource group %v", party.TaskResourceName, trg.Name)
			}
		}

		for _, p := range party.Pods {
			pod, err := h.podLister.Pods(party.DomainID).Get(p.Name)
			if err != nil {
				return err
			}

			if pod.Labels[kusciaapisv1alpha1.TaskResourceUID] == string(tr.UID) &&
				pod.Annotations[kusciaapisv1alpha1.TaskResourceKey] == tr.Name {
				continue
			}

			podCopy := pod.DeepCopy()
			if podCopy.Labels == nil {
				podCopy.Labels = map[string]string{}
			}
			if podCopy.Annotations == nil {
				podCopy.Annotations = map[string]string{}
			}

			podCopy.Labels[common.LabelTaskResourceGroupUID] = string(trg.UID)
			podCopy.Labels[kusciaapisv1alpha1.TaskResourceUID] = string(tr.UID)
			podCopy.Annotations[kusciaapisv1alpha1.TaskResourceKey] = party.TaskResourceName
			oldExtractedPod := utilsres.ExtractPodAnnotationsAndLabels(pod)
			newExtractedPod := utilsres.ExtractPodAnnotationsAndLabels(podCopy)
			if err = utilsres.PatchPod(context.Background(), h.kubeClient, oldExtractedPod, newExtractedPod); err != nil {
				return fmt.Errorf("patch pod %v/%v label failed, %v", pod.Namespace, pod.Name, err)
			}
		}
	}

	return nil
}

// buildTaskResource is used to build task resource.
func (h *CreatingHandler) buildTaskResource(party *kusciaapisv1alpha1.TaskResourceGroupParty,
	trg *kusciaapisv1alpha1.TaskResourceGroup,
	interConnDomains map[string]string) (*kusciaapisv1alpha1.TaskResource, error) {
	var err error
	var trPods []kusciaapisv1alpha1.TaskResourcePod
	trPods, err = h.buildTaskResourcePods(party.DomainID, party.Pods)
	if err != nil {
		return nil, fmt.Errorf("build task resource %v/%v pods failed, %v", party.DomainID, trg.Name, err.Error())
	}

	var jobID, taskID, taskAlias, protocolType, masterDomain, asInitiator string
	if trg.Annotations != nil {
		jobID = trg.Annotations[common.JobIDAnnotationKey]
		taskID = trg.Annotations[common.TaskIDAnnotationKey]
		taskAlias = trg.Annotations[common.TaskAliasAnnotationKey]
		masterDomain = trg.Annotations[common.KusciaPartyMasterDomainAnnotationKey]
		protocolType = trg.Labels[common.LabelInterConnProtocolType]
		asInitiator = trg.Labels[common.SelfClusterAsInitiatorAnnotationKey]
	}

	phase := kusciaapisv1alpha1.TaskResourcePhaseReserving
	condType := kusciaapisv1alpha1.TaskResourceCondReserving
	condReason := "Create task resource from task resource group"
	// Partners do not care about others resource status, just set all other task resource phase to reserved.
	if !utilsres.SelfClusterAsInitiator(h.namespaceLister, trg.Spec.Initiator, nil) &&
		utilsres.IsOuterBFIAInterConnDomain(h.namespaceLister, party.DomainID) {
		phase = kusciaapisv1alpha1.TaskResourcePhaseReserved
		condType = kusciaapisv1alpha1.TaskResourceCondReserved
		condReason = "Create task resource for outer domain"
	}

	isPartner, err := utilsres.IsPartnerDomain(h.namespaceLister, party.DomainID)
	if err != nil {
		return nil, err
	}
	minReservedPods := getMinReservedPods(party, isPartner)

	now := metav1.Now()
	tr := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: party.DomainID,
			Name:      party.TaskResourceName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(trg, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("TaskResourceGroup")),
			},
			Labels: map[string]string{
				common.LabelTaskResourceGroupUID:  string(trg.UID),
				common.LabelInterConnProtocolType: trg.Labels[common.LabelInterConnProtocolType],
			},
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:               trg.Spec.Initiator,
				common.SelfClusterAsInitiatorAnnotationKey:  asInitiator,
				common.JobIDAnnotationKey:                   jobID,
				common.TaskIDAnnotationKey:                  taskID,
				common.TaskAliasAnnotationKey:               taskAlias,
				common.TaskResourceGroupAnnotationKey:       trg.Name,
				common.KusciaPartyMasterDomainAnnotationKey: masterDomain,
			},
		},

		Spec: kusciaapisv1alpha1.TaskResourceSpec{
			Role:                    party.Role,
			MinReservedPods:         minReservedPods,
			ResourceReservedSeconds: trg.Spec.ResourceReservedSeconds,
			Initiator:               trg.Spec.Initiator,
			Pods:                    trPods,
		},
		Status: kusciaapisv1alpha1.TaskResourceStatus{
			Phase:              phase,
			StartTime:          &now,
			LastTransitionTime: &now,
			Conditions: []kusciaapisv1alpha1.TaskResourceCondition{{
				LastTransitionTime: &now,
				Status:             v1.ConditionTrue,
				Reason:             condReason,
				Type:               condType,
			}},
		},
	}

	if interConnPartyAnnotation, ok := interConnDomains[party.DomainID]; ok {
		tr.Annotations[interConnPartyAnnotation] = party.DomainID
		if protocolType == "" {
			protocolType = string(utilsres.GetInterConnProtocolTypeByPartyAnnotation(interConnPartyAnnotation))
		}
	}

	if protocolType != "" {
		tr.Labels[common.LabelInterConnProtocolType] = protocolType
	}

	return tr, nil
}

// buildTaskResourcePods is used to build task resource pods info.
func (h *CreatingHandler) buildTaskResourcePods(namespace string, ps []kusciaapisv1alpha1.TaskResourceGroupPartyPod) ([]kusciaapisv1alpha1.TaskResourcePod, error) {
	tPods := make([]kusciaapisv1alpha1.TaskResourcePod, 0, len(ps))
	isPartner, err := utilsres.IsPartnerDomain(h.namespaceLister, namespace)
	if err != nil {
		return nil, err
	}
	if isPartner {
		for _, p := range ps {
			tPod := kusciaapisv1alpha1.TaskResourcePod{
				Name: p.Name,
			}
			tPods = append(tPods, tPod)
		}
	} else {
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
func getMinReservedPods(party *kusciaapisv1alpha1.TaskResourceGroupParty, isPartner bool) int {
	if isPartner {
		return party.MinReservedPods
	}

	if party.MinReservedPods > 0 && party.MinReservedPods <= len(party.Pods) {
		return party.MinReservedPods
	}
	return len(party.Pods)
}
