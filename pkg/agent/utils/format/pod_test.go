/*
Copyright 2017 The Kubernetes Authors.

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

package format

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func fakeCreatePod(name, namespace string, uid types.UID) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       uid,
		},
	}
}

func fakeCreatePodWithDeletionTimestamp(name, namespace string, uid types.UID, deletionTimestamp *metav1.Time) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               uid,
			DeletionTimestamp: deletionTimestamp,
		},
	}
}

func TestPod(t *testing.T) {
	testCases := []struct {
		caseName      string
		pod           *v1.Pod
		expectedValue string
	}{
		{"field_empty_case", fakeCreatePod("", "", ""), "_()"},
		{"field_normal_case", fakeCreatePod("test-pod", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75"), "test-pod_default(551f5a43-9f2f-11e7-a589-fa163e148d75)"},
		{"nil_pod_case", nil, "<nil>"},
	}

	for _, testCase := range testCases {
		realPod := Pod(testCase.pod)
		assert.Equalf(t, testCase.expectedValue, realPod, "Failed to test: %s", testCase.caseName)
	}
}

func TestPods(t *testing.T) {
	testCases := []struct {
		caseName      string
		pods          []*v1.Pod
		expectedValue string
	}{
		{
			"field_normal_case",
			[]*v1.Pod{
				fakeCreatePod("test-pod-1", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75"),
				fakeCreatePod("test-pod-1", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d76"),
			},
			"[test-pod-1_default(551f5a43-9f2f-11e7-a589-fa163e148d75) test-pod-1_default(551f5a43-9f2f-11e7-a589-fa163e148d76)]",
		},
		{"nil_pod_case", []*v1.Pod{}, "[]"},
	}

	for _, testCase := range testCases {
		realPod := Pods(testCase.pods)
		assert.Equalf(t, testCase.expectedValue, realPod, "Failed to test: %s", testCase.caseName)
	}
}

func TestPodAndPodDesc(t *testing.T) {
	testCases := []struct {
		caseName      string
		podName       string
		podNamesapce  string
		podUID        types.UID
		expectedValue string
	}{
		{"field_empty_case", "", "", "", "_()"},
		{"field_normal_case", "test-pod", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75", "test-pod_default(551f5a43-9f2f-11e7-a589-fa163e148d75)"},
	}

	for _, testCase := range testCases {
		realPodDesc := PodDesc(testCase.podName, testCase.podNamesapce, testCase.podUID)
		assert.Equalf(t, testCase.expectedValue, realPodDesc, "Failed to test: %s", testCase.caseName)
	}
}

func TestPodWithDeletionTimestamp(t *testing.T) {
	normalDeletionTime := metav1.Date(2017, time.September, 26, 14, 37, 50, 00, time.UTC)

	testCases := []struct {
		caseName               string
		isPodNil               bool
		isdeletionTimestampNil bool
		deletionTimestamp      metav1.Time
		expectedValue          string
	}{
		{"timestamp_is_nil_case", false, true, normalDeletionTime, "test-pod_default(551f5a43-9f2f-11e7-a589-fa163e148d75)"},
		{"timestamp_is_normal_case", false, false, normalDeletionTime, "test-pod_default(551f5a43-9f2f-11e7-a589-fa163e148d75):DeletionTimestamp=2017-09-26T14:37:50Z"},
		{"pod_is_nil_case", true, false, normalDeletionTime, "<nil>"},
	}

	for _, testCase := range testCases {
		fakePod := fakeCreatePodWithDeletionTimestamp("test-pod", metav1.NamespaceDefault, "551f5a43-9f2f-11e7-a589-fa163e148d75", &testCase.deletionTimestamp)

		if testCase.isdeletionTimestampNil {
			fakePod.SetDeletionTimestamp(nil)
		}
		if testCase.isPodNil {
			fakePod = nil
		}

		realPodWithDeletionTimestamp := PodWithDeletionTimestamp(fakePod)
		assert.Equalf(t, testCase.expectedValue, realPodWithDeletionTimestamp, "Failed to test: %s", testCase.caseName)
	}
}
