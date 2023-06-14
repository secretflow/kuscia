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
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/util/flowcontrol"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	apitest "k8s.io/cri-api/pkg/apis/testing"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/features"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	containertest "github.com/secretflow/kuscia/pkg/agent/container/testing"
	proberesults "github.com/secretflow/kuscia/pkg/agent/prober/results"
)

var (
	fakeCreatedAt int64 = 1
)

func createTestRuntimeManager() (*apitest.FakeRuntimeService, *apitest.FakeImageService, *kubeGenericRuntimeManager, error) {
	return customTestRuntimeManager()
}

func customTestRuntimeManager() (*apitest.FakeRuntimeService, *apitest.FakeImageService, *kubeGenericRuntimeManager, error) {
	fakeRuntimeService := apitest.NewFakeRuntimeService()
	fakeImageService := apitest.NewFakeImageService()
	osInterface := &containertest.FakeOS{}
	manager, err := newFakeKubeRuntimeManager(fakeRuntimeService, fakeImageService, osInterface, &containertest.FakeRuntimeHelper{})
	return fakeRuntimeService, fakeImageService, manager, err
}

// sandboxTemplate is a sandbox template to create fake sandbox.
type sandboxTemplate struct {
	pod         *v1.Pod
	attempt     uint32
	createdAt   int64
	state       runtimeapi.PodSandboxState
	running     bool
	terminating bool
}

// containerTemplate is a container template to create fake container.
type containerTemplate struct {
	pod            *v1.Pod
	container      *v1.Container
	sandboxAttempt uint32
	attempt        int
	createdAt      int64
	state          runtimeapi.ContainerState
}

// makeAndSetFakePod is a helper function to create and set one fake sandbox for a pod and
// one fake container for each of its container.
func makeAndSetFakePod(t *testing.T, m *kubeGenericRuntimeManager, fakeRuntime *apitest.FakeRuntimeService,
	pod *v1.Pod) (*apitest.FakePodSandbox, []*apitest.FakeContainer) {
	sandbox := makeFakePodSandbox(t, m, sandboxTemplate{
		pod:       pod,
		createdAt: fakeCreatedAt,
		state:     runtimeapi.PodSandboxState_SANDBOX_READY,
	})

	var containers []*apitest.FakeContainer
	newTemplate := func(c *v1.Container) containerTemplate {
		return containerTemplate{
			pod:       pod,
			container: c,
			createdAt: fakeCreatedAt,
			state:     runtimeapi.ContainerState_CONTAINER_RUNNING,
		}
	}
	podutil.VisitContainers(&pod.Spec, podutil.AllFeatureEnabledContainers(), func(c *v1.Container, containerType podutil.ContainerType) bool {
		containers = append(containers, makeFakeContainer(t, m, newTemplate(c)))
		return true
	})

	fakeRuntime.SetFakeSandboxes([]*apitest.FakePodSandbox{sandbox})
	fakeRuntime.SetFakeContainers(containers)
	return sandbox, containers
}

// makeFakePodSandbox creates a fake pod sandbox based on a sandbox template.
func makeFakePodSandbox(t *testing.T, m *kubeGenericRuntimeManager, template sandboxTemplate) *apitest.FakePodSandbox {
	config, err := m.generatePodSandboxConfig(template.pod, template.attempt)
	assert.NoError(t, err, "generatePodSandboxConfig for sandbox template %+v", template)

	podSandboxID := apitest.BuildSandboxName(config.Metadata)
	podSandBoxStatus := &apitest.FakePodSandbox{
		PodSandboxStatus: runtimeapi.PodSandboxStatus{
			Id:        podSandboxID,
			Metadata:  config.Metadata,
			State:     template.state,
			CreatedAt: template.createdAt,
			Network: &runtimeapi.PodSandboxNetworkStatus{
				Ip: apitest.FakePodSandboxIPs[0],
			},
			Labels: config.Labels,
		},
	}
	// assign additional IPs
	additionalIPs := apitest.FakePodSandboxIPs[1:]
	additionalPodIPs := make([]*runtimeapi.PodIP, 0, len(additionalIPs))
	for _, ip := range additionalIPs {
		additionalPodIPs = append(additionalPodIPs, &runtimeapi.PodIP{
			Ip: ip,
		})
	}
	podSandBoxStatus.Network.AdditionalIps = additionalPodIPs
	return podSandBoxStatus

}

