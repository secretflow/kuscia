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

package configrender

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	resourcetest "github.com/secretflow/kuscia/pkg/agent/resource/testing"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/appconfig"
)

func setupTestConfigRender(t *testing.T) *configRender {
	configYaml := `
name: "config-render"
config:
`
	cfg := &config.PluginCfg{}
	assert.NoError(t, yaml.Unmarshal([]byte(configYaml), cfg))

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	assert.NoError(t, err)

	encValue, err := tls.EncryptOAEP(&privateKey.PublicKey, []byte(`{"user": "kuscia", "host": "localhost"}`))
	assert.NoError(t, err)

	configmap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "alice",
			Name:      "domain-config",
		},
		Data: map[string]string{
			"DB_INFO": encValue,
		},
	}
	kubeClient := kubefake.NewSimpleClientset(&configmap)
	agentConfig := config.DefaultAgentConfig()
	agentConfig.Namespace = "alice"
	agentConfig.DomainKey = privateKey
	dep := &plugin.Dependencies{
		AgentConfig: agentConfig,
		KubeClient:  kubeClient,
	}

	r := &configRender{}
	assert.Equal(t, hook.PluginType, r.Type())
	assert.NoError(t, r.Init(context.Background(), dep, cfg))

	return r
}

func TestConfigRender_ExecHookWithMakeMountsContext(t *testing.T) {
	rootDir := t.TempDir()
	hostPath := filepath.Join(rootDir, "test.conf")
	templateData := "{{.KEY_A}}-{{.KEY_B}}"
	assert.NoError(t, os.WriteFile(hostPath, []byte(templateData), 0644))

	pod := &corev1.Pod{}
	pod.Annotations = map[string]string{
		"kuscia.secretflow/config-template-volumes": "config-template",
	}
	container := corev1.Container{Name: "test-container"}
	pod.Spec.Containers = []corev1.Container{container}

	mount := corev1.VolumeMount{Name: "config-template", SubPath: "test.conf"}

	ctx := &hook.MakeMountsContext{
		Pod:           pod,
		Container:     &container,
		HostPath:      &hostPath,
		Mount:         &mount,
		Envs:          []pkgcontainer.EnvVar{{Name: "key_a", Value: "aaa"}, {Name: "key_b", Value: "bbb"}},
		PodVolumesDir: rootDir,
	}

	cr := setupTestConfigRender(t)
	assert.Equal(t, true, cr.CanExec(ctx))
	_, err := cr.ExecHook(ctx)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(rootDir, "config-render", container.Name, mount.Name, mount.SubPath), hostPath)
	assertFileContent(t, hostPath, "aaa-bbb")
}

func TestConfigRender_ExecHookWithSyncPodContext(t *testing.T) {
	podConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-template",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"config.yaml": "aa={{.AA}}",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "abc",
			Name:      "pod01",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				"kuscia.secretflow/config-template-volumes": "config-template",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "ctr01",
					Command: []string{"sleep 60"},
					Image:   "aa/bb:001",
					Env: []corev1.EnvVar{
						{
							Name:  "AA",
							Value: "BB",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config-template",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "config-template",
							},
						},
					},
				},
			},
		},
	}

	rm := resourcetest.FakeResourceManager("test-namespace", podConfig)

	bkPod := pod.DeepCopy()
	bkPod.Namespace = "bk-namespace"
	ctx := &hook.K8sProviderSyncPodContext{
		Pod:             pod,
		BkPod:           bkPod,
		ResourceManager: rm,
	}

	cr := setupTestConfigRender(t)
	assert.Equal(t, true, cr.CanExec(ctx))
	_, err := cr.ExecHook(ctx)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(ctx.Configmaps))
	assert.Equal(t, "aa=BB", ctx.Configmaps[0].Data["config.yaml"])
}

