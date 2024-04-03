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

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

const (
	// Domain
	UpdateDomainPath     = "/api/v1/domain/update"
	QueryDomainPath      = "/api/v1/domain/query"
	BatchQueryDomainPath = "/api/v1/domain/batchQuery"
	// Domain Route
	CreateDomainRoutePath     = "/api/v1/route/create"
	DeleteDomainRoutePath     = "/api/v1/route/delete"
	QueryDomainRoutePath      = "/api/v1/route/query"
	BatchQueryDomainRoutePath = "/api/v1/route/status/batchQuery"
	// Domain Data
	CreateDomainDataPath     = "/api/v1/domaindata/create"
	UpdateDomainDataPath     = "/api/v1/domaindata/update"
	DeleteDomainDataPath     = "/api/v1/domaindata/delete"
	QueryDomainDataPath      = "/api/v1/domaindata/query"
	BatchQueryDomainDataPath = "/api/v1/domaindata/batchQuery"
	ListDomainDataPath       = "/api/v1/domaindata/list"
	// Kuscia Job
	CreateJobPath     = "/api/v1/job/create"
	DeleteJobPath     = "/api/v1/job/delete"
	QueryJobPath      = "/api/v1/job/query"
	StopJobPath       = "/api/v1/job/stop"
	SuspendJobPath    = "/api/v1/job/suspend"
	RestartJobPath    = "/api/v1/job/restart"
	CancelJobPath     = "/api/v1/job/cancel"
	ApproveJobPath    = "/api/v1/job/approve"
	BatchQueryJobPath = "/api/v1/job/status/batchQuery"
	WatchJobPath      = "/api/v1/job/watch"
	// Kuscia Serving
	CreateServingPath     = "/api/v1/serving/create"
	UpdateServingPath     = "/api/v1/serving/update"
	DeleteServingPath     = "/api/v1/serving/delete"
	QueryServingPath      = "/api/v1/serving/query"
	BatchQueryServingPath = "/api/v1/serving/status/batchQuery"
	// KusciaAPIHostName kuscia API service name
	KusciaAPIHostName = "kusciaapi.master.svc"
)

type KusciaAPIClient interface {
	Send(ctx context.Context, request proto.Message, response proto.Message, path string) error

	UpdateDomain(ctx context.Context, request *kusciaapi.UpdateDomainRequest) (response *kusciaapi.UpdateDomainResponse, err error)

	QueryDomain(ctx context.Context, request *kusciaapi.QueryDomainRequest) (response *kusciaapi.QueryDomainResponse, err error)

	BatchQueryDomain(ctx context.Context, request *kusciaapi.BatchQueryDomainRequest) (response *kusciaapi.BatchQueryDomainResponse, err error)

	CreateDomainRoute(ctx context.Context, request *kusciaapi.CreateDomainRouteRequest) (response *kusciaapi.CreateDomainRouteResponse, err error)

	DeleteDomainRoute(ctx context.Context, request *kusciaapi.DeleteDomainRouteRequest) (response *kusciaapi.DeleteDomainRouteResponse, err error)

	QueryDomainRoute(ctx context.Context, request *kusciaapi.QueryDomainRouteRequest) (response *kusciaapi.QueryDomainRouteResponse, err error)

	BatchQueryDomainRoute(ctx context.Context, request *kusciaapi.BatchQueryDomainRouteStatusRequest) (response *kusciaapi.BatchQueryDomainRouteStatusResponse, err error)

	CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) (response *kusciaapi.CreateDomainDataResponse, err error)

	UpdateDomainData(ctx context.Context, request *kusciaapi.UpdateDomainDataRequest) (response *kusciaapi.UpdateDomainDataResponse, err error)

	DeleteDomainData(ctx context.Context, request *kusciaapi.DeleteDomainDataRequest) (response *kusciaapi.DeleteDomainDataResponse, err error)

	QueryDomainData(ctx context.Context, request *kusciaapi.QueryDomainDataRequest) (response *kusciaapi.QueryDomainDataResponse, err error)

	BatchQueryDomainData(ctx context.Context, request *kusciaapi.BatchQueryDomainDataRequest) (response *kusciaapi.BatchQueryDomainDataResponse, err error)

	ListDomainData(ctx context.Context, request *kusciaapi.ListDomainDataRequest) (response *kusciaapi.ListDomainDataResponse, err error)

	CreateJob(ctx context.Context, request *kusciaapi.CreateJobRequest) (response *kusciaapi.CreateJobResponse, err error)

	DeleteJob(ctx context.Context, request *kusciaapi.DeleteJobRequest) (response *kusciaapi.DeleteJobResponse, err error)

	QueryJob(ctx context.Context, request *kusciaapi.QueryJobRequest) (response *kusciaapi.QueryJobResponse, err error)

	StopJob(ctx context.Context, request *kusciaapi.StopJobRequest) (response *kusciaapi.StopJobResponse, err error)

	SuspendJob(ctx context.Context, request *kusciaapi.SuspendJobRequest) (response *kusciaapi.SuspendJobResponse, err error)

	RestartJob(ctx context.Context, request *kusciaapi.RestartJobRequest) (response *kusciaapi.RestartJobResponse, err error)

	CancelJob(ctx context.Context, request *kusciaapi.CancelJobRequest) (response *kusciaapi.CancelJobResponse, err error)

	BatchQueryJob(ctx context.Context, request *kusciaapi.BatchQueryJobStatusRequest) (response *kusciaapi.BatchQueryJobStatusResponse, err error)

	WatchJob(ctx context.Context, request *kusciaapi.WatchJobRequest, eventCh chan<- *kusciaapi.WatchJobEventResponse) error

	CreateServing(ctx context.Context, request *kusciaapi.CreateServingRequest) (response *kusciaapi.CreateServingResponse, err error)

	UpdateServing(ctx context.Context, request *kusciaapi.UpdateServingRequest) (response *kusciaapi.UpdateServingResponse, err error)

	DeleteServing(ctx context.Context, request *kusciaapi.DeleteServingRequest) (response *kusciaapi.DeleteServingResponse, err error)

	QueryServing(ctx context.Context, request *kusciaapi.QueryServingRequest) (response *kusciaapi.QueryServingResponse, err error)

	BatchQueryServing(ctx context.Context, request *kusciaapi.BatchQueryServingStatusRequest) (response *kusciaapi.BatchQueryServingStatusResponse, err error)

	ApproveJob(ctx context.Context, request *kusciaapi.ApproveJobRequest) (response *kusciaapi.ApproveJobResponse, err error)
}