// makeFakePodSandboxes creates a group of fake pod sandboxes based on the sandbox templates.
// The function guarantees the order of the fake pod sandboxes is the same with the templates.
func makeFakePodSandboxes(t *testing.T, m *kubeGenericRuntimeManager, templates []sandboxTemplate) []*apitest.FakePodSandbox {
	var fakePodSandboxes []*apitest.FakePodSandbox
	for _, template := range templates {
		fakePodSandboxes = append(fakePodSandboxes, makeFakePodSandbox(t, m, template))
	}
	return fakePodSandboxes
}

// makeFakeContainer creates a fake container based on a container template.
func makeFakeContainer(t *testing.T, m *kubeGenericRuntimeManager, template containerTemplate) *apitest.FakeContainer {
	sandboxConfig, err := m.generatePodSandboxConfig(template.pod, template.sandboxAttempt)
	assert.NoError(t, err, "generatePodSandboxConfig for container template %+v", template)

	containerConfig, _, err := m.generateContainerConfig(template.container, template.pod, template.attempt, "", template.container.Image, []string{})
	assert.NoError(t, err, "generateContainerConfig for container template %+v", template)

	podSandboxID := apitest.BuildSandboxName(sandboxConfig.Metadata)
	containerID := apitest.BuildContainerName(containerConfig.Metadata, podSandboxID)
	imageRef := containerConfig.Image.Image
	return &apitest.FakeContainer{
		ContainerStatus: runtimeapi.ContainerStatus{
			Id:          containerID,
			Metadata:    containerConfig.Metadata,
			Image:       containerConfig.Image,
			ImageRef:    imageRef,
			CreatedAt:   template.createdAt,
			State:       template.state,
			Labels:      containerConfig.Labels,
			Annotations: containerConfig.Annotations,
			LogPath:     filepath.Join(sandboxConfig.GetLogDirectory(), containerConfig.GetLogPath()),
		},
		SandboxID: podSandboxID,
	}
}

// makeFakeContainers creates a group of fake containers based on the container templates.
// The function guarantees the order of the fake containers is the same with the templates.
func makeFakeContainers(t *testing.T, m *kubeGenericRuntimeManager, templates []containerTemplate) []*apitest.FakeContainer {
	var fakeContainers []*apitest.FakeContainer
	for _, template := range templates {
		fakeContainers = append(fakeContainers, makeFakeContainer(t, m, template))
	}
	return fakeContainers
}

// makeTestContainer creates a test api container.
func makeTestContainer(name, image string) v1.Container {
	return v1.Container{
		Name:  name,
		Image: image,
	}
}

// makeTestPod creates a test api pod.
func makeTestPod(podName, podNamespace, podUID string, containers []v1.Container) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID(podUID),
			Name:      podName,
			Namespace: podNamespace,
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}
}

// verifyPods returns true if the two pod slices are equal.
func verifyPods(a, b []*pkgcontainer.Pod) bool {
	if len(a) != len(b) {
		return false
	}

	// Sort the containers within a pod.
	for i := range a {
		sort.Sort(containersByID(a[i].Containers))
	}
	for i := range b {
		sort.Sort(containersByID(b[i].Containers))
	}

	// Sort the pods by UID.
	sort.Sort(podsByID(a))
	sort.Sort(podsByID(b))

	return reflect.DeepEqual(a, b)
}

func verifyFakeContainerList(fakeRuntime *apitest.FakeRuntimeService, expected sets.String) (sets.String, bool) {
	actual := sets.NewString()
	for _, c := range fakeRuntime.Containers {
		actual.Insert(c.Id)
	}
	return actual, actual.Equal(expected)
}

// Only extract the fields of interests.
type cRecord struct {
	name    string
	attempt uint32
	state   runtimeapi.ContainerState
}

type cRecordList []*cRecord

func (b cRecordList) Len() int      { return len(b) }
func (b cRecordList) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b cRecordList) Less(i, j int) bool {
	if b[i].name != b[j].name {
		return b[i].name < b[j].name
	}
	return b[i].attempt < b[j].attempt
}

func verifyContainerStatuses(t *testing.T, runtime *apitest.FakeRuntimeService, expected []*cRecord, desc string) {
	actual := []*cRecord{}
	for _, cStatus := range runtime.Containers {
		actual = append(actual, &cRecord{name: cStatus.Metadata.Name, attempt: cStatus.Metadata.Attempt, state: cStatus.State})
	}
	sort.Sort(cRecordList(expected))
	sort.Sort(cRecordList(actual))
	assert.Equal(t, expected, actual, desc)
}

func TestNewKubeRuntimeManager(t *testing.T) {
	_, _, _, err := createTestRuntimeManager()
	assert.NoError(t, err)
}

func TestVersion(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	version, err := m.Version()
	assert.NoError(t, err)
	assert.Equal(t, kubeRuntimeAPIVersion, version.String())
}

