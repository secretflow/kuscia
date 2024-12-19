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

//nolint:dupl

package grpchandler

import (
	"context"

	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type logHandler struct {
	logService service.ILogService
	kusciaapi.UnimplementedLogServiceServer
}

func NewLogHandler(logService service.ILogService) kusciaapi.LogServiceServer {
	return &logHandler{
		logService: logService,
	}
}

func (h logHandler) QueryLog(request *kusciaapi.QueryLogRequest, srv kusciaapi.LogService_QueryLogServer) error {
	eventCh := make(chan *kusciaapi.QueryLogResponse, 10)
	go func() {
		defer close(eventCh)
		h.logService.QueryTaskLog(srv.Context(), request, eventCh)
	}()

	for e := range eventCh {
		if err := srv.Send(e); err != nil {
			nlog.Errorf("Send log response error: %v", err)
		}
	}
	return nil
}

func (h logHandler) QueryPodNode(ctx context.Context, request *kusciaapi.QueryPodNodeRequest) (*kusciaapi.QueryPodNodeResponse, error) {
	res := h.logService.QueryPodNode(ctx, request)
	return res, nil
}
