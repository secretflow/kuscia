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

//nolint:dupl
package errorcode

import (
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
)

func GetDomainErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainExists
	}
	return defaultErrorCode
}

func GetDomainRouteErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainRouteNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainRouteExists
	}
	return defaultErrorCode
}

func CreateDomainDataErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataExists
	}
	return defaultErrorCode
}

func GetDomainDataErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataExists
	}
	return defaultErrorCode
}

func CreateDomainDataGrantErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataGrantExists
	}
	return defaultErrorCode
}

func GetDomainDataGrantErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataGrantNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataGrantExists
	}
	return defaultErrorCode
}

func CreateDomainDataSourceErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataSourceExists
	}
	return defaultErrorCode
}

func GetDomainDataSourceErrorCode(err error, defaultErrorCode errorcode.ErrorCode) errorcode.ErrorCode {
	if errors.IsNotFound(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataSourceNotExists
	}
	if errors.IsAlreadyExists(err) {
		return errorcode.ErrorCode_KusciaAPIErrDomainDataSourceExists
	}
	return defaultErrorCode
}