func TestContainerRuntimeType(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	runtimeType := m.Type()
	assert.Equal(t, apitest.FakeRuntimeName, runtimeType)
}

func TestGetPodStatus(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}

	// Set fake sandbox and faked containers to fakeRuntime.
	makeAndSetFakePod(t, m, fakeRuntime, pod)

	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	assert.NoError(t, err)
	assert.Equal(t, pod.UID, podStatus.ID)
	assert.Equal(t, pod.Name, podStatus.Name)
	assert.Equal(t, pod.Namespace, podStatus.Namespace)
	assert.Equal(t, apitest.FakePodSandboxIPs, podStatus.IPs)
}

func TestStopContainerWithNotFoundError(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}

	// Set fake sandbox and faked containers to fakeRuntime.
	makeAndSetFakePod(t, m, fakeRuntime, pod)
	fakeRuntime.InjectError("StopContainer", status.Error(codes.NotFound, "No such container"))
	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	require.NoError(t, err)
	p := pkgcontainer.ConvertPodStatusToRunningPod("", podStatus)
	gracePeriod := int64(1)
	err = m.KillPod(pod, p, &gracePeriod)
	require.NoError(t, err)
}

func TestGetPodStatusWithNotFoundError(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}

	// Set fake sandbox and faked containers to fakeRuntime.
	makeAndSetFakePod(t, m, fakeRuntime, pod)
	fakeRuntime.InjectError("ContainerStatus", status.Error(codes.NotFound, "No such container"))
	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	require.NoError(t, err)
	require.Equal(t, pod.UID, podStatus.ID)
	require.Equal(t, pod.Name, podStatus.Name)
	require.Equal(t, pod.Namespace, podStatus.Namespace)
	require.Equal(t, apitest.FakePodSandboxIPs, podStatus.IPs)
}

func TestGetPods(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "foo1",
					Image: "busybox",
				},
				{
					Name:  "foo2",
					Image: "busybox",
				},
			},
		},
	}

	// Set fake sandbox and fake containers to fakeRuntime.
	fakeSandbox, fakeContainers := makeAndSetFakePod(t, m, fakeRuntime, pod)

	// Convert the fakeContainers to pkgcontainer.Container
	containers := make([]*pkgcontainer.Container, len(fakeContainers))
	for i := range containers {
		fakeContainer := fakeContainers[i]
		c, err := m.topkgcontainer(&runtimeapi.Container{
			Id:          fakeContainer.Id,
			Metadata:    fakeContainer.Metadata,
			State:       fakeContainer.State,
			Image:       fakeContainer.Image,
			ImageRef:    fakeContainer.ImageRef,
			Labels:      fakeContainer.Labels,
			Annotations: fakeContainer.Annotations,
		})
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		containers[i] = c
	}
	// Convert fakeSandbox to pkgcontainer.Container
	sandbox, err := m.sandboxTopkgcontainer(&runtimeapi.PodSandbox{
		Id:          fakeSandbox.Id,
		Metadata:    fakeSandbox.Metadata,
		State:       fakeSandbox.State,
		CreatedAt:   fakeSandbox.CreatedAt,
		Labels:      fakeSandbox.Labels,
		Annotations: fakeSandbox.Annotations,
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	expected := []*pkgcontainer.Pod{
		{
			ID:         types.UID("12345678"),
			Name:       "foo",
			Namespace:  "new",
			Containers: []*pkgcontainer.Container{containers[0], containers[1]},
			Sandboxes:  []*pkgcontainer.Container{sandbox},
		},
	}

	actual, err := m.GetPods(false)
	assert.NoError(t, err)

	if !verifyPods(expected, actual) {
		t.Errorf("expected %#v, got %#v", expected, actual)
	}
}

func TestKillPod(t *testing.T) {
	// Tests that KillPod also kills Ephemeral Containers
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.EphemeralContainers, true)()

	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "foo1",
					Image: "busybox",
				},
				{
					Name:  "foo2",
					Image: "busybox",
				},
			},
			EphemeralContainers: []v1.EphemeralContainer{
				{
					EphemeralContainerCommon: v1.EphemeralContainerCommon{
						Name:  "debug",
						Image: "busybox",
					},
				},
			},
		},
	}

	// Set fake sandbox and fake containers to fakeRuntime.
	fakeSandbox, fakeContainers := makeAndSetFakePod(t, m, fakeRuntime, pod)

	// Convert the fakeContainers to pkgcontainer.Container
	containers := make([]*pkgcontainer.Container, len(fakeContainers))
	for i := range containers {
		fakeContainer := fakeContainers[i]
		c, err := m.topkgcontainer(&runtimeapi.Container{
			Id:       fakeContainer.Id,
			Metadata: fakeContainer.Metadata,
			State:    fakeContainer.State,
			Image:    fakeContainer.Image,
			ImageRef: fakeContainer.ImageRef,
			Labels:   fakeContainer.Labels,
		})
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		containers[i] = c
	}
	runningPod := pkgcontainer.Pod{
		ID:         pod.UID,
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Containers: []*pkgcontainer.Container{containers[0], containers[1], containers[2]},
		Sandboxes: []*pkgcontainer.Container{
			{
				ID: pkgcontainer.CtrID{
					ID:   fakeSandbox.Id,
					Type: apitest.FakeRuntimeName,
				},
			},
		},
	}

	err = m.KillPod(pod, runningPod, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(fakeRuntime.Containers))
	assert.Equal(t, 1, len(fakeRuntime.Sandboxes))
	for _, sandbox := range fakeRuntime.Sandboxes {
		assert.Equal(t, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, sandbox.State)
	}
	for _, c := range fakeRuntime.Containers {
		assert.Equal(t, runtimeapi.ContainerState_CONTAINER_EXITED, c.State)
	}
}

