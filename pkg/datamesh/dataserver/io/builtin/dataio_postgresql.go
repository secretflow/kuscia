// Copyright 2025 Ant Group Co., Ltd.
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

package builtin

import (
	"context"
	"database/sql"
	"fmt"
	"net"

	"github.com/apache/arrow/go/v13/arrow/flight"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type BuiltinPostgresqlIO struct {
	driverName string
}

func NewBuiltinPostgresqlIOChannel() DataMeshDataIOInterface {
	return &BuiltinPostgresqlIO{
		driverName: "postgres",
	}
}

func (o *BuiltinPostgresqlIO) newPostgresqlSession(config *datamesh.DatabaseDataSourceInfo) (*sql.DB, error) {
	nlog.Debugf("Open Postgresql Session database(%s), user(%s)", config.GetDatabase(), config.GetUser())

	host, port, err := net.SplitHostPort(config.GetEndpoint())
	if err != nil {
		if addrErr, ok := err.(*net.AddrError); ok && addrErr.Err == "missing port in address" {
			host = config.GetEndpoint()
			port = "5432"
			err = nil
		}
	}
	if err != nil {
		nlog.Errorf("Endpoint \"%s\" can't be resolved with net.SplitHostPort()", config.GetEndpoint())
		return nil, err
	}
	dsn := fmt.Sprintf("user=%s password=%s host=%s dbname=%s port=%s", config.GetUser(), config.GetPassword(), host, config.GetDatabase(), port)
	db, err := sql.Open(o.driverName, dsn)
	if err != nil {
		nlog.Errorf("Postgresql client open error(%s)", err)
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		db.Close()
		nlog.Errorf("Postgresql client ping error(%s)", err)
		return nil, err
	}
	return db, nil
}

func (o *BuiltinPostgresqlIO) Read(ctx context.Context, rc *utils.DataMeshRequestContext, w utils.RecordWriter) error {
	// init bucket & object stream
	dd, ds, err := rc.GetDomainDataAndSource(ctx)
	if err != nil {
		nlog.Errorf("Get domaindata domaindatasource failed(%s)", err)
		return err
	}

	// init db connect
	db, err := o.newPostgresqlSession(ds.Info.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	downloader := NewPostgresqlDownloader(ctx, db, dd, rc.Query)
	switch rc.GetTransferContentType() {
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return downloader.DataProxyContentToFlightStreamSQL(w)
	default:
		return errors.Errorf("invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

func (o *BuiltinPostgresqlIO) Write(ctx context.Context, rc *utils.DataMeshRequestContext, stream *flight.Reader) error {
	dd, ds, err := rc.GetDomainDataAndSource(ctx)
	if err != nil {
		nlog.Errorf("Get domaindata domaindatasource failed(%s)", err)
		return err
	}
	nlog.Infof("DomainData(%s) try save to remote Postgresql(%s/%s)", dd.DomaindataId, ds.Info.Database.Endpoint, dd.RelativeUri)

	// init db connect
	db, err := o.newPostgresqlSession(ds.Info.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	uploader := NewPostgresqlUploader(ctx, db, dd, rc.Query)

	switch rc.GetTransferContentType() {
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return uploader.FlightStreamToDataProxyContentPostgresql(stream)
	default:
		return errors.Errorf("invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

func (o *BuiltinPostgresqlIO) GetEndpointURI() string {
	return utils.BuiltinFlightServerEndpointURI
}
