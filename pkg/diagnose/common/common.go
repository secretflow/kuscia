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

package common

import "fmt"

const (
	DiagnoseNetworkGroup         = "diagnose/v1/network"
	DiagnoseMockPath             = "mock"
	DiagnoseRegisterEndpointPath = "register/endpoint"
	DiagnoseSubmitReportPath     = "submit/report"
	DiagnoseHealthyPath          = "healthy"
	DiagnoseGetEnvoyLogByTask    = "envoy/log/task"
	DiagnoseGetTaskInfo          = "task/info"
	DiagnoseDonePath             = "done"
)

const (
	Pass    = "[PASS]"
	Fail    = "[FAIL]"
	Warning = "[WARNING]"
)

const (
	CRDDomainRoute = "cdr"
)

const (
	KusciaYamlPath     = "etc/conf/kuscia.yaml"
	EnvoyLogPath       = "var/logs/envoy"
	InternalTypeLog    = "internal"
	ExternalTypeLog    = "external"
	LogTimeFormat      = "20060102-15"
	CodeStatusMaxCount = 10
	InternalRegexStr   = `^(?P<remote_address>[\d\.]+) - (?P<start_time>\[\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\]) (?P<kuscia_source>[^\s]*) (?P<kuscia_host>[^\s]*) "(?P<method>[A-Z]+) (?P<path>[^\s]+) (?P<protocol>[A-Z]+/[\d\.]+)" (?P<trace_id>[^\s]*) (?P<span_id>[^\s]*) (?P<response_code>\d{3}) (?P<response_flags>[^\s]*) (?P<content_length>[^\s]*) (?P<upstream_wire_bytes_received>[^\s]*) (?P<duration>[^\s]*) (?P<request_duration>[^\s]*) (?P<response_duration>[^\s]*) (?P<response_tx_duration>[^\s]*) (?P<request_body>[^\s]*) (?P<response_body>[^\s]*)$`
	ExternalRegexStr   = `^(?P<remote_address>[\d\.]+) - (?P<start_time>\[\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\]) (?P<kuscia_source>[^\s]*) (?P<kuscia_host>[^\s]*) "(?P<method>[A-Z]+) (?P<path>[^\s]+) (?P<protocol>[A-Z]+/[\d\.]+)" (?P<trace_id>[^\s]*) (?P<span_id>[^\s]*) (?P<response_code>\d{3}) (?P<response_flags>[^\s]*) (?P<content_length>[^\s]*) (?P<upstream_wire_bytes_received>[^\s]*) (?P<duration>[^\s]*) (?P<request_duration>[^\s]*) (?P<response_duration>[^\s]*)$`
	MasterSvcName      = "master"
)

func GenerateDomainDataID(jobID, party string) string {
	return fmt.Sprintf("%v-%v", jobID, party)
}