func TestSyncPod(t *testing.T) {
	fakeRuntime, fakeImage, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "alpine",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}

	backOff := flowcontrol.NewBackOff(time.Second, time.Minute)
	result := m.SyncPod(pod, &pkgcontainer.PodStatus{}, &credentialprovider.AuthConfig{}, backOff)
	assert.NoError(t, result.Error())
	assert.Equal(t, 2, len(fakeRuntime.Containers))
	assert.Equal(t, 2, len(fakeImage.Images))
	assert.Equal(t, 1, len(fakeRuntime.Sandboxes))
	for _, sandbox := range fakeRuntime.Sandboxes {
		assert.Equal(t, runtimeapi.PodSandboxState_SANDBOX_READY, sandbox.State)
	}
	for _, c := range fakeRuntime.Containers {
		assert.Equal(t, runtimeapi.ContainerState_CONTAINER_RUNNING, c.State)
	}
}

func TestPruneInitContainers(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	init1 := makeTestContainer("init1", "busybox")
	init2 := makeTestContainer("init2", "busybox")
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{init1, init2},
		},
	}

	templates := []containerTemplate{
		{pod: pod, container: &init1, attempt: 3, createdAt: 3, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init1, attempt: 2, createdAt: 2, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init2, attempt: 1, createdAt: 1, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init1, attempt: 1, createdAt: 1, state: runtimeapi.ContainerState_CONTAINER_UNKNOWN},
		{pod: pod, container: &init2, attempt: 0, createdAt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{pod: pod, container: &init1, attempt: 0, createdAt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
	}
	fakes := makeFakeContainers(t, m, templates)
	fakeRuntime.SetFakeContainers(fakes)
	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	assert.NoError(t, err)

	m.pruneInitContainersBeforeStart(pod, podStatus)
	expectedContainers := sets.NewString(fakes[0].Id, fakes[2].Id)
	if actual, ok := verifyFakeContainerList(fakeRuntime, expectedContainers); !ok {
		t.Errorf("expected %v, got %v", expectedContainers, actual)
	}
}

func TestSyncPodWithInitContainers(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)

	initContainers := []v1.Container{
		{
			Name:            "init1",
			Image:           "init",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
		{
			Name:            "foo2",
			Image:           "alpine",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers:     containers,
			InitContainers: initContainers,
		},
	}

	backOff := flowcontrol.NewBackOff(time.Second, time.Minute)

	// 1. should only create the init container.
	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	assert.NoError(t, err)
	result := m.SyncPod(pod, podStatus, &credentialprovider.AuthConfig{}, backOff)
	assert.NoError(t, result.Error())
	expected := []*cRecord{
		{name: initContainers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
	}
	verifyContainerStatuses(t, fakeRuntime, expected, "start only the init container")

	// 2. should not create app container because init container is still running.
	podStatus, err = m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	assert.NoError(t, err)
	result = m.SyncPod(pod, podStatus, &credentialprovider.AuthConfig{}, backOff)
	assert.NoError(t, result.Error())
	verifyContainerStatuses(t, fakeRuntime, expected, "init container still running; do nothing")

	// 3. should create all app containers because init container finished.
	// Stop init container instance 0.
	sandboxIDs, err := m.getSandboxIDByPodUID(pod.UID, nil)
	require.NoError(t, err)
	sandboxID := sandboxIDs[0]
	initID0, err := fakeRuntime.GetContainerID(sandboxID, initContainers[0].Name, 0)
	require.NoError(t, err)
	_ = fakeRuntime.StopContainer(initID0, 0)
	// Sync again.
	podStatus, err = m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	assert.NoError(t, err)
	result = m.SyncPod(pod, podStatus, &credentialprovider.AuthConfig{}, backOff)
	assert.NoError(t, result.Error())
	expected = []*cRecord{
		{name: initContainers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{name: containers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
		{name: containers[1].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
	}
	verifyContainerStatuses(t, fakeRuntime, expected, "init container completed; all app containers should be running")

	// 4. should restart the init container if needed to create a new podsandbox
	// Stop the pod sandbox.
	_ = fakeRuntime.StopPodSandbox(sandboxID)
	// Sync again.
	podStatus, err = m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	assert.NoError(t, err)
	result = m.SyncPod(pod, podStatus, &credentialprovider.AuthConfig{}, backOff)
	assert.NoError(t, result.Error())
	expected = []*cRecord{
		// The first init container instance is purged and no longer visible.
		// The second (attempt == 1) instance has been started and is running.
		{name: initContainers[0].Name, attempt: 1, state: runtimeapi.ContainerState_CONTAINER_RUNNING},
		// All containers are killed.
		{name: containers[0].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
		{name: containers[1].Name, attempt: 0, state: runtimeapi.ContainerState_CONTAINER_EXITED},
	}
	verifyContainerStatuses(t, fakeRuntime, expected, "kill all app containers, purge the existing init container, and restart a new one")
}

// A helper function to get a basic pod and its status assuming all sandbox and
// containers are running and ready.
func makeBasePodAndStatus() (*v1.Pod, *pkgcontainer.PodStatus) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "foo-ns",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "foo1",
					Image: "busybox",
				},
				{
					Name:  "foo2",
					Image: "busybox",
				},
				{
					Name:  "foo3",
					Image: "busybox",
				},
			},
		},
	}
	podStatus := &pkgcontainer.PodStatus{
		ID:        pod.UID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		SandboxStatuses: []*runtimeapi.PodSandboxStatus{
			{
				Id:       "sandboxID",
				State:    runtimeapi.PodSandboxState_SANDBOX_READY,
				Metadata: &runtimeapi.PodSandboxMetadata{Name: pod.Name, Namespace: pod.Namespace, Uid: "sandboxuid", Attempt: uint32(0)},
				Network:  &runtimeapi.PodSandboxNetworkStatus{Ip: "10.0.0.1"},
			},
		},
		ContainerStatuses: []*pkgcontainer.Status{
			{
				ID:   pkgcontainer.CtrID{ID: "id1"},
				Name: "foo1", State: pkgcontainer.ContainerStateRunning,
				Hash: pkgcontainer.HashContainer(&pod.Spec.Containers[0]),
			},
			{
				ID:   pkgcontainer.CtrID{ID: "id2"},
				Name: "foo2", State: pkgcontainer.ContainerStateRunning,
				Hash: pkgcontainer.HashContainer(&pod.Spec.Containers[1]),
			},
			{
				ID:   pkgcontainer.CtrID{ID: "id3"},
				Name: "foo3", State: pkgcontainer.ContainerStateRunning,
				Hash: pkgcontainer.HashContainer(&pod.Spec.Containers[2]),
			},
		},
	}
	return pod, podStatus
}

func TestComputePodActions(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	// Creating a pair reference pod and status for the test cases to refer
	// the specific fields.
	basePod, baseStatus := makeBasePodAndStatus()
	noAction := podActions{
		SandboxID:         baseStatus.SandboxStatuses[0].Id,
		ContainersToStart: []int{},
		ContainersToKill:  map[pkgcontainer.CtrID]containerToKillInfo{},
	}

	for desc, test := range map[string]struct {
		mutatePodFn    func(*v1.Pod)
		mutateStatusFn func(*pkgcontainer.PodStatus)
		actions        podActions
		resetStatusFn  func(*pkgcontainer.PodStatus)
	}{
		"everying is good; do nothing": {
			actions: noAction,
		},
		"start pod sandbox and all containers for a new pod": {
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				// No container or sandbox exists.
				status.SandboxStatuses = []*runtimeapi.PodSandboxStatus{}
				status.ContainerStatuses = []*pkgcontainer.Status{}
			},
			actions: podActions{
				KillPod:           true,
				CreateSandbox:     true,
				Attempt:           uint32(0),
				ContainersToStart: []int{0, 1, 2},
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"restart exited containers if RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				// The first container completed, restart it,
				status.ContainerStatuses[0].State = pkgcontainer.ContainerStateExited
				status.ContainerStatuses[0].ExitCode = 0

				// The second container exited with failure, restart it,
				status.ContainerStatuses[1].State = pkgcontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 111
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToStart: []int{0, 1},
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"restart failed containers if RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				// The first container completed, don't restart it,
				status.ContainerStatuses[0].State = pkgcontainer.ContainerStateExited
				status.ContainerStatuses[0].ExitCode = 0

				// The second container exited with failure, restart it,
				status.ContainerStatuses[1].State = pkgcontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 111
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToStart: []int{1},
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"don't restart containers if RestartPolicy == Never": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				// Don't restart any containers.
				status.ContainerStatuses[0].State = pkgcontainer.ContainerStateExited
				status.ContainerStatuses[0].ExitCode = 0
				status.ContainerStatuses[1].State = pkgcontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 111
			},
			actions: noAction,
		},
		"Kill pod and recreate everything if the pod sandbox is dead, and RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
			},
			actions: podActions{
				KillPod:           true,
				CreateSandbox:     true,
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				Attempt:           uint32(1),
				ContainersToStart: []int{0, 1, 2},
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"Kill pod and recreate all containers (except for the succeeded one) if the pod sandbox is dead, and RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.ContainerStatuses[1].State = pkgcontainer.ContainerStateExited
				status.ContainerStatuses[1].ExitCode = 0
			},
			actions: podActions{
				KillPod:           true,
				CreateSandbox:     true,
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				Attempt:           uint32(1),
				ContainersToStart: []int{0, 2},
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"Kill pod and recreate all containers if the PodSandbox does not have an IP": {
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.SandboxStatuses[0].Network.Ip = ""
			},
			actions: podActions{
				KillPod:           true,
				CreateSandbox:     true,
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				Attempt:           uint32(1),
				ContainersToStart: []int{0, 1, 2},
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{}),
			},
		},
		"Kill and recreate the container if the container's spec changed": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyAlways
			},
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[1].Hash = uint64(432423432)
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{1}),
				ContainersToStart: []int{1},
			},
		},
		"Kill and recreate the container if the liveness check has failed": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyAlways
			},
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				m.livenessManager.Set(status.ContainerStatuses[1].ID, proberesults.Failure, basePod)
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{1}),
				ContainersToStart: []int{1},
			},
			resetStatusFn: func(status *pkgcontainer.PodStatus) {
				m.livenessManager.Remove(status.ContainerStatuses[1].ID)
			},
		},
		"Kill and recreate the container if the startup check has failed": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyAlways
			},
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				m.startupManager.Set(status.ContainerStatuses[1].ID, proberesults.Failure, basePod)
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{1}),
				ContainersToStart: []int{1},
			},
			resetStatusFn: func(status *pkgcontainer.PodStatus) {
				m.startupManager.Remove(status.ContainerStatuses[1].ID)
			},
		},
		"Verify we do not create a pod sandbox if no ready sandbox for pod with RestartPolicy=Never and all containers exited": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyNever
			},
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				// no ready sandbox
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.SandboxStatuses[0].Metadata.Attempt = uint32(1)
				// all containers exited
				for i := range status.ContainerStatuses {
					status.ContainerStatuses[i].State = pkgcontainer.ContainerStateExited
					status.ContainerStatuses[i].ExitCode = 0
				}
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				Attempt:           uint32(2),
				CreateSandbox:     false,
				KillPod:           true,
				ContainersToStart: []int{},
				ContainersToKill:  map[pkgcontainer.CtrID]containerToKillInfo{},
			},
		},
		"Verify we do not create a pod sandbox if no ready sandbox for pod with RestartPolicy=OnFailure and all containers succeeded": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure
			},
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				// no ready sandbox
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.SandboxStatuses[0].Metadata.Attempt = uint32(1)
				// all containers succeeded
				for i := range status.ContainerStatuses {
					status.ContainerStatuses[i].State = pkgcontainer.ContainerStateExited
					status.ContainerStatuses[i].ExitCode = 0
				}
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				Attempt:           uint32(2),
				CreateSandbox:     false,
				KillPod:           true,
				ContainersToStart: []int{},
				ContainersToKill:  map[pkgcontainer.CtrID]containerToKillInfo{},
			},
		},
		"Verify we create a pod sandbox if no ready sandbox for pod with RestartPolicy=Never and no containers have ever been created": {
			mutatePodFn: func(pod *v1.Pod) {
				pod.Spec.RestartPolicy = v1.RestartPolicyNever
			},
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				// no ready sandbox
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.SandboxStatuses[0].Metadata.Attempt = uint32(2)
				// no visible containers
				status.ContainerStatuses = []*pkgcontainer.Status{}
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				Attempt:           uint32(3),
				CreateSandbox:     true,
				KillPod:           true,
				ContainersToStart: []int{0, 1, 2},
				ContainersToKill:  map[pkgcontainer.CtrID]containerToKillInfo{},
			},
		},
		"Kill and recreate the container if the container is in unknown state": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[1].State = pkgcontainer.ContainerStateUnknown
			},
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToKill:  getKillMap(basePod, baseStatus, []int{1}),
				ContainersToStart: []int{1},
			},
		},
	} {
		pod, podStatus := makeBasePodAndStatus()
		if test.mutatePodFn != nil {
			test.mutatePodFn(pod)
		}
		if test.mutateStatusFn != nil {
			test.mutateStatusFn(podStatus)
		}
		actions := m.computePodActions(pod, podStatus)
		verifyActions(t, &test.actions, &actions, desc)
		if test.resetStatusFn != nil {
			test.resetStatusFn(podStatus)
		}
	}
}

