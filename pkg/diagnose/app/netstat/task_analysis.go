// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package netstat

import (
	"context"
	"fmt"
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/diagnose/app/client"
	"github.com/secretflow/kuscia/pkg/diagnose/app/server"
	dcommon "github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

type TaskAnalysis struct {
	taskID   string
	domainID string
	rootDir  string
	svcName  string
	runMode  common.RunModeType
}

func NewTaskAnalysis(taskID string, domainID, rootDir, svcName string, runMode common.RunModeType) *TaskAnalysis {
	return &TaskAnalysis{
		taskID:   taskID,
		domainID: domainID,
		rootDir:  rootDir,
		svcName:  svcName,
		runMode:  runMode,
	}
}

func (t *TaskAnalysis) Analysis() ([]*diagnose.EnvoyLogInfoResponse, error) {
	partyDomainIDs, createTime, err := t.getTaskPartyDomainInfo()
	if err != nil {
		return nil, fmt.Errorf("get party domain id failed, err: %v", err)
	}
	parseTime, err := time.Parse(dcommon.LogTimeFormat, createTime)
	if err != nil {
		return nil, fmt.Errorf("parse create time failed, err: %v", err)
	}
	parseTime = utils.CSTTimeCovertToUTC(parseTime)
	var result []*diagnose.EnvoyLogInfoResponse
	for _, partyDomainID := range partyDomainIDs {
		// get owner envoy log info
		if partyDomainID == t.domainID {
			envoyLogInfos, err := utils.GetLogAnalysisResult(t.taskID, t.rootDir, &parseTime)
			if err != nil {
				return nil, fmt.Errorf("get envoy log info failed, err: %v", err)
			}
			result = append(result, &diagnose.EnvoyLogInfoResponse{
				DomainId:      partyDomainID,
				EnvoyInfoList: envoyLogInfos,
			})
		} else {
			// get other envoy log info
			envoyLogInfos, err := t.getOtherDomainLogEntries(partyDomainID, t.taskID, &parseTime)
			if err != nil {
				return nil, fmt.Errorf("get other domain:%s log entries failed, err: %v", partyDomainID, err)
			}
			result = append(result, &diagnose.EnvoyLogInfoResponse{
				DomainId:      partyDomainID,
				EnvoyInfoList: envoyLogInfos.EnvoyInfoList,
			})
		}

	}

	return result, nil
}

func (t *TaskAnalysis) getTaskPartyDomainInfo() ([]string, string, error) {
	var partyDomainID []string
	svcName := fmt.Sprintf("%s:%d", constants.LocalhostIP, server.DIAGNOSE_SERVER_PORT)
	if t.runMode == common.RunModeLite {
		svcName = fmt.Sprintf("diagnose.%s.svc", dcommon.MasterSvcName)
	}
	cli := client.NewDiagnoseClient(svcName)
	response, err := cli.Task(context.Background(), &diagnose.TaskInfoRequest{TaskId: t.taskID})
	if err != nil {
		return nil, "", fmt.Errorf("get task info failed, err: %v", err)
	}
	if response.CreateTime == "" {
		return nil, "", fmt.Errorf("kuscia task is not exist")
	}
	for _, party := range response.Parties {
		partyDomainID = append(partyDomainID, party.DomainId)
	}
	return partyDomainID, response.CreateTime, nil
}

func (t *TaskAnalysis) getOtherDomainLogEntries(domainId, taskId string, createTime *time.Time) (*diagnose.EnvoyLogInfoResponse, error) {

	timeStr := createTime.Format(dcommon.LogTimeFormat)
	svcName := fmt.Sprintf("diagnose.%s.svc", domainId)
	if t.svcName != "" {
		svcName = t.svcName
	}
	cli := client.NewDiagnoseClient(svcName)
	response, err := cli.EnvoyLog(context.Background(), &diagnose.EnvoyLogRequest{TaskId: taskId, CreateTime: timeStr})
	if err != nil {
		return nil, fmt.Errorf("get other domain log entries failed, err: %v", err)
	}
	return response, nil
}
