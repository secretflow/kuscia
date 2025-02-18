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

package external

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const (
	dpRequestTimeout = time.Second * 10
)

type IDataProxyClient interface {
	GetFlightInfoDataMeshQuery(context.Context, *datamesh.CommandDataMeshQuery) (*flight.FlightInfo, error)
	GetFlightInfoDataMeshUpdate(context.Context, *datamesh.CommandDataMeshUpdate) (*flight.FlightInfo, error)
	GetFlightInfoDataMeshSqlQuery(context.Context, *datamesh.CommandDataMeshSqlQuery) (*flight.FlightInfo, error)
}

type DataProxyConfig struct {
	Addr            string
	ClientTLSConfig *kusciaconfig.TLSConfig
}

type DataProxyClient struct {
	conn         *grpc.ClientConn
	addr         string
	dialOpts     []grpc.DialOption
	flightClient flight.Client
}

func buildGrpcOptions(clientTLSConfig *tls.Config) []grpc.DialOption {
	dialOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                1 * time.Minute,
			Timeout:             30 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	if clientTLSConfig != nil {
		credentials := credentials.NewTLS(clientTLSConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return dialOpts
}

func NewDataProxyClient(conf *config.DataProxyConfig) (*DataProxyClient, error) {
	var (
		err             error
		conn            *grpc.ClientConn
		nativeTLSConfig *tls.Config
	)

	if conf.ClientTLSConfig != nil {
		if nativeTLSConfig, err = tlsutils.BuildClientTLSConfigViaPath(conf.ClientTLSConfig.CAFile,
			conf.ClientTLSConfig.CertFile, conf.ClientTLSConfig.KeyFile); err != nil {
			return nil, err
		}
	}

	dialOpts := buildGrpcOptions(nativeTLSConfig)
	if conn, err = grpc.Dial(conf.Endpoint, dialOpts...); err != nil {
		nlog.Errorf("create grpc conn to %s fail: %v", conf.Endpoint, err)
		return nil, err
	}

	flightClient := flight.NewClientFromConn(conn, nil)

	return &DataProxyClient{
		conn:         conn,
		addr:         conf.Endpoint,
		dialOpts:     dialOpts,
		flightClient: flightClient,
	}, nil
}

func generateDescriptor(query protoreflect.ProtoMessage) (*flight.FlightDescriptor, error) {
	dpDesc, err := utils.DescForCommand(query)
	if err != nil {
		nlog.Errorf("Generate Descriptor to dataproxy fail, %v", err)
		return nil, common.BuildGrpcErrorf(nil, codes.Internal, "Generate Descriptor to dataproxy fail")
	}
	return dpDesc, nil
}

func (dp *DataProxyClient) GetFlightInfoDataMeshQuery(ctx context.Context, query *datamesh.CommandDataMeshQuery) (*flight.FlightInfo, error) {
	dpDesc, err := generateDescriptor(query)
	if err != nil {
		return nil, err
	}
	dpCtx, cancel := context.WithTimeout(ctx, dpRequestTimeout)
	defer cancel()
	nlog.Infof("Get flightInfo from dataproxy begin, query domaindata: %+v", query.Domaindata)
	flightInfo, err := dp.flightClient.GetFlightInfo(dpCtx, dpDesc)
	if err != nil {
		nlog.Errorf("Get flightInfo from dataproxy failed, domaindata: %+v, error: %s", query.Domaindata, err.Error())
		return nil, err
	}
	nlog.Infof("Get flightInfo from dataproxy finish, flightInfo: %+v", flightInfo)
	if err = dp.complementFlightInfo(flightInfo); err != nil {
		return nil, common.BuildGrpcErrorf(nil, codes.Internal, "Complement flightInfo failed, error: %s.", err.Error())
	}
	return flightInfo, nil
}

func (dp *DataProxyClient) GetFlightInfoDataMeshUpdate(ctx context.Context, query *datamesh.CommandDataMeshUpdate) (*flight.FlightInfo, error) {
	dpDesc, err := generateDescriptor(query)
	if err != nil {
		return nil, err
	}
	dpCtx, cancel := context.WithTimeout(ctx, dpRequestTimeout)
	defer cancel()
	nlog.Infof("Get flightInfo from dataproxy begin, update domaindata: %+v", query.Domaindata)
	flightInfo, err := dp.flightClient.GetFlightInfo(dpCtx, dpDesc)
	if err != nil {
		nlog.Errorf("Get flightInfo from dataproxy failed, domaindata: %+v, error: %s", query.Domaindata, err.Error())
		return nil, err
	}
	nlog.Infof("Get flightInfo from dataproxy finish, flightInfo: %+v", flightInfo)
	if err = dp.complementFlightInfo(flightInfo); err != nil {
		return nil, common.BuildGrpcErrorf(nil, codes.Internal, "Complement flightInfo failed, error: %s.", err.Error())
	}
	return flightInfo, nil
}

func (dp *DataProxyClient) GetFlightInfoDataMeshSqlQuery(ctx context.Context, query *datamesh.CommandDataMeshSqlQuery) (*flight.FlightInfo, error) {
	dpDesc, err := generateDescriptor(query)
	if err != nil {
		return nil, err
	}
	dpCtx, cancel := context.WithTimeout(ctx, dpRequestTimeout)
	defer cancel()

	nlog.Infof("Get flightInfo from dataproxy begin, datasource: %+v", query.Datasource)
	flightInfo, err := dp.flightClient.GetFlightInfo(dpCtx, dpDesc)
	if err != nil {
		nlog.Errorf("Get flightInfo from dataproxy failed, datasource: %+v, error: %s", query.Datasource, err.Error())
		return nil, err
	}
	nlog.Infof("Get flightInfo from dataproxy finish, flightInfo: %+v", flightInfo)
	if err = dp.complementFlightInfo(flightInfo); err != nil {
		return nil, common.BuildGrpcErrorf(nil, codes.Internal, "Complement flightInfo failed, error: %s.", err.Error())
	}
	return flightInfo, nil
}

func (dp *DataProxyClient) complementFlightInfo(flightInfo *flight.FlightInfo) error {
	if len(flightInfo.Endpoint) == 0 {
		nlog.Errorf("FlightInfo's endpoints is nil, flightInfo detail: %+v", flightInfo)
		return errors.New("FlightInfo endpoints is nil")
	}
	if len(flightInfo.Endpoint[0].Location) == 0 {
		flightInfo.Endpoint[0].Location = append(flightInfo.Endpoint[0].Location,
			&flight.Location{
				Uri: dp.addr,
			})
	}
	return nil
}
