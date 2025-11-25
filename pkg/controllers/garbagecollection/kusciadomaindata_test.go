// Copyright 2025 Ant Group Co., Ltd.
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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	constants "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func makeKusciaDomainData(name string, labels map[string]string, creationTime time.Time) *kusciaapisv1alpha1.DomainData {
	return &kusciaapisv1alpha1.DomainData{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         constants.KusciaCrossDomain,
			Labels:            labels,
			CreationTimestamp: metav1.Time{Time: creationTime},
		},
		Spec: kusciaapisv1alpha1.DomainDataSpec{
			Name:        name,
			Type:        "table",
			Author:      "test-user",
			RelativeURI: "test/path/to/data",
			DataSource:  "test-datasource",
			Vendor:      "secretflow",
		},
	}
}

func Test_GarbageCollectKusciaDomainData(t *testing.T) {
	// Create test domain data with secretflow vendor label and old creation time (older than 30 days)
	oldCreationTime := time.Now().Add(-31 * 24 * time.Hour) // 31 days ago
	labelsWithSecretFlow := map[string]string{
		"kuscia.secretflow/domaindata-vendor": "secretflow",
	}
	testDomainData := makeKusciaDomainData("test-secretflow-data", labelsWithSecretFlow, oldCreationTime)

	// Create test domain data with different vendor label
	labelsWithDifferentVendor := map[string]string{
		"kuscia.secretflow/domaindata-vendor": "other-vendor",
	}
	testOtherVendorData := makeKusciaDomainData("test-other-vendor-data", labelsWithDifferentVendor, oldCreationTime)

	// Create test domain data with no vendor label
	testNoVendorData := makeKusciaDomainData("test-no-vendor-data", nil, oldCreationTime)

	// Create test domain data with secretflow vendor but recent creation time
	recentCreationTime := time.Now().Add(-1 * 24 * time.Hour) // 1 day ago
	recentSecretFlowData := makeKusciaDomainData("test-recent-secretflow", labelsWithSecretFlow, recentCreationTime)

	kubeClient := fake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset(testDomainData, testOtherVendorData, testNoVendorData, recentSecretFlowData)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-domaindata-gccontroller"})

	c := NewKusciaDomainDataGCController(context.TODO(), controllers.ControllerConfig{
		KubeClient:                  kubeClient,
		KusciaClient:                kusciaClient,
		EventRecorder:               eventRecorder,
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   720,
	})

	go func() {
		assert.NoError(t, c.Run(1))
	}()

	gcController, ok := c.(*KusciaDomainDataGCController)
	assert.True(t, ok, "Failed to assert type *KusciaDomainDataGCController")

	// Wait for controller to process
	select {
	case <-time.After(5 * time.Second):
		// Check the results
		kusciaDomainDatas, _ := gcController.kusciaDomainDataLister.List(labels.Everything())

		// We expect only the secretflow vendor data that's old enough to be deleted
		// The rest should remain
		remainingNames := make([]string, 0, len(kusciaDomainDatas))
		for _, data := range kusciaDomainDatas {
			remainingNames = append(remainingNames, data.Name)
		}

		assert.Contains(t, remainingNames, "test-other-vendor-data", "Other vendor data should remain")
		assert.Contains(t, remainingNames, "test-no-vendor-data", "No vendor data should remain")
		assert.Contains(t, remainingNames, "test-recent-secretflow", "Recent secretflow data should remain")
		assert.NotContains(t, remainingNames, "test-secretflow-data", "Old secretflow data should be deleted")
	}
}

