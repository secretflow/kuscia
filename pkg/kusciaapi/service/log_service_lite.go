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

package service

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type logServiceLite struct {
	kusciaAPIClient proxy.KusciaAPIClient
	conf            *config.KusciaAPIConfig
}

func (s logServiceLite) QueryPodNode(ctx context.Context, request *kusciaapi.QueryPodNodeRequest) *kusciaapi.QueryPodNodeResponse {
	// request the master api
	resp, err := s.kusciaAPIClient.QueryPodNode(ctx, request)
	if err != nil {
		return &kusciaapi.QueryPodNodeResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestMasterFailed, err.Error()),
		}
	}
	return resp
}

func (s logServiceLite) QueryTaskLog(ctx context.Context, request *kusciaapi.QueryLogRequest, eventCh chan<- *kusciaapi.QueryLogResponse) {
	// overrideRequestDomain(s.conf.RunMode, s.conf.DomainID, request)
	domain := s.conf.DomainID
	podName := fmt.Sprintf("%s/%s-%d", domain, request.TaskId, request.ReplicaIdx)
	if request.Local {
		// local query
		nlog.Infof("Perfrom local log query for pod %s", podName)
		if err := localQueryLog(request, domain, s.conf.StdoutPath, eventCh); err != nil {
			eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to local query, err: %v", err))}
		}
		return
	}

	// get pod node from master
	nodeResp := s.QueryPodNode(ctx, &kusciaapi.QueryPodNodeRequest{TaskId: request.TaskId, Domain: s.conf.DomainID, ReplicaIdx: request.ReplicaIdx})
	if !utils.IsSuccessCode(nodeResp.Status.Code) {
		eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to get pod node from master, err: %v", nodeResp.Status.Message))}
		return
	}

	nodeName := nodeResp.NodeName
	nlog.Infof("Query log get pod node %s, self node name: %s", nodeName, s.conf.NodeName)
	if nodeName == s.conf.NodeName {
		// local query
		nlog.Infof("Perfrom local log query for pod %s", podName)
		if err := localQueryLog(request, domain, s.conf.StdoutPath, eventCh); err != nil {
			eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to local query, err: %v", err))}
		}
	} else {
		// proxy to another node which owns the pod
		nlog.Infof("Perfrom proxy log query for pod %s, ip %s", podName, nodeResp.NodeIp)
		if err := proxyQueryLog(ctx, nodeResp.NodeIp, s.conf, request, eventCh); err != nil {
			eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to proxy query, err: %v", err))}
		}
	}
}
