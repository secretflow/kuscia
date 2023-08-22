package codec

import (
	pb "github.com/secretflow/kuscia/pkg/transport/proto/grpcPtp"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
)

//type Outbound grpcPtp.TransportOutbound

func BuildGrpcOutboundByErr(err *transerr.TransError) *pb.TransportOutbound {
	if err == nil {
		return BuildGrpcOutboundByPayload(nil)
	}

	return &pb.TransportOutbound{
		Payload: nil,
		Code:    err.Error(),
		Message: err.ErrorInfo(),
	}
}

func BuildGrpcOutboundByPayload(payload []byte) *pb.TransportOutbound {
	return &pb.TransportOutbound{
		Payload: payload,
		Code:    string(transerr.Success),
		Message: transerr.GetErrorInfo(transerr.Success),
	}
}
