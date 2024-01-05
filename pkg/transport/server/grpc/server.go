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
	"fmt"
	"net"

	"golang.org/x/net/context"
	"golang.org/x/net/netutil"
	"google.golang.org/grpc"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/transport/msq"
	pb "github.com/secretflow/kuscia/pkg/transport/proto/grpcptp"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type Server struct {
	grpcConfig *config.GRPCConfig
	sm         *msq.SessionManager

	codec codec.Codec

	pb.UnimplementedPrivateTransferTransportServer
	pb.UnimplementedPrivateTransferProtocolServer
}

func NewServer(grpcConfig *config.GRPCConfig, sm *msq.SessionManager) *Server {
	return &Server{
		grpcConfig: grpcConfig,
		sm:         sm,
		codec: codec.NewProtoCodec(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(s.grpcConfig.MaxConcurrentStreams),
		grpc.MaxRecvMsgSize(s.grpcConfig.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.grpcConfig.MaxSendMsgSize),
	}
	gs := grpc.NewServer(opts...)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.grpcConfig.Port))
	if err != nil {
		return err
	}

	limitedListener := netutil.LimitListener(listener, s.grpcConfig.MaxConns)

	pb.RegisterPrivateTransferProtocolServer(gs, s)
	pb.RegisterPrivateTransferTransportServer(gs, s)
	err = gs.Serve(limitedListener)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) Invoke(ctx context.Context, inbound *pb.Inbound) (*pb.Outbound, error) {
	params, err := getInboundParams(ctx, true)
	if err != nil {
		return codec.BuildInvokeOutboundByErr(err), nil
	}
	message, err := s.readMessage(inbound)
	if err != nil {
		return codec.BuildInvokeOutboundByErr(err), nil
	}
	err = s.sm.Push(params.sid, params.topic, message, getInboundTimeout(ctx))
	return codec.BuildInvokeOutboundByErr(err), nil
}

func (s *Server) Pop(ctx context.Context, inbound *pb.PopInbound) (*pb.TransportOutbound, error) {
	params, err := getInboundParams(ctx, false)
	if err != nil {
		return codec.BuildTransportOutboundByErr(err), nil
	}

	msg, err := s.sm.Pop(params.sid, params.topic, getInboundTimeout(ctx))
	if err != nil || msg == nil {
		return codec.BuildTransportOutboundByErr(err), nil
	}

	return codec.BuildTransportOutboundByPayload(msg.Content), nil
}

func (s *Server) Peek(ctx context.Context, inbound *pb.PeekInbound) (*pb.TransportOutbound, error) {
	params, err := getInboundParams(ctx, false)
	if err != nil {
		return codec.BuildTransportOutboundByErr(err), nil
	}

	msg, err := s.sm.Peek(params.sid, params.topic)
	if err != nil || msg == nil {
		return codec.BuildTransportOutboundByErr(err), nil
	}

	return codec.BuildTransportOutboundByPayload(msg.Content), nil
}

func (s *Server) Release(ctx context.Context, inbound *pb.ReleaseInbound) (*pb.TransportOutbound, error) {
	sidList := getMetaDataList(ctx, codec.PtpSessionID)
	if len(sidList) == 0 {
		nlog.Warnf("Empty session-id")
		return codec.BuildTransportOutboundByErr(transerr.NewTransError(transerr.InvalidRequest)), nil
	}
	sid := sidList[0]

	topicIdList := getMetaDataList(ctx, codec.PtpTopicID)
	if len(topicIdList) != 0 {
		topicPrefixList := getMetaDataList(ctx, codec.PtpTargetNodeID)
		var topicPrefix string
		if len(topicPrefixList) == 0 {
			topicPrefix = ""
		} else {
			topicPrefix = topicPrefixList[0]
		}
		fullTopic := fmt.Sprintf("%s-%s", topicPrefix, topicIdList[0])
		s.sm.ReleaseTopic(sid, fullTopic)
		return codec.BuildTransportOutboundByErr(nil), nil
	}
	s.sm.ReleaseSession(sid)
	return codec.BuildTransportOutboundByErr(nil), nil
}

func (s *Server) readMessage(inbound *pb.Inbound) (*msq.Message, *transerr.TransError) {
	if int64(len(inbound.GetPayload())) == 0 {
		nlog.Warnf("Empty request body")
		return nil, transerr.NewTransError(transerr.InvalidRequest)
	}
	return msq.NewMessage(inbound.GetPayload()), nil
}
