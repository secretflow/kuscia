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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	resourcetest "github.com/secretflow/kuscia/pkg/agent/resource/testing"
)

func setupTestConfigRender(t *testing.T) *configRender {
	configYaml := `
name: "config-render"
config:
`
	cfg := &config.PluginCfg{}
	assert.NoError(t, yaml.Unmarshal([]byte(configYaml), cfg))

	agentConfig := config.DefaultAgentConfig()

	dep := &plugin.Dependencies{
		AgentConfig: agentConfig,
	}

	r := &configRender{}
	assert.Equal(t, hook.PluginType, r.Type())
	assert.NoError(t, r.Init(dep, cfg))

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
