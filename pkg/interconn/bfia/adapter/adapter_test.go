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

package adapter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

var taskInputConfigForLogistic = `
{
  "name":"hetero_logistic_regression_1",
  "module_name":"hetero_logistic_regression",
  "output":[
    {
      "type":"training_set",
      "key":"data1"
    },
    {
      "type":"test_set",
      "key":"data1"
    },
    {
      "type":"model",
      "key":"data2"
    }
  ],
  "input":[
    {
      "type":"training_set",
      "key":"intersect_rsa_1.data0"
    }
  ],
  "role":{
    "host":[
      "jg0110017800000000"
    ],
    "guest":[
      "jg0100001100000000"
    ]
  },
  "initiator":{
    "role":"guest",
    "node_id":"jg0100001100000000"
  },
  "task_params":{
    "common":{
        "C":0.01,
        "batch_size":-1,
        "level":"4",
        "penalty":"L2"
    },
    "host":{
      "0":{}
    },
    "guest":{
      "0":{}
    }
  }
}
`

var taskInputConfigForIntersect = `
{
  "name":"intersect_rsa_1",
  "module_name":"intersect_rsa",
  "output":[
    {
      "type":"training_set",
      "key":"data0"
    },
    {
      "type":"test_set",
      "key":"data1"
    },
    {
      "type":"model",
      "key":"data2"
    }
  ],
  "role":{
    "arbiter":[
      "jg0110017800000000"
    ],
    "host":[
      "jg0100001100000000"
    ],
    "guest":[
      "jg0110017800000000"
    ]
  },
  "initiator":{
    "role":"guest",
    "node_id":"jg0110017800000000"
  },
  "task_params":{
    "arbiter":{
      "0":{}
    },
    "common":{
       "intersect_method": "rsa",
       "sync_intersect_ids": true,
       "only_output_key": false,
       "rsa_params": {
         "hash_method": "sha256",
         "final_hash_method": "sha256",
         "key_length": 2048
       }
    },
    "host":{
      "0":{
           "name": "test_host",
           "namespace": "testspace"
	  }
    },
    "guest":{
      "0":{
		  "name": "test_guest",
          "namespace": "testspace"
      }
    }
  }
}
`

var interConnJob = `{"dag":{"version":"2.0.0","components":[{"code":"secretflow","name":"intersect_rsa_1","module_name":"intersect_rsa","version":"v1.0.0","output":[{"type":"training_set","key":"data0"},{"type":"test_set","key":"data1"},{"type":"model","key":"data2"}]},{"code":"secretflow","name":"hetero_logistic_regression_1","module_name":"hetero_logistic_regression","version":"v1.0.0","input":[{"type":"training_set","key":"intersect_rsa_1.data0"}],"output":[{"type":"training_set","key":"data1"},{"type":"test_set","key":"data1"},{"type":"model","key":"data2"}]}]},"config":{"role":{"arbiter":["jg0110017800000000"],"host":["jg0100001100000000"],"guest":["jg0110017800000000"]},"initiator":{"role":"guest","node_id":"jg0110017800000000"},"job_params":{"host":{"0":{}},"arbiter":{"0":{}},"guest":{"0":{}}},"task_params":{"host":{"0":{"hetero_logistic_regression_1":{},"intersect_rsa_1":{"name":"test_host","namespace":"testspace"}}},"arbiter":{"0":{"intersect_rsa_1":{}}},"guest":{"0":{"hetero_logistic_regression_1":{},"intersect_rsa_1":{"name":"test_guest","namespace":"testspace"}}},"common":{"hetero_logistic_regression_1":{"C":0.01,"batch_size":-1,"level":"4","penalty":"L2"},"intersect_rsa_1":{"intersect_method":"rsa","only_output_key":false,"rsa_params":{"final_hash_method":"sha256","hash_method":"sha256","key_length":2048},"sync_intersect_ids":true}}},"version":"2.0.0"},"job_id":"secretflow","flow_id":"flow-1"}`

