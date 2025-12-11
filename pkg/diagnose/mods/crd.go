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
	"crypto/tls"
	"fmt"
	"net/http"

	"google.golang.org/grpc"

	"github.com/secretflow/kuscia/pkg/diagnose/common"
	util "github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/kusciaapi/constants"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type CRDItem struct {
	source      string
	destination string
	typ         string
}

type CRDMod struct {
	items         []*CRDItem
	kusciaAPIConn *grpc.ClientConn
	table         *util.Table
}

func NewCRDMod(crdItem []*CRDItem, reporter *util.Reporter, kusciaAPIConn *grpc.ClientConn) Mod {
	return &CRDMod{
		kusciaAPIConn: kusciaAPIConn,
		table:         reporter.NewTableWriter(),
		items:         crdItem,
	}
}

func (m *CRDMod) Run(ctx context.Context) error {
	nlog.Infof("diagnose crd config")
	m.table.SetTitle("CRD CONFIG CHECK:")
	m.table.AddHeader([]string{"NAME", "TYPE", "RESULT", "INFORMATION"})
	var ret error
	for _, item := range m.items {
		switch item.typ {
		case common.CRDDomainRoute:
			if err := m.ValidateClusterDomainRoute(ctx, item); err != nil {
				ret = err
			}
		// TODO: more crd types
		default:
			continue
		}
	}
	return ret
}

func (m *CRDMod) ValidateClusterDomainRoute(ctx context.Context, item *CRDItem) error {
	// check cdr token status
	req := &kusciaapi.QueryDomainRouteRequest{
		Source:      item.source,
		Destination: item.destination,
	}
	resp, err := m.QueryDomainRoute(ctx, req)
	if err != nil {
		var message string
		if resp != nil {
			message = fmt.Sprintf("invoke query api failed, %v", resp.Status.Message)
		} else {
			message = fmt.Sprintf("invoke query api failed, %v", err)
		}
		return m.OnFailure(item, message)
	}
	if resp.Status.Code != 0 {
		return m.OnFailure(item, fmt.Sprintf("query cdr failed, code:%v, message:%v", resp.Status.Code, resp.Status.Message))
	}
	if resp.Data.Status.Status != constants.RouteSucceeded {
		return m.OnFailure(item, fmt.Sprintf("cdr %v-%v status not succeeded, reason: %v", item.source, item.destination, resp.Data.Status.Reason))
	}

	// check connection
	if err := m.CheckConnection(resp.Data, item); err != nil {
		return m.OnFailure(item, err.Error())
	}

	// TODO: check certificate?
	m.OnSuccess(item)
	return nil
}

func (m *CRDMod) OnSuccess(item *CRDItem) {
	m.table.AddRow([]string{
		fmt.Sprintf("%v-%v", item.source, item.destination),
		item.typ,
		common.Pass,
		"",
	})
}

func (m *CRDMod) OnFailure(item *CRDItem, message string) error {
	m.table.AddRow([]string{
		fmt.Sprintf("%v-%v", item.source, item.destination),
		item.typ,
		common.Fail,
		message,
	})
	return fmt.Errorf("%s", message)
}

func (m *CRDMod) CheckConnection(domainroute *kusciaapi.QueryDomainRouteResponseData, item *CRDItem) error {
	// don't check domainroute synced by peer
	if len(domainroute.Endpoint.Ports) == 0 {
		return nil
	}
	var url string
	if domainroute.Endpoint.Ports[0].IsTLS {
		url = fmt.Sprintf("https://%s:%d%s", domainroute.Endpoint.Host, domainroute.Endpoint.Ports[0].Port, domainroute.Endpoint.Ports[0].PathPrefix)
	} else {
		url = fmt.Sprintf("http://%s:%d%s", domainroute.Endpoint.Host, domainroute.Endpoint.Ports[0].Port, domainroute.Endpoint.Ports[0].PathPrefix)
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // skip cert validation
			},
		},
	}
	resp, err := client.Get(url)
	// check http code 401 or 200
	if err != nil {
		return fmt.Errorf("fail to http get %s, err: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("fail to http get %s, status code: %v, kuscia error message: %v", url, resp.StatusCode, resp.Header.Get(utils.KusciaEnvoyMsgHeaderKey))
	}
	return nil
}

func (m *CRDMod) QueryDomainRoute(ctx context.Context, req *kusciaapi.QueryDomainRouteRequest) (*kusciaapi.QueryDomainRouteResponse, error) {
	return kusciaapi.NewDomainRouteServiceClient(m.kusciaAPIConn).QueryDomainRoute(ctx, req)
}
