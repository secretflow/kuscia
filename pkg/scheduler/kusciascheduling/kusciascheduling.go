/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kusciascheduling

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformer "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/scheduler/kusciascheduling/core"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// KusciaScheduling is a plugin that schedules pods in a group.
type KusciaScheduling struct {
	frameworkHandler        framework.Handle
	trMgr                   core.Manager
	resourceReservedSeconds *time.Duration
}

var _ framework.PreFilterPlugin = &KusciaScheduling{}
var _ framework.PostFilterPlugin = &KusciaScheduling{}
var _ framework.ReservePlugin = &KusciaScheduling{}
var _ framework.PermitPlugin = &KusciaScheduling{}
var _ framework.PreBindPlugin = &KusciaScheduling{}
var _ framework.PostBindPlugin = &KusciaScheduling{}
var _ framework.EnqueueExtensions = &KusciaScheduling{}

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "KusciaScheduling"
)

// New initializes and returns a new KusciaScheduling plugin.
func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	args, err := parseArgs(obj)
	if err != nil {
		nlog.Warnf("Can't parse task resource args, %v", err)
		return nil, err
	}

	nlog.Infof("%v plugin args: ResourceReservedSeconds=%d", Name, args.ResourceReservedSeconds)

	kubeConfig := *handle.KubeConfig()
	kubeConfig.ContentType = "application/json"

	kusciaClient := kusciaclientset.NewForConfigOrDie(&kubeConfig)
	kusciaInformerFactory := kusciainformer.NewSharedInformerFactory(kusciaClient, 0)
	trInformer := kusciaInformerFactory.Kuscia().V1alpha1().TaskResources()
	podInformer := handle.SharedInformerFactory().Core().V1().Pods()
	nsInformer := handle.SharedInformerFactory().Core().V1().Namespaces()

	var timeout time.Duration
	if args != nil && args.ResourceReservedSeconds > 0 {
		timeout = time.Duration(args.ResourceReservedSeconds)
	}

	trMgr := core.NewTaskResourceManager(kusciaClient, handle.SnapshotSharedLister(), trInformer, podInformer, nsInformer, &timeout)
	ks := &KusciaScheduling{
		frameworkHandler:        handle,
		trMgr:                   trMgr,
		resourceReservedSeconds: &timeout,
	}

	ctx := context.Background()
	kusciaInformerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), trInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("failed to wait for cache sync for %v scheduler plugin", Name)
	}

	return ks, nil
}

// parseArgs parses plugin arguments.
func parseArgs(obj runtime.Object) (*kusciaapisv1alpha1.SchedulerPluginArgs, error) {
	if obj == nil {
		return nil, nil
	}

	ob, ok := obj.(*runtime.Unknown)
	if !ok {
		return nil, fmt.Errorf("obj type is not runtime.Unknown")
	}

	if ob.ContentType != "application/json" {
		return nil, fmt.Errorf("obj content type is not application/json")
	}

	var trgArgs kusciaapisv1alpha1.SchedulerPluginArgs
	if err := json.Unmarshal(ob.Raw, &trgArgs); err != nil {
		return nil, err
	}
	return &trgArgs, nil
}

func (cs *KusciaScheduling) EventsToRegister() []framework.ClusterEvent {
	// To register a custom event, follow the naming convention at:
	// https://git.k8s.io/kubernetes/pkg/scheduler/eventhandlers.go#L403-L410
	return []framework.ClusterEvent{
		{Resource: framework.Pod, ActionType: framework.Add},
	}
}

// Name returns name of the plugin. It is used in logs, etc.
func (cs *KusciaScheduling) Name() string {
	return Name
}

// PreFilter performs the following validations.
// 1. Whether the TaskResourceGroup that the Pod belongs to is on the deny list.
// 2. Whether the total number of pods in a TaskResourceGroup is less than its `minReservedMember`.
func (cs *KusciaScheduling) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	if err := cs.trMgr.PreFilter(ctx, pod); err != nil {
		return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
	}
	return nil, framework.NewStatus(framework.Success, "")
}