func NewKusciaAPIClient(endpoint string) KusciaAPIClient {
	if endpoint == "" {
		endpoint = KusciaAPIHostName
	}
	return &KusciaAPIHttpClient{
		HostName: endpoint,
		Client:   &http.Client{},
	}
}

type KusciaAPIHttpClient struct {
	HostName string
	Client   *http.Client
}

func (c *KusciaAPIHttpClient) UpdateDomain(ctx context.Context, request *kusciaapi.UpdateDomainRequest) (response *kusciaapi.UpdateDomainResponse, err error) {
	response = &kusciaapi.UpdateDomainResponse{}
	err = c.Send(ctx, request, response, UpdateDomainPath)
	return
}

func (c *KusciaAPIHttpClient) QueryDomain(ctx context.Context, request *kusciaapi.QueryDomainRequest) (response *kusciaapi.QueryDomainResponse, err error) {
	response = &kusciaapi.QueryDomainResponse{}
	err = c.Send(ctx, request, response, QueryDomainPath)
	return
}

func (c *KusciaAPIHttpClient) BatchQueryDomain(ctx context.Context, request *kusciaapi.BatchQueryDomainRequest) (response *kusciaapi.BatchQueryDomainResponse, err error) {
	response = &kusciaapi.BatchQueryDomainResponse{}
	err = c.Send(ctx, request, response, BatchQueryDomainPath)
	return
}
func (c *KusciaAPIHttpClient) CreateDomainRoute(ctx context.Context, request *kusciaapi.CreateDomainRouteRequest) (response *kusciaapi.CreateDomainRouteResponse, err error) {
	response = &kusciaapi.CreateDomainRouteResponse{}
	err = c.Send(ctx, request, response, CreateDomainRoutePath)
	return
}

func (c *KusciaAPIHttpClient) DeleteDomainRoute(ctx context.Context, request *kusciaapi.DeleteDomainRouteRequest) (response *kusciaapi.DeleteDomainRouteResponse, err error) {
	response = &kusciaapi.DeleteDomainRouteResponse{}
	err = c.Send(ctx, request, response, DeleteDomainRoutePath)
	return
}

func (c *KusciaAPIHttpClient) QueryDomainRoute(ctx context.Context, request *kusciaapi.QueryDomainRouteRequest) (response *kusciaapi.QueryDomainRouteResponse, err error) {
	response = &kusciaapi.QueryDomainRouteResponse{}
	err = c.Send(ctx, request, response, QueryDomainRoutePath)
	return
}

func (c *KusciaAPIHttpClient) BatchQueryDomainRoute(ctx context.Context, request *kusciaapi.BatchQueryDomainRouteStatusRequest) (response *kusciaapi.BatchQueryDomainRouteStatusResponse, err error) {
	response = &kusciaapi.BatchQueryDomainRouteStatusResponse{}
	err = c.Send(ctx, request, response, BatchQueryDomainRoutePath)
	return
}

