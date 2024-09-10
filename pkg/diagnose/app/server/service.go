// Copyright 2024 Ant Group Co., Ltd.
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

package server

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"google.golang.org/protobuf/proto"
)

type DiagnoseService struct {
	PeerDone       bool
	CanStartClient bool
	RecReportDone  bool
	PeerEndpoint   string
	Report         []*diagnose.MetricItem
}

func NewService() *DiagnoseService {
	return &DiagnoseService{}
}

func (s *DiagnoseService) Mock(c *gin.Context) {
	req := new(diagnose.MockRequest)
	parseProto(c, req) // no need to check error for self use
	w := c.Writer
	header := w.Header()
	if req.EnableChunked { // chunked mode
		header.Set("Transfer-Encoding", "chunked")
		header.Set("Content-Type", "text/plain")
		chunkedbuffer := make([]byte, req.ChunkedSize)
		if req.Duration > 0 { // send chunked data by duration
			timeout := time.After(time.Duration(req.Duration) * time.Millisecond)
			for {
				select {
				case <-timeout:
					return
				default:
					if _, err := w.Write(chunkedbuffer); err != nil {
						nlog.Errorf("Send chunk failed, err: %v", err)
						return
					}
					w.(http.Flusher).Flush()
					time.Sleep(time.Duration(req.ChunkedInterval) * time.Millisecond)
				}
			}
		} else { // send chunked data by response body size
			iter := (req.RespBodySize + req.ChunkedSize - 1) / req.ChunkedSize
			for i := 1; i <= int(iter); i++ {
				if _, err := w.Write(chunkedbuffer); err != nil {
					nlog.Errorf("Send chunk failed, err: %v", err)
					return
				}
				w.(http.Flusher).Flush()
				if i == int(iter) {
					break
				}
				time.Sleep(time.Duration(req.ChunkedInterval) * time.Millisecond)
			}
		}
	} else { // short-link
		render(c, s.mock(req))
	}
}

func (s *DiagnoseService) mock(req *diagnose.MockRequest) *diagnose.MockResponse {
	resp := new(diagnose.MockResponse)
	resp.Data = make([]byte, req.RespBodySize)
	resp.Status = new(v1alpha1.Status)
	resp.Status.Code = 200
	time.Sleep(time.Duration(req.Duration) * time.Millisecond)
	return resp
}

func (s *DiagnoseService) healthy() *diagnose.StatusResponse {
	return &diagnose.StatusResponse{Status: &v1alpha1.Status{Code: 200}}
}

func (s *DiagnoseService) done() *diagnose.StatusResponse {
	s.PeerDone = true
	return &diagnose.StatusResponse{Status: &v1alpha1.Status{Code: 200}}
}

func (s *DiagnoseService) registerEndpoint(req *diagnose.RegisterEndpointRequest) *diagnose.StatusResponse {
	s.PeerEndpoint = req.Endpoint
	s.CanStartClient = true
	return &diagnose.StatusResponse{Status: &v1alpha1.Status{Code: 200}}
}

func (s *DiagnoseService) submitReport(req *diagnose.SubmitReportRequest) *diagnose.StatusResponse {
	s.Report = req.Items
	s.RecReportDone = true
	return &diagnose.StatusResponse{Status: &v1alpha1.Status{Code: 200}}
}

func (s *DiagnoseService) Heahlty(ctx *gin.Context) {
	nlog.Infof("Enter Heahlty")
	render(ctx, s.healthy())
}

func (s *DiagnoseService) Done(ctx *gin.Context) {
	nlog.Infof("Enter Done")
	render(ctx, s.done())
}

func (s *DiagnoseService) RegisterEndpoint(ctx *gin.Context) {
	nlog.Infof("Enter RegisterEndpoint")
	req := new(diagnose.RegisterEndpointRequest)
	parseProto(ctx, req)
	render(ctx, s.registerEndpoint(req))
}

func (s *DiagnoseService) SubmitReport(ctx *gin.Context) {
	nlog.Infof("Enter SubmitReport")
	req := new(diagnose.SubmitReportRequest)
	parseProto(ctx, req)
	render(ctx, s.submitReport(req))
}

func parseProto(c *gin.Context, req proto.Message) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	defer c.Request.Body.Close()
	if err := proto.Unmarshal(body, req); err != nil {
		nlog.Errorf("Body unmarshal failed, %v", err)
		return err
	}
	return nil
}

func render(ctx *gin.Context, resp proto.Message) {
	buf, _ := proto.Marshal(resp)
	ctx.Writer.Write(buf)
	ctx.Writer.(http.Flusher).Flush()
}
