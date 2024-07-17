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

package common

import (
	"testing"

	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

func TestBuildGrpcErrorf(t *testing.T) {
	err := BuildGrpcErrorf(nil, codes.Aborted, "xxxx")
	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = Aborted desc = xxxx", err.Error())

	err = BuildGrpcErrorf(&v1alpha1.Status{}, codes.Aborted, "xxxx")

	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = Aborted desc = xxxx", err.Error())

	err = BuildGrpcErrorf(&v1alpha1.Status{
		Message: "tttt",
	}, codes.Aborted, "xxxx")

	assert.Error(t, err)
	assert.Equal(t, "rpc error: code = Aborted desc = tttt", err.Error())
}