func TestConfigRender_renderConfigDirectory(t *testing.T) {
	rootDir := t.TempDir()
	templateDir := filepath.Join(rootDir, "template")
	templateSubDir := filepath.Join(templateDir, "sub")
	assert.NoError(t, os.MkdirAll(templateSubDir, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(templateDir, "a.conf"), []byte("{{.KEY_A}}"), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(templateSubDir, "b.conf"), []byte("{{.KEY_B}}"), 0644))

	cr := setupTestConfigRender(t)

	data := map[string]string{
		"KEY_A": "aaa",
		"KEY_B": "bbb",
	}
	assert.NoError(t, cr.renderConfigDirectory(templateDir, rootDir, data))
	assertFileContent(t, filepath.Join(rootDir, "a.conf"), "aaa")
	assertFileContent(t, filepath.Join(rootDir, "sub", "b.conf"), "bbb")
}

func assertFileContent(t *testing.T, file string, content string) {
	fileContent, err := os.ReadFile(file)
	assert.NoError(t, err)
	assert.Equal(t, content, string(fileContent))
}

func TestConfigFormat(t *testing.T) {
	t.Parallel()
	cr := setupTestConfigRender(t)

	tests := []struct {
		templateContent string
		format          string
	}{
		{
			templateContent: `
{
	"task_id": "{{.TASK_ID}}",
	"task_cluster_define": "{{.TASK_CLUSTER_DEFINE}}"
}`,
			format: "json",
		},
		{
			templateContent: `
task_id: "{{.TASK_ID}}"
task_cluster_define: "{{.TASK_CLUSTER_DEFINE}}"
`,
			format: "yaml",
		},
	}

	taskClusterDefine := map[string]string{
		"alice": "test.alice.svc",
		"bob":   "test.bob.svc",
	}
	taskInputConfigBytes, err := json.Marshal(taskClusterDefine)
	assert.NoError(t, err)
	envs := map[string]string{
		"TASK_ID":             "abc",
		"TASK_CLUSTER_DEFINE": string(taskInputConfigBytes),
	}
	data, err := cr.makeDataMap(map[string]string{}, envs)
	assert.NoError(t, err)
	t.Logf("%+v", data)

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test %d with %s format", i, tt.format), func(t *testing.T) {
			rootDir := t.TempDir()

			templateFile := filepath.Join(rootDir, "template.conf")
			assert.NoError(t, os.WriteFile(templateFile, []byte(tt.templateContent), 0644))

			configFile := filepath.Join(rootDir, "file.conf")

			assert.NoError(t, cr.renderConfigFile(templateFile, configFile, data))

			f, err := os.ReadFile(configFile)
			assert.NoError(t, err)
			conf := map[string]string{}

			switch tt.format {
			case "json":
				assert.NoError(t, json.Unmarshal(f, &conf))
			case "yaml":
				assert.NoError(t, yaml.Unmarshal(f, &conf))
			default:
				t.Error("Invalid config format", tt.format)
			}

			t.Logf("%+v", conf)

			taskClusterDefineConf := map[string]string{}
			assert.NoError(t, json.Unmarshal([]byte(conf["task_cluster_define"]), &taskClusterDefineConf))

			assert.Equal(t, taskClusterDefine, taskClusterDefineConf)
			assert.Equal(t, "abc", conf["task_id"])
		})
	}
}

func TestMergeDataMaps_WithJsonMap(t *testing.T) {
	t.Parallel()
	dst := buildStructMap(map[string]string{
		"KEY1": "{\"xyz\":1}",
		"KEY2": "124",
	})

	assert.Len(t, dst, 2)
	assert.Contains(t, dst, "KEY1")
	assert.Contains(t, dst, "KEY2")

	assert.Contains(t, dst["KEY1"], "xyz")
	assert.Equal(t, dst["KEY1"].(map[string]interface{})["xyz"], float64(1))
}

func TestBuildStructMap_WithJsonArray(t *testing.T) {
	t.Parallel()
	dst := buildStructMap(map[string]string{
		"KEY1": "[1,2,3]",
		"KEY2": "[\"1\"]",
	})

	assert.Len(t, dst, 2)
	assert.Contains(t, dst, "KEY1")
	assert.Contains(t, dst, "KEY2")

	assert.Len(t, dst["KEY1"], 3)
	assert.Len(t, dst["KEY2"], 1)
	assert.Len(t, dst["KEY2"], 1)
	assert.Equal(t, dst["KEY1"], []interface{}{float64(1), float64(2), float64(3)})
	assert.Equal(t, dst["KEY2"], []interface{}{"1"})
}

