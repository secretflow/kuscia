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

package domain

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	"github.com/secretflow/kuscia/pkg/utils/signals"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func TestNewController(t *testing.T) {
	stopCh := make(chan struct{})
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("default")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, apicorev1.EventSource{Component: "domain-controller"})

	c := NewController(signals.NewKusciaContextWithStopCh(stopCh), controllers.ControllerConfig{
		KubeClient:    kubeClient,
		KusciaClient:  kusciaClient,
		EventRecorder: eventRecorder,
	})
	go func() {
		time.Sleep(1 * time.Second)
		close(stopCh)
	}()
	err := c.Run(1)
	assert.NoError(t, err)
}

// Add test cases for handling node events
func TestHandleNodeAdd(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Test normal node addition
	node := &apicorev1.Node{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				common.LabelNodeNamespace: "test-domain",
			},
			ResourceVersion: "123",
		},
	}
	c.handleNodeAdd(node)
	assert.Equal(t, 1, c.nodeQueue.Len(), "Node queue should have 1 item")

	// Test empty ResourceVersion
	node.ResourceVersion = ""
	c.handleNodeAdd(node)
	assert.Equal(t, 1, c.nodeQueue.Len(), "Should skip node with empty ResourceVersion")
}

func TestHandleNodeUpdate(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	oldNode := &apicorev1.Node{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				common.LabelNodeNamespace: "test-domain",
			},
			ResourceVersion: "123",
		},
	}

	// Test status change
	newNode := oldNode.DeepCopy()
	newNode.ResourceVersion = "456"
	newNode.Status.Conditions = []apicorev1.NodeCondition{{
		Type:   apicorev1.NodeReady,
		Status: apicorev1.ConditionTrue,
	}}
	c.handleNodeUpdate(oldNode, newNode)
	assert.Equal(t, 1, c.nodeQueue.Len(), "Should enqueue node with status change")

	// Testing without actual changes
	newNode.Status = oldNode.Status
	c.handleNodeUpdate(oldNode, newNode)
	assert.Equal(t, 1, c.nodeQueue.Len(), "Should skip node with no status change")
}

func TestHandleNodeDelete(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Test normal node deletion
	node := &apicorev1.Node{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				common.LabelNodeNamespace: "test-domain",
			},
		},
	}
	c.handleNodeDelete(node)
	assert.Equal(t, 1, c.nodeQueue.Len(), "Node queue should have 1 item")

	// Test the DeletedFinalStateUnknown type
	dfsu := cache.DeletedFinalStateUnknown{
		Key: "test-node",
		Obj: node,
	}
	c.handleNodeDelete(dfsu)
	assert.Equal(t, 2, c.nodeQueue.Len(), "Should handle DeletedFinalStateUnknown")
}

// Add Pod event handling test
func TestHandlePodAdd(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Test normal Pod addition
	pod := &apicorev1.Pod{
		ObjectMeta: apismetav1.ObjectMeta{
			Name:            "test-pod",
			Namespace:       "test-domain",
			ResourceVersion: "123",
		},
		Spec: apicorev1.PodSpec{
			NodeName: "test-node",
		},
	}
	c.handlePodAdd(pod)
	assert.Equal(t, 1, c.podQueue.Len(), "Pod队列应有1个元素")

	// Test empty ResourceVersion filtering
	emptyRVPod := pod.DeepCopy()
	emptyRVPod.ResourceVersion = ""
	c.handlePodAdd(emptyRVPod)
	assert.Equal(t, 1, c.podQueue.Len(), "应过滤空ResourceVersion的Pod")
}

func TestHandlePodDelete(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Test normal Pod deletion
	pod := &apicorev1.Pod{
		ObjectMeta: apismetav1.ObjectMeta{
			Name:            "test-pod",
			Namespace:       "test-domain",
			ResourceVersion: "123",
		},
		Spec: apicorev1.PodSpec{
			NodeName: "test-node",
		},
	}
	c.handlePodDelete(pod)
	assert.Equal(t, 1, c.podQueue.Len(), "Pod队列应有1个元素")

	// Testing the handling of the DeletedFinalStateUnknown type
	dfsu := cache.DeletedFinalStateUnknown{
		Key: "test-pod",
		Obj: pod,
	}
	c.handlePodDelete(dfsu)
	assert.Equal(t, 2, c.podQueue.Len(), "应处理DeletedFinalStateUnknown类型")

	// Test invalid object type
	c.handlePodDelete("invalid-type")
	assert.Equal(t, 2, c.podQueue.Len(), "应跳过无效对象类型")
}