func (c *KusciaAPIHttpClient) CreateDomainData(ctx context.Context, request *kusciaapi.CreateDomainDataRequest) (response *kusciaapi.CreateDomainDataResponse, err error) {
	response = &kusciaapi.CreateDomainDataResponse{}
	err = c.Send(ctx, request, response, CreateDomainDataPath)
	return
}

func (c *KusciaAPIHttpClient) UpdateDomainData(ctx context.Context, request *kusciaapi.UpdateDomainDataRequest) (response *kusciaapi.UpdateDomainDataResponse, err error) {
	response = &kusciaapi.UpdateDomainDataResponse{}
	err = c.Send(ctx, request, response, UpdateDomainDataPath)
	return
}

func (c *KusciaAPIHttpClient) DeleteDomainData(ctx context.Context, request *kusciaapi.DeleteDomainDataRequest) (response *kusciaapi.DeleteDomainDataResponse, err error) {
	response = &kusciaapi.DeleteDomainDataResponse{}
	err = c.Send(ctx, request, response, DeleteDomainDataPath)
	return
}

func (c *KusciaAPIHttpClient) QueryDomainData(ctx context.Context, request *kusciaapi.QueryDomainDataRequest) (response *kusciaapi.QueryDomainDataResponse, err error) {
	response = &kusciaapi.QueryDomainDataResponse{}
	err = c.Send(ctx, request, response, QueryDomainDataPath)
	return
}

func (c *KusciaAPIHttpClient) BatchQueryDomainData(ctx context.Context, request *kusciaapi.BatchQueryDomainDataRequest) (response *kusciaapi.BatchQueryDomainDataResponse, err error) {
	response = &kusciaapi.BatchQueryDomainDataResponse{}
	err = c.Send(ctx, request, response, BatchQueryDomainDataPath)
	return
}

func (c *KusciaAPIHttpClient) ListDomainData(ctx context.Context, request *kusciaapi.ListDomainDataRequest) (response *kusciaapi.ListDomainDataResponse, err error) {
	response = &kusciaapi.ListDomainDataResponse{}
	err = c.Send(ctx, request, response, ListDomainDataPath)
	return
}

func (c *KusciaAPIHttpClient) CreateJob(ctx context.Context, request *kusciaapi.CreateJobRequest) (response *kusciaapi.CreateJobResponse, err error) {
	response = &kusciaapi.CreateJobResponse{}
	err = c.Send(ctx, request, response, CreateJobPath)
	return
}

func (c *KusciaAPIHttpClient) DeleteJob(ctx context.Context, request *kusciaapi.DeleteJobRequest) (response *kusciaapi.DeleteJobResponse, err error) {
	response = &kusciaapi.DeleteJobResponse{}
	err = c.Send(ctx, request, response, DeleteJobPath)
	return
}

func (c *KusciaAPIHttpClient) QueryJob(ctx context.Context, request *kusciaapi.QueryJobRequest) (response *kusciaapi.QueryJobResponse, err error) {
	response = &kusciaapi.QueryJobResponse{}
	err = c.Send(ctx, request, response, QueryJobPath)
	return
}

func (c *KusciaAPIHttpClient) StopJob(ctx context.Context, request *kusciaapi.StopJobRequest) (response *kusciaapi.StopJobResponse, err error) {
	response = &kusciaapi.StopJobResponse{}
	err = c.Send(ctx, request, response, StopJobPath)
	return
}

func (c *KusciaAPIHttpClient) SuspendJob(ctx context.Context, request *kusciaapi.SuspendJobRequest) (response *kusciaapi.SuspendJobResponse, err error) {
	response = &kusciaapi.SuspendJobResponse{}
	err = c.Send(ctx, request, response, SuspendJobPath)
	return
}

func (c *KusciaAPIHttpClient) RestartJob(ctx context.Context, request *kusciaapi.RestartJobRequest) (response *kusciaapi.RestartJobResponse, err error) {
	response = &kusciaapi.RestartJobResponse{}
	err = c.Send(ctx, request, response, RestartJobPath)
	return
}

func (c *KusciaAPIHttpClient) CancelJob(ctx context.Context, request *kusciaapi.CancelJobRequest) (response *kusciaapi.CancelJobResponse, err error) {
	response = &kusciaapi.CancelJobResponse{}
	err = c.Send(ctx, request, response, CancelJobPath)
	return
}

func (c *KusciaAPIHttpClient) ApproveJob(ctx context.Context, request *kusciaapi.ApproveJobRequest) (response *kusciaapi.ApproveJobResponse, err error) {
	response = &kusciaapi.ApproveJobResponse{}
	err = c.Send(ctx, request, response, ApproveJobPath)
	return
}

