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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1/interconn"
)

const (
	httpPrefix = "http://"

	createJobAPI      = "/v1/interconn/schedule/job/create"
	stopJobAPI        = "/v1/interconn/schedule/job/stop"
	startJobAPI       = "/v1/interconn/schedule/job/start"
	queryJobStatusAPI = "/v1/interconn/schedule/job/status_all"
	stopTaskAPI       = "/v1/interconn/schedule/task/stop"
	startTaskAPI      = "/v1/interconn/schedule/task/start"
	pollTaskStatusAPI = "/v1/interconn/schedule/task/poll"
)

const (
	authSignHeader  = "X-Auth-Sign"
	nodeIDHeader    = "X-Node-Id"
	nonceHeader     = "X-Nonce"
	traceIDHeader   = "X-Trace-Id"
	timestampHeader = "X-Timestamp"
)

const (
	retryTimes       = 3
	connectTimeout   = 10 * time.Second
	readWriteTimeout = 30 * time.Second
	keepAliveTimeout = 30 * time.Second
)

// Client defines a client which is used to access to kuscia storage service.
type Client struct {
	httpClient *http.Client
}

// New is used to create a client instance.
func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   connectTimeout,
					KeepAlive: keepAliveTimeout,
				}).DialContext,
			},
			Timeout: readWriteTimeout,
		},
	}
}

// CreateJob is used to send create-job request to other party.
func (c *Client) CreateJob(ctx context.Context, requesterID, host, jobID, flowID string, dag *interconn.DAG, config *interconn.Config) error {
	req := &interconn.CreateJobRequest{
		JobId:  jobID,
		FlowId: flowID,
		Dag:    dag,
		Config: config,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, requesterID, host, http.MethodPost, fmt.Sprintf("%s%s%s", httpPrefix, host, createJobAPI), body)
	return err
}

// StopJob is used to send stop-job request to other party.
func (c *Client) StopJob(ctx context.Context, requesterID, host, jobID string) error {
	req := &interconn.StopJobRequest{
		JobId: jobID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, requesterID, host, http.MethodPost, fmt.Sprintf("%s%s%s", httpPrefix, host, stopJobAPI), body)
	return err
}

// StartJob is used to send start-job request to other party.
func (c *Client) StartJob(ctx context.Context, requesterID, host, jobID string) error {
	req := &interconn.StartJobRequest{
		JobId: jobID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, requesterID, host, http.MethodPost, fmt.Sprintf("%s%s%s", httpPrefix, host, startJobAPI), body)
	return err
}

// QueryJobStatusAll is used to send query job status request to other party.
func (c *Client) QueryJobStatusAll(ctx context.Context, requesterID, host, jobID string) (*interconn.CommonResponse, error) {
	url := fmt.Sprintf("%s%s%s?job_id=%s", httpPrefix, host, queryJobStatusAPI, jobID)
	return c.do(ctx, requesterID, host, http.MethodGet, url, nil)
}

// StopTask is used to send stop-task request to other party.
func (c *Client) StopTask(ctx context.Context, requesterID, host, taskID string) error {
	req := &interconn.StopTaskRequest{
		TaskId: taskID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, requesterID, host, http.MethodPost, fmt.Sprintf("%s%s%s", httpPrefix, host, stopTaskAPI), body)
	return err
}

// StartTask is used to send start-task request to other party.
func (c *Client) StartTask(ctx context.Context, requesterID, host, jobID, taskID, taskName string) (*interconn.CommonResponse, error) {
	req := &interconn.StartTaskRequest{
		TaskId:   taskID,
		JobId:    jobID,
		TaskName: taskName,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, requesterID, host, http.MethodPost, fmt.Sprintf("%s%s%s", httpPrefix, host, startTaskAPI), body)
}

// PollTaskStatus is used to send poll task status request to other party.
func (c *Client) PollTaskStatus(ctx context.Context, requesterID, host, taskID, role string) (*interconn.CommonResponse, error) {
	req := &interconn.PollTaskStatusRequest{
		TaskId: taskID,
		Role:   role,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	return c.do(ctx, requesterID, host, http.MethodPost, fmt.Sprintf("%s%s%s", httpPrefix, host, pollTaskStatusAPI), body)
}

func (c *Client) do(ctx context.Context, requesterID, host, method, url string, body []byte) (*interconn.CommonResponse, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	nlog.Infof("bfia client send request path: %v, body: %v", req.URL.RequestURI(), string(body))
	req.Header.Set(nodeIDHeader, requesterID)
	req.Header.Set(timestampHeader, strconv.FormatInt(time.Now().Unix(), 10))
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Host = host
	var resp *http.Response
	for i := 0; ; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil {
			break
		}

		if i >= retryTimes {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed, %v", err)
	}

	nlog.Infof("response body: %v", string(respBody))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response status code: %v, body: %v", resp.StatusCode, string(respBody))
	}

	commonResp := &interconn.CommonResponse{}
	err = json.Unmarshal(respBody, commonResp)
	if err != nil {
		return nil, err
	}

	if commonResp.Code != http.StatusOK {
		return nil, fmt.Errorf("status code: %v, message: %v", commonResp.Code, commonResp.Msg)
	}
	return commonResp, nil
}