func makeKusciaJob() *kusciaapisv1alpha1.KusciaJob {
	return &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secretflow",
		},
		Spec: kusciaapisv1alpha1.KusciaJobSpec{
			Initiator: "alice",
			FlowID:    "flow-1",
			Tasks: []kusciaapisv1alpha1.KusciaTaskTemplate{
				{
					Alias:           "intersect_rsa_1",
					TaskID:          "secretflow-task-20230510103243",
					AppImage:        "secretflow",
					TaskInputConfig: taskInputConfigForIntersect,
					Priority:        0,
					Parties: []kusciaapisv1alpha1.Party{
						{
							DomainID: "jg0100001100000000",
							Role:     "host",
						},
						{
							DomainID: "jg0110017800000000",
							Role:     "arbiter",
						},
						{
							DomainID: "jg0110017800000000",
							Role:     "guest",
						},
					},
				},
				{
					Alias:           "hetero_logistic_regression_1",
					TaskID:          "secretflow-task-20230510103244",
					AppImage:        "secretflow",
					TaskInputConfig: taskInputConfigForLogistic,
					Priority:        0,
					Parties: []kusciaapisv1alpha1.Party{
						{
							DomainID: "jg0100001100000000",
							Role:     "host",
						},
						{
							DomainID: "jg0110017800000000",
							Role:     "guest",
						},
					},
				},
			},
		},
	}
}

