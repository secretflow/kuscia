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

	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

const (
	ErrRequestValidate     = 11100
	ErrForUnexpected       = 11101
	ErrAuthFailed          = 11102
	ErrRequestMasterFailed = 11103
	ErrLiteAPINotSupport   = 11104

	ErrCreateJob      = 11201
	ErrQueryJob       = 11202
	ErrQueryJobStatus = 11203
	ErrDeleteJob      = 11204
	ErrStopJob        = 11205
	ErrApproveJob     = 11206
	ErrSuspendJob     = 11207
	ErrRestartJob     = 11208
	ErrCancelJob      = 11209

	ErrCreateDomain      = 11300
	ErrQueryDomain       = 11301
	ErrQueryDomainStatus = 11302
	ErrUpdateDomain      = 11303
	ErrDeleteDomain      = 11304
	ErrDomainNotExists   = 11305
	ErrDomainExists      = 11306

	ErrCreateDomainRoute      = 11400
	ErrQueryDomainRoute       = 11401
	ErrQueryDomainRouteStatus = 11402
	ErrDeleteDomainRoute      = 11403
	ErrDomainRouteNotExists   = 11404
	ErrDomainRouteExists      = 11405

	ErrCreateDomainDataFailed = 11500
	ErrDeleteDomainDataFailed = 11501
	ErrGetDomainDataFailed    = 11502
	ErrListDomainDataFailed   = 11503
	ErrMergeDomainDataFailed  = 11504
	ErrPatchDomainDataFailed  = 11505
	ErrDomainDataNotExists    = 11506
	ErrDomainDataExists       = 11507

	ErrCreateServing      = 11600
	ErrQueryServing       = 11601
	ErrQueryServingStatus = 11602
	ErrUpdateServing      = 11603
	ErrDeleteServing      = 11604

	ErrCreateDomainDataGrant    = 11700
	ErrUpdateDomainDataGrant    = 11701
	ErrQueryDomainDataGrant     = 11702
	ErrDeleteDomainDataGrant    = 11703
	ErrDomainDataGrantExists    = 11704
	ErrDomainDataGrantNotExists = 11705

	ErrCreateDomainDataSource           = 11800
	ErrUpdateDomainDataSource           = 11801
	ErrQueryDomainDataSource            = 11802
	ErrBatchQueryDomainDataSource       = 11803
	ErrDeleteDomainDataSource           = 11804
	ErrDomainDataSourceExists           = 11805
	ErrDomainDataSourceNotExists        = 11806
	ErrDomainDataSourceInfoEncodeFailed = 11907
)

func GetDomainErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainExists
	}
	return defaultErrorCode
}

func GetDomainRouteErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainRouteNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainRouteExists
	}
	return defaultErrorCode
}

func CreateDomainDataErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainDataExists
	}
	return defaultErrorCode
}

func GetDomainDataErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainDataNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainDataExists
	}
	return defaultErrorCode
}

func CreateDomainDataGrantErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainDataGrantExists
	}
	return defaultErrorCode
}

func GetDomainDataGrantErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainDataGrantNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainDataGrantExists
	}
	return defaultErrorCode
}

func CreateDomainDataSourceErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainDataSourceExists
	}
	return defaultErrorCode
}

func GetDomainDataSourceErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainDataSourceNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainDataSourceExists
	}
	return defaultErrorCode
}