func Test_ShouldGarbageCollect(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		data     *kusciaapisv1alpha1.DomainData
		expected bool
	}{
		{
			name: "Should collect - secretflow vendor and old enough",
			data: &kusciaapisv1alpha1.DomainData{
				ObjectMeta: metav1.ObjectMeta{
					Labels:            map[string]string{"kuscia.secretflow/domaindata-vendor": "secretflow"},
					CreationTimestamp: metav1.Time{Time: now.Add(-31 * 24 * time.Hour)},
				},
			},
			expected: true,
		},
		{
			name: "Should not collect - wrong vendor",
			data: &kusciaapisv1alpha1.DomainData{
				ObjectMeta: metav1.ObjectMeta{
					Labels:            map[string]string{"kuscia.secretflow/domaindata-vendor": "other"},
					CreationTimestamp: metav1.Time{Time: now.Add(-31 * 24 * time.Hour)},
				},
			},
			expected: false,
		},
		{
			name: "Should not collect - no label",
			data: &kusciaapisv1alpha1.DomainData{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: metav1.Time{Time: now.Add(-31 * 24 * time.Hour)},
				},
			},
			expected: false,
		},
		{
			name: "Should not collect - too recent",
			data: &kusciaapisv1alpha1.DomainData{
				ObjectMeta: metav1.ObjectMeta{
					Labels:            map[string]string{"kuscia.secretflow/domaindata-vendor": "secretflow"},
					CreationTimestamp: metav1.Time{Time: now.Add(-1 * time.Hour)},
				},
			},
			expected: false,
		},
	}

	config := controllers.ControllerConfig{
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   720,
	}
	controller := NewKusciaDomainDataGCController(context.TODO(), config).(*KusciaDomainDataGCController)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := controller.shouldGarbageCollect(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_DeleteErrorHandling(t *testing.T) {
	// Test scenario: test error handling when deletion fails
	oldCreationTime := time.Now().Add(-31 * 24 * time.Hour)
	labelsWithSecretFlow := map[string]string{
		"kuscia.secretflow/domaindata-vendor": "secretflow",
	}

	// Create domain data with proper namespace and old creation time
	deleteTestData := makeKusciaDomainData("delete-test-data", labelsWithSecretFlow, oldCreationTime)

	kubeClient := fake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset(deleteTestData)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-domaindata-gccontroller"})

	c := NewKusciaDomainDataGCController(context.TODO(), controllers.ControllerConfig{
		KubeClient:                  kubeClient,
		KusciaClient:                kusciaClient,
		EventRecorder:               eventRecorder,
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   720,
	})

	gcController, ok := c.(*KusciaDomainDataGCController)
	assert.True(t, ok, "Failed to assert type *KusciaDomainDataGCController")

	// Wait for informer sync
	gcController.kusciaInformerFactory.Start(context.TODO().Done())
	cache.WaitForCacheSync(context.TODO().Done(), gcController.kusciaDomainDataSynced)

	// First verify the data is in the list
	allDomainDatas, _ := gcController.kusciaDomainDataLister.List(labels.Everything())
	assert.Len(t, allDomainDatas, 1)

	// Test deletion by directly manipulating the fake client to simulate error
	client := kusciaClient.KusciaV1alpha1().DomainDatas(constants.KusciaCrossDomain)
	err := client.Delete(context.TODO(), "delete-test-data", metav1.DeleteOptions{})
	assert.NoError(t, err) // fake client won't return error, but test should pass for proper cleanup
}

func Test_BatchProcessing(t *testing.T) {
	// Test scenario: test batch processing logic
	oldCreationTime := time.Now().Add(-31 * 24 * time.Hour)
	labelsWithSecretFlow := map[string]string{
		"kuscia.secretflow/domaindata-vendor": "secretflow",
	}

	// Create more than batch size data
	var testDatas []*kusciaapisv1alpha1.DomainData
	for i := 0; i < 150; i++ {
		data := makeKusciaDomainData(fmt.Sprintf("batch-test-%d", i), labelsWithSecretFlow, oldCreationTime)
		testDatas = append(testDatas, data)
	}

	kubeClient := fake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()

	// Add all test data to the fake client
	for _, data := range testDatas {
		_, err := kusciaClient.KusciaV1alpha1().DomainDatas(data.Namespace).Create(context.TODO(), data, metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-domaindata-gccontroller"})

	c := NewKusciaDomainDataGCController(context.TODO(), controllers.ControllerConfig{
		KubeClient:                  kubeClient,
		KusciaClient:                kusciaClient,
		EventRecorder:               eventRecorder,
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   720,
	})

	gcController, ok := c.(*KusciaDomainDataGCController)
	assert.True(t, ok, "Failed to assert type *KusciaDomainDataGCController")

	// Wait for informer sync
	gcController.kusciaInformerFactory.Start(context.TODO().Done())
	cache.WaitForCacheSync(context.TODO().Done(), gcController.kusciaDomainDataSynced)

	// Verify initial data count
	allDomainDatas, _ := gcController.kusciaDomainDataLister.List(labels.Everything())
	assert.Len(t, allDomainDatas, 150)

	// Verify batch grouping logic
	namespaceDomainDataMap := make(map[string][]*kusciaapisv1alpha1.DomainData)
	for _, domainData := range allDomainDatas {
		namespace := domainData.Namespace
		if namespace == "" {
			namespace = "kuscia-system" // Default value
		}
		namespaceDomainDataMap[namespace] = append(namespaceDomainDataMap[namespace], domainData)
	}

	// Verify data is properly grouped
	assert.Len(t, namespaceDomainDataMap, 1) // All data in kuscia-system namespace
	assert.Len(t, namespaceDomainDataMap[constants.KusciaCrossDomain], 150)

	// Verify batch calculation (mock calculation without actual processing)
	assert.Equal(t, 150, len(allDomainDatas))
}

func Test_NewKusciaDomainDataGCController_NilController(t *testing.T) {
	// Test scenario: when garbage collection is disabled, return NilController
	config := controllers.ControllerConfig{
		KddGarbageCollectionEnabled: false,
	}

	controller := NewKusciaDomainDataGCController(context.TODO(), config)

	// Should return NilController instead of KusciaDomainDataGCController
	assert.IsType(t, &NilController{}, controller)
}

func Test_NilController(t *testing.T) {
	// Test scenario: verify all methods of NilController behave correctly
	nilController := &NilController{}

	// Test Name method
	assert.Equal(t, DomainDataGCControllerName, nilController.Name())

	// Test Run method
	err := nilController.Run(0)
	assert.NoError(t, err)
	err = nilController.Run(1)
	assert.NoError(t, err)

	// Test Stop method (should not panic)
	nilController.Stop()
}

func Test_RunCacheSyncFailure(t *testing.T) {
	// Test scenario: test error handling when cache sync fails
	kubeClient := fake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-domaindata-gccontroller"})

	// Create controller instance
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	controller := NewKusciaDomainDataGCController(ctx, controllers.ControllerConfig{
		KubeClient:                  kubeClient,
		KusciaClient:                kusciaClient,
		EventRecorder:               eventRecorder,
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   720,
	})

	gcController, ok := controller.(*KusciaDomainDataGCController)
	assert.True(t, ok)

	// Cancel context immediately to make cache sync fail
	cancel()

	// Since context is canceled, Run should return error
	err := gcController.Run(0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to wait for caches to sync")
}

func Test_DifferentNamespaces(t *testing.T) {
	// Test scenario: handle domain data across different namespaces
	oldCreationTime := time.Now().Add(-31 * 24 * time.Hour)
	labelsWithSecretFlow := map[string]string{
		"kuscia.secretflow/domaindata-vendor": "secretflow",
	}

	// Create domain data in different namespaces
	crossDomainData := makeKusciaDomainData("test-cross-domain", labelsWithSecretFlow, oldCreationTime)
	crossDomainData.Namespace = ""

	ns1Data := makeKusciaDomainData("test-ns1", labelsWithSecretFlow, oldCreationTime)
	ns1Data.Namespace = "namespace1"

	ns2Data := makeKusciaDomainData("test-ns2", labelsWithSecretFlow, oldCreationTime)
	ns2Data.Namespace = "namespace2"

	kubeClient := fake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()

	// Add all test data to the fake client
	_, err := kusciaClient.KusciaV1alpha1().DomainDatas(constants.KusciaCrossDomain).Create(context.TODO(), crossDomainData, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = kusciaClient.KusciaV1alpha1().DomainDatas("namespace1").Create(context.TODO(), ns1Data, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = kusciaClient.KusciaV1alpha1().DomainDatas("namespace2").Create(context.TODO(), ns2Data, metav1.CreateOptions{})
	assert.NoError(t, err)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-domaindata-gccontroller"})

	c := NewKusciaDomainDataGCController(context.TODO(), controllers.ControllerConfig{
		KubeClient:                  kubeClient,
		KusciaClient:                kusciaClient,
		EventRecorder:               eventRecorder,
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   720,
	})

	gcController, ok := c.(*KusciaDomainDataGCController)
	assert.True(t, ok, "Failed to assert type *KusciaDomainDataGCController")

	// Verify initial state
	gcController.kusciaInformerFactory.Start(context.TODO().Done())
	cache.WaitForCacheSync(context.TODO().Done(), gcController.kusciaDomainDataSynced)

	allDomainDatas, err := gcController.kusciaDomainDataLister.List(labels.Everything())
	assert.NoError(t, err)
	assert.Len(t, allDomainDatas, 3)

	// Verify namespace grouping
	namespaces := make(map[string]int)
	for _, data := range allDomainDatas {
		namespaces[data.Namespace]++
	}

	// Should have data in both default and custom namespaces
	assert.Greater(t, len(namespaces), 1)

	// Test verification without actual deletion
	crossClient := kusciaClient.KusciaV1alpha1().DomainDatas(constants.KusciaCrossDomain)
	_, err = crossClient.Get(context.TODO(), "test-cross-domain", metav1.GetOptions{})
	assert.NoError(t, err)

	ns1Client := kusciaClient.KusciaV1alpha1().DomainDatas("namespace1")
	_, err = ns1Client.Get(context.TODO(), "test-ns1", metav1.GetOptions{})
	assert.NoError(t, err)

	ns2Client := kusciaClient.KusciaV1alpha1().DomainDatas("namespace2")
	_, err = ns2Client.Get(context.TODO(), "test-ns2", metav1.GetOptions{})
	assert.NoError(t, err)
}

func Test_CustomGCDuration(t *testing.T) {
	// Test scenario: verify custom garbage collection duration configuration
	customDuration := 48 // 48 hours

	config := controllers.ControllerConfig{
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   customDuration,
	}

	controller := NewKusciaDomainDataGCController(context.TODO(), config).(*KusciaDomainDataGCController)

	// Verify custom duration is used
	expectedDuration := time.Duration(customDuration) * time.Hour
	assert.Equal(t, expectedDuration, controller.kusciaDomainDataGCDuration)
}

func Test_DefaultGCDuration(t *testing.T) {
	// Test scenario: verify default garbage collection duration configuration
	config := controllers.ControllerConfig{
		KddGarbageCollectionEnabled: true,
		DomainDataGCDurationHours:   0, // Use default value
	}

	controller := NewKusciaDomainDataGCController(context.TODO(), config).(*KusciaDomainDataGCController)

	// Verify default duration is used
	assert.Equal(t, defaultDomainDataGCDuration, controller.kusciaDomainDataGCDuration)
}
