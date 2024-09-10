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

package builtin

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/apache/arrow/go/v13/arrow/flight"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"

	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

// BuiltinMySQLIO defined the MySQL read & write methond
type BuiltinMySQLIO struct {
	// backend sql driver name
	driverName string
}

func NewBuiltinMySQLIOChannel() DataMeshDataIOInterface {
	return &BuiltinMySQLIO{
		driverName: "mysql",
	}
}

// new mysql client
func (o *BuiltinMySQLIO) newMySQLSession(config *datamesh.DatabaseDataSourceInfo) (*sql.DB, error) {
	nlog.Debugf("Open MySQL Session database(%s), user(%s)", config.GetDatabase(), config.GetUser())

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", config.GetUser(), config.GetPassword(), config.GetEndpoint(), config.GetDatabase())
	db, err := sql.Open(o.driverName, dsn)
	if err != nil {
		nlog.Errorf("MySQL client open error(%s)", err)
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		nlog.Errorf("MySQL client ping error(%s)", err)
		return nil, err
	}
	return db, nil
}

// DataFlow: RemoteStorage(MySQL/...)  --> DataProxy --> Client
func (o *BuiltinMySQLIO) Read(ctx context.Context, rc *utils.DataMeshRequestContext, w utils.RecordWriter) error {
	// init bucket & object stream
	dd, ds, err := rc.GetDomainDataAndSource(ctx)
	if err != nil {
		nlog.Errorf("Get domaindata domaindatasource failed(%s)", err)
		return err
	}

	// init db connect
	db, err := o.newMySQLSession(ds.Info.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	downloader := NewMySQLDownloader(ctx, db, dd, rc.Query)
	switch rc.GetTransferContentType() {
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return downloader.DataProxyContentToFlightStreamSQL(w)
	default:
		return errors.Errorf("Invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

// DataFlow: Client --> DataProxy --> RemoteStorage(MySQL/...)
func (o *BuiltinMySQLIO) Write(ctx context.Context, rc *utils.DataMeshRequestContext, stream *flight.Reader) error {
	dd, ds, err := rc.GetDomainDataAndSource(ctx)
	if err != nil {
		nlog.Errorf("Get domaindata domaindatasource failed(%s)", err)
		return err
	}
	nlog.Infof("DomainData(%s) try save to remote MySQL(%s/%s)", dd.DomaindataId, ds.Info.Database.Endpoint, dd.RelativeUri)

	// init db connect
	db, err := o.newMySQLSession(ds.Info.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	uploader := NewMySQLUploader(ctx, db, dd, rc.Query)

	switch rc.GetTransferContentType() {
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return uploader.FlightStreamToDataProxyContentMySQL(stream)
	default:
		return errors.Errorf("Invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

func (o *BuiltinMySQLIO) GetEndpointURI() string {
	return utils.BuiltinFlightServerEndpointURI
}
