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

package bfia

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/interconn/bfia/bean"
	bfiacommon "github.com/secretflow/kuscia/pkg/interconn/bfia/common"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/controller"
	"github.com/secretflow/kuscia/pkg/interconn/bfia/handler"
	"github.com/secretflow/kuscia/pkg/interconn/common"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
)

// Server implements the inter connection with bfia protocol.
type Server struct {
	NewController   common.NewControllerFunc
	ResourceManager *handler.ResourcesManager
	APPEngine       *engine.Engine
}

// NewServer returns a server instance.
func NewServer(ctx context.Context, clients *kubeconfig.KubeClients) (*Server, error) {
	rm, err := handler.NewResourcesManager(ctx, clients.KusciaClient)
	if err != nil {
		return nil, err
	}

	appEngine := engine.New(&framework.AppConfig{
		Name:  bfiacommon.ServerName,
		Usage: "bfia service provides inter connection with bfia protocol",
	})

	if err = appEngine.UseBeanWithConfig(bfiacommon.ServerName, bean.NewHTTPServerBean(rm)); err != nil {
		return nil, fmt.Errorf("initialize bfia server failed, %v", err.Error())
	}

	s := &Server{
		NewController:   controller.NewController,
		ResourceManager: rm,
		APPEngine:       appEngine,
	}
	return s, nil
}

// Run runs the bfia server.
func (s *Server) Run(ctx context.Context) error {
	go s.ResourceManager.Run(ctx)
	return s.APPEngine.Run(ctx)
}
