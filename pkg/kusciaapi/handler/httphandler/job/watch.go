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

package job

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type WatchJobHandler struct {
	jobService service.IJobService
}

func NewWatchJobHandler(jobService service.IJobService) *WatchJobHandler {
	return &WatchJobHandler{
		jobService: jobService,
	}
}

const (
	heartBeatDuration  = time.Second * 10
	eventChannelLength = 10
)

func (h WatchJobHandler) Handle(ginCtx *gin.Context) {
	// get request from gin context
	req := &kusciaapi.WatchJobRequest{}
	if err := ginCtx.ShouldBind(req); err != nil {
		nlog.Errorf("Watch job handler parse request failed, error: %s", err.Error())
		ginCtx.AbortWithError(http.StatusBadRequest, err)
		return
	}
	// call watch job function of job service
	eventCh := make(chan *kusciaapi.WatchJobEventResponse, eventChannelLength)
	closeCh := make(chan error)
	go func() {
		err := h.jobService.WatchJob(ginCtx, req, eventCh)
		closeCh <- err
		if err != nil {
			nlog.Errorf("Call watchJob function failed, error: %s", err.Error())
			return
		}
	}()
	// set header of the response   TODO: support multiple data formats, such as protobuf, json, yaml
	ginCtx.Header("Content-Type", "application/json; charset=utf-8")
	ginCtx.Header("Transfer-Encoding", "chunked")
	// set a heartbeat ticker
	ticker := time.NewTicker(heartBeatDuration)
	defer ticker.Stop()
	// write the watch events to the response stream
	clientGone := ginCtx.Stream(func(w io.Writer) bool {
		resp := &kusciaapi.WatchJobEventResponse{
			Type:   0,
			Object: nil,
		}
		select {
		case resp = <-eventCh:
			ticker.Reset(heartBeatDuration)
		case <-ticker.C:
			// heartbeat
			resp.Type = kusciaapi.EventType_HEARTBEAT
		case err := <-closeCh:
			if err != nil {
				nlog.Errorf("Watch job stats failed, error: %s.", err.Error())
			}
			return false
		}
		respBody, err := json.Marshal(resp)
		if err != nil {
			nlog.Errorf("Marshal response body failed, error: %s.", err.Error())
			return false
		}
		nlog.Debugf("Watch response body: %s.", respBody)
		// write body
		if _, err := w.Write(respBody); err != nil {
			nlog.Errorf("Write response body failed, error:%s.", err.Error())
			return false
		}
		return true
	})
	if clientGone {
		nlog.Infof("Client finished watch!")
		return
	}
	nlog.Infof("Server finished watch!")
}