func TestBuildStructMap_WithJsonMapArray(t *testing.T) {
	t.Parallel()
	dst := buildStructMap(map[string]string{
		"KEY3": "[{\"xyz\":\"1\"}]",
		"KEY4": "{\"xyz\":[\"2\"]}",
	})

	assert.Len(t, dst, 2)
	assert.Contains(t, dst, "KEY3")
	assert.Contains(t, dst, "KEY4")

	assert.Len(t, dst["KEY3"], 1)
	assert.NotNil(t, dst["KEY3"].([]interface{})[0])
	assert.Contains(t, dst["KEY3"].([]interface{})[0], "xyz")
	assert.Contains(t, dst["KEY3"].([]interface{})[0].(map[string]interface{})["xyz"], "1")

	assert.Contains(t, dst["KEY4"], "xyz")
	assert.Equal(t, dst["KEY4"].(map[string]interface{})["xyz"], []interface{}{"2"})
}

func TestBuildStructMap_WithStruct(t *testing.T) {
	t.Parallel()
	cr := setupTestConfigRender(t)

	configTemplate := `
	{
		"task_id": "{{.TASK_ID}}",
		"task_cluster_define": "{{.TASK_CLUSTER_DEFINE}}",
		"my_party": "{{{.TASK_CLUSTER_DEFINE.selfPartyIdx}}}"
	}`

	taskInputConfigBytes, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(&appconfig.ClusterDefine{
		SelfPartyIdx: 1,
	})
	assert.NoError(t, err)
	envs := map[string]string{
		"TASK_ID":             "abc",
		"TASK_CLUSTER_DEFINE": string(taskInputConfigBytes),
	}
	data, err := cr.makeDataMap(map[string]string{}, envs)
	assert.NoError(t, err)
	t.Logf("%+v", data)

	rootDir := t.TempDir()

	templateFile := filepath.Join(rootDir, "template.conf")
	assert.NoError(t, os.WriteFile(templateFile, []byte(configTemplate), 0644))

	configFile := filepath.Join(rootDir, "file.conf")

	assert.NoError(t, cr.renderConfigFile(templateFile, configFile, data))

	f, err := os.ReadFile(configFile)
	assert.NoError(t, err)

	var conf interface{}
	assert.NoError(t, json.Unmarshal(f, &conf))

	t.Logf("Unmarshal config=%+v", conf)

	r := conf.(map[string]interface{})
	assert.Equal(t, "abc", r["task_id"])
	assert.Equal(t, "1", r["my_party"])
}

func TestBuildStructMap_WithOrg(t *testing.T) {
	configTemplate := `
	{
		"task_id": "{{.TASK_ID}}",
		"task_cluster_define": "{{.TASK_CLUSTER_DEFINE}}",
		"my_party": "{{.TASK_CLUSTER_DEFINE.selfPartyIdx}}"
	}`

	taskInputConfigBytes, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(&appconfig.ClusterDefine{
		SelfPartyIdx: 1,
		Parties: []*appconfig.Party{
			{
				Name: "xyz",
			},
		},
	})
	assert.NoError(t, err)
	nlog.Infof("byte=%s", string(taskInputConfigBytes))

	var value interface{}
	assert.NoError(t, json.Unmarshal(taskInputConfigBytes, &value))

	data := make(map[string]interface{})
	data["TASK_CLUSTER_DEFINE"] = value
	data["TASK_ID"] = "abc"

	tmpl, err := template.New("test").Option(defaultTemplateRenderOption).Parse(configTemplate)
	assert.NoError(t, err)

	var buf bytes.Buffer
	assert.NoError(t, tmpl.Execute(&buf, data))
	nlog.Infof(buf.String())

	var test interface{}
	assert.NoError(t, json.Unmarshal(buf.Bytes(), &test))
	assert.Equal(t, "1", test.(map[string]interface{})["my_party"])

	configTemplate1 := `
	{
		"task_id": "{{.TASK_ID}}",
		"task_cluster_define": "{{.TASK_CLUSTER_DEFINE}}",
		"my_party": "{{.TASK_CLUSTER_DEFINE.selfPartyIdx}}",
		"p1_name": "{{.TASK_CLUSTER_DEFINE.parties[0].name}}"
	}`

	_, err = template.New("test").Option(defaultTemplateRenderOption).Parse(configTemplate1)
	assert.Error(t, err)

	configTemplate2 := `
	{
		"task_id": "{{.TASK_ID}}",
		"task_cluster_define": "{{.TASK_CLUSTER_DEFINE}}",
		"my_party": "{{.TASK_CLUSTER_DEFINE.selfPartyIdx}}",
		"p1_name": "{{.TASK_CLUSTER_DEFINE.parties[name=test].name}}"
	}`

	_, err = template.New("test").Option(defaultTemplateRenderOption).Parse(configTemplate2)
	assert.Error(t, err)
}

