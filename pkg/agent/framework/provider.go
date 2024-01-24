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

package framework

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
)

// NodeProvider is the interface used for registering a node and updating its
// status in Kubernetes.
//
// Note: Implementers can choose to manage a node themselves, in which case
// it is not needed to provide an implementation for this interface.
type NodeProvider interface { // nolint:golint
	// Ping checks if the node is still active.
	// This is intended to be lightweight as it will be called periodically as a
	// heartbeat to keep the node marked as ready in Kubernetes.
	Ping(context.Context) error

	// ConfigureNode enables a provider to configure the node object that
	// will be used for Kubernetes.
	ConfigureNode(context.Context, string) *v1.Node

	// RefreshNodeStatus return if the node status changes
	RefreshNodeStatus(ctx context.Context, nodeStatus *v1.NodeStatus) bool

	// SetStatusUpdateCallback is used to asynchronously monitor the node.
	// The passed in callback should be called any time there is a change to the
	// node's status.
	// This will generally trigger a call to the Kubernetes API server to update
	// the status.
	//
	// SetStatusUpdateCallback should not block callers.
	SetStatusUpdateCallback(ctx context.Context, cb func(*v1.Node))
}

// PodLifecycleHandler defines the interface used by the PodsController to react
// to new and changed pods scheduled to the node that is being managed.
type PodLifecycleHandler interface {
	SyncPod(ctx context.Context, pod *v1.Pod, podStatus *pkgcontainer.PodStatus, reasonCache *ReasonCache) error

	KillPod(ctx context.Context, pod *v1.Pod, runningPod pkgcontainer.Pod, gracePeriodOverride *int64) error

	// DeletePod Pod object in master is gone, so just delete pod in provider and no need to call NotifyPods
	// after deletion.
	DeletePod(ctx context.Context, pod *v1.Pod) error

	CleanupPods(ctx context.Context, pods []*v1.Pod, runningPods []*pkgcontainer.Pod, possiblyRunningPods map[types.UID]sets.Empty) error

	RefreshPodStatus(pod *v1.Pod, podStatus *v1.PodStatus)

	GetPodStatus(ctx context.Context, pod *v1.Pod) (*pkgcontainer.PodStatus, error)

	GetPods(ctx context.Context, all bool) ([]*pkgcontainer.Pod, error)

	// Start sync loop
	Start(ctx context.Context) error

	// Stop sync loop
	Stop()
}

type PodProvider interface {
	PodLifecycleHandler
}
