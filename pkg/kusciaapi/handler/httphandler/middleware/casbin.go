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

//nolint:golint
package middleware

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/web/constants"
)

//go:embed rbac/casbin_model.conf
var casbinModel []byte

//go:embed rbac/casbin_policy.csv
var casbinPolicy []byte

const (
	modelFile  = "casbin_model.conf"
	policyFile = "casbin_policy.csv"
)

var casbinEnforcer *casbin.CachedEnforcer

func InitConfig(confPath string) {
	modelPath := path.Join(confPath, modelFile)
	policyPath := path.Join(confPath, policyFile)
	_ = os.WriteFile(modelPath, casbinModel, 0644)
	_ = os.WriteFile(policyPath, casbinPolicy, 0644)
	casbinEnforcer, _ = casbin.NewCachedEnforcer(modelPath, policyPath)
}

func PermissionMiddleWare(ctx *gin.Context) {
	role, _ := ctx.Get(constants.AuthRole)
	isPass, err := casbinEnforcer.Enforce(role, ctx.Request.URL.Path, ctx.Request.Method)
	if err != nil {
		ctx.AbortWithError(http.StatusUnauthorized, err)
		return
	}
	if !isPass {
		err = fmt.Errorf("check role failed,role :%s,path:%s", role, ctx.Request.URL.Path)
		ctx.AbortWithError(http.StatusUnauthorized, err)
		return
	}
	ctx.Next()
}