func getKillMap(pod *v1.Pod, status *pkgcontainer.PodStatus, cIndexes []int) map[pkgcontainer.CtrID]containerToKillInfo {
	m := map[pkgcontainer.CtrID]containerToKillInfo{}
	for _, i := range cIndexes {
		m[status.ContainerStatuses[i].ID] = containerToKillInfo{
			container: &pod.Spec.Containers[i],
			name:      pod.Spec.Containers[i].Name,
		}
	}
	return m
}

func getKillMapWithInitContainers(pod *v1.Pod, status *pkgcontainer.PodStatus, cIndexes []int) map[pkgcontainer.CtrID]containerToKillInfo {
	m := map[pkgcontainer.CtrID]containerToKillInfo{}
	for _, i := range cIndexes {
		m[status.ContainerStatuses[i].ID] = containerToKillInfo{
			container: &pod.Spec.InitContainers[i],
			name:      pod.Spec.InitContainers[i].Name,
		}
	}
	return m
}

func verifyActions(t *testing.T, expected, actual *podActions, desc string) {
	if actual.ContainersToKill != nil {
		// Clear the message and reason fields since we don't need to verify them.
		for k, info := range actual.ContainersToKill {
			info.message = ""
			info.reason = ""
			actual.ContainersToKill[k] = info
		}
	}
	assert.Equal(t, expected, actual, desc)
}

