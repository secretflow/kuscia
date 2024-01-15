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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.ame
// See the License for the specific language governing permissions and
// limitations under the License.

package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type EndpointValidator interface {
	PingDatastoreEndpoint(datastoreEndpoint string) error
}

type MySQLDatastoreEndpointValidator struct{}

func (mysqlValidator MySQLDatastoreEndpointValidator) PingDatastoreEndpoint(datastoreEndpoint string) error {
	errorFormat := "DatastoreEndpoint config error: %s"
	c, err := mysql.ParseDSN(datastoreEndpoint)
	if err != nil {
		return fmt.Errorf(errorFormat, err.Error())
	}

	return pingDatastoreEndpointByDriverName(c.FormatDSN(), "mysql")
}

func CheckDatastoreEndpoint(datastoreEndpoint string) error {

	if datastoreEndpoint == "" {
		nlog.Warn("Kuscia 'datastoreEndpoint' config is empty, will use sqlite.")
		return nil
	}

	parts := strings.SplitN(datastoreEndpoint, "://", 2)
	if len(parts) < 2 {
		return fmt.Errorf("Configured 'datastoreEndpoint' is invalid, expected format: mysql://username:password@tcp(hostname:3306)/database-name")
	}

	driveName := parts[0]
	datastoreDSN := parts[1]

	var datastoreEndpointValidator EndpointValidator

	switch driveName {
	case "mysql":
		datastoreEndpointValidator = MySQLDatastoreEndpointValidator{}
	default:
		errMsg := fmt.Sprintf("Kuscia 'datastoreEndpoint' config: Driver Name is '%s' Not supported", driveName)
		return fmt.Errorf("%s", errMsg)
	}
	return datastoreEndpointValidator.PingDatastoreEndpoint(datastoreDSN)
}

func pingDatastoreEndpointByDriverName(datastoreEndpoint, driveName string) error {

	db, err := sql.Open(driveName, datastoreEndpoint)
	if err != nil {
		return fmt.Errorf("Open datastore endpoint error: %s", err.Error())
	}

	defer db.Close()

	// timeout 5s
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// db ping
	err = db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("Ping datastore endpoint error: %s", err.Error())
	}
	nlog.Infof("Datastore endpoint is effective.")
	return nil
}