var clusterDefine = `
{
	"parties": [{
		"name": "alice",
		"role": "",
		"services": [{
			"portName": "spu",
			"endpoints": ["secretflow-task-20240816114341-single-psi-0-spu.alice.svc"]
		}, {
			"portName": "fed",
			"endpoints": ["secretflow-task-20240816114341-single-psi-0-fed.alice.svc"]
		}, {
			"portName": "global",
			"endpoints": ["secretflow-task-20240816114341-single-psi-0-global.alice.svc:23321"]
		}]
	}, {
		"name": "bob",
		"role": "",
		"services": [{
			"portName": "spu",
			"endpoints": ["secretflow-task-20240816114341-single-psi-0-spu.bob.svc"]
		}, {
			"portName": "fed",
			"endpoints": ["secretflow-task-20240816114341-single-psi-0-fed.bob.svc"]
		}, {
			"portName": "global",
			"endpoints": ["secretflow-task-20240816114341-single-psi-0-global.bob.svc:20002"]
		}]
	}],
	"selfPartyIdx": 1,
	"selfEndpointIdx": 0
}
`

func TestRenderConfig_WithKusciaCM(t *testing.T) {
	t.Parallel()
	cr := setupTestConfigRender(t)

	configTemplate := `
	{
		"db_info": "{{.DB_INFO}}",
		"db_info_user": "{{{.DB_INFO.user}}}",
		"db_info_host": "{{{.DB_INFO.host}}}",
	   "db_info_user_host": "{{{.DB_INFO.user}}}-{{{.DB_INFO.host}}}",
	   "db_info_user.host": "{{{.DB_INFO.user}}}.{{{.DB_INFO.host}}}"
	}`

	data := map[string]string{}
	got, err := cr.renderConfig(configTemplate, data)
	assert.NoError(t, err)

	output := map[string]string{}
	err = json.Unmarshal([]byte(got), &output)
	assert.NoError(t, err)
	assert.Equal(t, `{"user": "kuscia", "host": "localhost"}`, output["db_info"])
	assert.Equal(t, "kuscia", output["db_info_user"])
	assert.Equal(t, "localhost", output["db_info_host"])
	assert.Equal(t, "kuscia-localhost", output["db_info_user_host"])
	assert.Equal(t, "kuscia.localhost", output["db_info_user.host"])

	// doesn't fetch from cm
	data = map[string]string{
		"DB_INFO": `{"user": "kuscia", "host":"localhost"}`,
	}
	got, err = cr.renderConfig(configTemplate, data)
	err = json.Unmarshal([]byte(got), &output)
	assert.NoError(t, err)
	assert.Equal(t, data["DB_INFO"], output["db_info"])
	assert.Equal(t, "kuscia", output["db_info_user"])
	assert.Equal(t, "localhost", output["db_info_host"])
	assert.Equal(t, "kuscia-localhost", output["db_info_user_host"])
	assert.Equal(t, "kuscia.localhost", output["db_info_user.host"])

	data = map[string]string{"TASK_CLUSTER_DEFINE": clusterDefine}
	configTemplate = `
	{
        "self_endpoint": "{{{.TASK_CLUSTER_DEFINE.parties[.TASK_CLUSTER_DEFINE.selfPartyIdx].services[0].endpoints[0]}}}"
	}`
	got, _ = cr.renderConfig(configTemplate, data)
	err = json.Unmarshal([]byte(got), &output)
	assert.NoError(t, err)
	assert.Equal(t, "secretflow-task-20240816114341-single-psi-0-spu.bob.svc", output["self_endpoint"])
}
