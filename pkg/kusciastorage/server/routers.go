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

package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/kusciastorage/common"
	"github.com/secretflow/kuscia/pkg/kusciastorage/controller"
	"github.com/secretflow/kuscia/pkg/web/api"
	"github.com/secretflow/kuscia/pkg/web/decorator"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
	"github.com/secretflow/kuscia/pkg/web/framework/router"
)

const (
	domainResourceAPIPath     = "/namespaces/:domain/:kind/:kind_instance_name/:resource_name"
	clusterResourceAPIPath    = "/:kind/:kind_instance_name/:resource_name"
	domainResourceRefAPIPath  = "/namespaces/:domain/:kind/:kind_instance_name/:resource_name/reference"
	clusterResourceRefAPIPath = "/:kind/:kind_instance_name/:resource_name/reference"
)

// GetPublicRouterGroups returns public router groups.
func GetPublicRouterGroups(s *Server, engine *engine.Engine) router.GroupsRouters {
	return []*router.GroupRouters{
		{
			Group:           "/api/v1",
			GroupMiddleware: nil,
			Routes: []*router.Router{
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: domainResourceAPIPath,
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceCreateHandler(s.ReqBodyMaxSize))},
				},
				{
					HTTPMethod:   http.MethodGet,
					RelativePath: domainResourceAPIPath,
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceQueryHandler())},
				},
				{
					HTTPMethod:   http.MethodDelete,
					RelativePath: domainResourceAPIPath,
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceDeleteHandler())},
				},
				{
					HTTPMethod:   http.MethodPost,
					RelativePath: clusterResourceAPIPath,
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceCreateHandler(s.ReqBodyMaxSize))},
				},
				{
					HTTPMethod:   http.MethodGet,
					RelativePath: clusterResourceAPIPath,
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceQueryHandler())},
				},
				{
					HTTPMethod:   http.MethodDelete,
					RelativePath: clusterResourceAPIPath,
					Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceDeleteHandler())},
				},
			},
		},
	}
}

// GetInternalRouterGroups returns internal router groups.
func GetInternalRouterGroups(s *Server, engine *engine.Engine) router.GroupsRouters {
	routerGroups := []*router.GroupRouters{{
		Group:           "/api/v1",
		GroupMiddleware: nil,
		Routes: []*router.Router{
			{
				HTTPMethod:   http.MethodPost,
				RelativePath: domainResourceRefAPIPath,
				Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceCreateHandler(s.ReqBodyMaxSize))},
			},
			{
				HTTPMethod:   http.MethodPost,
				RelativePath: clusterResourceRefAPIPath,
				Handlers:     []gin.HandlerFunc{protoDecorator(engine, controller.NewResourceCreateHandler(s.ReqBodyMaxSize))},
			},
		}}}

	return append(routerGroups, GetPublicRouterGroups(s, engine)...)
}

// protoDecorator is used to wrap handler.
func protoDecorator(engine *engine.Engine, handler api.ProtoHandler) gin.HandlerFunc {
	return decorator.DefaultProtoDecoratorMaker(common.ErrorCodeForRequestIsInvalid, common.ErrorCodeForServerInternalErr)(engine, handler)
}
