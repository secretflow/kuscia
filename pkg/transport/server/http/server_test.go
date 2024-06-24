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

package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/transport/msq"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/stretchr/testify/assert"
)

const (
	testServer = "http://127.0.0.1:2001"
)

func generatePath(method Method) string {
	return fmt.Sprintf("%s/v1/interconn/chan/%s", testServer, string(method))
}

var server *Server
var httpConfig *config.ServerConfig
var msqConfig *msq.Config

func NewRandomStr(l int) []byte {
	const str = "0123456789abcdefghijklmnopqrstuvwxyz"
	content := make([]byte, l, l)
	for l > 0 {
		c := len(str)
		if l < len(str) {
			c = l
		}

		for i := 0; i < c; i++ {
			l--
			content[l] = str[i]
		}
	}
	return content
}

func NewStr(str string) []byte {
	return []byte(str)
}

func verifyResponse(t *testing.T, req *http.Request, code transerr.ErrorCode) *codec.Outbound {
	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	outbound, err := server.codec.UnMarshal(body)
	assert.Equal(t, string(code), outbound.Code)
	return outbound
}

func TestMain(m *testing.M) {
	httpConfig = config.DefaultServerConfig()
	httpConfig.Port = 2001
	msqConfig = msq.DefaultMsgConfig()

	msq.Init(msqConfig)
	server = NewServer(httpConfig, msq.NewSessionManager())
	go server.Start(context.Background())
	// wait server startup
	time.Sleep(time.Millisecond * 200)
	os.Exit(m.Run())
}

func TestTransportAndPop(t *testing.T) {
	req, err := http.NewRequest("POST", generatePath(invoke), bytes.NewBuffer(NewStr("123456789")))
	assert.NoError(t, err)

	req.Header.Set(codec.PtpTopicID, "topic1")
	req.Header.Set(codec.PtpSessionID, "session1")
	req.Header.Set(codec.PtpSourceNodeID, "node0")
	verifyResponse(t, req, transerr.Success)

	popReq, err := http.NewRequest("POST", generatePath(pop), bytes.NewBuffer(nil))
	popReq.Header.Set(codec.PtpTopicID, "topic1")
	popReq.Header.Set(codec.PtpSessionID, "session1")
	popReq.Header.Set(codec.PtpTargetNodeID, "node0")
	outbound := verifyResponse(t, popReq, transerr.Success)
	msg := string(outbound.Payload)
	assert.Equal(t, msg, "123456789")
}

func TestPeek(t *testing.T) {
	peekReq, _ := http.NewRequest("POST", generatePath(peek), bytes.NewBuffer(nil))
	peekReq.Header.Set(codec.PtpTopicID, "topic1")
	peekReq.Header.Set(codec.PtpSessionID, "session2")
	peekReq.Header.Set(codec.PtpTargetNodeID, "node0")
	outbound := verifyResponse(t, peekReq, transerr.Success)
	msg := string(outbound.Payload)
	assert.Equal(t, msg, "")
}

func TestPopWithData(t *testing.T) {
	server.sm = msq.NewSessionManager()

	popReq, _ := http.NewRequest("POST", generatePath(pop), bytes.NewBuffer(nil))
	popReq.Header.Set(codec.PtpTopicID, "topic2")
	popReq.Header.Set(codec.PtpSessionID, "session3")
	popReq.Header.Set(codec.PtpTargetNodeID, "node0")
	params := url.Values{}
	params.Add("timeout", "5")
	popReq.URL.RawQuery = params.Encode()

	go func() {
		time.Sleep(time.Millisecond * 100)
		err := server.sm.Push("session3", "node0-topic2", &msq.Message{Content: NewRandomStr(10)}, time.Second)
		assert.Nil(t, err)
	}()

	start := time.Now()
	outbound := verifyResponse(t, popReq, transerr.Success)
	assert.Equal(t, len(outbound.Payload), 10)

	processTime := time.Now().Sub(start)
	assert.True(t, processTime >= time.Millisecond*50)
}

