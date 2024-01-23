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
	"github.com/secretflow/kuscia/pkg/transport/server/common"
	"google.golang.org/grpc/metadata"
	"net"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/netutil"
	"google.golang.org/grpc"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/transport/msq"
	pb "github.com/secretflow/kuscia/pkg/transport/proto/mesh"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type InboundParams struct {
	sid   string
	topic string
}


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
		grpc.ConnectionTimeout(time.Duration(s.grpcConfig.ConnectionTimeout)*time.Second),
		grpc.ReadBufferSize(s.grpcConfig.ReadBufferSize),
		grpc.WriteBufferSize(s.grpcConfig.WriteBufferSize),
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
	params, err := getInboundParams(ctx, inbound,true)
	if err != nil {
		return codec.BuildInvokeOutboundByErr(err), nil
	}
	message, err := s.readMessage(inbound)
	if err != nil {
		return codec.BuildInvokeOutboundByErr(err), nil
	}
	err = s.sm.Push(params.sid, params.topic, message, getTimeout(ctx, inbound))
	return codec.BuildInvokeOutboundByErr(err), nil
}

func (s *Server) Pop(ctx context.Context, inbound *pb.PopInbound) (*pb.TransportOutbound, error) {
	params, err := getInboundParams(ctx, inbound,false)
	if err != nil {
		return codec.BuildTransportOutboundByErr(err), nil
	}

	msg, err := s.sm.Pop(params.sid, params.topic, getTimeout(ctx, inbound))
	if err != nil || msg == nil {
		return codec.BuildTransportOutboundByErr(err), nil
	}

	return codec.BuildTransportOutboundByPayload(msg.Content), nil
}

func (s *Server) Peek(ctx context.Context, inbound *pb.PeekInbound) (*pb.TransportOutbound, error) {
	params, err := getInboundParams(ctx, inbound,false)
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
	sid , ok:= getPramFromCtx(ctx, codec.PtpSessionID)
	if !ok || len(sid)==0 {
		nlog.Warnf("Empty session-id")
		return codec.BuildTransportOutboundByErr(transerr.NewTransError(transerr.InvalidRequest)), nil
	}

	topic := getTopic(ctx, inbound)

	if len(topic) != 0 {
		topicPrefix, ok := getPramFromCtx(ctx, codec.PtpTargetNodeID)

		if !ok {
			return codec.BuildTransportOutboundByErr(transerr.NewTransError(transerr.InvalidRequest)), nil
		}

		fullTopic := fmt.Sprintf("%s-%s", topicPrefix, topic)
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


func getTimeout(ctx context.Context, inbound interface{}) time.Duration{
	popInbound, ok:= inbound.(*pb.PopInbound)
	if ok{
		return convertTimeout(popInbound.GetTimeout())
	}
	releaseInbound, ok := inbound.(*pb.ReleaseInbound)
	if ok{
		return convertTimeout(releaseInbound.GetTimeout())
	}

	timeout, ok := getPramFromCtx(ctx, "timeout")
	if !ok{
		return common.DefaultTimeout
	}else {
		timeout, err := strconv.ParseInt(timeout,10, 32)
		if err == nil {
			return common.DefaultTimeout
		}
		return convertTimeout(int32(timeout))
	}
}

func convertTimeout(t int32) time.Duration{
	timeoutDuration := time.Duration(t)*time.Second
	if timeoutDuration < common.MinTimeout {
		return common.MinTimeout
	}
	if timeoutDuration > common.MaxTimeout {
		return common.MaxTimeout
	}
	return timeoutDuration
}

func getTopic(ctx context.Context, inbound interface{}) string {
	popInbound, ok := inbound.(*pb.PopInbound)
	if ok {
		return popInbound.GetTopic()
	}

	peekInbound, ok := inbound.(*pb.PeekInbound)
	if ok {
		return peekInbound.GetTopic()
	}

	pushInbound, ok := inbound.(*pb.PushInbound)
	if ok {
		return pushInbound.GetTopic()
	}

	releaseInbound, ok := inbound.(*pb.ReleaseInbound)
	if ok {
		return releaseInbound.GetTopic()
	}

	topic, ok := getPramFromCtx(ctx, codec.PtpTopicID)
	if !ok {
		return ""
	}
	return topic
}

func getInboundParams(ctx context.Context, inbound interface{} ,isPush bool) (*InboundParams, *transerr.TransError) {
	var nodeID string
	if isPush {
		nodeID = codec.PtpSourceNodeID
	} else {
		nodeID = codec.PtpTargetNodeID
	}

	topicPrefix, ok := getPramFromCtx(ctx, nodeID)
	if !ok {
		nlog.Warnf("Empty session-id or topic or %s", nodeID)
		return nil, transerr.NewTransError(transerr.InvalidRequest)
	}

	sid, ok := getPramFromCtx(ctx, codec.PtpSessionID)
	if !ok || len(sid)==0 {
		nlog.Warnf("Empty session-id or topic or %s", nodeID)
		return nil, transerr.NewTransError(transerr.InvalidRequest)
	}

	topic := getTopic(ctx, inbound)
	if len(sid) == 0 || len(topic) == 0 || len(topicPrefix) == 0 {
		nlog.Warnf("Empty session-id or topic or %s", nodeID)
		return nil, transerr.NewTransError(transerr.InvalidRequest)
	}

	return &InboundParams{
		sid:   sid,
		topic: fmt.Sprintf("%s-%s", topicPrefix, topic),
	}, nil
}

func getPramFromCtx(ctx context.Context, param string) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok{
		return "", false
	}

	values := md.Get(param)
	if len(values)==0{
		return "", true
	}

	return values[0], true
}
