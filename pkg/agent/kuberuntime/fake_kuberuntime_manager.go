/*
Copyright 2016 The Kubernetes Authors.

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

// Modified by Ant Group in 2023.

package kuberuntime

import (
	"time"

	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/component-base/logs/logreduction"
	internalapi "k8s.io/cri-api/pkg/apis"
	"k8s.io/kubernetes/pkg/kubelet/logs"

	"github.com/secretflow/kuscia/pkg/agent/images"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"

	proberesults "github.com/secretflow/kuscia/pkg/agent/prober/results"
)

type fakePodStateProvider struct {
	terminated map[types.UID]struct{}
	removed    map[types.UID]struct{}
}

func newFakePodStateProvider() *fakePodStateProvider {
	return &fakePodStateProvider{
		terminated: make(map[types.UID]struct{}),
		removed:    make(map[types.UID]struct{}),
	}
}

func (f *fakePodStateProvider) IsPodTerminationRequested(uid types.UID) bool {
	_, found := f.removed[uid]
	return found
}

func (f *fakePodStateProvider) ShouldPodRuntimeBeRemoved(uid types.UID) bool {
	_, found := f.terminated[uid]
	return found
}

func (f *fakePodStateProvider) ShouldPodContentBeRemoved(uid types.UID) bool {
	_, found := f.removed[uid]
	return found
}

func newFakeKubeRuntimeManager(runtimeService internalapi.RuntimeService, imageService internalapi.ImageManagerService, osInterface pkgcontainer.OSInterface, runtimeHelper pkgcontainer.RuntimeHelper) (*kubeGenericRuntimeManager, error) {
	recorder := &record.FakeRecorder{}
	logManager, err := logs.NewContainerLogManager(runtimeService, osInterface, "1", 2)
	if err != nil {
		return nil, err
	}
	kubeRuntimeManager := &kubeGenericRuntimeManager{
		recorder:        recorder,
		cpuCFSQuota:     false,
		livenessManager: proberesults.NewManager(),
		startupManager:  proberesults.NewManager(),
		osInterface:     osInterface,
		runtimeHelper:   runtimeHelper,
		runtimeService:  runtimeService,
		imageService:    imageService,
		logReduction:    logreduction.NewLogReduction(identicalErrorDelay),
		logManager:      logManager,
	}

	typedVersion, err := runtimeService.Version(context.Background(), kubeRuntimeAPIVersion)
	if err != nil {
		return nil, err
	}

	podStateProvider := newFakePodStateProvider()
	kubeRuntimeManager.containerGC = newContainerGC(runtimeService, podStateProvider, kubeRuntimeManager)
	kubeRuntimeManager.podStateProvider = podStateProvider
	kubeRuntimeManager.runtimeName = typedVersion.RuntimeName
	kubeRuntimeManager.imagePuller = images.NewImageManager(
		pkgcontainer.FilterEventRecorder(recorder),
		kubeRuntimeManager,
		flowcontrol.NewBackOff(time.Second, 300*time.Second),
		false,
		0, // Disable image pull throttling by setting QPS to 0,
		0,
	)

	return kubeRuntimeManager, nil
}
