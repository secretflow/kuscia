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

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

const (
	labelKusciaTaskPodIdentity = "kuscia.secretflow/pod-identity"
	labelKusciaTaskPodRole     = "kuscia.secretflow/pod-role"

	configTemplateVolumeName = "config-template"
)

const (
	defaultResourceReservedSeconds = 30
	defaultLifecycleSeconds        = 300
	defaultRetryIntervalSeconds    = 30
)

func getTaskResourceGroup(ctx context.Context,
	name string,
	trgLister kuscialistersv1alpha1.TaskResourceGroupLister,
	kusciaClient kusciaclientset.Interface) (*kusciaapisv1alpha1.TaskResourceGroup, error) {
	trg, err := trgLister.Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			trg, err = kusciaClient.KusciaV1alpha1().TaskResourceGroups().Get(ctx, name, metav1.GetOptions{})
		}
		if err != nil {
			return nil, err
		}
	}
	return trg, nil
}

func refreshKtResourcesStatus(kubeClient kubernetes.Interface,
	podsLister corelisters.PodLister,
	servicesLister corelisters.ServiceLister,
	ktStatus *kusciaapisv1alpha1.KusciaTaskStatus) {
	refreshPodStatuses(kubeClient, podsLister, ktStatus.PodStatuses)
	refreshServiceStatuses(kubeClient, servicesLister, ktStatus.ServiceStatuses)
}

func refreshPodStatuses(kubeClient kubernetes.Interface,
	podsLister corelisters.PodLister,
	podStatuses map[string]*kusciaapisv1alpha1.PodStatus) {
	for _, st := range podStatuses {
		if st.PodPhase == v1.PodSucceeded || st.PodPhase == v1.PodFailed {
			continue
		}

		ns := st.Namespace
		name := st.PodName
		pod, err := podsLister.Pods(ns).Get(name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				pod, err = kubeClient.CoreV1().Pods(ns).Get(context.Background(), name, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					st.PodPhase = v1.PodFailed
					st.Reason = "PodNotExist"
					st.Message = "Does not find the pod"
					continue
				}
			}

			if err != nil {
				st.PodPhase = v1.PodFailed
				st.Reason = "GetPodFailed"
				st.Message = err.Error()
			}
			continue
		}

		st.PodPhase = pod.Status.Phase
		st.NodeName = pod.Spec.NodeName
		st.Reason = pod.Status.Reason
		st.Message = pod.Status.Message

		// check pod container terminated state
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				if st.Reason == "" && cs.State.Terminated.Reason != "" {
					st.Reason = cs.State.Terminated.Reason
				}
				if cs.State.Terminated.Message != "" {
					st.TerminationLog = fmt.Sprintf("container[%v] terminated state reason %q, message: %q", cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.Message)
				}
			} else if cs.LastTerminationState.Terminated != nil {
				if st.Reason == "" && cs.LastTerminationState.Terminated.Reason != "" {
					st.Reason = cs.LastTerminationState.Terminated.Reason
				}
				if cs.LastTerminationState.Terminated.Message != "" {
					st.TerminationLog = fmt.Sprintf("container[%v] last terminated state reason %q, message: %q", cs.Name, cs.LastTerminationState.Terminated.Reason, cs.LastTerminationState.Terminated.Message)
				}
			}

			// set terminated log from one of containers
			if st.TerminationLog != "" {
				break
			}
		}

		if st.Reason != "" && st.Message != "" {
			return
		}

		// podStatus createTime
		st.CreateTime = func() *metav1.Time {
			createTime := pod.GetCreationTimestamp()
			return &createTime
		}()

		// podStatus startTime
		st.StartTime = pod.Status.StartTime

		// Check if the pod has been scheduled
		// set scheduleTime、readyTime
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse {
				if st.Reason == "" {
					st.Reason = cond.Reason
				}

				if st.Message == "" {
					st.Message = cond.Message
				}
				return
			}
			// cond.Status=True indicates complete availability
			if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
				st.ReadyTime = func() *metav1.Time {
					readyTime := cond.LastTransitionTime
					return &readyTime
				}()
			}
		}

		// check pod container waiting state
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				if st.Reason == "" && cs.State.Waiting.Reason != "" {
					st.Reason = cs.State.Waiting.Reason
				}

				if cs.State.Waiting.Message != "" {
					st.Message = fmt.Sprintf("container[%v] waiting state reason: %q, message: %q", cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
				}
			}

			if st.Message != "" {
				return
			}
		}
	}
}

func refreshServiceStatuses(kubeClient kubernetes.Interface,
	servicesLister corelisters.ServiceLister,
	serviceStatuses map[string]*kusciaapisv1alpha1.ServiceStatus) {
	for _, st := range serviceStatuses {
		ns := st.Namespace
		name := st.ServiceName
		srv, err := servicesLister.Services(ns).Get(name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				srv, err = kubeClient.CoreV1().Services(ns).Get(context.Background(), name, metav1.GetOptions{})
				if k8serrors.IsNotFound(err) {
					st.Reason = "ServiceNotExist"
					st.Message = "Does not find the service"
					continue
				}
			}

			if err != nil {
				st.Reason = "GetServiceFailed"
				st.Message = err.Error()
			}
			continue
		}
		st.CreateTime = func() *metav1.Time {
			createTime := srv.GetCreationTimestamp()
			return &createTime
		}()

		// set readyTime
		if v, ok := srv.Annotations[common.ReadyTimeAnnotationKey]; ok {
			st.ReadyTime = func() *metav1.Time {
				readyTime := &metav1.Time{}
				readyTime.UnmarshalQueryParameter(v)
				return readyTime
			}()
		}
	}
}
