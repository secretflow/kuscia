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

// test file for service.go
package service

import (
	"context"
	"testing"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/reporter/config"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/reporter"
	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestReportService_Success(t *testing.T) {

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(kusciaClient, 0)
	kusciaInformerFactory.Start(context.Background().Done())

	taskID := "mockTask1"
	task := &v1alpha1.KusciaTask{
		ObjectMeta: v1.ObjectMeta{
			Name:      taskID,
			Namespace: common.KusciaCrossDomain,
		},
	}
	kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Create(context.Background(), task, v1.CreateOptions{})

	// prepare reporter service
	reporterConfig := config.NewDefaultReporterConfig("")
	reporterConfig.KubeClient = kubeClient
	reporterConfig.KusciaClient = kusciaClient
	rs := NewReportService(reporterConfig)
	resp := rs.ReportProgress(context.Background(),
		&reporter.ReportProgressRequest{Progress: 1},
		taskID,
	)
	assert.Equal(t, resp.Status.Code, int32(pberrorcode.ErrorCode_SUCCESS))
}

func TestReportService_TaskNotFound(t *testing.T) {

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(kusciaClient, 0)
	kusciaInformerFactory.Start(context.Background().Done())

	taskID := "mockTask1"
	task := &v1alpha1.KusciaTask{
		ObjectMeta: v1.ObjectMeta{
			Name:      taskID,
			Namespace: common.KusciaCrossDomain,
		},
	}
	kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Create(context.Background(), task, v1.CreateOptions{})

	// prepare reporter service
	reporterConfig := config.NewDefaultReporterConfig("")
	reporterConfig.KubeClient = kubeClient
	reporterConfig.KusciaClient = kusciaClient
	rs := NewReportService(reporterConfig)
	resp := rs.ReportProgress(context.Background(),
		&reporter.ReportProgressRequest{Progress: 1},
		"wrongID",
	)
	assert.Equal(t, resp.Status.Code, int32(pberrorcode.ErrorCode_ReporterErrForUnexptected))
	assert.Equal(t, resp.Status.Message, "Failed to get Kuscia Task")
}

func TestReportService_TaskEmpty(t *testing.T) {

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(kusciaClient, 0)
	kusciaInformerFactory.Start(context.Background().Done())

	taskID := "mockTask1"
	task := &v1alpha1.KusciaTask{
		ObjectMeta: v1.ObjectMeta{
			Name:      taskID,
			Namespace: common.KusciaCrossDomain,
		},
	}
	kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Create(context.Background(), task, v1.CreateOptions{})

	// prepare reporter service
	reporterConfig := config.NewDefaultReporterConfig("")
	reporterConfig.KubeClient = kubeClient
	reporterConfig.KusciaClient = kusciaClient
	rs := NewReportService(reporterConfig)
	resp := rs.ReportProgress(context.Background(),
		&reporter.ReportProgressRequest{Progress: 1},
		"",
	)
	assert.Equal(t, resp.Status.Code, int32(pberrorcode.ErrorCode_ReporterErrRequestInvalidate))
	assert.Equal(t, resp.Status.Message, "TaskID should not be empty")
}

func TestReportService_TaskBadProgress(t *testing.T) {

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(kusciaClient, 0)
	kusciaInformerFactory.Start(context.Background().Done())

	taskID := "mockTask1"
	task := &v1alpha1.KusciaTask{
		ObjectMeta: v1.ObjectMeta{
			Name:      taskID,
			Namespace: common.KusciaCrossDomain,
		},
	}
	kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Create(context.Background(), task, v1.CreateOptions{})

	// prepare reporter service
	reporterConfig := config.NewDefaultReporterConfig("")
	reporterConfig.KubeClient = kubeClient
	reporterConfig.KusciaClient = kusciaClient
	rs := NewReportService(reporterConfig)
	resp := rs.ReportProgress(context.Background(),
		&reporter.ReportProgressRequest{Progress: 1.1},
		taskID,
	)
	assert.Equal(t, resp.Status.Code, int32(pberrorcode.ErrorCode_ReporterErrRequestInvalidate))
	assert.Equal(t, resp.Status.Message, "Task progress should be a valid percentage number")

	resp = rs.ReportProgress(context.Background(),
		&reporter.ReportProgressRequest{Progress: 0.5},
		taskID,
	)
	assert.Equal(t, resp.Status.Code, int32(pberrorcode.ErrorCode_SUCCESS))

	resp = rs.ReportProgress(context.Background(),
		&reporter.ReportProgressRequest{Progress: 0.4},
		taskID,
	)
	assert.Equal(t, resp.Status.Code, int32(pberrorcode.ErrorCode_SUCCESS))
	assert.Equal(t, resp.Status.Message, "ignored")
}

func TestReportService_Health(t *testing.T) {

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(kusciaClient, 0)
	kusciaInformerFactory.Start(context.Background().Done())

	taskID := "mockTask1"
	task := &v1alpha1.KusciaTask{
		ObjectMeta: v1.ObjectMeta{
			Name:      taskID,
			Namespace: common.KusciaCrossDomain,
		},
	}
	kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Create(context.Background(), task, v1.CreateOptions{})

	// prepare reporter service
	reporterConfig := config.NewDefaultReporterConfig("")
	reporterConfig.KubeClient = kubeClient
	reporterConfig.KusciaClient = kusciaClient
	rs := NewHealthService(reporterConfig)
	resp := rs.HealthCheck(context.Background(),
		&reporter.HealthRequest{},
	)
	assert.Equal(t, resp.Status.Code, int32(pberrorcode.ErrorCode_SUCCESS))
}
