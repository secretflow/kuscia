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

type BuiltinPostgresIO struct {
	driverName string
}

func NewBuiltinPostgresIOChannel() DataMeshDataIOInterface {
	return &BuiltinPostgresIO{
		driverName: "postgres",
	}
}

func (o *BuiltinPostgresIO) newPostgresSession(config *datamesh.DatabaseDataSourceInfo) (*sql.DB, error) {
	nlog.Debugf("Open Postgres Session database(%s), user(%s)", config.GetDatabase(), config.GetUser())

	host, port, err := net.SplitHostPort(config.GetEndpoint())
	if err != nil {
		nlog.Errorf("config.GetEndPoint() can't resolve with net.SplitHostPort()")
		return nil, err
	}
	dsn := fmt.Sprintf("user=%s password=%s host=%s dbname=%s port = %s", config.GetUser(), config.GetPassword(), host, config.GetDatabase(), port)
	db, err := sql.Open(o.driverName, dsn)
	if err != nil {
		nlog.Errorf("Postgres client open error(%s)", err)
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		nlog.Errorf("Postgres client ping error(%s)", err)
		return nil, err
	}
	return db, nil
}

func (o *BuiltinPostgresIO) Read(ctx context.Context, rc *utils.DataMeshRequestContext, w utils.RecordWriter) error {
	// init bucket & object stream
	dd, ds, err := rc.GetDomainDataAndSource(ctx)
	if err != nil {
		nlog.Errorf("Get domaindata domaindatasource failed(%s)", err)
		return err
	}

	// init db connect
	db, err := o.newPostgresSession(ds.Info.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	downloader := NewPostgresDownloader(ctx, db, dd, rc.Query)
	switch rc.GetTransferContentType() {
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return downloader.DataProxyContentToFlightStreamSQL(w)
	default:
		return errors.Errorf("Invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

func (o *BuiltinPostgresIO) Write(ctx context.Context, rc *utils.DataMeshRequestContext, stream *flight.Reader) error {
	dd, ds, err := rc.GetDomainDataAndSource(ctx)
	if err != nil {
		nlog.Errorf("Get domaindata domaindatasource failed(%s)", err)
		return err
	}
	nlog.Infof("DomainData(%s) try save to remote Postgres(%s/%s)", dd.DomaindataId, ds.Info.Database.Endpoint, dd.RelativeUri)

	// init db connect
	db, err := o.newPostgresSession(ds.Info.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	uploader := NewPostgresUploader(ctx, db, dd, rc.Query)

	switch rc.GetTransferContentType() {
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return uploader.FlightStreamToDataProxyContentPostgres(stream)
	default:
		return errors.Errorf("Invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

func (o *BuiltinPostgresIO) GetEndpointURI() string {
	return utils.BuiltinFlightServerEndpointURI
}
