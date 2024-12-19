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

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"gotest.tools/v3/assert"
)

var (
	client       = fake.NewSimpleClientset()
	createReqStr = `{
		"name": "appimage-template",
		"config_templates": {
			"task-config.conf": "{\n  \"task_id\": \"{{.TASK_ID}}\",\n  \"task_input_config\": \"{{.TASK_INPUT_CONFIG}}\",\n  \"task_input_cluster_def\": \"{{.TASK_CLUSTER_DEFINE}}\",\n  \"allocated_ports\": \"{{.ALLOCATED_PORTS}}\"\n}\n"
		},
		"deploy_templates": [
			{
				"name": "app",
				"network_policy": {
					"ingresses": [
						{
							"from": {
								"roles": [
									"client"
								]
							},
							"ports": [
								{
									"port": "global"
								}
							]
						}
					]
				},
				"replicas": 1,
				"role": "server",
				"containers": [
					{
						"args": [
							"-c",
							"./app --role=server --task_config_path=/etc/kuscia/task-config.conf"
						],
						"command": [
							"sh"
						],
						"config_volume_mounts": [
							{
								"mount_path": "/etc/kuscia/task-config.conf",
								"sub_path": "task-config.conf"
							}
						],
						"env": [
							{
								"name": "APP_NAME",
								"value": "app"
							}
						],
						"env_from": [
							{
								"prefix": "xxx",
								"config_map_ref": {
									"name": "config-template"
								},
								"secret_map_ref": {
									"name": "secret-template"
								}
							}
						],
						"image_pull_policy": "IfNotPresent",
						"liveness_probe": {
							"failure_threshold": 1,
							"http_get": {
								"path": "/healthz",
								"port": "global"
							},
							"period_seconds": 20
						},
						"name": "app",
						"ports": [
							{
								"name": "global",
								"protocol": "HTTP",
								"scope": "Cluster"
							}
						],
						"readiness_probe": {
							"exec": {
								"command": [
									"cat",
									"/tmp/healthy"
								]
							},
							"initial_delay_seconds": 5,
							"period_seconds": 5
						},
						"resources": {
							"limits": {
								"cpu": "100m",
								"memory": "100Mi"
							},
							"requests": {
								"cpu": "100m",
								"memory": "100Mi"
							}
						},
						"startup_probe": {
							"failure_threshold": 30,
							"http_get": {
								"path": "/healthz",
								"port": "global"
							},
							"period_seconds": 10
						},
						"working_dir": "/work"
					}
				],
				"restart_policy": "Never"
			}
		],
		"image": {
			"id": "adlipoidu8yuahd6",
			"name": "app-image",
			"sign": "nkdy7pad09iuadjd",
			"tag": "v1.0.0"
		}
	}`
)

func TestCreateAppImage(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: client,
		RunMode:      common.RunModeAutonomy,
	}
	s := NewAppImageService(conf)

	req := new(kusciaapi.CreateAppImageRequest)
	if err := json.Unmarshal([]byte(createReqStr), req); err != nil {
		t.Errorf("unmarshall failed")
		return
	}
	resp := s.CreateAppImage(context.Background(), req)
	assert.Equal(t, utils.IsSuccessCode(resp.Status.Code), true)
}

func TestQueryAppImage(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: client,
		RunMode:      common.RunModeAutonomy,
	}
	s := NewAppImageService(conf)
	queryReq := &kusciaapi.QueryAppImageRequest{Name: "appimage-template"}
	resp := s.QueryAppImage(context.Background(), queryReq)
	assert.Equal(t, utils.IsSuccessCode(resp.Status.Code), true)

	createReq := new(kusciaapi.CreateAppImageRequest)
	if err := json.Unmarshal([]byte(createReqStr), createReq); err != nil {
		t.Errorf("unmarshall failed")
		return
	}
	assert.Equal(t, resp.Data.Name, createReq.Name)
	assert.Equal(t, resp.Data.Image.String(), createReq.Image.String())
	assert.DeepEqual(t, resp.Data.ConfigTemplates, createReq.ConfigTemplates)
	assert.DeepEqual(t, len(resp.Data.DeployTemplates), len(createReq.DeployTemplates))
	for i := 0; i < len(resp.Data.DeployTemplates); i++ {
		assert.Equal(t, resp.Data.DeployTemplates[i].String(), createReq.DeployTemplates[i].String())
	}

}

func TestUpdateAppImage(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: client,
		RunMode:      common.RunModeAutonomy,
	}
	s := NewAppImageService(conf)

	req := new(kusciaapi.UpdateAppImageRequest)
	if err := json.Unmarshal([]byte(createReqStr), req); err != nil {
		t.Errorf("unmarshall failed")
		return
	}
	resp := s.UpdateAppImage(context.Background(), req)
	assert.Equal(t, utils.IsSuccessCode(resp.Status.Code), true)
}

func TestDeleteAppImage(t *testing.T) {
	conf := &config.KusciaAPIConfig{
		KusciaClient: client,
		RunMode:      common.RunModeAutonomy,
	}
	s := NewAppImageService(conf)

	req := &kusciaapi.DeleteAppImageRequest{Name: "appimage-template"}
	resp := s.DeleteAppImage(context.Background(), req)
	assert.Equal(t, utils.IsSuccessCode(resp.Status.Code), true)
}
