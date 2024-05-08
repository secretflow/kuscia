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
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/config"
	"github.com/secretflow/kuscia/pkg/transport/msq"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type Method string

const (
	invoke  Method = "invoke"
	pop     Method = "pop"
	peek    Method = "peek"
	release Method = "release"
)

type TransHandler func(r *http.Request) *codec.Outbound

type Server struct {
	svrConfig *config.ServerConfig
	sm        *msq.SessionManager

	factory map[Method]TransHandler
	codec   codec.Codec
}

func NewServer(config *config.ServerConfig, sm *msq.SessionManager) *Server {
	return &Server{
		svrConfig: config,
		sm:        sm,
		factory:   make(map[Method]TransHandler),
		codec:     codec.NewProtoCodec(),
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.registerInnerHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/interconn/chan/transport", s.generateHandler(invoke))
	mux.HandleFunc("/v1/interconn/chan/invoke", s.generateHandler(invoke))
	mux.HandleFunc("/v1/interconn/chan/pop", s.generateHandler(pop))
	mux.HandleFunc("/v1/interconn/chan/peek", s.generateHandler(peek))
	mux.HandleFunc("/v1/interconn/chan/release", s.generateHandler(release))

	sr := &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%d", s.svrConfig.Port),
		Handler:        mux,
		ReadTimeout:    time.Duration(s.svrConfig.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(s.svrConfig.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(s.svrConfig.IdleTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	errChan := make(chan error)
	go func() {
		errChan <- sr.ListenAndServe()
	}()

	select {
	case err := <-errChan:
		nlog.Errorf("Transport server exit with error: %v", err)
		return err
	case <-ctx.Done():
		if err := sr.Shutdown(ctx); err != nil {
			nlog.Warnf("Transport shutdown fail:%v ", err)
		}
		nlog.Errorf("Transport server has been canceled")
		return fmt.Errorf("transport server has been canceled")
	}
}

func Run(ctx context.Context, configFile string) error {
	transConfig, err := config.LoadTransConfig(configFile)
	if err != nil {
		return err
	}

	msq.Init(transConfig.MsqConfig)
	sessionManager := msq.NewSessionManager()
	server := NewServer(transConfig.HTTPConfig, sessionManager)
	return server.Start(ctx)
}
