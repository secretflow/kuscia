// OPENSOURCE-CLEANUP DELETE_FILE

// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubebackend

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

func TestSigma_PreSyncPod(t *testing.T) {
	s := &sigma{}

	configYaml := `
backend:
  name: sigma
  config:
    allowOverQuota: true
`
	providerCfg := &config.K8sProviderCfg{}
	assert.NoError(t, yaml.Unmarshal([]byte(configYaml), providerCfg))

	backendConfig := providerCfg.Backend.Config
	assert.NoError(t, s.Init(&backendConfig))

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod01",
			Namespace: "test-namespace",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "ctr01",
					Command: []string{"sleep 60"},
					Image:   "aa/bb:001",
				},
			},
		},
	}

	s.PreSyncPod(pod)

	assert.Equal(t, "L3", pod.Labels[labelSigmaMigrationLevel])
	assert.Equal(t, "true", pod.Labels[labelEnableDefaultRoute])
	assert.Equal(t, "true", pod.Annotations[annotationSigmaAutoEviction])
	assert.Equal(t, 1, len(pod.Spec.Tolerations))
	assert.True(t, pod.Spec.Affinity != nil)
}
