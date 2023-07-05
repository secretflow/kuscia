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

package controller

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	pkgcommon "github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

const (
	testAppImageName = "image_a"
)

func makeKusciaTask(name string, labels map[string]string) *kusciaapisv1alpha1.KusciaTask {
	return &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			Initiator: "alice",
		},
	}
}

func TestHandleAddedOrDeletedKusciaTask(t *testing.T) {
	kubeFakeClient := kubefake.NewSimpleClientset()
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	c := NewController(context.Background(), kubeFakeClient, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}

	kt := makeKusciaTask("task-1", nil)

	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{
			name: "obj type is invalid",
			obj:  "job",
			want: 0,
		},
		{
			name: "kuscia task is valid",
			obj:  kt,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := c.(*Controller)
			cc.handleAddedOrDeletedKusciaTask(tt.obj)
			assert.Equal(t, tt.want, cc.ktQueue.Len())
		})
	}
}

func makeTestAppImage(t *testing.T) *kusciaapisv1alpha1.AppImage {
	componentSpec := map[string]string{
		"component.1.name":                "component_a",
		"component.1.input.1.categories":  "dataset",
		"component.1.input.1.name":        "data",
		"component.1.output.1.categories": "dataset",
		"component.1.output.1.name":       "data",
		"component.2.name":                "component_b",
		"component.2.input.1.categories":  "dataset",
		"component.2.input.1.name":        "data",
		"component.2.output.1.categories": "training_set",
		"component.2.output.1.name":       "train_data",
		"component.2.output.2.categories": "test_set",
		"component.2.output.2.name":       "test_data",
		"component.2.output.3.categories": "model",
		"component.2.output.3.name":       "model",
	}

	componentSpecBytes, err := json.Marshal(componentSpec)
	assert.NoError(t, err)

	appImage := &kusciaapisv1alpha1.AppImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: testAppImageName,
			Annotations: map[string]string{
				pkgcommon.ComponentSpecAnnotationKey: string(componentSpecBytes),
			},
		},
	}

	return appImage
}

func makeTestJob(t *testing.T) *kusciaapisv1alpha1.KusciaJob {
	job := &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "job001",
		},
		Spec: kusciaapisv1alpha1.KusciaJobSpec{
			Tasks: []kusciaapisv1alpha1.KusciaTaskTemplate{
				{
					Alias:           "component_a_1",
					TaskID:          "task001",
					TaskInputConfig: makeTestTaskInputConfig1(t),
				},
				{
					Alias:           "component_b_1",
					TaskID:          "task002",
					TaskInputConfig: makeTestTaskInputConfig2(t),
				},
			},
		},
	}

	return job
}

func makeTestTaskInputConfig1(t *testing.T) string {
	taskInputConfig := &interconn.TaskInputConfig{
		Name:       "component_a_1",
		ModuleName: "component_a",
		Input:      []*interconn.ComponentIO{},
		Output: []*interconn.ComponentIO{
			{
				Type: "dataset",
				Key:  "data0",
			},
		},
		Role: &interconn.ConfigRole{
			Host:  []string{"alice"},
			Guest: []string{"bob"},
		},
		Initiator: &interconn.ConfigInitiator{
			Role:   "host",
			NodeId: "alice",
		},

		TaskParams: &interconn.ConfigParams{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"aa": structpb.NewNumberValue(0.01),
					"bb": structpb.NewNumberValue(4),
					"cc": structpb.NewStringValue("test"),
					"dd": structpb.NewBoolValue(false),
					"ee": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"ee01": structpb.NewStringValue("ee-test"),
							"ee02": structpb.NewNumberValue(10),
						},
					}),
				},
			},
		},
	}

	var err error
	taskInputConfig.TaskParams.Host, err = structpb.NewStruct(map[string]interface{}{
		"0": map[string]interface{}{
			"name":      "host_test_file",
			"namespace": "testspace",
		},
	})
	taskInputConfig.TaskParams.Guest, err = structpb.NewStruct(map[string]interface{}{
		"0": map[string]interface{}{
			"name":      "guest_test_file",
			"namespace": "testspace",
		},
	})

	taskInputConfigBytes, err := json.Marshal(taskInputConfig)
	assert.NoError(t, err)

	return string(taskInputConfigBytes)
}

