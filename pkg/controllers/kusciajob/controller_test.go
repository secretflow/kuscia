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

package kusciajob

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	constants "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func makeKusciaJob() *kusciaapisv1alpha1.KusciaJob {
	parties := []kusciaapisv1alpha1.Party{
		{Role: "client", DomainID: "alice"},
		{Role: "client", DomainID: "bob"},
	}
	return &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secretflow-job",
			Namespace: constants.KusciaCrossDomain,
		},
		Spec: kusciaapisv1alpha1.KusciaJobSpec{
			Initiator:    "alice",
			ScheduleMode: kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort,
			Tasks: []kusciaapisv1alpha1.KusciaTaskTemplate{
				{
					Alias:           "a",
					TaskID:          "a",
					Priority:        100,
					TaskInputConfig: "meta://secretflow-1/task-input-config",
					AppImage:        "test-image-1",
					Parties:         parties,
					Dependencies:    []string{},
				},
				{
					Alias:           "b",
					TaskID:          "b",
					Priority:        100,
					TaskInputConfig: "meta://secretflow-1/task-input-config2",
					AppImage:        "test-image-2",
					Parties:         parties,
					Dependencies:    []string{"a"},
				},
			},
		},
		Status: kusciaapisv1alpha1.KusciaJobStatus{
			Phase: kusciaapisv1alpha1.KusciaJobRunning,
			Conditions: []kusciaapisv1alpha1.KusciaJobCondition{
				{
					Type:   kusciaapisv1alpha1.JobStartInitialized,
					Status: v1.ConditionTrue,
				},
				{
					Type:   kusciaapisv1alpha1.JobStartSucceeded,
					Status: v1.ConditionTrue,
				},
			},
		},
	}
}

func Test_KusciaJobControllerHandleTaskSucceed(t *testing.T) {
	t.Parallel()
	testKusciaJob := makeKusciaJob()
	aliceNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
	}

	bobNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bob",
		},
	}
	kubeClient := kubefake.NewSimpleClientset(aliceNs, bobNs)
	kusciaClient := kusciafake.NewSimpleClientset(testKusciaJob)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kusciajobcontroller"})
	c := NewController(context.TODO(), controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})

	go func() {
		assert.NoError(t, c.Run(3))
	}()

	waitAndCheckKusciaJobStatus(t, c.(*Controller), testKusciaJob.Name, []kusciaapisv1alpha1.KusciaJobPhase{kusciaapisv1alpha1.KusciaJobRunning, kusciaapisv1alpha1.KusciaJobPending})
	waitAndChangeKusciaTaskStatus(t, c.(*Controller), 1, map[string]kusciaapisv1alpha1.KusciaTaskPhase{
		"a": kusciaapisv1alpha1.TaskSucceeded,
	})
	waitAndCheckKusciaJobStatus(t, c.(*Controller), testKusciaJob.Name, []kusciaapisv1alpha1.KusciaJobPhase{kusciaapisv1alpha1.KusciaJobRunning})
	waitAndChangeKusciaTaskStatus(t, c.(*Controller), 2, map[string]kusciaapisv1alpha1.KusciaTaskPhase{
		"a": kusciaapisv1alpha1.TaskSucceeded,
		"b": kusciaapisv1alpha1.TaskSucceeded,
	})
	waitAndCheckKusciaJobStatus(t, c.(*Controller), testKusciaJob.Name, []kusciaapisv1alpha1.KusciaJobPhase{kusciaapisv1alpha1.KusciaJobSucceeded})
}

func waitAndChangeKusciaTaskStatus(t *testing.T, c *Controller, expectTaskCount int, taskStatus map[string]kusciaapisv1alpha1.KusciaTaskPhase) {
	err := wait.Poll(1*time.Second, 30*time.Second, func() (done bool, err error) {
		tasks, err := c.kusciaTaskLister.List(labels.Everything())
		tasksString := make([]string, 0)
		for _, t := range tasks {
			tasksString = append(tasksString, fmt.Sprintf("%s", t.Status.Phase))
		}
		nlog.Infof("tasks: %s, err: %v", tasksString, err)
		return len(tasks) == expectTaskCount, err
	})
	assert.NoError(t, err)

	tasks, err := c.kusciaTaskLister.KusciaTasks(constants.KusciaCrossDomain).List(labels.Everything())
	assert.NoError(t, err)

	for _, task := range tasks {
		status, exist := taskStatus[task.Name]
		if exist {
			task.Status.Phase = status
			_, err := c.kusciaClient.KusciaV1alpha1().KusciaTasks(constants.KusciaCrossDomain).UpdateStatus(context.TODO(), task, metav1.UpdateOptions{})
			assert.NoError(t, err)
		}
	}

}

func Test_KusciaTaskControllerHandlerTaskFailed(t *testing.T) {
	t.Parallel()
	testKusciaJob := makeKusciaJob()
	aliceNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
	}

	bobNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bob",
		},
	}
	kubeClient := kubefake.NewSimpleClientset(aliceNs, bobNs)
	kusciaClient := kusciafake.NewSimpleClientset(testKusciaJob)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-job-controller"})
	c := NewController(context.TODO(), controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})

	go func() {
		assert.NoError(t, c.Run(3))
	}()

	waitAndCheckKusciaJobStatus(t, c.(*Controller), testKusciaJob.Name, []kusciaapisv1alpha1.KusciaJobPhase{kusciaapisv1alpha1.KusciaJobRunning, kusciaapisv1alpha1.KusciaJobPending})
	waitAndChangeKusciaTaskStatus(t, c.(*Controller), 1, map[string]kusciaapisv1alpha1.KusciaTaskPhase{
		"a": kusciaapisv1alpha1.TaskSucceeded,
	})
	waitAndCheckKusciaJobStatus(t, c.(*Controller), testKusciaJob.Name, []kusciaapisv1alpha1.KusciaJobPhase{kusciaapisv1alpha1.KusciaJobRunning})
	waitAndChangeKusciaTaskStatus(t, c.(*Controller), 2, map[string]kusciaapisv1alpha1.KusciaTaskPhase{
		"a": kusciaapisv1alpha1.TaskSucceeded,
		"b": kusciaapisv1alpha1.TaskFailed,
	})
	waitAndCheckKusciaJobStatus(t, c.(*Controller), testKusciaJob.Name, []kusciaapisv1alpha1.KusciaJobPhase{kusciaapisv1alpha1.KusciaJobFailed})
}

func waitAndCheckKusciaJobStatus(t *testing.T, c *Controller, kusciaJobName string, statusSet []kusciaapisv1alpha1.KusciaJobPhase) {
	err := wait.Poll(1*time.Second, 60*time.Second, func() (done bool, err error) {
		kusciaJob, err := c.kusciaJobLister.KusciaJobs(constants.KusciaCrossDomain).Get(kusciaJobName)
		nlog.Infof("kusciaJob: %s, err: %s", kusciaJob.Status, err)
		return kusciaJob != nil && statusContains(statusSet, kusciaJob.Status.Phase), err
	})
	assert.NoError(t, err)
}

func statusContains(statusSet []kusciaapisv1alpha1.KusciaJobPhase, phase kusciaapisv1alpha1.KusciaJobPhase) bool {
	for _, it := range statusSet {
		if it == phase {
			return true
		}
	}
	return false
}
