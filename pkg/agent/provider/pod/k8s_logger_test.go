// Copyright 2024 Ant Group Co., Ltd.
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

package pod

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK8sLogWorker(t *testing.T) {
	testcases := []struct {
		name   string
		stream bool
	}{
		{
			name:   "mainWorker",
			stream: true,
		},
		{
			name:   "backupWorker",
			stream: false,
		},
	}
	rootDir := t.TempDir()
	bkNamespace := "autonomy-alice"
	namespace := "alice"
	name := "job-psi-0"
	log := "hello world"
	logPathPostfix := "alice_job-psi-0_abcde/secretflow/1.log"
	restart := 1
	UID := "abcde"
	container := "secretflow"
	nodeName := "xxx"
	pod, bkPod := buildPodCase(name, namespace, bkNamespace, nodeName, UID, container, restart, v1.PodSucceeded)

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			_, err := client.CoreV1().Pods(bkNamespace).Create(context.TODO(), bkPod, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("can't create pods")
				return
			}
			worker := NewK8sLogWorker(rootDir, pod, client, bkNamespace, tt.stream)
			patches := gomonkey.ApplyMethod(reflect.TypeOf(worker), "RequestLog", func(_ *K8sLogWorker, ctx context.Context, container string, follow bool) (io.ReadCloser, error) {
				return newMyReadCloser(log), nil
			})
			defer patches.Reset()

			if tt.stream {
				worker.Start(context.Background())
				time.Sleep(time.Second)
				worker.Stop()
			} else {
				worker.BackupLog(context.Background())
			}

			logPath := filepath.Join(rootDir, logPathPostfix)
			if !paths.CheckFileExist(logPath) {
				t.Errorf("log file %s not exist", logPath)
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Errorf("log file %s unreadable, %v", logPath, err)
			}
			assert.Equal(t, string(data), log)
			os.Remove(logPath)

		})
	}

}

func TestK8sLogManager(t *testing.T) {
	ctx := context.Background()
	rootDir := t.TempDir()
	bkNamespace := "autonomy-alice"
	namespace := "alice"
	name := "job-psi-0"
	log := "hello world"
	logPathPostfix := "alice_job-psi-0_abcde/secretflow/1.log"
	restart := 1
	UID := "abcde"
	container := "secretflow"
	nodeName := "xxx"

	testcases := []struct {
		name      string
		podStatus v1.PodPhase
	}{
		{
			name:      "mainWorker",
			podStatus: v1.PodRunning,
		},
		{
			name:      "backupWorker",
			podStatus: v1.PodFailed,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			pod, bkPod := buildPodCase(name, namespace, bkNamespace, nodeName, UID, container, restart, tt.podStatus)

			bkClient := fake.NewSimpleClientset()
			_, err := bkClient.CoreV1().Pods(bkNamespace).Create(context.TODO(), bkPod, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("can't create pods")
			}

			kubeClient := fake.NewSimpleClientset()
			k8sLogManager, err := NewK8sLogManager(nodeName, rootDir, bkClient, bkNamespace, namespace, kubeClient, "10Mi", 3)
			if err != nil {
				t.Errorf("error in creating k8sLogManager, %v", err)
			}
			go func() {
				if err := k8sLogManager.Start(ctx); err != nil {
					t.Errorf("Failed to start pod log manager: %v", err)
				}
			}()
			_, err = kubeClient.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("can't create mirror pod")
			}
			var k8sLogWorker *K8sLogWorker
			patches := gomonkey.ApplyMethod(reflect.TypeOf(k8sLogWorker), "RequestLog", func(_ *K8sLogWorker, ctx context.Context, container string, follow bool) (io.ReadCloser, error) {
				return newMyReadCloser(log), nil
			})
			defer patches.Reset()
			time.Sleep(time.Second)
			logPath := filepath.Join(rootDir, logPathPostfix)
			if !paths.CheckFileExist(logPath) {
				t.Errorf("log file %s not exist", logPath)
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Errorf("log file %s unreadable, %v", logPath, err)
			}
			assert.Equal(t, string(data), log)
			os.Remove(logPath)

		})
	}

}

type MyReadCloser struct {
	reader io.Reader
}

func newMyReadCloser(data string) *MyReadCloser {
	return &MyReadCloser{reader: strings.NewReader(data)}
}

func (m *MyReadCloser) Read(p []byte) (n int, err error) {
	return m.reader.Read(p)
}

func (m *MyReadCloser) Close() error {
	return nil
}

func buildPodCase(name, namespace, bkNamespace, nodeName, uid, container string, restart int, podPhase v1.PodPhase) (*v1.Pod, *v1.Pod) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: container,
				},
			},
			NodeName: nodeName,
		},
		Status: v1.PodStatus{
			Phase: podPhase,
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         container,
					RestartCount: int32(restart),
				},
			},
		},
	}

	bkPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: bkNamespace,
			UID:       types.UID(uid),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: container,
				},
			},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         container,
					RestartCount: int32(restart),
				},
			},
		},
	}
	return pod, bkPod
}