func makeTestTaskInputConfig2(t *testing.T) string {
	taskInputConfig := &interconn.TaskInputConfig{
		Name:       "component_b_1",
		ModuleName: "component_b",
		Input: []*interconn.ComponentIO{
			{
				Type: "dataset",
				Key:  "component_a_1.data0",
			},
		},
		Output: []*interconn.ComponentIO{
			{
				Type: "training_set",
				Key:  "data0",
			},
			{
				Type: "test_set",
				Key:  "data1",
			},
			{
				Type: "model",
				Key:  "data2",
			},
		},
		Role: &interconn.ConfigRole{
			Host:  []string{"alice"},
			Guest: []string{"bob"},
		},
		Initiator: &interconn.ConfigInitiator{
			Role:   "host",
			NodeId: "alice",
		},
		TaskParams: &interconn.ConfigParams{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hh": structpb.NewNumberValue(0.02),
					"ii": structpb.NewNumberValue(5),
					"jj": structpb.NewStringValue("test"),
					"kk": structpb.NewBoolValue(true),
					"ll": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"ll01": structpb.NewStringValue("ll-test"),
							"ll02": structpb.NewNumberValue(20),
						},
					}),
				},
			},
		},
	}
	taskInputConfigBytes, err := json.Marshal(taskInputConfig)
	assert.NoError(t, err)
	return string(taskInputConfigBytes)
}

func makeTestTask1(t *testing.T, job *kusciaapisv1alpha1.KusciaJob) *kusciaapisv1alpha1.KusciaTask {
	task := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task001",
			Labels: map[string]string{
				pkgcommon.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnBFIA),
				pkgcommon.LabelTaskUnschedulable:     pkgcommon.True,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(job, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("KusciaJob")),
			},
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			Initiator:       "alice",
			TaskInputConfig: makeTestTaskInputConfig1(t),
			Parties: []kusciaapisv1alpha1.PartyInfo{
				{
					DomainID:    "alice",
					AppImageRef: testAppImageName,
					Role:        "host",
					Template: kusciaapisv1alpha1.PartyTemplate{
						Spec: kusciaapisv1alpha1.PodSpec{
							Containers: []kusciaapisv1alpha1.Container{},
						},
					},
				},
				{
					DomainID:    "bob",
					AppImageRef: testAppImageName,
					Role:        "guest",
					Template: kusciaapisv1alpha1.PartyTemplate{
						Spec: kusciaapisv1alpha1.PodSpec{
							Containers: []kusciaapisv1alpha1.Container{},
						},
					},
				},
			},
		},
	}

	return task
}

func makeTestTask2(t *testing.T, job *kusciaapisv1alpha1.KusciaJob) *kusciaapisv1alpha1.KusciaTask {
	task := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task002",
			Labels: map[string]string{
				pkgcommon.LabelInterConnProtocolType: string(kusciaapisv1alpha1.InterConnBFIA),
				pkgcommon.LabelTaskUnschedulable:     pkgcommon.True,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(job, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("KusciaJob")),
			},
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			Initiator:       "alice",
			TaskInputConfig: makeTestTaskInputConfig2(t),
			Parties: []kusciaapisv1alpha1.PartyInfo{
				{
					DomainID:    "alice",
					AppImageRef: testAppImageName,
					Role:        "host",
					Template: kusciaapisv1alpha1.PartyTemplate{
						Spec: kusciaapisv1alpha1.PodSpec{
							Containers: []kusciaapisv1alpha1.Container{
								{
									Name: "container_a",
								},
							},
						},
					},
				},
				{
					DomainID:    "bob",
					AppImageRef: testAppImageName,
					Role:        "guest",
					Template: kusciaapisv1alpha1.PartyTemplate{
						Spec: kusciaapisv1alpha1.PodSpec{
							Containers: []kusciaapisv1alpha1.Container{
								{
									Name: "container_a",
								},
							},
						},
					},
				},
			},
		},
	}

	return task
}