func TestPopTimeout(t *testing.T) {
	popReq, _ := http.NewRequest("POST", generatePath(pop), bytes.NewBuffer(nil))
	popReq.Header.Set(codec.PtpTopicID, "topic2")
	popReq.Header.Set(codec.PtpSessionID, "session4")
	popReq.Header.Set(codec.PtpTargetNodeID, "node0")
	params := url.Values{}
	params.Add("timeout", "2")
	popReq.URL.RawQuery = params.Encode()

	start := time.Now()
	outbound := verifyResponse(t, popReq, transerr.Success)
	assert.True(t, outbound.Payload == nil)

	processTime := time.Now().Sub(start)
	assert.Greater(t, processTime, time.Millisecond*1500) // 1.5s
	assert.Less(t, processTime, time.Millisecond*2500)    // 2.5s
}

func TestReleaseTopic(t *testing.T) {
	server.sm.Push("session5", "-topic2", &msq.Message{Content: NewRandomStr(1024)}, time.Second)
	server.sm.Push("session5", "-topic3", &msq.Message{Content: NewRandomStr(1024)}, time.Second)

	req, _ := http.NewRequest("POST", generatePath(release), bytes.NewBuffer(nil))
	req.Header.Set(codec.PtpSessionID, "session5")
	req.Header.Set(codec.PtpTopicID, "topic2")

	verifyResponse(t, req, transerr.Success)

	msg, err := server.sm.Peek("session5", "-topic3")
	assert.Nil(t, err)
	assert.NotNil(t, msg)

	msg, err = server.sm.Peek("session5", "-topic2")
	assert.Nil(t, err)
	assert.Nil(t, msg)
}

func TestReleaseSession(t *testing.T) {
	server.sm.Push("session6", "node0-topic2", &msq.Message{Content: NewRandomStr(1024)}, time.Second)
	server.sm.Push("session6", "node0-topic3", &msq.Message{Content: NewRandomStr(1024)}, time.Second)

	req, _ := http.NewRequest("POST", generatePath(release), bytes.NewBuffer(nil))
	req.Header.Set(codec.PtpSessionID, "session6")

	verifyResponse(t, req, transerr.Success)

	_, err := server.sm.Peek("session6", "node0-topic3")
	assert.NotNil(t, err)

	pushReq, _ := http.NewRequest("POST", generatePath(invoke), bytes.NewBuffer(NewStr("123456789")))
	pushReq.Header.Set(codec.PtpTopicID, "topic2")
	pushReq.Header.Set(codec.PtpSessionID, "session6")
	pushReq.Header.Set(codec.PtpSourceNodeID, "node0")
	verifyResponse(t, pushReq, transerr.SessionReleased)
}

func TestTooLargeBody(t *testing.T) {
	{
		pushReq, _ := http.NewRequest("POST", generatePath(invoke), bytes.NewBuffer(NewRandomStr(int(httpConfig.
			ReqBodyMaxSize+1))))
		pushReq.Header.Set(codec.PtpTopicID, "topic2")
		pushReq.Header.Set(codec.PtpSessionID, "session7")
		pushReq.Header.Set(codec.PtpSourceNodeID, "node0")
		verifyResponse(t, pushReq, transerr.BodyTooLarge)
	}

	{ // session buffer is too small
		pushReq, _ := http.NewRequest("POST", generatePath(invoke), bytes.NewBuffer(NewRandomStr(int(msq.DefaultMsgConfig().PerSessionByteSizeLimit+1))))
		pushReq.Header.Set(codec.PtpTopicID, "topic2")
		pushReq.Header.Set(codec.PtpSessionID, "session7")
		pushReq.Header.Set(codec.PtpSourceNodeID, "node0")
		verifyResponse(t, pushReq, transerr.BufferOverflow)
	}
}

func TestPushWait(t *testing.T) {
	server.sm.Push("session8", "topic2", &msq.Message{Content: NewRandomStr(int(msqConfig.PerSessionByteSizeLimit))},
		time.Second)

	go func() {
		time.Sleep(time.Second)
		server.sm.Pop("session8", "topic2", time.Second)
	}()

	pushReq, _ := http.NewRequest("POST", generatePath(invoke), bytes.NewBuffer(NewStr("123456789")))
	pushReq.Header.Set(codec.PtpTopicID, "topic2")
	pushReq.Header.Set(codec.PtpSessionID, "session8")
	pushReq.Header.Set(codec.PtpSourceNodeID, "node0")
	params := url.Values{}
	params.Add("timeout", "2")
	pushReq.URL.RawQuery = params.Encode()

	start := time.Now()
	verifyResponse(t, pushReq, transerr.Success)
	processTime := time.Now().Sub(start)
	assert.Greater(t, processTime, time.Millisecond*500) // 0.5s
	assert.Less(t, processTime, time.Millisecond*2500)   // 2.5s
}