func TestComputePodActionsWithInitContainers(t *testing.T) {
	_, _, m, err := createTestRuntimeManager()
	require.NoError(t, err)

	// Creating a pair reference pod and status for the test cases to refer
	// the specific fields.
	basePod, baseStatus := makeBasePodAndStatusWithInitContainers()
	noAction := podActions{
		SandboxID:         baseStatus.SandboxStatuses[0].Id,
		ContainersToStart: []int{},
		ContainersToKill:  map[pkgcontainer.CtrID]containerToKillInfo{},
	}

	for desc, test := range map[string]struct {
		mutatePodFn    func(*v1.Pod)
		mutateStatusFn func(*pkgcontainer.PodStatus)
		actions        podActions
	}{
		"initialization completed; start all containers": {
			actions: podActions{
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToStart: []int{0, 1, 2},
				ContainersToKill:  getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization in progress; do nothing": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[2].State = pkgcontainer.ContainerStateRunning
			},
			actions: noAction,
		},
		"Kill pod and restart the first init container if the pod sandbox is dead": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
			},
			actions: podActions{
				KillPod:                  true,
				CreateSandbox:            true,
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				Attempt:                  uint32(1),
				NextInitContainerToStart: &basePod.Spec.InitContainers[0],
				ContainersToStart:        []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization failed; restart the last init container if RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[2].ExitCode = 137
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization failed; restart the last init container if RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[2].ExitCode = 137
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"initialization failed; kill pod if RestartPolicy == Never": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[2].ExitCode = 137
			},
			actions: podActions{
				KillPod:           true,
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToStart: []int{},
				ContainersToKill:  getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"init container state unknown; kill and recreate the last init container if RestartPolicy == Always": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyAlways },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[2].State = pkgcontainer.ContainerStateUnknown
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{2}),
			},
		},
		"init container state unknown; kill and recreate the last init container if RestartPolicy == OnFailure": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[2].State = pkgcontainer.ContainerStateUnknown
			},
			actions: podActions{
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				NextInitContainerToStart: &basePod.Spec.InitContainers[2],
				ContainersToStart:        []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{2}),
			},
		},
		"init container state unknown; kill pod if RestartPolicy == Never": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.ContainerStatuses[2].State = pkgcontainer.ContainerStateUnknown
			},
			actions: podActions{
				KillPod:           true,
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				ContainersToStart: []int{},
				ContainersToKill:  getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"Pod sandbox not ready, init container failed, but RestartPolicy == Never; kill pod only": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
			},
			actions: podActions{
				KillPod:           true,
				CreateSandbox:     false,
				SandboxID:         baseStatus.SandboxStatuses[0].Id,
				Attempt:           uint32(1),
				ContainersToStart: []int{},
				ContainersToKill:  getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"Pod sandbox not ready, and RestartPolicy == Never, but no visible init containers;  create a new pod sandbox": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyNever },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.ContainerStatuses = []*pkgcontainer.Status{}
			},
			actions: podActions{
				KillPod:                  true,
				CreateSandbox:            true,
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				Attempt:                  uint32(1),
				NextInitContainerToStart: &basePod.Spec.InitContainers[0],
				ContainersToStart:        []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
		"Pod sandbox not ready, init container failed, and RestartPolicy == OnFailure; create a new pod sandbox": {
			mutatePodFn: func(pod *v1.Pod) { pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure },
			mutateStatusFn: func(status *pkgcontainer.PodStatus) {
				status.SandboxStatuses[0].State = runtimeapi.PodSandboxState_SANDBOX_NOTREADY
				status.ContainerStatuses[2].ExitCode = 137
			},
			actions: podActions{
				KillPod:                  true,
				CreateSandbox:            true,
				SandboxID:                baseStatus.SandboxStatuses[0].Id,
				Attempt:                  uint32(1),
				NextInitContainerToStart: &basePod.Spec.InitContainers[0],
				ContainersToStart:        []int{},
				ContainersToKill:         getKillMapWithInitContainers(basePod, baseStatus, []int{}),
			},
		},
	} {
		pod, podStatus := makeBasePodAndStatusWithInitContainers()
		if test.mutatePodFn != nil {
			test.mutatePodFn(pod)
		}
		if test.mutateStatusFn != nil {
			test.mutateStatusFn(podStatus)
		}
		actions := m.computePodActions(pod, podStatus)
		verifyActions(t, &test.actions, &actions, desc)
	}
}