// PostFilter is used to reject a group of pods if a pod does not pass PreFilter or Filter.
func (cs *KusciaScheduling) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	filteredNodeStatusMap framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	_, tr, labelExist := cs.trMgr.GetTaskResource(pod)
	if !labelExist {
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable, "task resource does not exist")
	}

	if tr == nil {
		nlog.Warnf("Can't find related task resource for pod %v/%v", pod.Namespace, pod.Name)
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable, "can not find related task resource")
	}

	// This indicates there are already enough Pods satisfying the TaskResource,
	// so don't bother to reject the whole TaskResource.
	assigned := cs.trMgr.CalculateAssignedPods(tr, pod)
	if assigned >= tr.Spec.MinReservedPods {
		nlog.Infof("PostFilter assigned pods count %v is greater than minReservedPods %v for task resource %v/%v", assigned, tr.Spec.MinReservedPods, tr.Namespace, tr.Name)
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Success)
	}

	// It's based on an implicit assumption: if the nth Pod failed,
	// it's inferrable other Pods belonging to the same TaskResource would be very likely to fail.
	cs.frameworkHandler.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		trName, _ := core.GetTaskResourceName(waitingPod.GetPod())
		if trName == tr.Name && waitingPod.GetPod().Namespace == pod.Namespace {
			nlog.Infof("PostFilter rejects the waiting pod %s/%s under task resource %v", pod.Namespace, pod.Name, tr.Name)
			waitingPod.Reject(cs.Name(), "optimistic rejection in PostFilter")
		}
	})

	return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable,
		fmt.Sprintf("reject the pod %v even after PostFilter", pod.Name))
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one.
func (cs *KusciaScheduling) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (cs *KusciaScheduling) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	cs.trMgr.Reserve(ctx, pod)
	return nil
}

// Unreserve rejects all other Pods in the TaskResource when one of the pods in the times out.
func (cs *KusciaScheduling) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	_, tr, _ := cs.trMgr.GetTaskResource(pod)
	if tr == nil {
		return
	}
	cs.frameworkHandler.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		trName, _ := core.GetTaskResourceName(waitingPod.GetPod())
		if trName == tr.Name && waitingPod.GetPod().Namespace == pod.Namespace {
			nlog.Infof("Unreserve rejects the waiting pod %s/%s under task resource %v", pod.Namespace, pod.Name, tr.Name)
			waitingPod.Reject(cs.Name(), "rejection in Unreserve")
		}
	})
	cs.trMgr.Unreserve(ctx, tr, pod)
}

// Permit is the functions invoked by the framework at "Permit" extension point.
func (cs *KusciaScheduling) Permit(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	var (
		waitTime  time.Duration
		retStatus *framework.Status
	)
	s := cs.trMgr.Permit(ctx, pod)
	switch s {
	case core.TaskResourceNotSpecified:
		return framework.NewStatus(framework.Success, ""), 0
	case core.TaskResourceNotFound:
		return framework.NewStatus(framework.Unschedulable, "TaskResource not found"), 0
	case core.Wait:
		nlog.Infof("Pod %v/%v is waiting to be scheduled to node %v", pod.Namespace, pod.Name, nodeName)
		_, tr, _ := cs.trMgr.GetTaskResource(pod)
		if wait := core.GetWaitTimeDuration(tr, cs.resourceReservedSeconds); wait != 0 {
			waitTime = wait
		}
		retStatus = framework.NewStatus(framework.Wait)
		// We will also request to move the sibling pods back to activeQ.
		cs.trMgr.ActivateSiblings(pod, state)
	case core.Success:
		trName, _ := core.GetTaskResourceName(pod)
		cs.frameworkHandler.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
			wTrName, _ := core.GetTaskResourceName(waitingPod.GetPod())
			if wTrName == trName && waitingPod.GetPod().Namespace == pod.Namespace {
				nlog.Infof("Permit allows the waiting pod %v/%v", waitingPod.GetPod().Namespace, waitingPod.GetPod().Name)
				waitingPod.Allow(cs.Name())
			}
		})
		nlog.Infof("Permit allows the pod %v/%v", pod.Namespace, pod.Name)
		retStatus = framework.NewStatus(framework.Success)
		waitTime = 0
	}

	return retStatus, waitTime
}

// PreBind is used to patch task resource status.
func (cs *KusciaScheduling) PreBind(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	code, err := cs.trMgr.PreBind(ctx, pod)
	if err != nil {
		nlog.Warnf("PreBind pod %v/%v failed, %v", pod.Namespace, pod.Name, err)
		return framework.NewStatus(code, err.Error())
	}
	return framework.NewStatus(code, "")
}

// PostBind is called after a pod is successfully bound. These plugins are used update TaskResource when pod is bound.
func (cs *KusciaScheduling) PostBind(ctx context.Context, _ *framework.CycleState, pod *v1.Pod, nodeName string) {
	nlog.Infof("PostBind pod %v/%v", pod.Namespace, pod.Name)
	_, tr, _ := cs.trMgr.GetTaskResource(pod)
	if tr == nil {
		return
	}
	cs.trMgr.DeletePermittedTaskResource(tr)
}
