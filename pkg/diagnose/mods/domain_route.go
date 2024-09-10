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

package mods

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
	util_common "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"google.golang.org/grpc"
)

type DomainRouteMod struct {
	source        string
	destination   string
	peerEndpoint  string
	id            string
	manual        bool
	config        *netstat.NetworkParam
	kusciaAPIConn *grpc.ClientConn
	reporter      *util.Reporter
	crdMod        Mod
}

func NewDomainRouteMod(reporter *util.Reporter, kusciaAPIConn *grpc.ClientConn, config *DiagnoseConfig) Mod {
	items := []*CRDItem{
		{
			source:      config.Source,
			destination: config.Destination,
			typ:         common.CRDDomainRoute,
		},
	}
	if config.Bidirection {
		items = append(items, &CRDItem{
			source:      config.Destination,
			destination: config.Source,
			typ:         common.CRDDomainRoute,
		})
	}
	crdMod := NewCRDMod(items, reporter, kusciaAPIConn)

	return &DomainRouteMod{
		source:        config.Source,
		destination:   config.Destination,
		peerEndpoint:  config.PeerEndpoint,
		manual:        config.Manual,
		config:        &config.NetworkParam,
		reporter:      reporter,
		kusciaAPIConn: kusciaAPIConn,
		crdMod:        crdMod,
	}
}

func (m *DomainRouteMod) RunManualMode(ctx context.Context, peerEndpoint string) error {
	// create single-party job
	m.id = m.generateJobID(true)
	req := m.buildCreateJobRequest(true, peerEndpoint)
	if err := m.CreateJob(ctx, req); err != nil {
		return err
	}

	if peerEndpoint == "" { // need peer to create job manually
		// wait job start
		if err := m.WaitJobStart(ctx); err != nil {
			nlog.Infof("Fail to start manual job <%v> , %v", m.id, err)
			return err
		}

		// display
		m.DisplayManualInstruction()
	}

	// wait job done
	PrintToConsole("waiting diagnose job <%s> done, which may take several miniutes...\n", m.id)
	enableTimeout := peerEndpoint != ""
	if err := m.WaitJobDone(ctx, enableTimeout); err != nil {
		nlog.Errorf("Fail to wait job done, %v", err)
		return err
	}
	return nil
}

func (m *DomainRouteMod) Run(ctx context.Context) error {
	if m.manual {
		return m.RunManualMode(ctx, m.peerEndpoint)
	}
	PrintToConsole("diagnose <%s-%s> network statitsics\n", m.source, m.destination)
	// crd config diagnose
	if err := m.crdMod.Run(ctx); err != nil {
		return err
	}
	// try creating double-party job
	m.id = m.generateJobID(false)
	req := m.buildCreateJobRequest(false, "")
	if err := m.CreateJob(ctx, req); err != nil {
		return err
	}

	// wait job sync
	PrintToConsole("waiting diagnose job <%s> syncronize to peer...\n", m.id)
	if err := m.WaitJobStart(ctx); err != nil {
		nlog.Infof("Fail to sync job to peer, err: %v, fallback to manual mode", err)
		// delete double party job
		m.DeleteJob()
		// run manual mode
		if err := m.RunManualMode(ctx, ""); err != nil {
			return err
		}
	} else {
		// wait job done
		PrintToConsole("waiting diagnose job <%s> done, which may take several miniutes...\n", m.id)
		if err := m.WaitJobDone(ctx, true); err != nil {
			nlog.Errorf("Fail to wait job done, %v", err)
			return err
		}
	}

	// query domaindata
	fmt.Println("query job result...")
	// load domain data to reporter
	if err := m.LoadDomainDataToReporter(ctx, true); err != nil {
		return err
	}
	if m.config.Bidirection {
		fmt.Println("query peer job result...")
		if err := m.LoadDomainDataToReporter(ctx, false); err != nil {
			return err
		}
	}
	return nil

}

func (m *DomainRouteMod) generateJobID(singleParty bool) string {
	if singleParty {
		return fmt.Sprintf("diagnose-%v-%v", m.source, util_common.GenerateID(10))
	}
	return fmt.Sprintf("diagnose-%v-%v-%v", m.source, m.destination, util_common.GenerateID(10))

}

