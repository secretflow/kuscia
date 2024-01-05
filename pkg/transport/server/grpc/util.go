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

package grpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	kHttp "github.com/secretflow/kuscia/pkg/transport/server/http"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)


type InboundParams struct {
	sid   string
	topic string
}

type Method string

func getInboundParams(ctx context.Context, isPush bool) (*InboundParams, *transerr.TransError) {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok{
		return &InboundParams{},transerr.NewTransError(transerr.InvalidRequest)
	}
	sidList := md.Get(codec.PtpSessionID)
	topicList := md.Get(codec.PtpTopicID)

	var nodeID string
	if isPush {
		nodeID = codec.PtpSourceNodeID
	} else {
		nodeID = codec.PtpTargetNodeID
	}
	topicPrefixList := md.Get(nodeID)

	if len(sidList) == 0 || len(topicList) == 0 || len(topicPrefixList) == 0 {
		nlog.Warnf("Empty session-id or topic or %s", nodeID)
		return nil, transerr.NewTransError(transerr.InvalidRequest)
	}

	sid := sidList[0]
	topic := topicList[0]
	topicPrefix := topicPrefixList[0]

	return &InboundParams{
		sid:   sid,
		topic: fmt.Sprintf("%s-%s", topicPrefix, topic),
	}, nil
}

func getInboundTimeout(ctx context.Context) time.Duration {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return kHttp.DefaultTimeout
	}
	timeoutList := md.Get(kHttp.ParamTimeout)
	if len(timeoutList) == 0 {
		return kHttp.DefaultTimeout
	}

	num, err := strconv.Atoi(timeoutList[0])
	if err != nil {
		return kHttp.DefaultTimeout
	}
	timeout := time.Duration(num) * time.Second
	if timeout < kHttp.MinTimeout {
		return kHttp.MinTimeout
	}

	if timeout > kHttp.MaxTimeout {
		return kHttp.MaxTimeout
	}

	return timeout
}

func getMetaDataList(ctx context.Context, element string) []string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok{
		return make([]string,0)
	}
	return md.Get(element)
}
