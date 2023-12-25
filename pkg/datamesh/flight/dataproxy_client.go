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

package flight

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const (
	dpRequestTimeout = time.Second * 180
)

type DataServer interface {
	GetFlightInfoDataMeshQuery(context.Context, *datamesh.CommandDataMeshQuery) (*flight.FlightInfo, error)
	GetFlightInfoDataMeshUpdate(context.Context, *datamesh.CommandDataMeshUpdate) (*flight.FlightInfo, error)
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

func NewDataProxyClient(conf *config.ExternalDataProxyConfig) (*DataProxyClient, error) {
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

func (dp *DataProxyClient) GetFlightInfoDataMeshQuery(ctx context.Context,
	query *datamesh.CommandDataMeshQuery) (*flight.FlightInfo, error) {
	dpDesc, err := DescForCommand(query)
	if err != nil {
		nlog.Errorf("Generate Descriptor to dataproxy fail, %v", err)
		return nil, buildGrpcErrorf(nil, codes.Internal, "Generate Descriptor to dataproxy fail")
	}

	dpCtx, cancel := context.WithTimeout(ctx, dpRequestTimeout)
	defer cancel()

	flightInfo, err := dp.flightClient.GetFlightInfo(dpCtx, dpDesc)
	if err != nil {
		return nil, err
	}

	return flightInfo, nil
}

func (dp *DataProxyClient) GetFlightInfoDataMeshUpdate(ctx context.Context,
	query *datamesh.CommandDataMeshUpdate) (*flight.FlightInfo, error) {
	dpDesc, err := DescForCommand(query)
	if err != nil {
		nlog.Errorf("Generate Descriptor to dataproxy fail, %v", err)
		return nil, buildGrpcErrorf(nil, codes.Internal, "Generate Descriptor to dataproxy fail")
	}

	dpCtx, cancel := context.WithTimeout(ctx, dpRequestTimeout)
	defer cancel()

	flightInfo, err := dp.flightClient.GetFlightInfo(dpCtx, dpDesc)
	if err != nil {
		return nil, err
	}

	return flightInfo, nil
}
