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

//nolint:dupl

package garbagecollection

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	constants "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func makeKusciaJob() *kusciaapisv1alpha1.KusciaJob {
	parties := []kusciaapisv1alpha1.Party{
		{Role: "client", DomainID: "hello"},
		{Role: "client", DomainID: "world"},
	}
	return &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secretflow-job-test",
			Namespace: constants.KusciaCrossDomain,
		},
		Spec: kusciaapisv1alpha1.KusciaJobSpec{
			Initiator:    "hello",
			ScheduleMode: kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort,
			Tasks: []kusciaapisv1alpha1.KusciaTaskTemplate{
				{
					Alias:           "h",
					TaskID:          "h",
					Priority:        100,
					TaskInputConfig: "meta://secretflow-1/task-input-config",
					AppImage:        "test-image-1",
					Parties:         parties,
					Dependencies:    []string{},
				},
				{
					Alias:           "w",
					TaskID:          "w",
					Priority:        100,
					TaskInputConfig: "meta://secretflow-1/task-input-config2",
					AppImage:        "test-image-2",
					Parties:         parties,
					Dependencies:    []string{"h"},
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

func Test_GarbageCollectKusciaJob(t *testing.T) {
	testKusciaJob := makeKusciaJob()
	aliceNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "hello",
		},
	}

	bobNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "world",
		},
	}

	kubeClient := fake.NewSimpleClientset(aliceNs, bobNs)
	kusciaClient := kusciafake.NewSimpleClientset(testKusciaJob)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-job-gccontroller"})

	c := NewKusciaJobGCController(context.TODO(), controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})

	go func() {
		assert.NoError(t, c.Run(1))
	}()

	gcController, ok := c.(*KusciaJobGCController)
	assert.True(t, ok, "Failed to assert type *KusciaJobGCController")

	// Modify the KusciaJob to be outdated
	testKusciaJob.Status.CompletionTime = &metav1.Time{Time: time.Now().Add(-defaultGCDuration - time.Hour)}
	_, err := kusciaClient.KusciaV1alpha1().KusciaJobs(constants.KusciaCrossDomain).Update(context.TODO(), testKusciaJob, metav1.UpdateOptions{})
	assert.NoError(t, err, "Failed to update KusciaJob")

	select {
	case <-time.After(5 * time.Second):
		// Check if the KusciaJob was deleted
		kusciaJobs, _ := gcController.kusciaJobLister.KusciaJobs(constants.KusciaCrossDomain).List(labels.Everything())
		assert.Emptyf(t, kusciaJobs, "Error getting %d KusciaJobs", len(kusciaJobs))
	}
}
