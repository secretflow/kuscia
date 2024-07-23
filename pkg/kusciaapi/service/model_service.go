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

//nolint:dulp
package service

import (
	"context"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"os"
)

// TODO LLY-UMF 待生成
type IModelService interface {
	UploadModel(ctx context.Context, request *kusciaapi.UploadModelRequest) *kusciaapi.UploadModelResponse
}

type modelService struct {
	conf *config.KusciaAPIConfig
}

func NewModelService(config *config.KusciaAPIConfig) IModelService {
	return &modelService{
		conf: config,
	}
}

func (m modelService) UploadModel(ctx context.Context, request *kusciaapi.UploadModelRequest) *kusciaapi.UploadModelResponse {
	// TODO LLY-UMF implement me -- UploadModel UploadModelRequest & UploadModelResponse 是自动生成 -- model.pb.go by model.proto
	// TODO LLY-UMF 接收文件并存放于 /home/kuscia/var/storage/data/
	filePath := common.DefaultModelLocalFSPath + "/" + request.GetFilename()

	err := os.WriteFile(filePath, request.GetContent(), 0755)
	if err != nil {
		nlog.Errorf("UploadModel failed, error: %s", err.Error())
		return &kusciaapi.UploadModelResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrUploadModelFailed, err.Error()),
		}
	}
	return &kusciaapi.UploadModelResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}
