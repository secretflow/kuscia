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

package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/secretflow/kuscia/pkg/agent/config"
)

func TestSourceRun(t *testing.T) {
	pods := []*corev1.Pod{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				UID:       "10001",
				Name:      "pod1",
				Namespace: "default",
			},
		},
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				UID:       "10002",
				Name:      "pod2",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "image",
					Image: "test/image",
				}},
			},
		},
	}
	kubeClient := fake.NewSimpleClientset(pods[0])

	dirName, err := mkTempDir("file-test")
	assert.NoError(t, err)
	defer os.RemoveAll(dirName)

	fileName := filepath.Join(dirName, "test_pod_manifest")
	updates := make(chan kubetypes.PodUpdate, 10)

	initCfg := &InitConfig{
		Namespace: "default",
		NodeName:  "test-node",
		SourceCfg: &config.SourceCfg{
			Apiserver: config.ApiserverSourceCfg{KubeConnCfg: config.KubeConnCfg{KubeconfigFile: "/tmp/kubeconfig"}},
			File:      config.FileSourceCfg{Enable: true, Path: dirName, Period: time.Second * 30},
		},
		KubeClient: kubeClient,
		Updates:    updates,
		Recorder:   record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: "agent"}),
	}

	m := NewManager(initCfg)
	stopCh := make(chan struct{})
	assert.NoError(t, m.Run(stopCh))

	expectUpdateInSource(t, "expect pod add event from apiserver", updates,
		kubetypes.PodUpdate{Op: kubetypes.ADD, Source: kubetypes.ApiserverSource, Pods: []*corev1.Pod{pods[0]}})

	// update event from file
	fileContents, err := runtime.Encode(scheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion), pods[1])
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(fileName, fileContents, 0644))

	expectUpdateInSource(t, "expect pod add event from file", updates,
		kubetypes.PodUpdate{Op: kubetypes.ADD, Source: kubetypes.FileSource, Pods: []*corev1.Pod{pods[1]}})
}

func expectUpdateInSource(t *testing.T, testDesc string, ch chan kubetypes.PodUpdate, expected kubetypes.PodUpdate) {
	timer := time.After(5 * time.Second)
	for {
		select {
		case update := <-ch:
			assert.Equal(t, expected.Op, update.Op)
			assert.Equal(t, expected.Source, update.Source)
			return
		case <-timer:
			t.Fatalf("%s: Expected update, timeout instead", testDesc)
		}
	}
}
