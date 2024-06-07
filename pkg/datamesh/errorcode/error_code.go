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

package errorcode

import (
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

func GetDomainDataSourceErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_DataMeshErrDomainDataSourceNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_DataMeshErrDomainDataSourceExists
	}
	return defaultErrorCode
}

func GetDomainDataGrantErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_DataMeshErrDomainDataGrantNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_DataMeshErrDomainDataGrantExists
	}
	return defaultErrorCode
}