func TestController_syncTaskHandler(t *testing.T) {
	job := makeTestJob(t)
	appImage := makeTestAppImage(t)
	task1 := makeTestTask1(t, job)
	task2 := makeTestTask2(t, job)

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset(job, appImage, task1, task2)

	cc := NewController(context.Background(), kubeClient, kusciaClient, nil)
	c := cc.(*Controller)
	c.kusciaInformerFactory.Start(c.ctx.Done())
	if ok := cache.WaitForCacheSync(c.ctx.Done(), c.kjSynced, c.ktSynced, c.appImageSynced); !ok {
		t.Fatal("failed to wait for caches to sync")
	}

	assert.NoError(t, c.syncTaskHandler(context.Background(), task1.Name))
	assert.NoError(t, c.syncTaskHandler(context.Background(), task2.Name))

	var err error
	task1, err = kusciaClient.KusciaV1alpha1().KusciaTasks().Get(context.Background(), task1.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	task2, err = kusciaClient.KusciaV1alpha1().KusciaTasks().Get(context.Background(), task2.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	envs := getTestPartyTaskEnvMap(&task1.Spec.Parties[0])
	assert.Equal(t, task1.Name, envs["config.task_id"])
	assert.Equal(t, "alice", envs["config.node_id.host.0"])
	assert.Equal(t, "bob", envs["config.node_id.guest.0"])
	assert.Equal(t, "alice", envs["config.inst_id.host.0"])
	assert.Equal(t, "bob", envs["config.inst_id.guest.0"])
	assert.Equal(t, "host.0", envs["config.self_role"])
	assert.Equal(t, "session_"+task1.Name, envs["config.session_id"])
	assert.Equal(t, "trace_"+task1.Name, envs["config.trace_id"])
	assert.Equal(t, "token_"+task1.Name, envs["config.token"])
	assert.Equal(t, "component_a", envs["runtime.component.name"])
	assert.Equal(t, "0.01", envs["runtime.component.parameter.aa"])
	assert.Equal(t, "4", envs["runtime.component.parameter.bb"])
	assert.Equal(t, "test", envs["runtime.component.parameter.cc"])
	assert.Equal(t, "false", envs["runtime.component.parameter.dd"])
	assert.Equal(t, "{\"ee01\":\"ee-test\",\"ee02\":10}", envs["runtime.component.parameter.ee"])
	assertComponentIO(t, &componentIO{Namespace: "testspace", Name: "host_test_file"}, envs["runtime.component.input.data"])
	assertComponentIO(t, &componentIO{Namespace: "job001-host-0", Name: "task001-data0"}, envs["runtime.component.output.data"])

	envs = getTestPartyTaskEnvMap(&task2.Spec.Parties[1])
	assert.Equal(t, task2.Name, envs["config.task_id"])
	assert.Equal(t, "alice", envs["config.node_id.host.0"])
	assert.Equal(t, "bob", envs["config.node_id.guest.0"])
	assert.Equal(t, "alice", envs["config.inst_id.host.0"])
	assert.Equal(t, "bob", envs["config.inst_id.guest.0"])
	assert.Equal(t, "guest.0", envs["config.self_role"])
	assert.Equal(t, "session_"+task2.Name, envs["config.session_id"])
	assert.Equal(t, "trace_"+task2.Name, envs["config.trace_id"])
	assert.Equal(t, "token_"+task2.Name, envs["config.token"])
	assert.Equal(t, "component_b", envs["runtime.component.name"])
	assert.Equal(t, "0.02", envs["runtime.component.parameter.hh"])
	assert.Equal(t, "5", envs["runtime.component.parameter.ii"])
	assert.Equal(t, "test", envs["runtime.component.parameter.jj"])
	assert.Equal(t, "true", envs["runtime.component.parameter.kk"])
	assert.Equal(t, "{\"ll01\":\"ll-test\",\"ll02\":20}", envs["runtime.component.parameter.ll"])
	assertComponentIO(t, &componentIO{Namespace: "job001-guest-0", Name: "task001-data0"}, envs["runtime.component.input.data"])
	assertComponentIO(t, &componentIO{Namespace: "job001-guest-0", Name: "task002-data0"}, envs["runtime.component.output.train_data"])
	assertComponentIO(t, &componentIO{Namespace: "job001-guest-0", Name: "task002-data1"}, envs["runtime.component.output.test_data"])
	assertComponentIO(t, &componentIO{Namespace: "job001-guest-0", Name: "task002-data2"}, envs["runtime.component.output.model"])
}

func getTestPartyTaskEnvMap(party *kusciaapisv1alpha1.PartyInfo) map[string]string {
	envMap := map[string]string{}

	for _, env := range party.Template.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}

	return envMap
}

func assertComponentIO(t *testing.T, expected *componentIO, actual string) {
	stActual := &componentIO{}
	assert.NoError(t, json.Unmarshal([]byte(actual), stActual))
	assert.Equal(t, expected.Namespace, stActual.Namespace)
	assert.Equal(t, expected.Name, stActual.Name)
}
