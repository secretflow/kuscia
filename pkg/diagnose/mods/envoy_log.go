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

package mods

import (
	"context"
	"fmt"
	"strings"

	ksuciacommon "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

type EnvoyLog interface {
	TaskAnalysis(ctx context.Context) error
}

type EnvoyLogTaskAnalysis struct {
	reporter *util.Reporter
	taskID   string
	domainID string
	svcName  string
	rootDir  string
	runMode  ksuciacommon.RunModeType
}

func NewEnvoyLogTaskAnalysis(reporter *util.Reporter, taskID, domainID, svcName, rootDir string, runMode ksuciacommon.RunModeType) EnvoyLog {
	return &EnvoyLogTaskAnalysis{
		reporter: reporter,
		taskID:   taskID,
		domainID: domainID,
		svcName:  svcName,
		rootDir:  rootDir,
		runMode:  runMode,
	}
}

func (e *EnvoyLogTaskAnalysis) TaskAnalysis(ctx context.Context) error {
	nlog.Infof("Envoy log task <%s> analysis", e.taskID)
	taskAnalysis := netstat.NewTaskAnalysis(e.taskID, e.domainID, e.rootDir, e.svcName, e.runMode)
	envoyLogInfos, err := taskAnalysis.Analysis()
	if err != nil {
		return err
	}
	dataMap := e.dataParse(envoyLogInfos)
	if len(dataMap) == 0 {
		nlog.Infof("Envoy log task <%s> analysis result is no failed requests", e.taskID)
		return nil
	}
	for key, value := range dataMap {
		table := e.reporter.NewTableWriter()
		table.SetTitle(key)
		table.AddHeader([]string{"IP", "TIMESTAMP", "SERVICENAME", "INTERFACEADDR", "HTTPMETHOD", "TRACEID", "STATUSCODE", "CONTENTLENGTH", "REQUESTTIME"})
		for _, log := range value {
			table.AddRow(e.ToStringArray(log))
		}
	}
	return nil
}

func (e *EnvoyLogTaskAnalysis) dataParse(data []*diagnose.EnvoyLogInfoResponse) map[string][]*diagnose.EnvoyLog {

	result := make(map[string][]*diagnose.EnvoyLog)
	codeCountMap := make(map[string]int)
	for _, envoyLogInfo := range data {
		domainID := envoyLogInfo.DomainId
		for _, info := range envoyLogInfo.EnvoyInfoList {
			networkAccess := info.Type
			for _, log := range info.EnvoyLogList {
				if codeCountMap[log.StatusCode] >= common.CodeStatusMaxCount {
					continue
				}
				var key string
				if networkAccess == common.InternalTypeLog {
					// job's serviceName example: secretflow-task-20250528142451-single-psi-0-spu.alice.svc
					serviceNameSplit := strings.Split(log.ServiceName, ".")
					if len(serviceNameSplit) > 2 {
						destinationNode := serviceNameSplit[1]
						key = fmt.Sprintf("DOMAIN(%s) TASK(%s) NETWORK %s ---> %s", domainID, e.taskID, domainID, destinationNode)
					}
				} else if networkAccess == common.ExternalTypeLog {
					key = fmt.Sprintf("DOMAIN(%s) TASK(%s) NETWORK %s ---> %s", domainID, e.taskID, log.NodeName, domainID)
				}
				if key != "" {
					result[key] = append(result[key], log)
				}
				codeCountMap[log.StatusCode]++
			}
		}
	}

	return result
}

func (e *EnvoyLogTaskAnalysis) ToStringArray(info *diagnose.EnvoyLog) []string {
	return []string{
		info.Ip,
		info.Timestamp,
		info.ServiceName,
		info.InterfaceAddr,
		info.HttpMethod,
		info.TraceId,
		info.StatusCode,
		info.ContentLength,
		info.RequestTime,
	}
}
