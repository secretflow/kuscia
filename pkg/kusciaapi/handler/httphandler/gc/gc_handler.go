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

package gc

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
)

type gcHandler struct {
	gcService service.IGCService
}

// NewGCHandler 创建 GC Handler
func NewGCHandler(gcService service.IGCService) *gcHandler {
	return &gcHandler{
		gcService: gcService,
	}
}

// TriggerGC 触发 GC
func (h *gcHandler) TriggerGC(c *gin.Context) {
	var request service.TriggerGCRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	response, err := h.gcService.TriggerGC(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("internal error: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateGCConfig 更新 GC 配置
func (h *gcHandler) UpdateGCConfig(c *gin.Context) {
	var request service.UpdateGCConfigRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	response, err := h.gcService.UpdateGCConfig(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("internal error: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// QueryGCConfig 查询 GC 配置
func (h *gcHandler) QueryGCConfig(c *gin.Context) {
	var request service.QueryGCConfigRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	response, err := h.gcService.QueryGCConfig(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("internal error: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// QueryGCStatus 查询 GC 状态
func (h *gcHandler) QueryGCStatus(c *gin.Context) {
	var request service.QueryGCStatusRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	response, err := h.gcService.QueryGCStatus(c.Request.Context(), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, service.Status{
			Code:    -1,
			Message: fmt.Sprintf("internal error: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
