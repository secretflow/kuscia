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
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	
	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/transport/msq"
	pb "github.com/secretflow/kuscia/pkg/transport/proto/mesh"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	testServer = "127.0.0.1:9090"
)

var server *Server
var grpcConfig *config.GrpcConfig
var msqConfig *msq.Config

func NewRandomStr(l int) []byte {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	content := make([]byte, l, l)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		content[i] = bytes[r.Intn(len(bytes))]
	}
	return content
}

func NewStr(str string) []byte {
	return []byte(str)
}

func verifyPopResponse(t *testing.T, ctx context.Context, in *pb.PopInbound, code transerr.ErrorCode) *pb.TransportOutbound {
	dial, err := grpc.Dial(testServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer func(dial *grpc.ClientConn) {
		err := dial.Close()
		assert.NoError(t, err)
	}(dial)
	client := pb.NewPrivateTransferTransportClient(dial)
	out, err := client.Pop(ctx, in)
	assert.NoError(t, err)
	assert.Equal(t, out.Code, string(code))
	return out
}

func verifyPeekResponse(t *testing.T, ctx context.Context, in *pb.PeekInbound, code transerr.ErrorCode) *pb.TransportOutbound {
	dial, err := grpc.Dial(testServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer func(dial *grpc.ClientConn) {
		err := dial.Close()
		assert.NoError(t, err)
	}(dial)
	client := pb.NewPrivateTransferTransportClient(dial)
	out, err := client.Peek(ctx, in)
	assert.NoError(t, err)
	assert.Equal(t, out.Code, string(code))
	return out
}

func verifyReleaseResponse(t *testing.T, ctx context.Context, in *pb.ReleaseInbound, code transerr.ErrorCode) *pb.TransportOutbound {
	dial, err := grpc.Dial(testServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer func(dial *grpc.ClientConn) {
		err := dial.Close()
		assert.NoError(t, err)
	}(dial)
	client := pb.NewPrivateTransferTransportClient(dial)
	out, err := client.Release(ctx, in)
	assert.NoError(t, err)
	assert.Equal(t, out.Code, string(code))
	return out
}

func verifyInvokeResponse(t *testing.T, ctx context.Context, in *pb.Inbound, code transerr.ErrorCode) *pb.Outbound {
	dial, err := grpc.Dial(testServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer func(dial *grpc.ClientConn) {
		err := dial.Close()
		assert.NoError(t, err)
	}(dial)

	client := pb.NewPrivateTransferProtocolClient(dial)
	out, err := client.Invoke(ctx, in)
	assert.NoError(t, err)
	assert.Equal(t, out.Code, string(code))
	return out
}

func TestMain(m *testing.M) {
	grpcConfig = config.DefaultGrpcConfig()
	msqConfig = msq.DefaultMsgConfig()

	msq.Init(msqConfig)

	server = NewServer(grpcConfig, msq.NewSessionManager())
	go server.Start(context.Background())
	time.Sleep(time.Second * 2)
	os.Exit(m.Run())
}

func TestTransportAndPop(t *testing.T) {
	md := metadata.New(map[string]string{
		codec.PtpSessionID:    "session1",
		codec.PtpSourceNodeID: "node0",
		codec.PtpTopicID: "topic1",
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	invokeInbound := &pb.Inbound{
		Payload: NewStr("123456789"),
	}
	verifyInvokeResponse(t, ctx, invokeInbound, transerr.Success)

	popMd := metadata.New(map[string]string{
		codec.PtpSessionID:    "session1",
		codec.PtpTargetNodeID: "node0",
	})
	popCtx := metadata.NewOutgoingContext(context.Background(), popMd)
	popInbound := &pb.PopInbound{
		Topic: "topic1",
	}
	outbound := verifyPopResponse(t, popCtx, popInbound, transerr.Success)
	assert.Equal(t, string(outbound.GetPayload()), "123456789")
}

func TestPeek(t *testing.T) {
	peekMd := metadata.New(map[string]string{
		codec.PtpSessionID:    "session1",
		codec.PtpTargetNodeID: "node0",
	})
	peekCtx := metadata.NewOutgoingContext(context.Background(), peekMd)
	peekInbound := &pb.PeekInbound{Topic: "topic1"}
	outbound := verifyPeekResponse(t, peekCtx, peekInbound, transerr.Success)
	assert.Equal(t, string(outbound.Payload), "")
}

func TestPopWithData(t *testing.T) {
	server.sm = msq.NewSessionManager()
	go func() {
		time.Sleep(time.Second * 1)
		err := server.sm.Push("session3", "node0-topic2", &msq.Message{Content: NewRandomStr(10)}, time.Second)
		assert.Nil(t, err)
	}()

	popMd := metadata.New(map[string]string{
		codec.PtpSessionID:    "session3",
		codec.PtpTargetNodeID: "node0",
	})
	popCtx := metadata.NewOutgoingContext(context.Background(), popMd)
	popInbound := &pb.PopInbound{
		Topic: "topic2",
		Timeout: 5,
	}
	start := time.Now()

	outbound := verifyPopResponse(t, popCtx, popInbound, transerr.Success)
	assert.Equal(t, len(outbound.Payload), 10)
	processTime := time.Now().Sub(start)
	assert.True(t, processTime > time.Second)
}

func TestPopTimeout(t *testing.T) {
	popMd := metadata.New(map[string]string{
		codec.PtpSessionID:    "session4",
		codec.PtpTargetNodeID: "node0",
	})
	popCtx := metadata.NewOutgoingContext(context.Background(), popMd)

	popInbound := &pb.PopInbound{
		Topic: "topic2",
		Timeout: 2,
	}
	start := time.Now()
	outbound := verifyPopResponse(t, popCtx, popInbound, transerr.Success)
	assert.True(t, outbound.Payload == nil)

	processTime := time.Now().Sub(start)

	assert.True(t, processTime >= time.Second*2 && processTime <= time.Second*3)
}

func TestReleaseTopic(t *testing.T) {
	server.sm.Push("session5", "-topic2", &msq.Message{Content: NewRandomStr(1024)}, time.Second)
	server.sm.Push("session5", "-topic3", &msq.Message{Content: NewRandomStr(1024)}, time.Second)

	releaseMd := metadata.New(map[string]string{
		codec.PtpSessionID: "session5",
	})
	releaseCtx := metadata.NewOutgoingContext(context.Background(), releaseMd)
	releaseInbound := &pb.ReleaseInbound{
		Topic: "topic2",
	}
	verifyReleaseResponse(t, releaseCtx, releaseInbound, transerr.Success)

	msg, err := server.sm.Peek("session5", "-topic3")
	assert.Nil(t, err)
	assert.NotNil(t, msg)

	msg, err = server.sm.Peek("session5", "-topic2")
	assert.Nil(t, err)
	assert.Nil(t, msg)
}

func TestReleaseSession(t *testing.T) {
	server.sm.Push("session6", "node0-topic2",
		&msq.Message{Content: NewRandomStr(1024)}, time.Second)
	server.sm.Push("session6", "node0-topic3",
		&msq.Message{Content: NewRandomStr(1024)}, time.Second)

	releaseMd := metadata.New(map[string]string{
		codec.PtpSessionID: "session6",
	})
	releaseCtx := metadata.NewOutgoingContext(context.Background(), releaseMd)

	verifyReleaseResponse(t, releaseCtx, &pb.ReleaseInbound{}, transerr.Success)
	_, err := server.sm.Peek("session6", "node0-topic3")
	assert.NotNil(t, err)

	invokeMd := metadata.New(map[string]string{
		codec.PtpTopicID:      "topic2",
		codec.PtpSessionID:    "session6",
		codec.PtpSourceNodeID: "node0",
	})
	releaseCtx2 := metadata.NewOutgoingContext(context.Background(), invokeMd)
	verifyInvokeResponse(t, releaseCtx2, &pb.Inbound{Payload: NewStr("123456789")}, transerr.SessionReleased)
}

func TestPushWait(t *testing.T) {
	server.sm.Push("session8", "topic2", &msq.Message{Content: NewRandomStr(int(msqConfig.PerSessionByteSizeLimit))},
		time.Second)

	go func() {
		time.Sleep(time.Second)
		server.sm.Pop("session8", "topic2", time.Second)
	}()
	invokeMd := metadata.New(map[string]string{
		codec.PtpTopicID:      "topic2",
		codec.PtpSessionID:    "session8",
		codec.PtpSourceNodeID: "node0",
	})
	invokeMd.Append("timeout", "2")
	start := time.Now()
	pushCtx := metadata.NewOutgoingContext(context.Background(), invokeMd)
	verifyInvokeResponse(t, pushCtx, &pb.Inbound{Payload: NewStr("123456789")}, transerr.Success)
	processTime := time.Now().Sub(start)
	assert.True(t, processTime >= time.Second && processTime <= time.Second*2)
}

func TestBadRequestParam(t *testing.T) {
	invokeMd := metadata.New(map[string]string{
		codec.PtpSessionID: "session9",
	})
	invokeCtx := metadata.NewOutgoingContext(context.Background(), invokeMd)
	verifyInvokeResponse(t, invokeCtx, &pb.Inbound{Payload: NewStr("123456789")}, transerr.InvalidRequest)
}

var sessionCount int = 10
var topicCount int = 5

var stop bool = false

func producer(t *testing.T, pushSucceedCount, pushFailCount *int64) {
	msgLength := 256 * 1024
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sid := fmt.Sprintf("sessionx-%d", r.Intn(sessionCount))
	topic := fmt.Sprintf("topic-%d", r.Intn(topicCount))
	content := NewRandomStr(r.Intn(msgLength) + 256*1024)

	invokeMd := metadata.New(map[string]string{
		codec.PtpTopicID:      topic,
		codec.PtpSessionID:    sid,
		codec.PtpSourceNodeID: "node0",
	})
	invokeMd.Append("timeout", "3")
	invokeCtx := metadata.NewOutgoingContext(context.Background(), invokeMd)

	dial, err := grpc.Dial(testServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer func(dial *grpc.ClientConn) {
		err := dial.Close()
		assert.NoError(t, err)
	}(dial)

	client := pb.NewPrivateTransferProtocolClient(dial)

	outbound, err := client.Invoke(invokeCtx, &pb.Inbound{Payload: content})

	//assert.NoError(t, err)

	if err == nil && outbound != nil && outbound.Code == string(transerr.Success) {
		atomic.AddInt64(pushSucceedCount, 1)
	} else {
		if outbound != nil {
			nlog.Warnf("%v", outbound)
		}
		atomic.AddInt64(pushFailCount, 1)
	}
}

func consumer(t *testing.T, popSucceedCount, popFailCount *int64) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sid := fmt.Sprintf("sessionx-%d", r.Intn(sessionCount))
	topic := fmt.Sprintf("topic-%d", r.Intn(topicCount))

	popMd := metadata.New(map[string]string{
		codec.PtpSessionID:    sid,
		codec.PtpTargetNodeID: "node0",
	})
	popCtx := metadata.NewOutgoingContext(context.Background(), popMd)

	popInbound := &pb.PopInbound{
		Topic: topic,
		Timeout: 2,
	}

	dial, err := grpc.Dial(testServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer func(dial *grpc.ClientConn) {
		err := dial.Close()
		assert.NoError(t, err)
	}(dial)

	client := pb.NewPrivateTransferTransportClient(dial)

	var outbound *pb.TransportOutbound
	outbound, err = client.Pop(popCtx, popInbound)

	//assert.NoError(t, err)

	if outbound != nil && outbound.Payload != nil {
		atomic.AddInt64(popSucceedCount, 1)
	}

	if outbound == nil || outbound.Code != string(transerr.Success) {
		atomic.AddInt64(popFailCount, 1)
	}
}

func TestPerformance(t *testing.T) {
	var pushSucceedCount int64 = 0
	var pushFailCount int64 = 0
	var popSucceedCount int64 = 0
	var popFailCount int64 = 0

	var workerCount int = 5
	stop = false
	wg := sync.WaitGroup{}
	wg.Add(workerCount * 2)

	produceFn := func(idx int) {
		for !stop {
			producer(t, &pushSucceedCount, &pushFailCount)
		}
		wg.Done()
	}

	consumerFn := func(idx int) {
		for !stop {
			consumer(t, &popSucceedCount, &popFailCount)
		}
		wg.Done()
	}

	for i := 0; i < workerCount; i++ {
		go produceFn(i)
		go consumerFn(i)
	}
	start := time.Now()
	go func() {
		for !stop {
			time.Sleep(time.Second * 30)

			fmt.Printf("-----Current time: %s, Cost: %s----\n", time.Now().Format(time.RFC3339), time.Now().Sub(start))
			fmt.Printf("pushSucceedCount=%d pushFailCount=%d\n", pushSucceedCount, pushFailCount)
			fmt.Printf("popSucceedCount=%d popFailCount=%d \n\n\n", popSucceedCount, popFailCount)
		}
	}()

	time.Sleep(time.Second * 20)
	stop = true
	wg.Wait()

	var leftCount int64 = 0
	for i := 0; i <= sessionCount; i++ {
		sid := fmt.Sprintf("sessionx-%d", i)
		for j := 0; j <= topicCount; j++ {
			topic := fmt.Sprintf("node0-topic-%d", j)
			for true {
				msg, _ := server.sm.Peek(sid, topic)
				if msg == nil {
					break
				}
				leftCount++
			}
		}
	}

	assert.Equal(t, pushSucceedCount, popSucceedCount+leftCount)

	fmt.Printf("pushSucceedCount=%d pushFailCount=%d\n", pushSucceedCount, pushFailCount)
	fmt.Printf("popSucceedCount=%d popFailCount=%d leftCount=%d totalRecvCount=%d\n",
		popSucceedCount, popFailCount, leftCount, leftCount+popSucceedCount)
}

func TestBuildGrpcOutboundByErr(t *testing.T) {
	md1 := metadata.New(map[string]string{
		codec.PtpSessionID: "session10",
	})
	ctx1 := metadata.NewOutgoingContext(context.Background(), md1)
	verifyInvokeResponse(t, ctx1, &pb.Inbound{Payload: NewStr("123456789")}, transerr.InvalidRequest)

	md2 := metadata.New(map[string]string{
		codec.PtpTopicID:      "topic1",
		codec.PtpSessionID:    "session10",
		codec.PtpSourceNodeID: "node0",
	})
	ctx2 := metadata.NewOutgoingContext(context.Background(), md2)
	verifyInvokeResponse(t, ctx2, &pb.Inbound{}, transerr.InvalidRequest)

	md3 := metadata.New(map[string]string{
		codec.PtpTopicID:      "topic2",
		codec.PtpSourceNodeID: "node1",
	})

	ctx3 := metadata.NewOutgoingContext(context.Background(), md3)
	verifyInvokeResponse(t, ctx3, &pb.Inbound{}, transerr.InvalidRequest)
}

func TestLoadOverrideGrpcTransConfig(t *testing.T) {
	grpcTransConfig := config.DefaultGrpcTransConfig()
	newGrpcTransConfig, err := config.LoadOverrideGrpcTransConfig(grpcTransConfig,
		"../../../../etc/conf/transport/transport.yaml")
	assert.NoError(t, err)

	newGrpcConfig := newGrpcTransConfig.GrpcConfig
	newMsqConfig := newGrpcTransConfig.MsqConfig
	msq.Init(newMsqConfig)

	newServer := NewServer(newGrpcConfig, msq.NewSessionManager())
	go newServer.Start(context.Background())

	dial, err := grpc.Dial("127.0.0.1:9091",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)

	defer func(dial *grpc.ClientConn) {
		err := dial.Close()
		assert.NoError(t, err)
	}(dial)

	invokeClient := pb.NewPrivateTransferProtocolClient(dial)

	md := metadata.New(map[string]string{
		codec.PtpTopicID:      "topic1",
		codec.PtpSessionID:    "session11",
		codec.PtpSourceNodeID: "node0",
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	invokeOut, err := invokeClient.Invoke(ctx, &pb.Inbound{Payload: []byte("123456789")})
	assert.NoError(t, err)
	assert.Equal(t, invokeOut.Code, string(transerr.Success))

	transportClient := pb.NewPrivateTransferTransportClient(dial)

	popMd := metadata.New(map[string]string{
		codec.PtpSessionID:    "session11",
		codec.PtpTargetNodeID: "node0",
	})
	popCtx := metadata.NewOutgoingContext(context.Background(), popMd)

	popInbound := &pb.PopInbound{
		Topic: "topic1",
	}

	popOut, err := transportClient.Pop(popCtx, popInbound)
	assert.NoError(t, err)

	assert.Equal(t, string(popOut.Payload), "123456789")
}
