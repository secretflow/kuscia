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

package netstat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/app/server"
	dcommon "github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
	"google.golang.org/grpc"
)

type Task interface {
	Run(ctx context.Context)
	Output() *TaskOutput
	Name() string
}

type TaskOutput diagnose.MetricItem

func (t *TaskOutput) ToStringArray() []string {
	return []string{t.Name, t.DetectedValue, t.Threshold, t.Result, t.Information}
}

func OutputsToMetricItems(elements []*TaskOutput) []*diagnose.MetricItem {
	items := make([]*diagnose.MetricItem, 0)
	for _, element := range elements {
		items = append(items, (*diagnose.MetricItem)(element))
	}
	return items
}

func MetricItemsToOutputs(elements []*diagnose.MetricItem) []*TaskOutput {
	items := make([]*TaskOutput, 0)
	for _, element := range elements {
		items = append(items, (*TaskOutput)(element))
	}
	return items
}

type TaskGroup struct {
	tasks            []Task
	diagnoseClient   *client.Client
	diagnoseServer   *server.HTTPServerBean
	DatameshConn     *grpc.ClientConn
	SelfDomainDataID string
	PeerDomainDataID string
	JobConfig        *NetworkJobConfig
}

type NetworkJobConfig struct {
	ServerPort   int
	SelfDomain   string
	SelfEndpoint string
	PeerDomain   string `json:"peer_domain"`
	PeerEndpoint string `json:"peer_endpoint"`
	Manual       bool   `json:"manual"`
	JobID        string `json:"job_id"`
	*NetworkParam
}

type NetworkParam struct {
	Pass              bool `json:"pass"`
	Speed             bool `json:"speed"`
	SpeedThres        int  `json:"speed_thres"`
	RTT               bool `json:"rtt"`
	RTTTres           int  `json:"rtt_thres"`
	ProxyTimeout      bool `json:"proxy_timeout"`
	ProxyTimeoutThres int  `json:"proxy_timeout_thres"`
	Size              bool `json:"size"`
	SizeThres         int  `json:"size_thres"`
	ProxyBuffer       bool `json:"proxy_buffer"`
	Bidirection       bool `json:"bi_direction"`
}

func NewTaskGroup(server *server.HTTPServerBean, cli *client.Client, config *NetworkJobConfig) *TaskGroup {
	tg := new(TaskGroup)
	tg.diagnoseServer = server
	tg.diagnoseClient = cli

	var err error
	tg.DatameshConn, err = client.NewDatameshConn()
	if err != nil {
		nlog.Fatalf("init datamesh conn failed, %v", err)
	}
	tg.SelfDomainDataID = dcommon.GenerateDomainDataID(config.JobID, config.SelfDomain)
	tg.PeerDomainDataID = dcommon.GenerateDomainDataID(config.JobID, config.PeerDomain)
	tg.JobConfig = config

	tg.tasks = append(tg.tasks, NewConnectionTask(tg.diagnoseClient))

	if config.Speed {
		tg.tasks = append(tg.tasks, NewBandWidthTask(tg.diagnoseClient, config.SpeedThres))
	}
	if config.RTT {
		tg.tasks = append(tg.tasks, NewLatencyTask(tg.diagnoseClient, config.RTTTres))
	}
	if config.ProxyTimeout {
		tg.tasks = append(tg.tasks, NewProxyTimeoutTask(tg.diagnoseClient, config.ProxyTimeoutThres))
	}
	if config.Size {
		tg.tasks = append(tg.tasks, NewReqSizeTask(tg.diagnoseClient, config.SizeThres))
	}
	if config.ProxyBuffer {
		tg.tasks = append(tg.tasks, NewBufferTask(tg.diagnoseClient))
	}
	return tg
}

func (tg *TaskGroup) Start(ctx context.Context) error {
	// validate connection task first
	results := make([]*TaskOutput, 0)
	for i, task := range tg.tasks {
		task.Run(ctx)
		results = append(results, task.Output())
		if i == 0 && task.Output().Result == dcommon.Fail {
			break
		}
	}

	// save self report in local domaindata
	nlog.Infof("Save self report")
	if err := tg.SaveReport(ctx, tg.SelfDomainDataID, results); err != nil {
		return err
	}

	// send report to peer and wait report from peer in bidirection mode
	if tg.JobConfig.Bidirection {
		nlog.Infof("Send report to peer")
		tg.SendReportToPeer(ctx, results)
		nlog.Infof("Wait report from peer")
		tg.RecvReportFromPeer(ctx)
	}
	return nil
}

func (tg *TaskGroup) RecvReportFromPeer(ctx context.Context) {
	report := tg.diagnoseServer.WaitReport(ctx)
	if report == nil {
		nlog.Warnf("Fail to receive report from peer")
	} else {
		if err := tg.SaveReport(ctx, tg.PeerDomainDataID, MetricItemsToOutputs(report)); err != nil {
			nlog.Warnf("Fail to save peer report")
		}
	}
}

func (tg *TaskGroup) SendReportToPeer(ctx context.Context, outputs []*TaskOutput) {
	items := OutputsToMetricItems(outputs)
	submitReportReq := new(diagnose.SubmitReportRequest)
	submitReportReq.Items = items
	if _, err := tg.diagnoseClient.SubmitReport(ctx, submitReportReq); err != nil {
		nlog.Warnf("Fail tot submit report, %v", err)
	}
}

func (tg *TaskGroup) SaveReport(ctx context.Context, ID string, items []*TaskOutput) error {
	req := tg.buildCreateDomainDataRequest(ID, items)
	resp, err := datamesh.NewDomainDataServiceClient(tg.DatameshConn).CreateDomainData(ctx, req)
	// resp, err := tg.DatameshClient.CreateDomainData(ctx, tg.buildCreateDomainDataRequest(ID, items))
	if err != nil {
		nlog.Errorf("Create domaindata failed, %v", err)
		return err
	} else if resp.Status != nil && resp.Status.Code != 0 {
		nlog.Errorf("Create domaindata failed, code: %v, message:%v", resp.Status.Code, resp.Status.Message)
		return fmt.Errorf("create domaindata failed, code:%v, message:%v", resp.Status.Code, resp.Status.Message)
	}
	return nil
}

func (tg *TaskGroup) buildCreateDomainDataRequest(ID string, items []*TaskOutput) *datamesh.CreateDomainDataRequest {
	results := make(map[string]string)
	for _, item := range items {
		outJSON, _ := json.Marshal(item)
		results[item.Name] = string(outJSON)
	}
	return &datamesh.CreateDomainDataRequest{
		DomaindataId: ID,
		Name:         ID,
		Type:         "unknown",
		RelativeUri:  "N/A",
		Attributes:   results,
	}
}