func (m *DomainRouteMod) buildCreateJobRequest(manual bool, peerEndpoint string) *kusciaapi.CreateJobRequest {
	taskInputConfig := &netstat.NetworkJobConfig{
		JobID:        m.id,
		NetworkParam: m.config,
		Manual:       manual,
		PeerDomain:   m.destination,
		PeerEndpoint: peerEndpoint,
	}

	taskInputConfigStr, _ := json.Marshal(taskInputConfig)
	parties := []*kusciaapi.Party{{DomainId: m.source}}
	if !manual {
		parties = append(parties, &kusciaapi.Party{DomainId: m.destination})
	}
	req := &kusciaapi.CreateJobRequest{
		JobId:          m.id,
		Initiator:      m.source,
		MaxParallelism: 1,
		Tasks: []*kusciaapi.Task{
			{
				TaskId:          m.id,
				Alias:           "netstat",
				AppImage:        "diagnose-image",
				Priority:        100,
				TaskInputConfig: string(taskInputConfigStr),
				Parties:         parties,
			},
		},
	}
	return req
}

func (m *DomainRouteMod) buildQueryDomainDataRequest(domain string) *kusciaapi.QueryDomainDataRequest {
	return &kusciaapi.QueryDomainDataRequest{
		Data: &kusciaapi.QueryDomainDataRequestData{
			DomainId:     m.source,
			DomaindataId: fmt.Sprintf("%s-%s", m.id, domain),
		},
	}
}

func (m *DomainRouteMod) WaitJobStart(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	stop := time.After(time.Minute)
	for {
		select {
		case <-ticker.C:
			resp, err := kusciaapi.NewJobServiceClient(m.kusciaAPIConn).QueryJob(ctx, &kusciaapi.QueryJobRequest{JobId: m.id})
			if err != nil {
				nlog.Errorf("Query kuscia job %v failed, %v", m.id, err)
			} else if resp.Status != nil && resp.Status.Code != 0 {
				return fmt.Errorf("query kuscia job %v failed, code: %v, message: %v", m.id, resp.Status.Code, resp.Status.Message)
			} else if resp.Data != nil && resp.Data.Status != nil {
				jobState := kusciaapi.JobState_State_value[resp.Data.Status.State]
				switch jobState {
				case int32(kusciaapi.JobState_Succeeded), int32(kusciaapi.JobState_Running):
					return nil
				case int32(kusciaapi.JobState_Pending), int32(kusciaapi.JobState_AwaitingApproval):
				default:
					return fmt.Errorf("query kuscia job %v failed, job status: %v", m.id, resp.Data.Status.State)
				}
			}
		case <-stop:
			return fmt.Errorf("wait job start %v reach timeout", m.id)
		case <-ctx.Done():
			m.DeleteJob()
			return fmt.Errorf("wait job start %v receive context done", m.id)
		}
	}
}

func (m *DomainRouteMod) WaitJobDone(ctx context.Context, enableTimeout bool) error {
	var stop <-chan time.Time
	if enableTimeout {
		timeout := 120 // 2min
		if m.config.ProxyTimeout {
			timeout += m.config.ProxyTimeoutThres
		}
		stop = time.After(time.Duration(timeout) * time.Second)
	} else {
		stop = time.After(24 * time.Hour)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			resp, err := kusciaapi.NewJobServiceClient(m.kusciaAPIConn).QueryJob(ctx, &kusciaapi.QueryJobRequest{JobId: m.id})
			if err != nil {
				nlog.Errorf("Query kuscia job %v failed, %v", m.id, err)
			} else if resp.Status != nil && resp.Status.Code != 0 {
				return fmt.Errorf("query kuscia job %v failed, code: %v, message: %v", m.id, resp.Status.Code, resp.Status.Message)
			} else if resp.Data != nil && resp.Data.Status != nil {
				jobState := kusciaapi.JobState_State_value[resp.Data.Status.State]
				switch jobState {
				case int32(kusciaapi.JobState_Succeeded):
					return nil
				case int32(kusciaapi.JobState_Running):
				default:
					return fmt.Errorf("query kuscia job %v failed, job status: %v", m.id, resp.Data.Status.State)
				}
			}
		case <-stop:
			return fmt.Errorf("wait job %v reach timeout", m.id)
		case <-ctx.Done():
			m.DeleteJob()
			return fmt.Errorf("wait job %v receive context done", m.id)
		}
	}

}

