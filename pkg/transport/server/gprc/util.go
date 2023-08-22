package gprc

import (
	"context"
	"fmt"
	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"google.golang.org/grpc/metadata"
	"strconv"
	"time"
)

const (
	//paramTimeout   = "timeout"
	defaultTimeout = time.Second * 120
	minTimeout     = time.Second
	maxTimeout     = time.Second * 300
)

type InboundParams struct {
	sid   string
	topic string
}

type Method string

/*func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, hanlder grpc.StreamHandler) error {
	p, _ := peer2.FromContext(ss.Context())
	if p!=nil{
		p.Max
	}
}*/

func getInboundParams(ctx context.Context, isPush bool) (*InboundParams, *transerr.TransError) {
	md, _ := metadata.FromIncomingContext(ctx)
	sidList := md.Get(codec.PtpSessionID)
	topicList := md.Get(codec.PtpTopicID)

	//var topicPrefix string
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
	md, _ := metadata.FromIncomingContext(ctx)
	timeoutList := md.Get("timeout")
	if len(timeoutList) == 0 {
		return defaultTimeout
	}

	num, err := strconv.Atoi(timeoutList[0])
	if err != nil {
		return defaultTimeout
	}
	timeout := time.Duration(num) * time.Second
	if timeout < minTimeout {
		return minTimeout
	}

	if timeout > maxTimeout {
		return maxTimeout
	}

	return timeout
}

func getMetaDataList(ctx context.Context, element string) []string {
	md, _ := metadata.FromIncomingContext(ctx)
	return md.Get(element)
}
