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

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kbfake "k8s.io/client-go/kubernetes/fake"
)

func TestFindLargestRestartLogPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "example")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	for i := 0; i <= 20; i++ {
		filenames := []string{fmt.Sprintf("%d.log", i), fmt.Sprintf("%d.log.xxxxx", i), fmt.Sprintf("%d.log.xxxxx.gz", i)}
		for _, fileName := range filenames {
			filePath := filepath.Join(tempDir, fileName)
			file, err := os.Create(filePath)
			if err != nil {
				t.Fatalf("Failed to create file %s: %v", fileName, err)
			}
			file.Close()
		}
	}
	logPath, err := findLargestRestartLogPath(tempDir)
	assert.Equal(t, logPath, filepath.Join(tempDir, "20.log"))
	assert.NilError(t, err)

}

func TestFindNewestDirWithPrefix(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "example")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dirNames := []string{
		"task1-1-uuid1",
		"task1-1-uuid2",
		"task2-1-uuid1",
		"task2-2-uuid2",
	}
	for _, dirName := range dirNames {
		path := filepath.Join(tempDir, dirName)
		err := os.Mkdir(path, os.ModePerm)
		assert.NilError(t, err)
		time.Sleep(10 * time.Millisecond)
	}
	dirName, err := findNewestDirWithPrefix(tempDir, "task1-1")
	assert.Equal(t, dirName, filepath.Join(tempDir, "task1-1-uuid2"))
	assert.NilError(t, err)

	dirName, err = findNewestDirWithPrefix(tempDir, "task2-1")
	assert.Equal(t, dirName, filepath.Join(tempDir, "task2-1-uuid1"))
	assert.NilError(t, err)
}

func TestGetPodNode(t *testing.T) {
	task := &v1alpha1.KusciaTask{
		Status: v1alpha1.KusciaTaskStatus{
			// Phase: v1alpha1.TaskPending,
			PodStatuses: map[string]*v1alpha1.PodStatus{
				"pod1": {NodeName: "node"},
				"pod2": {PodPhase: v1.PodPending, NodeName: "node"},
			},
		},
	}
	var nodeName string
	var err error
	nodeName, err = getPodNode(task, "pod1")
	assert.NilError(t, err)
	assert.Equal(t, nodeName, "node")
	_, err = getPodNode(task, "pod2")
	assert.Error(t, err, "task pod pod2 status pending, please try later")
}

func TestQueryLog(t *testing.T) {
	ctx := context.Background()
	logService := buildLogService(ctx, t).(*logService)

	logDir := filepath.Join(logService.conf.StdoutPath, "alice_task1-0_xxxx/secretflow")
	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Error creating log dir: %v\n", err)
	}
	logFile, err := os.Create(filepath.Join(logDir, "0.log"))
	if err != nil {
		t.Fatalf("Error creating log file: %v\n", err)
	}
	defer logFile.Close()
	message := "hello world"
	logFile.WriteString(message)
	request := &kusciaapi.QueryLogRequest{
		TaskId:     "task1",
		ReplicaIdx: 0,
		Container:  "",
		Follow:     false,
	}
	eventCh := make(chan *kusciaapi.QueryLogResponse, 1)
	defer close(eventCh)

	logService.QueryTaskLog(ctx, request, eventCh)
	log := <-eventCh
	assert.Equal(t, log.Log, "hello world")
}

func TestQueryPodNode(t *testing.T) {
	ctx := context.Background()
	logService := buildLogService(ctx, t)
	request := &kusciaapi.QueryPodNodeRequest{
		TaskId:     "task1",
		ReplicaIdx: 0,
		Domain:     "alice",
	}
	resp := logService.QueryPodNode(ctx, request)
	assert.Equal(t, resp.NodeName, "node")
	assert.Equal(t, resp.NodeIp, "192.168.1.1")

}

func buildLogService(ctx context.Context, t *testing.T) ILogService {
	client := buildKusciaClient(ctx)
	stdoutPath, err := os.MkdirTemp("", "example")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(stdoutPath)

	kusciaAPIConfig := &config.KusciaAPIConfig{
		DomainID:     "alice",
		StdoutPath:   stdoutPath,
		NodeName:     "node",
		RunMode:      common.RunModeAutonomy,
		KusciaClient: client,
		KubeClient:   buildKubeClient(),
	}
	logService := NewLogService(kusciaAPIConfig)
	return logService
}

func buildKusciaClient(ctx context.Context) *fake.Clientset {
	client := fake.NewSimpleClientset()
	var replicates *int32 = new(int32)
	*replicates = 1
	appImage := &v1alpha1.AppImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secretflow-appimage",
		},
		Spec: v1alpha1.AppImageSpec{
			DeployTemplates: []v1alpha1.DeployTemplate{
				{
					Name:     "",
					Replicas: replicates,
					Spec: v1alpha1.PodSpec{

						Containers: []v1alpha1.Container{
							{
								Name: "secretflow",
							},
						},
					},
				},
			},
		},
	}
	task := &v1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "task1",
		},
		Spec: v1alpha1.KusciaTaskSpec{
			Parties: []v1alpha1.PartyInfo{
				{
					DomainID:    "alice",
					AppImageRef: "secretflow-appimage",
				},
			},
		},
		Status: v1alpha1.KusciaTaskStatus{
			PodStatuses: map[string]*v1alpha1.PodStatus{
				"alice/task1-0": {NodeName: "node"},
				"alice/task1-1": {PodPhase: v1.PodPending, NodeName: "node"},
			},
		},
	}
	client.KusciaV1alpha1().AppImages().Create(ctx, appImage, metav1.CreateOptions{})
	client.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Create(ctx, task, metav1.CreateOptions{})
	return client
}

func buildKubeClient() *kbfake.Clientset {
	return kbfake.NewSimpleClientset(
		&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node",
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: "192.168.1.1",
					},
				},
			},
		},
	)
}