func (m *DomainRouteMod) LoadDomainDataToReporter(ctx context.Context, self bool) error {
	domain := m.source
	if !self {
		domain = m.destination
	}
	req := m.buildQueryDomainDataRequest(domain)
	resp, err := m.QueryDomainData(ctx, req)
	if err != nil {
		return err
	}

	table := m.reporter.NewTableWriter()
	if self {
		table.SetTitle(fmt.Sprintf("NETWORK STATSTICS(%s-%s):", m.source, m.destination))
	} else {
		table.SetTitle(fmt.Sprintf("NETWORK STATSTICS(%s-%s):", m.destination, m.source))
	}
	table.AddHeader([]string{"NAME", "DETECTED VALUE", "THRESHOLD", "RESULT", "INFORMATION"})

	// sort keys
	keys := make([]string, 0, len(resp.Data.Attributes))
	for k := range resp.Data.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		taskOutput := new(netstat.TaskOutput)
		if err := json.Unmarshal([]byte(resp.Data.Attributes[k]), taskOutput); err != nil {
			return m.OnFailure(fmt.Sprintf("unmarshal attribute %v failed, err: %v", resp.Data.Attributes[k], err))
		}
		taskOutput.Name = k
		table.AddRow(taskOutput.ToStringArray())
	}
	return nil

}

func (m *DomainRouteMod) OnFailure(message string) error {
	nlog.Errorf(message)
	return fmt.Errorf(message)
}

func (m *DomainRouteMod) DisplayManualInstruction() {
	selfEndpoint := fmt.Sprintf("%s-0-test-server.%s.svc", m.id, m.source)
	PrintToConsole(`diagnose job can't syncronize from %s to %s, there might be some issues in your network. please enter the following command in %s's node to continue the diagnose procedure:

	kuscia diagnose network -m -e %s %s %s

`, m.source, m.destination, m.destination, selfEndpoint, m.destination, m.source)
}

func (m *DomainRouteMod) CreateJob(ctx context.Context, req *kusciaapi.CreateJobRequest) error {
	resp, err := kusciaapi.NewJobServiceClient(m.kusciaAPIConn).CreateJob(ctx, req)
	if err != nil {
		nlog.Errorf("Create kuscia job: invoke kusciaapi failed, %v", err)
		return err
	} else if resp.Status != nil && resp.Status.Code != 0 {
		nlog.Errorf("Create kuscia job: invoke kusciaapi failed, code: %v, message: %v", resp.Status.Code, resp.Status.Message)
		return err
	}
	return nil
}

func (m *DomainRouteMod) QueryDomainData(ctx context.Context, req *kusciaapi.QueryDomainDataRequest) (*kusciaapi.QueryDomainDataResponse, error) {
	resp, err := kusciaapi.NewDomainDataServiceClient(m.kusciaAPIConn).QueryDomainData(ctx, req)
	if err != nil {
		return nil, m.OnFailure(fmt.Sprintf("query domain data: invoke kusciaapi failed, %v", err))
	} else if resp.Status != nil && resp.Status.Code != 0 {
		return nil, m.OnFailure(fmt.Sprintf("query domain data: invoke kusciaapi failed, code: %v, message: %v", resp.Status.Code, resp.Status.Message))
	} else if resp.Data == nil {
		return nil, m.OnFailure("query domain data: invoke kusciaapi failed, no domaindata")
	}
	return resp, nil
}

func (m *DomainRouteMod) DeleteJob() {
	req := &kusciaapi.DeleteJobRequest{JobId: m.id}
	// use background context for deletion
	if _, err := kusciaapi.NewJobServiceClient(m.kusciaAPIConn).DeleteJob(context.Background(), req); err != nil {
		nlog.Warnf("Failed to delete job %s, %v", m.id, err)
	}
}
