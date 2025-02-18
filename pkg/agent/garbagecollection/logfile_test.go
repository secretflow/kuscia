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

package garbagecollection

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/agent/utils/fileutils"
	"github.com/stretchr/testify/assert"
)

// create temp directory for test
func CreateTempDir() (string, error) {
	dir, err := os.MkdirTemp("", "gc-test-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	return dir, nil
}

func CreateTempFile(dir string, fileName string, isDir bool) (string, error) {
	path := filepath.Join(dir, fileName)
	var err error
	if isDir {
		err = os.Mkdir(path, 0644)
	} else {
		_, err = os.Create(path)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	return path, nil
}

func changePathTime(path string, duration time.Duration) error {
	oldTime := time.Now().Add(-duration)
	return os.Chtimes(path, oldTime, oldTime)
}

// return all path, the first level sub dir/files, and the dir/files will be left after gc
func CreateGCTestData() (dir string, allPaths []string, childPaths []string, leftPaths []string, pods []*v1.Pod, err error) {
	dir, err = CreateTempDir()

	pods = []*v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sf-job-00",
				Namespace: "test-ns",
			},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
			},
		},

		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sf-job-01",
				Namespace: "test-ns",
			},
			Status: v1.PodStatus{
				Phase: v1.PodFailed,
			},
		},
	}

	// create old file
	// left after gc
	path, err := CreateTempFile(dir, "alice_sf-job-00_12345678-1234-1234-1234-12345678abcd", true)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)
	leftPaths = append(leftPaths, path)

	path, err = CreateTempFile(path, "secretflow", true)
	allPaths = append(allPaths, path)
	leftPaths = append(leftPaths, path)

	path, err = CreateTempFile(path, "0.log", false)
	allPaths = append(allPaths, path)
	leftPaths = append(leftPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 24*31*time.Hour)
	changePathTime(allPaths[len(allPaths)-2], 24*31*time.Hour)
	changePathTime(allPaths[len(allPaths)-3], 24*31*time.Hour)

	// removed after gc
	path, err = CreateTempFile(dir, "alice_sf-job-01_12345678-1234-1234-1234-12345678abcd", true)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)

	path, err = CreateTempFile(path, "secretflow", true)
	allPaths = append(allPaths, path)

	path, err = CreateTempFile(path, "0.log", false)
	allPaths = append(allPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 24*31*time.Hour)
	changePathTime(allPaths[len(allPaths)-2], 24*31*time.Hour)
	changePathTime(allPaths[len(allPaths)-3], 24*31*time.Hour)

	// left after gc
	path, err = CreateTempFile(dir, "alice_sf-job-02_12345678-1234-1234-1234-12345678abcd", true)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)
	leftPaths = append(leftPaths, path)

	path, err = CreateTempFile(path, "secretflow", true)
	allPaths = append(allPaths, path)
	leftPaths = append(leftPaths, path)

	path, err = CreateTempFile(path, "0.log", false)
	allPaths = append(allPaths, path)
	leftPaths = append(leftPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 31*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-2], 31*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-3], 1*24*time.Hour)

	// left after gc
	path, err = CreateTempFile(dir, "alice_sf-job-03_12345678-1234-1234-1234-12345678abcd", true)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)
	leftPaths = append(leftPaths, path)

	path, err = CreateTempFile(path, "secretflow", true)
	allPaths = append(allPaths, path)
	leftPaths = append(leftPaths, path)

	path, err = CreateTempFile(path, "0.log", false)
	allPaths = append(allPaths, path)
	leftPaths = append(leftPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 1*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-2], 31*24*time.Hour)
	changePathTime(allPaths[len(allPaths)-3], 31*24*time.Hour)

	// invalid kuscia pod path, should be ignored, left after gc
	path, err = CreateTempFile(dir, "alice_12345678-1234-1234-1234-12345678abcd", true)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)
	leftPaths = append(leftPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 31*24*time.Hour)

	// left after gc
	path, err = CreateTempFile(dir, "temp.log", false)
	allPaths = append(allPaths, path)
	childPaths = append(childPaths, path)
	leftPaths = append(leftPaths, path)
	changePathTime(allPaths[len(allPaths)-1], 31*24*time.Hour)

	sort.StringSlice(allPaths).Sort()
	sort.StringSlice(childPaths).Sort()
	sort.StringSlice(leftPaths).Sort()
	return
}

func TestStartAndStop(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "test")
	assert.NoError(t, err)
	cfg := DefaultLogFileGCConfig()
	cfg.LogFilePath = tmpDir
	cfg.KubeClient = kubefake.NewSimpleClientset()
	cfg.Namespace = "test-ns"
	cfg.GCInterval = 5 * time.Second
	service := NewLogFileGCService(context.Background(), cfg)
	assert.NotNil(t, service)
	service.Start()
	time.Sleep(cfg.GCInterval * 2)
	service.Stop()
}

func TestStartAndStop_Run_GCWithErr(t *testing.T) {
	t.Parallel()
	rootDir, _, _, _, _, err := CreateGCTestData()
	assert.NoError(t, err)
	// remove dir to make error
	os.RemoveAll(rootDir)
	cfg := DefaultLogFileGCConfig()
	cfg.LogFilePath = rootDir
	cfg.GCInterval = 5 * time.Second
	cfg.KubeClient = kubefake.NewSimpleClientset()
	cfg.Namespace = "test-ns"
	service := NewLogFileGCService(context.Background(), cfg)
	assert.NotNil(t, service)

	service.Start()
	time.Sleep(cfg.GCInterval * 2)
	service.Stop()
}

func TestOnce(t *testing.T) {
	t.Parallel()
	rootDir, _, _, leftPaths, pods, err := CreateGCTestData()
	defer os.RemoveAll(rootDir)
	assert.NoError(t, err)
	cfg := DefaultLogFileGCConfig()
	cfg.LogFilePath = rootDir
	cfg.GCInterval = 5 * time.Second
	// mock kuscia
	cfg.KubeClient = kubefake.NewSimpleClientset()
	cfg.Namespace = "test-ns"
	for _, pod := range pods {
		cfg.KubeClient.CoreV1().Pods(cfg.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	}
	service := NewLogFileGCService(context.Background(), cfg)
	assert.NotNil(t, service)

	err = service.Once()
	assert.NoError(t, err)

	// check gc result
	gcResult, err := fileutils.ListTree(rootDir)
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(gcResult, leftPaths))
}
