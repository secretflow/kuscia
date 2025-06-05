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

package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin/binding"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/common"
	dcommon "github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
	"github.com/secretflow/kuscia/pkg/web/interceptor"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

type Client struct {
	HostName string
	client   *http.Client
}

func NewDiagnoseClient(hostName string) *Client {
	return &Client{
		client:   &http.Client{},
		HostName: hostName,
	}
}

func (c *Client) invokeProto(ctx context.Context, request proto.Message, response proto.Message, method, path string) error {
	var byteReq []byte
	if request != nil {
		var err error
		byteReq, err = proto.Marshal(request)
		if err != nil {
			nlog.Errorf("Send request %+v ,marshal request failed: %s", request, err.Error())
			return err
		}
	}
	byteBody, err := c.invokeProtoRequest(ctx, method, path, byteReq)
	if err != nil {
		nlog.Errorf("Send request %+v failed: %s", request, err.Error())
		return err
	}
	err = proto.Unmarshal(byteBody, response)
	if err != nil {
		nlog.Errorf("Send request %+v ,Unmarshal response body %s failed: %s", request, byteBody, err.Error())
		return err
	}
	return nil
}

func (c *Client) invokeProtoWithRetry(ctx context.Context, request proto.Message, response proto.Message, method, path string, attempt int) error {
	var err error
	backoff := time.Second
	for i := 1; i <= attempt; i++ {
		err = c.invokeProto(ctx, request, response, method, path)
		if err == nil {
			return nil
		}
		nlog.Errorf("Attempt postProtoWithRetry %d failed: %v\n", i, err)
		time.Sleep(backoff)
		backoff *= 2
	}
	return err
}

func (c *Client) invokeProtoRequest(ctx context.Context, method, path string, request []byte) ([]byte, error) {
	// construct http request
	var reader io.Reader
	if request != nil {
		reader = bytes.NewReader(request)
	}
	req, err := http.NewRequest(method, c.getURL(path), reader)
	if err != nil {
		return nil, err
	}
	// set header
	req.Host = c.HostName
	if reader != nil {
		req.Header.Set("Content-Type", binding.MIMEPROTOBUF)
	}

	// send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// read response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// check http status code
	statusCode := resp.StatusCode
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected error, status_code: '%d', msg: %q", resp.StatusCode, string(bodyBytes))
	}
	return bodyBytes, nil
}

func (c *Client) getURL(path string) string {
	address := c.HostName
	if !strings.HasPrefix(address, "http") {
		address = fmt.Sprintf("http://%s", address)
	}
	return fmt.Sprintf("%s%s", address, path)
}

func (c *Client) SubmitReport(ctx context.Context, request *diagnose.SubmitReportRequest) (response *diagnose.StatusResponse, err error) {
	response = &diagnose.StatusResponse{}
	err = c.invokeProtoWithRetry(ctx, request, response, http.MethodPost, fmt.Sprintf("/%v/%v", dcommon.DiagnoseNetworkGroup, dcommon.DiagnoseSubmitReportPath), 5)
	return
}

func (c *Client) Done(ctx context.Context) (response *diagnose.StatusResponse, err error) {
	response = &diagnose.StatusResponse{}
	err = c.invokeProtoWithRetry(ctx, nil, response, http.MethodGet, fmt.Sprintf("/%v/%v", dcommon.DiagnoseNetworkGroup, dcommon.DiagnoseDonePath), 5)
	return
}

func (c *Client) Healthy(ctx context.Context) (response *diagnose.StatusResponse, err error) {
	response = &diagnose.StatusResponse{}
	err = c.invokeProtoWithRetry(ctx, nil, response, http.MethodGet, fmt.Sprintf("/%v/%v", dcommon.DiagnoseNetworkGroup, dcommon.DiagnoseHealthyPath), 5)
	return
}

func (c *Client) Mock(ctx context.Context, request *diagnose.MockRequest) (response *diagnose.MockResponse, err error) {
	response = &diagnose.MockResponse{}
	err = c.invokeProtoWithRetry(ctx, request, response, http.MethodPost, fmt.Sprintf("/%v/%v", dcommon.DiagnoseNetworkGroup, dcommon.DiagnoseMockPath), 1)
	return
}

func (c *Client) MockChunk(req proto.Message, url string) (*http.Response, error) {
	jsonData, err := proto.Marshal(req)
	if err != nil {
		nlog.Errorf("Error marshaling proto: %v", err)
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		nlog.Errorf("Error creating request: %v", err)
		return nil, err
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		nlog.Errorf("Error sending POST request: %v", err)
		return nil, err
	}
	return resp, nil
}

func NewKusciaAPIConn() (*grpc.ClientConn, error) {
	kusciaConfig := confloader.DefaultKusciaConfig(common.DefaultKusciaHomePath())
	tokenFile := kusciaConfig.KusciaAPI.Token.TokenFile
	tlsConfig := &config.TLSConfig{
		CAPath:         kusciaConfig.CACertFile,
		ServerCertPath: kusciaConfig.KusciaAPI.TLS.ServerCertFile,
		ServerKeyPath:  kusciaConfig.KusciaAPI.TLS.ServerKeyFile,
	}
	grpcAddr := fmt.Sprintf("%s:%d", constants.LocalhostIP, kusciaConfig.KusciaAPI.GRPCPort)
	return NewGrpcConn(grpcAddr, tlsConfig, tokenFile)
}

func NewGrpcConn(address string, tlsconfig *config.TLSConfig, tokenFile string) (*grpc.ClientConn, error) {
	dialOpts := make([]grpc.DialOption, 0)
	// use token if token file exists
	if tokenFile != "" && paths.CheckFileExist(tokenFile) {
		token, err := os.ReadFile(tokenFile)
		if err != nil {
			return nil, err
		}
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(interceptor.GrpcClientTokenInterceptor(string(token))))
	}
	// use tls if ca file exists
	if err := paths.CheckAllFileExist(tlsconfig.CAPath, tlsconfig.ServerCertPath, tlsconfig.ServerKeyPath); err != nil {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	} else {
		clientTLSConfig, err := tlsutils.BuildClientTLSConfigViaPath(tlsconfig.CAPath, tlsconfig.ServerCertPath, tlsconfig.ServerKeyPath)
		if err != nil {
			nlog.Errorf("local tls config error: %v", err)
			return nil, err
		}
		creds := credentials.NewTLS(clientTLSConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	}

	grpcConn, err := grpc.Dial(address, dialOpts...)
	if err != nil {
		nlog.Errorf("grpc connect fail: %v", err)
		return nil, err
	}
	return grpcConn, nil
}
