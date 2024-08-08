package garbagecollection

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	constants "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
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

func Test_GarbageCollectKusciajob(t *testing.T) {
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

	kubeClient := fake.NewSimpleClientset(aliceNs, bobNs)
	kusciaClient := kusciafake.NewSimpleClientset(testKusciaJob)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-job-gccontroller"})

	c := NewKusciajobGCController(context.TODO(), controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})

	go func() {
		assert.NoError(t, c.Run(1))
	}()

	gcController, ok := c.(*KusciajobGCController)
	if !ok {
		t.Errorf("Failed to assert type *KusciajobGCController")
		return
	}

	// Modify the KusciaJob to be outdated
	testKusciaJob.Status.CompletionTime = &metav1.Time{Time: time.Now().Add(-defaultGCDuration - time.Hour)}
	_, err := kusciaClient.KusciaV1alpha1().KusciaJobs(constants.KusciaCrossDomain).Update(context.TODO(), testKusciaJob, metav1.UpdateOptions{})
	if err != nil {
		t.Errorf("Failed to update KusciaJob: %v", err)
		return
	}

	select {
	case <-time.After(5 * time.Second):
		// Check if the KusciaJob was deleted
		kusciaJobListers, _ := gcController.kusciaJobLister.KusciaJobs(constants.KusciaCrossDomain).List(labels.Everything())
		if len(kusciaJobListers) == 0 {
			fmt.Printf("Successfully deleted outdated kusciajob\n")
		} else {
			t.Errorf("Error getting %v KusciaJobs\n", len(kusciaJobListers))
		}
	}
}
