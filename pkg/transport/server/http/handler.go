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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/msq"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// params
const (
	paramTimeout   = "timeout"
	defaultTimeout = time.Second * 120
	minTimeout     = time.Second
	maxTimeout     = time.Second * 300
)

type ReqParams struct {
	sid   string
	topic string
}

func (s *Server) generateHandler(method Method) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		outbound := s.factory[method](r)
		body, err := s.codec.Marshal(outbound)
		if err != nil {
			nlog.Warnf("Marshal outbound fail :%v", outbound)
			errMsg := fmt.Sprintf("Body encode fail")
			w.Header().Set("Content-Type", "application/text")
			w.WriteHeader(500)
			if _, err := io.WriteString(w, errMsg); err != nil {
				nlog.Warnf("Write error msg fail")
			}
		} else {
			w.Header().Set("Content-Type", s.codec.ContentType())
			w.WriteHeader(200)
			if _, err := w.Write(body); err != nil {
				nlog.Warnf("Write resp body fail")
			}
		}
	}
}

func (s *Server) registerInnerHandler() {
	s.factory = map[Method]TransHandler{
		invoke:  s.handleInvoke,
		pop:     s.handlePop,
		peek:    s.handlePeek,
		release: s.handleRelease,
	}
}

func (s *Server) handleInvoke(r *http.Request) *codec.Outbound {
	params, err := getReqParams(r, true)
	if err != nil {
		return codec.BuildOutboundByErr(err)
	}

	message, err := s.readMessage(r)
	if err != nil {
		return codec.BuildOutboundByErr(err)
	}

	err = s.sm.Push(params.sid, params.topic, message, getTimeout(r))
	return codec.BuildOutboundByErr(err)
}

func (s *Server) handlePop(r *http.Request) *codec.Outbound {
	params, err := getReqParams(r, false)
	if err != nil {
		return codec.BuildOutboundByErr(err)
	}

	msg, err := s.sm.Pop(params.sid, params.topic, getTimeout(r))
	if err != nil || msg == nil {
		return codec.BuildOutboundByErr(err)
	}

	return codec.BuildOutboundByPayload(msg.Content)
}

func (s *Server) handlePeek(r *http.Request) *codec.Outbound {
	params, err := getReqParams(r, false)
	if err != nil {
		return codec.BuildOutboundByErr(err)
	}

	msg, err := s.sm.Peek(params.sid, params.topic)
	if err != nil || msg == nil {
		return codec.BuildOutboundByErr(err)
	}

	return codec.BuildOutboundByPayload(msg.Content)
}

func (s *Server) handleRelease(r *http.Request) *codec.Outbound {
	sid := r.Header.Get(codec.PtpSessionID)
	if len(sid) == 0 {
		nlog.Warnf("Empty session-id")
		return codec.BuildOutboundByErr(transerr.NewTransError(transerr.InvalidRequest))
	}

	topic := r.Header.Get(codec.PtpTopicID)
	if len(topic) != 0 {
		topicPrefix := r.Header.Get(codec.PtpTargetNodeID)
		fullTopic := fmt.Sprintf("%s-%s", topicPrefix, topic)
		s.sm.ReleaseTopic(sid, fullTopic)
		return codec.BuildOutboundByErr(nil)
	}

	s.sm.ReleaseSession(sid)
	return codec.BuildOutboundByErr(nil)
}

func (s *Server) readMessage(r *http.Request) (*msq.Message, *transerr.TransError) {
	if r.ContentLength > s.svrConfig.ReqBodyMaxSize {
		return nil, transerr.NewTransError(transerr.BodyTooLarge)
	}

	maxSize := s.svrConfig.ReqBodyMaxSize + 1
	body, err := io.ReadAll(io.LimitReader(r.Body, maxSize))
	if err != nil {
		nlog.Warnf("Read body err: %v", err)
		return nil, transerr.NewTransError(transerr.ServerError)
	}

	if int64(len(body)) == maxSize {
		return nil, transerr.NewTransError(transerr.BodyTooLarge)
	}

	if int64(len(body)) == 0 {
		nlog.Warnf("Empty request body")
		return nil, transerr.NewTransError(transerr.InvalidRequest)
	}

	return msq.NewMessage(body), nil
}

func getReqParams(r *http.Request, isPush bool) (*ReqParams, *transerr.TransError) {
	sid := r.Header.Get(codec.PtpSessionID)
	topic := r.Header.Get(codec.PtpTopicID)
	var topicPrefix string
	var nodeID string
	if isPush {
		nodeID = codec.PtpSourceNodeID
	} else {
		nodeID = codec.PtpTargetNodeID
	}
	topicPrefix = r.Header.Get(nodeID)

	if len(sid) == 0 || len(topic) == 0 || len(topicPrefix) == 0 {
		nlog.Warnf("Empty session-id or topic or %s", nodeID)
		return nil, transerr.NewTransError(transerr.InvalidRequest)
	}

	return &ReqParams{
		sid:   sid,
		topic: fmt.Sprintf("%s-%s", topicPrefix, topic),
	}, nil
}

func getTimeout(r *http.Request) time.Duration {
	val := r.URL.Query().Get(paramTimeout)
	if len(val) == 0 {
		return defaultTimeout
	}

	num, err := strconv.Atoi(val)
	if err != nil {
		nlog.Warnf("Invalid param timeout:%d", num)
		return defaultTimeout
	}

	timeout := time.Second * time.Duration(num)

	if timeout < minTimeout {
		return minTimeout
	}

	if timeout > maxTimeout {
		return maxTimeout
	}

	return timeout
}
