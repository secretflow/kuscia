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

package container

import (
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// Status is the status of a container.
type Status struct {
	// Pid is the init process id of the container.
	Pid int
	// CreatedAt is the created timestamp.
	CreatedAt int64
	// StartedAt is the started timestamp.
	StartedAt int64
	// FinishedAt is the finished timestamp.
	FinishedAt int64
	// ExitCode is the container exit code.
	ExitCode int
	// CamelCase string explaining why container is in its current state.
	Reason string
	// Human-readable message indicating details about why container is in its
	// current state.
	Message string
	// Unknown indicates that the container status is not fully loaded.
	// This field doesn't need to be checkpointed.
	Unknown bool `json:"-"`
	// Released indicates that the container is released.
	Released bool
	// Resources has container runtime resource constraints
	Resources *runtime.ContainerResources
}

// State returns current state of the container based on the container status.
func (s *Status) State() runtime.ContainerState {
	if s.Unknown {
		return runtime.ContainerState_CONTAINER_UNKNOWN
	}
	if s.FinishedAt != 0 {
		return runtime.ContainerState_CONTAINER_EXITED
	}
	if s.StartedAt != 0 {
		return runtime.ContainerState_CONTAINER_RUNNING
	}
	if s.CreatedAt != 0 {
		return runtime.ContainerState_CONTAINER_CREATED
	}
	return runtime.ContainerState_CONTAINER_UNKNOWN
}