func makeBasePodAndStatusWithInitContainers() (*v1.Pod, *pkgcontainer.PodStatus) {
	pod, podStatus := makeBasePodAndStatus()
	pod.Spec.InitContainers = []v1.Container{
		{
			Name:  "init1",
			Image: "bar-image",
		},
		{
			Name:  "init2",
			Image: "bar-image",
		},
		{
			Name:  "init3",
			Image: "bar-image",
		},
	}
	// Replace the original statuses of the containers with those for the init
	// containers.
	podStatus.ContainerStatuses = []*pkgcontainer.Status{
		{
			ID:   pkgcontainer.CtrID{ID: "initid1"},
			Name: "init1", State: pkgcontainer.ContainerStateExited,
			Hash: pkgcontainer.HashContainer(&pod.Spec.InitContainers[0]),
		},
		{
			ID:   pkgcontainer.CtrID{ID: "initid2"},
			Name: "init2", State: pkgcontainer.ContainerStateExited,
			Hash: pkgcontainer.HashContainer(&pod.Spec.InitContainers[0]),
		},
		{
			ID:   pkgcontainer.CtrID{ID: "initid3"},
			Name: "init3", State: pkgcontainer.ContainerStateExited,
			Hash: pkgcontainer.HashContainer(&pod.Spec.InitContainers[0]),
		},
	}
	return pod, podStatus
}

func TestSyncPodWithSandboxAndDeletedPod(t *testing.T) {
	fakeRuntime, _, m, err := createTestRuntimeManager()
	assert.NoError(t, err)
	fakeRuntime.ErrorOnSandboxCreate = true

	containers := []v1.Container{
		{
			Name:            "foo1",
			Image:           "busybox",
			ImagePullPolicy: v1.PullIfNotPresent,
		},
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "12345678",
			Name:      "foo",
			Namespace: "new",
		},
		Spec: v1.PodSpec{
			Containers: containers,
		},
	}

	backOff := flowcontrol.NewBackOff(time.Second, time.Minute)
	m.podStateProvider.(*fakePodStateProvider).removed = map[types.UID]struct{}{pod.UID: {}}

	// GetPodStatus and the following SyncPod will not return errors in the
	// case where the pod has been deleted. We are not adding any pods into
	// the fakePodProvider so they are 'deleted'.
	podStatus, err := m.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
	assert.NoError(t, err)
	result := m.SyncPod(pod, podStatus, &credentialprovider.AuthConfig{}, backOff)
	// This will return an error if the pod has _not_ been deleted.
	assert.NoError(t, result.Error())
}
