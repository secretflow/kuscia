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

package builtin

import (
	"context"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
)

type builtinDirectIO struct {
	config *config.DataProxyConfig
}

func NewBuiltinDirectIO(config *config.DataProxyConfig) DataMeshDataIOInterface {
	return &builtinDirectIO{
		config: config,
	}
}

func (dio *builtinDirectIO) Read(ctx context.Context, rc *utils.DataMeshRequestContext, w utils.RecordWriter) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (dio *builtinDirectIO) Write(ctx context.Context, rc *utils.DataMeshRequestContext, stream *flight.Reader) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

func (dio *builtinDirectIO) GetEndpointURI() string {
	return dio.config.Endpoint
}
