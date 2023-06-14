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

package prober

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"

	"github.com/secretflow/kuscia/pkg/agent/prober/results"
)

func init() {
}

var defaultProbe = &v1.Probe{
	ProbeHandler: v1.ProbeHandler{
		TCPSocket: &v1.TCPSocketAction{},
	},
	TimeoutSeconds:   1,
	PeriodSeconds:    1,
	SuccessThreshold: 1,
	FailureThreshold: 3,
}

func TestAddRemovePods(t *testing.T) {
	noProbePod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: "no_probe_pod",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name: "no_probe1",
			}, {
				Name: "no_probe2",
			}, {
				Name: "no_probe3",
			}},
		},
	}

	probePod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: "probe_pod",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name: "probe1",
			}, {
				Name:           "readiness",
				ReadinessProbe: defaultProbe,
			}, {
				Name: "probe2",
			}, {
				Name:          "liveness",
				LivenessProbe: defaultProbe,
			}, {
				Name: "probe3",
			}, {
				Name:         "startup",
				StartupProbe: defaultProbe,
			}},
		},
	}

	m := newTestManager()
	defer cleanup(t, m)
	if err := expectProbes(m, nil); err != nil {
		t.Error(err)
	}

	// Adding a pod with no probes should be a no-op.
	m.AddPod(&noProbePod)
	if err := expectProbes(m, nil); err != nil {
		t.Error(err)
	}

	// Adding a pod with probes.
	m.AddPod(&probePod)
	probePaths := []probeKey{
		{"probe_pod", "readiness", readiness},
		{"probe_pod", "liveness", liveness},
		{"probe_pod", "startup", startup},
	}
	if err := expectProbes(m, probePaths); err != nil {
		t.Error(err)
	}

	// Removing un-probed pod.
	m.RemovePod(&noProbePod)
	if err := expectProbes(m, probePaths); err != nil {
		t.Error(err)
	}

	// Removing probed pod.
	m.RemovePod(&probePod)
	if err := waitForWorkerExit(t, m, probePaths); err != nil {
		t.Fatal(err)
	}
	if err := expectProbes(m, nil); err != nil {
		t.Error(err)
	}

	// Removing already removed pods should be a no-op.
	m.RemovePod(&probePod)
	if err := expectProbes(m, nil); err != nil {
		t.Error(err)
	}
}

func TestCleanupPods(t *testing.T) {
	m := newTestManager()
	defer cleanup(t, m)
	podToCleanup := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: "pod_cleanup",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:           "prober1",
				ReadinessProbe: defaultProbe,
			}, {
				Name:          "prober2",
				LivenessProbe: defaultProbe,
			}, {
				Name:         "prober3",
				StartupProbe: defaultProbe,
			}},
		},
	}
	podToKeep := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: "pod_keep",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:           "prober1",
				ReadinessProbe: defaultProbe,
			}, {
				Name:          "prober2",
				LivenessProbe: defaultProbe,
			}, {
				Name:         "prober3",
				StartupProbe: defaultProbe,
			}},
		},
	}
	m.AddPod(&podToCleanup)
	m.AddPod(&podToKeep)

	desiredPods := map[types.UID]sets.Empty{}
	desiredPods[podToKeep.UID] = sets.Empty{}
	m.CleanupPods(desiredPods)

	removedProbes := []probeKey{
		{"pod_cleanup", "prober1", readiness},
		{"pod_cleanup", "prober2", liveness},
		{"pod_cleanup", "prober3", startup},
	}
	expectedProbes := []probeKey{
		{"pod_keep", "prober1", readiness},
		{"pod_keep", "prober2", liveness},
		{"pod_keep", "prober3", startup},
	}
	if err := waitForWorkerExit(t, m, removedProbes); err != nil {
		t.Fatal(err)
	}
	if err := expectProbes(m, expectedProbes); err != nil {
		t.Error(err)
	}
}

func TestCleanupRepeated(t *testing.T) {
	m := newTestManager()
	defer cleanup(t, m)
	podTemplate := v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:           "prober1",
				ReadinessProbe: defaultProbe,
				LivenessProbe:  defaultProbe,
				StartupProbe:   defaultProbe,
			}},
		},
	}

	const numTestPods = 100
	for i := 0; i < numTestPods; i++ {
		pod := podTemplate
		pod.UID = types.UID(strconv.Itoa(i))
		m.AddPod(&pod)
	}

	for i := 0; i < 10; i++ {
		m.CleanupPods(map[types.UID]sets.Empty{})
	}
}

