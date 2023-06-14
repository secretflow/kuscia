/*
Copyright 2015 The Kubernetes Authors.

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

package testing

import (
	"context"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/flowcontrol"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/volume"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
)

type FakePod struct {
	Pod       *pkgcontainer.Pod
	NetnsPath string
}

// FakeRuntime is a fake container runtime for testing.
type FakeRuntime struct {
	sync.Mutex
	CalledFunctions   []string
	PodList           []*FakePod
	AllPodList        []*FakePod
	ImageList         []pkgcontainer.Image
	APIPodStatus      v1.PodStatus
	PodStatus         pkgcontainer.PodStatus
	StartedPods       []string
	KilledPods        []string
	StartedContainers []string
	KilledContainers  []string
	RuntimeStatus     *pkgcontainer.RuntimeStatus
	VersionInfo       string
	APIVersionInfo    string
	RuntimeType       string
	Err               error
	InspectErr        error
	StatusErr         error
	T                 *testing.T
}

const FakeHost = "localhost:12345"

type FakeStreamingRuntime struct {
	*FakeRuntime
}

// FakeRuntime should implement Runtime.
var _ pkgcontainer.Runtime = &FakeRuntime{}

type FakeVersion struct {
	Version string
}

func (fv *FakeVersion) String() string {
	return fv.Version
}

func (fv *FakeVersion) Compare(other string) (int, error) {
	result := 0
	if fv.Version > other {
		result = 1
	} else if fv.Version < other {
		result = -1
	}
	return result, nil
}

type podsGetter interface {
	GetPods(bool) ([]*pkgcontainer.Pod, error)
}

type FakeRuntimeCache struct {
	getter podsGetter
}

func NewFakeRuntimeCache(getter podsGetter) pkgcontainer.RuntimeCache {
	return &FakeRuntimeCache{getter}
}

func (f *FakeRuntimeCache) GetPods() ([]*pkgcontainer.Pod, error) {
	return f.getter.GetPods(false)
}

func (f *FakeRuntimeCache) ForceUpdateIfOlder(time.Time) error {
	return nil
}

// ClearCalls resets the FakeRuntime to the initial state.
func (f *FakeRuntime) ClearCalls() {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = []string{}
	f.PodList = []*FakePod{}
	f.AllPodList = []*FakePod{}
	f.APIPodStatus = v1.PodStatus{}
	f.StartedPods = []string{}
	f.KilledPods = []string{}
	f.StartedContainers = []string{}
	f.KilledContainers = []string{}
	f.RuntimeStatus = nil
	f.VersionInfo = ""
	f.RuntimeType = ""
	f.Err = nil
	f.InspectErr = nil
	f.StatusErr = nil
}

// UpdatePodCIDR fulfills the cri interface.
func (f *FakeRuntime) UpdatePodCIDR(c string) error {
	return nil
}

func (f *FakeRuntime) assertList(expect []string, test []string) bool {
	if !reflect.DeepEqual(expect, test) {
		f.T.Errorf("AssertList: expected %#v, got %#v", expect, test)
		return false
	}
	return true
}

// AssertCalls test if the invoked functions are as expected.
func (f *FakeRuntime) AssertCalls(calls []string) bool {
	f.Lock()
	defer f.Unlock()
	return f.assertList(calls, f.CalledFunctions)
}

func (f *FakeRuntime) AssertStartedPods(pods []string) bool {
	f.Lock()
	defer f.Unlock()
	return f.assertList(pods, f.StartedPods)
}

func (f *FakeRuntime) AssertKilledPods(pods []string) bool {
	f.Lock()
	defer f.Unlock()
	return f.assertList(pods, f.KilledPods)
}

func (f *FakeRuntime) AssertStartedContainers(containers []string) bool {
	f.Lock()
	defer f.Unlock()
	return f.assertList(containers, f.StartedContainers)
}

func (f *FakeRuntime) AssertKilledContainers(containers []string) bool {
	f.Lock()
	defer f.Unlock()
	return f.assertList(containers, f.KilledContainers)
}

func (f *FakeRuntime) Type() string {
	return f.RuntimeType
}

func (f *FakeRuntime) Version() (pkgcontainer.Version, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "Version")
	return &FakeVersion{Version: f.VersionInfo}, f.Err
}

func (f *FakeRuntime) APIVersion() (pkgcontainer.Version, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "APIVersion")
	return &FakeVersion{Version: f.APIVersionInfo}, f.Err
}

func (f *FakeRuntime) Status() (*pkgcontainer.RuntimeStatus, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "Status")
	return f.RuntimeStatus, f.StatusErr
}

func (f *FakeRuntime) GetPods(all bool) ([]*pkgcontainer.Pod, error) {
	f.Lock()
	defer f.Unlock()

	var pods []*pkgcontainer.Pod

	f.CalledFunctions = append(f.CalledFunctions, "GetPods")
	if all {
		for _, fakePod := range f.AllPodList {
			pods = append(pods, fakePod.Pod)
		}
	} else {
		for _, fakePod := range f.PodList {
			pods = append(pods, fakePod.Pod)
		}
	}
	return pods, f.Err
}

func (f *FakeRuntime) SyncPod(pod *v1.Pod, _ *pkgcontainer.PodStatus, _ *credentialprovider.AuthConfig, backOff *flowcontrol.Backoff) (result pkgcontainer.PodSyncResult) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "SyncPod")
	f.StartedPods = append(f.StartedPods, string(pod.UID))
	for _, c := range pod.Spec.Containers {
		f.StartedContainers = append(f.StartedContainers, c.Name)
	}
	// TODO(random-liu): Add SyncResult for starting and killing containers
	if f.Err != nil {
		result.Fail(f.Err)
	}
	return
}

func (f *FakeRuntime) KillPod(pod *v1.Pod, runningPod pkgcontainer.Pod, gracePeriodOverride *int64) error {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "KillPod")
	f.KilledPods = append(f.KilledPods, string(runningPod.ID))
	for _, c := range runningPod.Containers {
		f.KilledContainers = append(f.KilledContainers, c.Name)
	}
	return f.Err
}

func (f *FakeRuntime) RunContainerInPod(container v1.Container, pod *v1.Pod, volumeMap map[string]volume.VolumePlugin) error {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "RunContainerInPod")
	f.StartedContainers = append(f.StartedContainers, container.Name)

	pod.Spec.Containers = append(pod.Spec.Containers, container)
	for _, c := range pod.Spec.Containers {
		if c.Name == container.Name { // Container already in the pod.
			return f.Err
		}
	}
	pod.Spec.Containers = append(pod.Spec.Containers, container)
	return f.Err
}

func (f *FakeRuntime) KillContainerInPod(container v1.Container, pod *v1.Pod) error {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "KillContainerInPod")
	f.KilledContainers = append(f.KilledContainers, container.Name)
	return f.Err
}

func (f *FakeRuntime) GetPodStatus(uid types.UID, name, namespace string) (*pkgcontainer.PodStatus, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "GetPodStatus")
	status := f.PodStatus
	return &status, f.Err
}

func (f *FakeRuntime) GetContainerLogs(_ context.Context, pod *v1.Pod, containerID pkgcontainer.CtrID, logOptions *v1.PodLogOptions, stdout, stderr io.Writer) (err error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "GetContainerLogs")
	return f.Err
}

func (f *FakeRuntime) PullImage(image pkgcontainer.ImageSpec, auth *credentialprovider.AuthConfig, podSandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "PullImage")
	if f.Err == nil {
		i := pkgcontainer.Image{
			ID:   image.Image,
			Spec: image,
		}
		f.ImageList = append(f.ImageList, i)
	}
	return image.Image, f.Err
}

func (f *FakeRuntime) GetImageRef(image pkgcontainer.ImageSpec) (string, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "GetImageRef")
	for _, i := range f.ImageList {
		if i.ID == image.Image {
			return i.ID, nil
		}
	}
	return "", f.InspectErr
}

func (f *FakeRuntime) ListImages() ([]pkgcontainer.Image, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "ListImages")
	return f.ImageList, f.Err
}

func (f *FakeRuntime) RemoveImage(image pkgcontainer.ImageSpec) error {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "RemoveImage")
	index := 0
	for i := range f.ImageList {
		if f.ImageList[i].ID == image.Image {
			index = i
			break
		}
	}
	f.ImageList = append(f.ImageList[:index], f.ImageList[index+1:]...)

	return f.Err
}

func (f *FakeRuntime) GarbageCollect(gcPolicy pkgcontainer.GCPolicy, ready bool, evictNonDeletedPods bool) error {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "GarbageCollect")
	return f.Err
}

func (f *FakeRuntime) DeleteContainer(containerID pkgcontainer.CtrID) error {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "DeleteContainer")
	return f.Err
}

func (f *FakeRuntime) ImageStats() (*pkgcontainer.ImageStats, error) {
	f.Lock()
	defer f.Unlock()

	f.CalledFunctions = append(f.CalledFunctions, "ImageStats")
	return nil, f.Err
}
