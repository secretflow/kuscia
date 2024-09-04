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
	"encoding/base64"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"os"
)

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
	filePath := common.DefaultModelLocalFSPath + "/" + request.GetFilename()

	bytes, errDecode := base64.StdEncoding.DecodeString(request.GetContent())
	if errDecode != nil {
		nlog.Errorf("UploadModel file base64 decode failed, error: %s", errDecode.Error())
		return &kusciaapi.UploadModelResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrUploadModelFailed, errDecode.Error()),
		}
	}
	err := os.WriteFile(filePath, bytes, 0755)
	if err != nil {
		nlog.Errorf("UploadModel write file failed, error: %s", err.Error())
		return &kusciaapi.UploadModelResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrUploadModelFailed, err.Error()),
		}
	}
	return &kusciaapi.UploadModelResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}