func TestUpdatePodStatus(t *testing.T) {
	unprobed := v1.ContainerStatus{
		Name:        "unprobed_container",
		ContainerID: "test://unprobed_container_id",
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{},
		},
	}
	probedReady := v1.ContainerStatus{
		Name:        "probed_container_ready",
		ContainerID: "test://probed_container_ready_id",
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{},
		},
	}
	probedPending := v1.ContainerStatus{
		Name:        "probed_container_pending",
		ContainerID: "test://probed_container_pending_id",
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{},
		},
	}
	probedUnready := v1.ContainerStatus{
		Name:        "probed_container_unready",
		ContainerID: "test://probed_container_unready_id",
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{},
		},
	}
	notStartedNoReadiness := v1.ContainerStatus{
		Name:        "not_started_container_no_readiness",
		ContainerID: "test://not_started_container_no_readiness_id",
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{},
		},
	}
	startedNoReadiness := v1.ContainerStatus{
		Name:        "started_container_no_readiness",
		ContainerID: "test://started_container_no_readiness_id",
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{},
		},
	}
	terminated := v1.ContainerStatus{
		Name:        "terminated_container",
		ContainerID: "test://terminated_container_id",
		State: v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{},
		},
	}
	podStatus := v1.PodStatus{
		Phase: v1.PodRunning,
		ContainerStatuses: []v1.ContainerStatus{
			unprobed, probedReady, probedPending, probedUnready, notStartedNoReadiness, startedNoReadiness, terminated,
		},
	}

	m := newTestManager()
	// no cleanup: using fake workers.

	// Setup probe "workers" and cached results.
	m.workers = map[probeKey]*worker{
		{testPodUID, unprobed.Name, liveness}:             {},
		{testPodUID, probedReady.Name, readiness}:         {},
		{testPodUID, probedPending.Name, readiness}:       {},
		{testPodUID, probedUnready.Name, readiness}:       {},
		{testPodUID, notStartedNoReadiness.Name, startup}: {},
		{testPodUID, startedNoReadiness.Name, startup}:    {},
		{testPodUID, terminated.Name, readiness}:          {},
	}
	m.readinessManager.Set(pkgcontainer.ParseContainerID(probedReady.ContainerID), results.Success, &v1.Pod{})
	m.readinessManager.Set(pkgcontainer.ParseContainerID(probedUnready.ContainerID), results.Failure, &v1.Pod{})
	m.startupManager.Set(pkgcontainer.ParseContainerID(startedNoReadiness.ContainerID), results.Success, &v1.Pod{})
	m.readinessManager.Set(pkgcontainer.ParseContainerID(terminated.ContainerID), results.Success, &v1.Pod{})

	m.UpdatePodStatus(testPodUID, &podStatus)

	expectedReadiness := map[probeKey]bool{
		{testPodUID, unprobed.Name, readiness}:              true,
		{testPodUID, probedReady.Name, readiness}:           true,
		{testPodUID, probedPending.Name, readiness}:         false,
		{testPodUID, probedUnready.Name, readiness}:         false,
		{testPodUID, notStartedNoReadiness.Name, readiness}: false,
		{testPodUID, startedNoReadiness.Name, readiness}:    true,
		{testPodUID, terminated.Name, readiness}:            false,
	}
	for _, c := range podStatus.ContainerStatuses {
		expected, ok := expectedReadiness[probeKey{testPodUID, c.Name, readiness}]
		if !ok {
			t.Fatalf("Missing expectation for test case: %v", c.Name)
		}
		if expected != c.Ready {
			t.Errorf("Unexpected readiness for container %v: Expected %v but got %v",
				c.Name, expected, c.Ready)
		}
	}
}

func (m *manager) extractedReadinessHandling() {
	update := <-m.readinessManager.Updates()
	// This code corresponds to an extract from kubelet.syncLoopIteration()
	ready := update.Result == results.Success
	m.statusManager.SetContainerReadiness(update.PodUID, update.ContainerID, ready)
}

func expectProbes(m *manager, expectedProbes []probeKey) error {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()

	var unexpected []probeKey
	missing := make([]probeKey, len(expectedProbes))
	copy(missing, expectedProbes)

outer:
	for probePath := range m.workers {
		for i, expectedPath := range missing {
			if probePath == expectedPath {
				missing = append(missing[:i], missing[i+1:]...)
				continue outer
			}
		}
		unexpected = append(unexpected, probePath)
	}

	if len(missing) == 0 && len(unexpected) == 0 {
		return nil // Yay!
	}

	return fmt.Errorf("Unexpected probes: %v; Missing probes: %v;", unexpected, missing)
}

const interval = 1 * time.Second

// Wait for the given workers to exit & clean up.
func waitForWorkerExit(t *testing.T, m *manager, workerPaths []probeKey) error {
	for _, w := range workerPaths {
		condition := func() (bool, error) {
			_, exists := m.getWorker(w.podUID, w.containerName, w.probeType)
			return !exists, nil
		}
		if exited, _ := condition(); exited {
			continue // Already exited, no need to poll.
		}
		t.Logf("Polling %v", w)
		if err := wait.Poll(interval, wait.ForeverTestTimeout, condition); err != nil {
			return err
		}
	}

	return nil
}

// Wait for the given workers to exit & clean up.
func waitForReadyStatus(t *testing.T, m *manager, ready bool) error {
	condition := func() (bool, error) {
		status, ok := m.statusManager.GetPodStatus(testPodUID)
		if !ok {
			return false, fmt.Errorf("status not found: %q", testPodUID)
		}
		if len(status.ContainerStatuses) != 1 {
			return false, fmt.Errorf("expected single container, found %d", len(status.ContainerStatuses))
		}
		if status.ContainerStatuses[0].ContainerID != testContainerID.String() {
			return false, fmt.Errorf("expected container %q, found %q",
				testContainerID, status.ContainerStatuses[0].ContainerID)
		}
		return status.ContainerStatuses[0].Ready == ready, nil
	}
	t.Logf("Polling for ready state %v", ready)
	if err := wait.Poll(interval, wait.ForeverTestTimeout, condition); err != nil {
		return err
	}

	return nil
}

// cleanup running probes to avoid leaking goroutines.
func cleanup(t *testing.T, m *manager) {
	m.CleanupPods(nil)

	condition := func() (bool, error) {
		workerCount := m.workerCount()
		if workerCount > 0 {
			t.Logf("Waiting for %d workers to exit...", workerCount)
		}
		return workerCount == 0, nil
	}
	if exited, _ := condition(); exited {
		return // Already exited, no need to poll.
	}
	if err := wait.Poll(interval, wait.ForeverTestTimeout, condition); err != nil {
		t.Fatalf("Error during cleanup: %v", err)
	}
}