// Add handlePodCommon test
func TestHandlePodCommon(t *testing.T) {
	c, _, _ := setupController(t)

	t.Run("AddOperation", func(t *testing.T) {
		validPod := &apicorev1.Pod{ObjectMeta: apismetav1.ObjectMeta{ResourceVersion: "123"}}
		c.handlePodCommon(validPod, common.Add)
		require.Equal(t, 1, c.podQueue.Len())
	})

	t.Run("DeleteOperation", func(t *testing.T) {
		validPod := &apicorev1.Pod{ObjectMeta: apismetav1.ObjectMeta{ResourceVersion: "456"}}
		c.handlePodCommon(validPod, common.Delete)
		require.Equal(t, 2, c.podQueue.Len())
	})
}

func TestHandleNodeCommon(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Test normal node addition
	node := &apicorev1.Node{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				common.LabelNodeNamespace: "test-domain",
			},
			ResourceVersion: "123",
		},
	}
	c.handleNodeCommon(node, common.Add)
	assert.Equal(t, 1, c.nodeQueue.Len(), "应入队正常节点")

	// Test empty ResourceVersion filtering
	emptyRVNode := node.DeepCopy()
	emptyRVNode.ResourceVersion = ""
	c.handleNodeCommon(emptyRVNode, common.Add)
	assert.Equal(t, 1, c.nodeQueue.Len(), "应过滤空ResourceVersion节点")

	// Testing the handling of the DeletedFinalStateUnknown type
	dfsu := cache.DeletedFinalStateUnknown{
		Key: "test-node",
		Obj: "invalid-type",
	}
	c.handleNodeCommon(dfsu, common.Delete)
	assert.Equal(t, 1, c.nodeQueue.Len(), "应处理无效对象类型")
}

func TestNodeHandler(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Prepare test nodes
	testNode := &apicorev1.Node{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				common.LabelNodeNamespace: "test-domain",
			},
		},
		Status: apicorev1.NodeStatus{
			Conditions: []apicorev1.NodeCondition{{
				Type:               apicorev1.NodeReady,
				Status:             apicorev1.ConditionTrue,
				LastHeartbeatTime:  apismetav1.Now(),
				LastTransitionTime: apismetav1.Now(),
			}},
		},
	}

	// Execute processing
	err := c.nodeHandler(&queue.NodeQueueItem{Node: testNode, Op: common.Add})
	assert.NoError(t, err, "处理节点不应返回错误")

	// Verify status update
	statuses := c.nodeStatusManager.GetAll()
	assert.Equal(t, 1, len(statuses), "应更新节点状态")
	assert.Equal(t, "test-node", statuses["test-node"].Name, "节点名称匹配")
	assert.Equal(t, nodeStatusReady, statuses["test-node"].Status, "节点状态应为Ready")

	// Test invalid node status
	invalidNode := testNode.DeepCopy()
	invalidNode.Status.Conditions[0].Status = apicorev1.ConditionFalse
	err = c.nodeHandler(&queue.NodeQueueItem{Node: invalidNode, Op: common.Add})
	assert.NoError(t, err)
	updatedStatus := c.nodeStatusManager.Get("test-node")
	assert.Equal(t, nodeStatusNotReady, updatedStatus.Status, "应检测到节点不可用状态")
}

// Test the calRequestResource function
func TestCalRequestResource(t *testing.T) {
	c := NewController(context.Background(), controllers.ControllerConfig{}).(*Controller)

	// Test Pod without Resource Requests
	emptyPod := &apicorev1.Pod{
		Spec: apicorev1.PodSpec{
			Containers: []apicorev1.Container{{
				Name: "test-container",
			}},
		},
	}
	cpu, mem := c.calRequestResource(emptyPod)
	assert.Equal(t, int64(0), cpu, "应处理无资源请求的Pod")
	assert.Equal(t, int64(0), mem, "应处理无资源请求的Pod")

	// Test multiple container resource requests
	multiContainerPod := &apicorev1.Pod{
		Spec: apicorev1.PodSpec{
			Containers: []apicorev1.Container{{
				Resources: apicorev1.ResourceRequirements{
					Requests: apicorev1.ResourceList{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("1Gi"),
					},
				},
			}, {
				Resources: apicorev1.ResourceRequirements{
					Requests: apicorev1.ResourceList{
						"cpu":    resource.MustParse("1"),
						"memory": resource.MustParse("2Gi"),
					},
				},
			}},
		},
	}
	cpu, mem = c.calRequestResource(multiContainerPod)
	assert.Equal(t, int64(1500), cpu, "应正确累加多容器CPU请求")
	assert.Equal(t, int64(3221225472), mem, "应正确累加多容器内存请求")
}

