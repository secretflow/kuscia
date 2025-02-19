// Copyright 2024 Ant Group Co., Ltd.
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

package garbagecollection

import (
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

const (
	// usually, a pod related path is combined like namespace_podname_uuid, for example:
	// alice_secretflow-task-20241211172300-psi-0_12345678-1234-1234-1234-12345678abcd
	// according to input rules, namespace and podname shouldn't have '_'
	// pod might have multiple containers, and all their logs will be checked
	defaultPodDirName = ".*_(.*)_.*"
)

type GCConfig struct {
	// namespace
	Namespace string
	// kube client
	KubeClient kubernetes.Interface
	// the interval of every gc operation
	GCInterval time.Duration
	// maximum number of resources to be deleted in one gc operation
	MaxDeleteNum int
}

func DefaultGCConfig() GCConfig {
	return GCConfig{
		GCInterval:   10 * time.Minute,
		MaxDeleteNum: 100,
	}
}

// the path attribute that should be checked by gc operation
// default procedure: list all subdirs and files -> check pattern ->
// check duration -> recursively check duration -> check pod status
type LogFileGCConfig struct {
	GCConfig

	PodNamePattern           string
	IsDurationCheck          bool
	IsRecursiveDurationCheck bool
	// the duration of resource to be deleted
	GCDuration  time.Duration
	LogFilePath string
}

func DefaultLogFileGCConfig() LogFileGCConfig {
	return LogFileGCConfig{
		GCConfig:                 DefaultGCConfig(),
		PodNamePattern:           defaultPodDirName,
		IsDurationCheck:          true,
		IsRecursiveDurationCheck: true,
		GCDuration:               config.DefaultLogRotateMaxAgeDays * 24 * time.Hour,
	}
}
