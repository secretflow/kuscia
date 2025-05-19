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
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/reporter/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pbv1alpha "github.com/secretflow/kuscia/proto/api/v1alpha1"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/reporter"
)

type IReportService interface {
	ReportProgress(ctx context.Context, request *reporter.ReportProgressRequest, taskID string) *reporter.CommonReportResponse
}

type reportService struct {
	kusciaClient     kusciaclientset.Interface
	kusciaTaskLister kuscialistersv1alpha1.KusciaTaskLister
}

func NewReportService(config *config.ReporterConfig) IReportService {
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(config.KusciaClient, 5*time.Minute)
	kusciaTaskInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks()

	return &reportService{
		kusciaClient:     config.KusciaClient,
		kusciaTaskLister: kusciaTaskInformer.Lister(),
	}
}
func (r *reportService) ReportProgress(ctx context.Context, request *reporter.ReportProgressRequest, taskID string) *reporter.CommonReportResponse {
	// TODO: save report to kuscia status crd
	if taskID == "" {
		return &reporter.CommonReportResponse{
			Status: &pbv1alpha.Status{
				Code:    int32(pberrorcode.ErrorCode_ReporterErrRequestInvalidate),
				Message: "TaskID should not be empty"},
		}
	}
	// use request.Progress to update process field in kuscia task crd
	kt, err := r.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(ctx, taskID, metav1.GetOptions{})
	if err != nil {
		nlog.Infof("Failed to get Kuscia Task %v", err)
		return &reporter.CommonReportResponse{
			Status: &pbv1alpha.Status{
				Code:    int32(pberrorcode.ErrorCode_ReporterErrForUnexptected),
				Message: "Failed to get Kuscia Task"},
		}
	}
	// check request.Progress is valid percentage number
	if request.Progress < 0 || request.Progress > 1 {
		nlog.Errorf("Invalid progress report, taskID: %s, %v", taskID, request.Progress)
		return &reporter.CommonReportResponse{
			Status: &pbv1alpha.Status{
				Code:    int32(pberrorcode.ErrorCode_ReporterErrRequestInvalidate),
				Message: "Task progress should be a valid percentage number"},
		}
	}
	if request.Progress <= kt.Status.Progress {
		return &reporter.CommonReportResponse{
			Status: &pbv1alpha.Status{
				Code:    int32(pberrorcode.ErrorCode_SUCCESS),
				Message: "ignored"},
		}
	}
	// update: cacheProgress is invalid or reqProgress > cacheProgress
	newKT := kt.DeepCopy()
	newKT.Status.Progress = request.Progress

	if kt, err := r.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).UpdateStatus(ctx, newKT, metav1.UpdateOptions{}); err != nil {
		nlog.Infof("Failed to update Kuscia Task Status Progress %v %e", kt, err)
		return &reporter.CommonReportResponse{
			Status: &pbv1alpha.Status{
				Code:    int32(pberrorcode.ErrorCode_ReporterErrForUnexptected),
				Message: "Failed to update Kuscia Task Status Progress"},
		}
	}
	// return success
	return &reporter.CommonReportResponse{
		Status: &pbv1alpha.Status{
			Code:    int32(pberrorcode.ErrorCode_SUCCESS),
			Message: "success"},
	}
}

type IHealthService interface {
	HealthCheck(ctx context.Context, request *reporter.HealthRequest) *reporter.CommonReportResponse
}

type healthService struct {
}

func NewHealthService(config *config.ReporterConfig) IHealthService {
	return &healthService{}
}

func (h *healthService) HealthCheck(ctx context.Context, request *reporter.HealthRequest) *reporter.CommonReportResponse {
	return &reporter.CommonReportResponse{
		Status: &pbv1alpha.Status{
			Code:    int32(pberrorcode.ErrorCode_SUCCESS),
			Message: "success"},
	}
}
