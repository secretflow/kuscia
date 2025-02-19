// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type QueryHandler struct {
	logService service.ILogService
}

func NewQueryHandler(logService service.ILogService) *QueryHandler {
	return &QueryHandler{
		logService: logService,
	}
}

func (h QueryHandler) Handle(ginCtx *gin.Context) {
	req := &kusciaapi.QueryLogRequest{}
	if err := ginCtx.ShouldBind(req); err != nil {
		nlog.Errorf("Query log handler parse request failed, error: %s", err.Error())
		_ = ginCtx.AbortWithError(http.StatusBadRequest, err)
		return
	}
	nlog.Infof("Query log request: %+v", req)

	eventCh := make(chan *kusciaapi.QueryLogResponse, 10)
	go func() {
		defer close(eventCh)
		h.logService.QueryTaskLog(ginCtx, req, eventCh)
	}()
	ginCtx.Header("Content-Type", "application/json; charset=utf-8")
	ginCtx.Header("Transfer-Encoding", "chunked")
	ginCtx.Header("Cache-Control", "no-cache")

	ginCtx.Stream(func(w io.Writer) bool {
		resp, ok := <-eventCh
		if !ok {
			nlog.Infof("QueryLog stream reach end, close connection")
			return false
		}
		respBody, err := json.Marshal(resp)
		if err != nil {
			nlog.Errorf("Marshal response body failed, error: %s.", err.Error())
			return false
		}
		nlog.Debugf("Query log response body: %s.", respBody)
		// write body
		if _, err := w.Write(respBody); err != nil {
			nlog.Errorf("Write response body failed, error:%s.", err.Error())
			return false
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return true
	})

}
