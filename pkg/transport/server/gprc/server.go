package gprc

import (
	"fmt"
	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/transport/msq"
	pb "github.com/secretflow/kuscia/pkg/transport/proto/grpcPtp"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"golang.org/x/net/context"
	"golang.org/x/net/netutil"
	"google.golang.org/grpc"
	"net"
)

type Server struct {
	grpcConfig *config.GRPCConfig
	sm         *msq.SessionManager

	//factory map[Method]TransHandler
	codec codec.Codec
	pb.UnimplementedTransportServer
}

func NewServer(grpcConfig *config.GRPCConfig, sm *msq.SessionManager) *Server {
	return &Server{
		grpcConfig: grpcConfig,
		sm:         sm,
		//factory:   make(map[Method]TransHandler),
		codec: codec.NewProtoCodec(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(s.grpcConfig.MaxConcurrentStreams),
		grpc.MaxRecvMsgSize(s.grpcConfig.MaxReadFrameSize),
		grpc.MaxSendMsgSize(s.grpcConfig.MaxReadFrameSize),
	}
	gs := grpc.NewServer(opts...)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.grpcConfig.Port))
	if err != nil {
		return err
	}

	limitedListener := netutil.LimitListener(listener, s.grpcConfig.MaxConns)

	pb.RegisterTransportServer(gs, s)
	err = gs.Serve(limitedListener)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) Invoke(ctx context.Context, inbound *pb.InvokeTransportInbound) (*pb.TransportOutbound, error) {
	params, err := getInboundParams(ctx, true)
	if err != nil {
		return codec.BuildGrpcOutboundByErr(err), nil
	}
	message, err := s.readMessage(inbound)
	if err != nil {
		return codec.BuildGrpcOutboundByErr(err), nil
	}
	err = s.sm.Push(params.sid, params.topic, message, getInboundTimeout(ctx))
	return codec.BuildGrpcOutboundByErr(err), nil
}

func (s *Server) Pop(ctx context.Context, inbound *pb.TransportInbound) (*pb.TransportOutbound, error) {
	params, err := getInboundParams(ctx, false)
	if err != nil {
		return codec.BuildGrpcOutboundByErr(err), nil
	}

	msg, err := s.sm.Pop(params.sid, params.topic, getInboundTimeout(ctx))
	if err != nil || msg == nil {
		return codec.BuildGrpcOutboundByErr(err), nil
	}

	return codec.BuildGrpcOutboundByPayload(msg.Content), nil
}

func (s *Server) Peek(ctx context.Context, inbound *pb.TransportInbound) (*pb.TransportOutbound, error) {
	params, err := getInboundParams(ctx, false)
	if err != nil {
		return codec.BuildGrpcOutboundByErr(err), nil
	}

	msg, err := s.sm.Peek(params.sid, params.topic)
	if err != nil || msg == nil {
		return codec.BuildGrpcOutboundByErr(err), nil
	}

	return codec.BuildGrpcOutboundByPayload(msg.Content), nil
}

func (s *Server) Release(ctx context.Context, inbound *pb.TransportInbound) (*pb.TransportOutbound, error) {
	sidList := getMetaDataList(ctx, codec.PtpSessionID)
	if len(sidList) == 0 {
		nlog.Warnf("Empty session-id")
		return codec.BuildGrpcOutboundByErr(transerr.NewTransError(transerr.InvalidRequest)), nil
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
		return codec.BuildGrpcOutboundByErr(nil), nil
	}
	s.sm.ReleaseSession(sid)
	return codec.BuildGrpcOutboundByErr(nil), nil
}

func (s *Server) readMessage(inbound *pb.InvokeTransportInbound) (*msq.Message, *transerr.TransError) {
	if int64(len(inbound.GetMsg())) == 0 {
		nlog.Warnf("Empty request body")
		return nil, transerr.NewTransError(transerr.BodyTooLarge)
	}
	return msq.NewMessage(inbound.GetMsg()), nil
}