func makeAppImage() *kusciaapisv1alpha1.AppImage {
	return &kusciaapisv1alpha1.AppImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secretflow",
		},
		Spec: kusciaapisv1alpha1.AppImageSpec{
			Image: kusciaapisv1alpha1.AppImageInfo{
				Name: "secretflow",
				Tag:  "v1.0.0",
			},
			DeployTemplates: []kusciaapisv1alpha1.DeployTemplate{
				{
					Name: "secretflow",
					Role: "host",
					Spec: kusciaapisv1alpha1.PodSpec{
						Containers: []kusciaapisv1alpha1.Container{
							{
								Name: "secretflow",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:     resource.MustParse("0.5"),
										corev1.ResourceMemory:  resource.MustParse("10.05"),
										corev1.ResourceStorage: resource.MustParse("100"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestGenerateInterConnJobInfoFrom(t *testing.T) {
	appImage := makeAppImage()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	appImageInformer := kusciaInformerFactory.Kuscia().V1alpha1().AppImages()
	appImageInformer.Informer().GetStore().Add(appImage)

	kusciaJob := makeKusciaJob()
	jobSpec, err := GenerateInterConnJobInfoFrom(kusciaJob, appImageInformer.Lister())
	if err != nil {
		t.Fatal(err)
	}

	jobSpecRet, _ := json.Marshal(jobSpec)
	assert.Equal(t, interConnJob, string(jobSpecRet))
}

func TestBuildInterConnJobParamsResources(t *testing.T) {
	appImage := makeAppImage()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	appImageInformer := kusciaInformerFactory.Kuscia().V1alpha1().AppImages()
	appImageInformer.Informer().GetStore().Add(appImage)

	role := "host"
	name := "secretflow"
	ret, err := buildInterConnJobParamsResources(appImageInformer.Lister(), role, name)
	if err != nil {
		t.Fatal(err)
	}

	expectedResources := &JobParamsResources{
		Disk:   100,
		Memory: 10.05,
		CPU:    0.5,
	}

	assert.Equal(t, expectedResources, ret)
}

func TestBuildKusciaJobDependenciesFrom(t *testing.T) {
	jobInfo := &InterConnJobInfo{}
	jobInfo.DAG = &interconn.DAG{
		Components: []*interconn.Component{
			{
				Name:       "d1",
				ModuleName: "d1",
				Output: []*interconn.ComponentIO{
					{
						Type: "dataset",
						Key:  "data1",
					},
				},
			},
			{
				Name:       "d2",
				ModuleName: "d2",
				Input: []*interconn.ComponentIO{
					{
						Type: "dataset",
						Key:  "d1.data1",
					},
				},
				Output: []*interconn.ComponentIO{
					{
						Type: "dataset",
						Key:  "data2",
					},
				},
			},
			{
				Name:       "d3",
				ModuleName: "d3",
				Input: []*interconn.ComponentIO{
					{
						Type: "dataset",
						Key:  "d1.data1",
					},
					{
						Type: "dataset",
						Key:  "d2.data2",
					},
				},
				Output: []*interconn.ComponentIO{
					{
						Type: "dataset",
						Key:  "d3",
					},
				},
			},
			{
				Name:       "d4",
				ModuleName: "d4",
				Input: []*interconn.ComponentIO{
					{
						Type: "dataset",
						Key:  "d1.data1",
					},
				},
				Output: []*interconn.ComponentIO{
					{
						Type: "dataset",
						Key:  "data4",
					},
				},
			},
		},
	}

	expected := map[string][]string{
		"d2": {"d1"},
		"d3": {"d1", "d2"},
		"d4": {"d1"},
	}
	deps := buildKusciaJobDependenciesFrom(jobInfo)
	assert.Equal(t, expected, deps)
}

func TestBuildTaskInputConfigAndPartiesFrom(t *testing.T) {
	cpt := &interconn.Component{
		Code:       "secretflow",
		Name:       "intersect_rsa_1",
		ModuleName: "intersection",
		Input: []*interconn.ComponentIO{
			{
				Type: "dataset",
				Key:  "d1",
			},
		},
		Output: []*interconn.ComponentIO{
			{
				Type: "dataset",
				Key:  "d2",
			},
		},
	}

	expectedTaskInputConfig := `{"name":"intersect_rsa_1","module_name":"intersection","input":[{"type":"dataset","key":"d1"}],"output":[{"type":"dataset","key":"d2"}],"role":{"arbiter":["jg0110017800000000"],"host":["jg0100001100000000"],"guest":["jg0110017800000000"]},"initiator":{"role":"guest","node_id":"jg0110017800000000"},"task_params":{"host":{"0":{"name":"test_host","namespace":"testspace"}},"guest":{"0":{"name":"test_guest","namespace":"testspace"}},"common":{"id":"id","intersect_type":"intersect_rsa","node_center":"guest.0"}}}`
	expectedParties := []kusciaapisv1alpha1.Party{
		{
			DomainID: "jg0100001100000000",
			Role:     "host",
		},
		{
			DomainID: "jg0110017800000000",
			Role:     "guest",
		},
	}

	taskParams := `{"arbiter":{"0":{"hetero_logistic_regression_1":{}}},"common":{"hetero_logistic_regression_1":{"C":0.01,"batch_size":-1,"level":"4","penalty":"L2","early_stop":"weight_diff","label":"y","use_spark":true,"tol":0.0001,"encrypt_type":"paillier","fit_intercept":true,"id":"id","init_method":"init_zero","shuffle":true,"max_iter":5,"learning_rate":0.15,"train":0.7,"solver":"nesterov_momentum_sgd"},"intersect_rsa_1":{"node_center":"guest.0","intersect_type":"intersect_rsa","id":"id"}},"host":{"0":{"hetero_logistic_regression_1":{},"intersect_rsa_1":{"name":"test_host","namespace":"testspace"}}},"guest":{"0":{"hetero_logistic_regression_1":{},"intersect_rsa_1":{"name":"test_guest","namespace":"testspace"}}}}`
	config := &interconn.Config{
		Role: &interconn.ConfigRole{
			Arbiter: []string{"jg0110017800000000"},
			Host:    []string{"jg0100001100000000"},
			Guest:   []string{"jg0110017800000000"},
		},
		Initiator: &interconn.ConfigInitiator{
			Role:   "guest",
			NodeId: "jg0110017800000000",
		},
	}
	config.TaskParams = &interconn.ConfigParams{}
	json.Unmarshal([]byte(taskParams), config.TaskParams)
	taskInputConf, parties, _ := buildTaskInputConfigAndPartiesFrom(cpt, config)
	assert.Equal(t, expectedTaskInputConfig, taskInputConf)
	assert.Equal(t, expectedParties, parties)
}

func TestGenerateKusciaJobFrom(t *testing.T) {
	jobInfo := &InterConnJobInfo{}
	json.Unmarshal([]byte(interConnJob), jobInfo)
	kusciaJob, _ := GenerateKusciaJobFrom(jobInfo)
	kj, _ := json.Marshal(kusciaJob)
	expectedKj := `{"metadata":{"name":"secretflow","namespace":"cross-domain","creationTimestamp":null,"labels":{"kuscia.secretflow/interconn-protocol-type":"bfia","kuscia.secretflow/job-stage":"Create"}},"spec":{"flowID":"flow-1","initiator":"jg0110017800000000","scheduleMode":"Strict","maxParallelism":2,"tasks":[{"alias":"intersect_rsa_1","tolerable":false,"appImage":"secretflow","taskInputConfig":"{\"name\":\"intersect_rsa_1\",\"module_name\":\"intersect_rsa\",\"output\":[{\"type\":\"training_set\",\"key\":\"data0\"},{\"type\":\"test_set\",\"key\":\"data1\"},{\"type\":\"model\",\"key\":\"data2\"}],\"role\":{\"arbiter\":[\"jg0110017800000000\"],\"host\":[\"jg0100001100000000\"],\"guest\":[\"jg0110017800000000\"]},\"initiator\":{\"role\":\"guest\",\"node_id\":\"jg0110017800000000\"},\"task_params\":{\"host\":{\"0\":{\"name\":\"test_host\",\"namespace\":\"testspace\"}},\"arbiter\":{\"0\":{}},\"guest\":{\"0\":{\"name\":\"test_guest\",\"namespace\":\"testspace\"}},\"common\":{\"intersect_method\":\"rsa\",\"only_output_key\":false,\"rsa_params\":{\"final_hash_method\":\"sha256\",\"hash_method\":\"sha256\",\"key_length\":2048},\"sync_intersect_ids\":true}}}","parties":[{"domainID":"jg0110017800000000","role":"arbiter"},{"domainID":"jg0100001100000000","role":"host"},{"domainID":"jg0110017800000000","role":"guest"}]},{"alias":"hetero_logistic_regression_1","dependencies":["intersect_rsa_1"],"tolerable":false,"appImage":"secretflow","taskInputConfig":"{\"name\":\"hetero_logistic_regression_1\",\"module_name\":\"hetero_logistic_regression\",\"input\":[{\"type\":\"training_set\",\"key\":\"intersect_rsa_1.data0\"}],\"output\":[{\"type\":\"training_set\",\"key\":\"data1\"},{\"type\":\"test_set\",\"key\":\"data1\"},{\"type\":\"model\",\"key\":\"data2\"}],\"role\":{\"arbiter\":[\"jg0110017800000000\"],\"host\":[\"jg0100001100000000\"],\"guest\":[\"jg0110017800000000\"]},\"initiator\":{\"role\":\"guest\",\"node_id\":\"jg0110017800000000\"},\"task_params\":{\"host\":{\"0\":{}},\"guest\":{\"0\":{}},\"common\":{\"C\":0.01,\"batch_size\":-1,\"level\":\"4\",\"penalty\":\"L2\"}}}","parties":[{"domainID":"jg0100001100000000","role":"host"},{"domainID":"jg0110017800000000","role":"guest"}]}]},"status":{}}`
	assert.Equal(t, expectedKj, string(kj))
}