func TestBadRequestParam(t *testing.T) {
	pushReq, _ := http.NewRequest("POST", generatePath(invoke), bytes.NewBuffer(NewStr("123456789")))
	pushReq.Header.Set(codec.PtpSessionID, "session9")

	verifyResponse(t, pushReq, transerr.InvalidRequest)
}

var sessionCount int = 10
var topicCount int = 5

var stop bool = false

func producer(t *testing.T, sendSucceedCount, sendFailCount *int64, sessionIdx, topicIdx int) {
	msgLength := 256 * 1024
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sid := fmt.Sprintf("sessionx-%d", sessionIdx)
	topic := fmt.Sprintf("topic-%d", topicIdx)
	content := NewRandomStr(r.Intn(msgLength) + 128*1024)
	nlog.Infof("new send content length=%d", len(content))

	req, _ := http.NewRequest("POST", generatePath(invoke), bytes.NewBuffer(content))
	req.Header.Set(codec.PtpTopicID, topic)
	req.Header.Set(codec.PtpSessionID, sid)
	req.Header.Set(codec.PtpSourceNodeID, "node0")
	params := url.Values{}
	params.Add("timeout", "3")
	req.URL.RawQuery = params.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
	body, err := io.ReadAll(resp.Body)
	outbound, err := server.codec.UnMarshal(body)

	if err == nil && outbound.Code == string(transerr.Success) {
		atomic.AddInt64(sendSucceedCount, 1)
	} else {
		if outbound != nil {
			nlog.Warnf("producer failed with: %v", outbound)
		}
		atomic.AddInt64(sendFailCount, 1)
	}
}

func consumer(t *testing.T, popMsgCount, popFailCount *int64, sessionIdx, topicIdx int) {
	sid := fmt.Sprintf("sessionx-%d", sessionIdx)
	topic := fmt.Sprintf("topic-%d", topicIdx)

	req, _ := http.NewRequest("POST", generatePath(pop), bytes.NewBuffer(nil))
	req.Header.Set(codec.PtpTopicID, topic)
	req.Header.Set(codec.PtpSessionID, sid)
	req.Header.Set(codec.PtpTargetNodeID, "node0")
	params := url.Values{}
	params.Add("timeout", "2")
	req.URL.RawQuery = params.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)
	body, err := io.ReadAll(resp.Body)
	outbound, err := server.codec.UnMarshal(body)

	if outbound != nil && outbound.Payload != nil {
		atomic.AddInt64(popMsgCount, 1)
	}

	if outbound.Code != string(transerr.Success) {
		atomic.AddInt64(popFailCount, 1)
	}
}

func TestPerformance(t *testing.T) {
	var popMsgCount int64 = 0
	var popFailCount int64 = 0
	var sendSucceedCount int64 = 0
	var sendFailCount int64 = 0

	var workerCount int = 5
	wg := sync.WaitGroup{}
	wg.Add(workerCount * 2)

	produceFn := func(idx int) {
		for i := 0; i < sessionCount/workerCount; i++ {
			for j := 0; j < topicCount; j++ {
				producer(t, &sendSucceedCount, &sendFailCount, idx*workerCount+i, j)
			}
		}

		wg.Done()
	}

	consumerFn := func(idx int) {
		for i := 0; i < sessionCount/workerCount; i++ {
			for j := 0; j < topicCount; j++ {
				consumer(t, &popMsgCount, &popFailCount, idx*workerCount+i, j)
			}
		}

		wg.Done()
	}

	for i := 0; i < workerCount; i++ {
		go produceFn(i)
		go consumerFn(i)
	}

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

	assert.Equal(t, int64(0), sendFailCount)
	assert.Equal(t, int64(0), popFailCount)
	assert.Equal(t, sendSucceedCount, popMsgCount+leftCount)
	nlog.Infof("sendSucceedCount=%d sendFailCount=%d", sendSucceedCount, sendFailCount)
	nlog.Infof("popMsgCount=%d popFailCount=%d leftCount=%d totalRecvCount=%d",
		popMsgCount, popFailCount, leftCount, leftCount+popMsgCount)
}