// Test the podHandler function
func TestPodHandler(t *testing.T) {
	c, _, _ := setupController(t)

	c.nodeStatusManager.UpdateStatus(common.LocalNodeStatus{
		Name:            "valid-node",
		TotalCPURequest: 1000,
		TotalMemRequest: 1024,
	}, "add")

	testPod := &apicorev1.Pod{
		Spec: apicorev1.PodSpec{
			NodeName: "valid-node",
			Containers: []apicorev1.Container{{
				Resources: apicorev1.ResourceRequirements{
					Requests: apicorev1.ResourceList{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}

	t.Run("AddOperation", func(t *testing.T) {
		err := c.podHandler(&queue.PodQueueItem{Pod: testPod, Op: common.Add})
		require.NoError(t, err)

		localNodeStatus := c.nodeStatusManager.Get("valid-node")
		require.Equal(t, int64(1500), localNodeStatus.TotalCPURequest)
	})
}

// Test addPodHandler resource to increase logic
func TestAddPodHandler(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Initialize node status
	c.nodeStatusManager.UpdateStatus(common.LocalNodeStatus{
		Name:            "test-node",
		TotalCPURequest: 1000,
		TotalMemRequest: 1024,
	}, "add")

	// Create Test Pod
	testPod := &apicorev1.Pod{
		Spec: apicorev1.PodSpec{
			NodeName: "test-node",
			Containers: []apicorev1.Container{{
				Resources: apicorev1.ResourceRequirements{
					Requests: apicorev1.ResourceList{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}

	// Perform add operation
	err := c.addPodHandler(testPod)
	assert.NoError(t, err, "应成功添加Pod资源")

	// Verify resource statistics
	status := c.nodeStatusManager.Get("test-node")
	assert.Equal(t, int64(1500), status.TotalCPURequest, "CPU请求应正确累加")
	assert.Equal(t, int64(1073741824+1024), status.TotalMemRequest, "内存请求应正确累加")

	// Test invalid nodes
	invalidPod := testPod.DeepCopy()
	invalidPod.Spec.NodeName = "non-existent"
	err = c.addPodHandler(invalidPod)
	assert.Error(t, err, "应处理不存在的节点")
}

// Test deletePodHandler resource reduction logic
func TestDeletePodHandler(t *testing.T) {
	c, _, _ := setupController(t)
	c.nodeStatusManager.UpdateStatus(common.LocalNodeStatus{
		Name:            "test-node",
		TotalCPURequest: 5000,
		TotalMemRequest: 4294967296,
	}, "add")

	testPod := &apicorev1.Pod{
		Spec: apicorev1.PodSpec{
			NodeName: "test-node",
			Containers: []apicorev1.Container{{
				Resources: apicorev1.ResourceRequirements{
					Requests: apicorev1.ResourceList{
						"cpu":    resource.MustParse("1000m"),
						"memory": resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}

	t.Run("ResourceUnderflow", func(t *testing.T) {
		err := c.deletePodHandler(testPod)
		require.Equal(t, nil, err)
	})
}

// Add matchNodeLabels test
func TestMatchNodeLabels(t *testing.T) {
	c, _, domainStore := setupController(t)
	domain := &kusciaapisv1alpha1.Domain{ObjectMeta: apismetav1.ObjectMeta{Name: "test-domain"}}
	err := domainStore.Add(domain)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		node     *apicorev1.Node
		expected bool
	}{
		{
			name: "valid domain label",
			node: &apicorev1.Node{
				ObjectMeta: apismetav1.ObjectMeta{
					Labels: map[string]string{
						common.LabelNodeNamespace: "test-domain",
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := c.matchNodeLabels(tc.node)
			require.Equal(t, tc.expected, actual)
		})
	}
}

// Add initLocalNodeStatus test
func TestInitLocalNodeStatus(t *testing.T) {
	c, nodeStore, domainStore := setupController(t)

	node := &apicorev1.Node{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				common.LabelNodeNamespace: "test-domain",
			},
		},

		Status: apicorev1.NodeStatus{
			Conditions: []apicorev1.NodeCondition{
				{
					Type:               apicorev1.NodeReady,
					Status:             apicorev1.ConditionTrue,
					LastHeartbeatTime:  apismetav1.Now(),
					LastTransitionTime: apismetav1.Now(),
				},
			},
		},
	}

	err1 := nodeStore.Add(node)
	require.NoError(t, err1)
	domain := &kusciaapisv1alpha1.Domain{ObjectMeta: apismetav1.ObjectMeta{Name: "test-domain"}}
	err := domainStore.Add(domain)
	require.NoError(t, err)

	t.Run("ValidInitialization", func(t *testing.T) {
		err := c.initLocalNodeStatus()
		require.NoError(t, err)

		statuses := c.nodeStatusManager.GetAll()
		require.Equal(t, 1, len(statuses), "应初始化1个节点状态")
		require.Equal(t, "test-node", statuses["test-node"].Name)
	})
}

func setupController(t *testing.T) (*Controller, cache.Store, cache.Store) {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()

	nodeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0).Core().V1().Nodes()
	domainInformer := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0).Kuscia().V1alpha1().Domains()

	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	}).(*Controller)

	// Set test specific fields through direct assignment method
	c.nodeStatusManager = common.NewNodeStatusManager()
	c.podQueue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.nodeQueue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.nodeStatusManager.ReplaceAll(make(map[string]common.LocalNodeStatus))
	c.nodeLister = nodeInformer.Lister()
	c.domainLister = domainInformer.Lister()

	nodeStore := nodeInformer.Informer().GetStore()
	domainStore := domainInformer.Informer().GetStore()

	t.Cleanup(func() {
		c.podQueue.ShutDown()
		c.nodeQueue.ShutDown()
	})

	return c, nodeStore, domainStore
}

func TestMatchLabels(t *testing.T) {
	c := Controller{}
	testCases := []struct {
		name string
		obj  apismetav1.Object
		want bool
	}{
		{
			name: "label match",
			obj: &apicorev1.Namespace{
				ObjectMeta: apismetav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{common.LabelDomainName: "test"},
				},
			},
			want: true,
		},
		{
			name: "label doesn't match",
			obj: &apicorev1.Namespace{
				ObjectMeta: apismetav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"test": "test"},
				},
			},
			want: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok := c.matchLabels(tc.obj)
			assert.Equal(t, tc.want, ok)
		})
	}
}

func TestEnqueueDomain(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	domain := makeTestDomain("ns1")
	cc.enqueueDomain(domain)
	assert.Equal(t, 1, cc.workqueue.Len())
}

func TestEnqueueResourceQuota(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	rq := makeTestResourceQuota("ns1")
	cc.enqueueDomain(rq)
	assert.Equal(t, 1, cc.workqueue.Len())
}

func TestEnqueueNamespace(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	cc := c.(*Controller)

	rq := makeTestNamespace("ns1")
	cc.enqueueDomain(rq)
	assert.Equal(t, 1, cc.workqueue.Len())
}

func TestSyncHandler(t *testing.T) {
	domain1 := makeTestDomain("ns1")
	domain2 := makeTestDomain("ns2")
	ns1 := makeTestNamespace("ns1")
	ns3 := makeTestNamespace("ns3")

	kusciaFakeClient := kusciafake.NewSimpleClientset(domain1, domain2)
	kubeFakeClient := kubefake.NewSimpleClientset(ns1)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)

	dmInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()

	nsInformer := kubeInformerFactory.Core().V1().Namespaces()
	nsInformer.Informer().GetStore().Add(ns1)
	nsInformer.Informer().GetStore().Add(ns3)

	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:    kubeFakeClient,
		KusciaClient:  kusciaFakeClient,
		EventRecorder: record.NewFakeRecorder(2),
	})
	cc := c.(*Controller)
	cc.domainLister = dmInformer.Lister()
	cc.namespaceLister = nsInformer.Lister()

	testCases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "domain is not found",
			key:     "ns3",
			wantErr: false,
		},
		{
			name:    "create domain namespace",
			key:     "ns2",
			wantErr: false,
		},
		{
			name:    "update domain namespace",
			key:     "ns1",
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cc.syncHandler(context.Background(), tc.key)
			if tc.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestName(t *testing.T) {
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	kubeFakeClient := kubefake.NewSimpleClientset()
	c := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeFakeClient,
		KusciaClient: kusciaFakeClient,
	})
	got := c.Name()
	assert.Equal(t, controllerName, got)
}