func (c *KusciaAPIHttpClient) BatchQueryJob(ctx context.Context, request *kusciaapi.BatchQueryJobStatusRequest) (response *kusciaapi.BatchQueryJobStatusResponse, err error) {
	response = &kusciaapi.BatchQueryJobStatusResponse{}
	err = c.Send(ctx, request, response, BatchQueryJobPath)
	return
}

func (c *KusciaAPIHttpClient) WatchJob(ctx context.Context, request *kusciaapi.WatchJobRequest, eventCh chan<- *kusciaapi.WatchJobEventResponse) error {
	// construct http request
	byteReq, err := proto.Marshal(request)
	if err != nil {
		nlog.Errorf("Send request %+v ,marshal request failed: %s", request.String(), err.Error())
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.getURL(WatchJobPath), bytes.NewReader(byteReq))
	if err != nil {
		return err
	}
	// set header
	req.Host = c.HostName
	req.Header.Set("Content-Type", binding.MIMEPROTOBUF)
	// send request
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// check http status code
	if resp.StatusCode != http.StatusOK {
		nlog.Errorf("Send watch request %+v failed, http code: %d", request.String(), resp.StatusCode)
		return fmt.Errorf("unexpected error, status_code: '%d'", resp.StatusCode)
	}
	// decode response
	decoder := json.NewDecoder(resp.Body)
	// Loop: parse the stream response
	for {
		select {
		case <-ctx.Done():
			nlog.Warnf("The watch context has canceled or timeout")
			return nil
		default:
			respOne := &kusciaapi.WatchJobEventResponse{}
			// parse the next response
			err := decoder.Decode(&respOne)
			if err != nil {
				if err.Error() == "EOF" {
					nlog.Warnf("The watch stream has finished")
					return nil
				}
				nlog.Errorf("Decoding response to JSON failed, error : %s.", err.Error())
				return err
			}
			// ignore the heartbeat event
			if respOne.Type == kusciaapi.EventType_HEARTBEAT {
				continue
			}
			nlog.Debugf("Watch job: %+v", respOne)
			// send a response to channel
			eventCh <- respOne
		}
	}
}

func (c *KusciaAPIHttpClient) CreateServing(ctx context.Context, request *kusciaapi.CreateServingRequest) (response *kusciaapi.CreateServingResponse, err error) {
	response = &kusciaapi.CreateServingResponse{}
	err = c.Send(ctx, request, response, CreateServingPath)
	return
}

func (c *KusciaAPIHttpClient) UpdateServing(ctx context.Context, request *kusciaapi.UpdateServingRequest) (response *kusciaapi.UpdateServingResponse, err error) {
	response = &kusciaapi.UpdateServingResponse{}
	err = c.Send(ctx, request, response, UpdateServingPath)
	return
}

func (c *KusciaAPIHttpClient) DeleteServing(ctx context.Context, request *kusciaapi.DeleteServingRequest) (response *kusciaapi.DeleteServingResponse, err error) {
	response = &kusciaapi.DeleteServingResponse{}
	err = c.Send(ctx, request, response, DeleteServingPath)
	return
}

func (c *KusciaAPIHttpClient) QueryServing(ctx context.Context, request *kusciaapi.QueryServingRequest) (response *kusciaapi.QueryServingResponse, err error) {
	response = &kusciaapi.QueryServingResponse{}
	err = c.Send(ctx, request, response, QueryServingPath)
	return
}

func (c *KusciaAPIHttpClient) BatchQueryServing(ctx context.Context, request *kusciaapi.BatchQueryServingStatusRequest) (response *kusciaapi.BatchQueryServingStatusResponse, err error) {
	response = &kusciaapi.BatchQueryServingStatusResponse{}
	err = c.Send(ctx, request, response, BatchQueryServingPath)
	return
}

func (c *KusciaAPIHttpClient) Send(ctx context.Context, request proto.Message, response proto.Message, path string) error {
	byteReq, err := proto.Marshal(request)
	if err != nil {
		nlog.Errorf("Send request %+v ,marshal request failed: %s", request, err.Error())
		return err
	}
	byteBody, err := c.sendProtoRequest(ctx, path, byteReq)
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

func (c *KusciaAPIHttpClient) sendProtoRequest(ctx context.Context, path string, request []byte) ([]byte, error) {
	// construct http request
	req, err := http.NewRequest(http.MethodPost, c.getURL(path), bytes.NewReader(request))
	if err != nil {
		return nil, err
	}
	// set header
	req.Host = c.HostName
	req.Header.Set("Content-Type", binding.MIMEPROTOBUF)
	// send request
	resp, err := c.Client.Do(req)
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

func (c *KusciaAPIHttpClient) getURL(path string) string {
	address := c.HostName
	if !strings.HasPrefix(address, "http") {
		address = fmt.Sprintf("http://%s", address)
	}
	return fmt.Sprintf("%s%s", address, path)
}
