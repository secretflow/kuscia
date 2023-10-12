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

	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

const (
	ErrRequestInvalidate = 12100
	ErrForUnexpected     = 12101

	ErrCreateDomainData            = 12200
	ErrQueryDomainData             = 12201
	ErrGetDomainDataFromKubeFailed = 12202
	ErrMergeDomainDataFailed       = 12203
	ErrPatchDomainDataFailed       = 12204
	ErrDeleteDomainDataFailed      = 12205

	ErrCreateDomainDataSource       = 12300
	ErrParseDomainDataSourceFailed  = 12301
	ErrQueryDomainDataSource        = 12302
	ErrDeleteDomainDataSourceFailed = 12303
	ErrDomainDataSourceExists       = 12304
	ErrDomainDataSourceNotExists    = 12305

	ErrCreateDomainDataGrant    = 12400
	ErrUpdateDomainDataGrant    = 12401
	ErrQueryDomainDataGrant     = 12402
	ErrDeleteDomainDataGrant    = 12403
	ErrDomainDataGrantExists    = 12404
	ErrDomainDataGrantNotExists = 12405
)

func GetDomainDataSourceErrorCode(err error, defaultErrorCode errorcode.KusciaErrorCode) errorcode.KusciaErrorCode {
	if errors.IsNotFound(err) {
		return ErrDomainDataSourceNotExists
	}
	if errors.IsAlreadyExists(err) {
		return ErrDomainDataSourceExists
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
