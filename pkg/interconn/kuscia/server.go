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

package kuscia

import (
	"context"

	"github.com/secretflow/kuscia/pkg/interconn/common"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

// Server implements the inter connection with bfia protocol.
type Server struct {
	NewController common.NewControllerFunc
	CheckCRD      common.CheckCRDExistsFunc
}

// NewServer returns a server instance.
func NewServer(clients *kubeconfig.KubeClients) *Server {
	s := &Server{
		NewController: NewController,
		CheckCRD:      CheckCRDExists,
	}
	return s
}

// Run runs the bfia server.
func (s *Server) Run(ctx context.Context) error {
	return nil
}
