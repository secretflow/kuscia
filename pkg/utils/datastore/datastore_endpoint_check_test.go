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

package datastore

import (
	"fmt"
	"net"
	"os"
	"testing"
)

var (
	user                string
	pass                string
	prot                string
	addr                string
	dbname              string
	dsn                 string
	netAddr             string
	availableMySQL      bool
	pgHost              string
	pgPort              string
	pgUser              string
	pgPassword          string
	pgDatabase          string
	pgNetAddr           string
	pgSslmode           string
	availablePostgreSQL bool
)

// See https://github.com/go-sql-driver/mysql/wiki/Testing
// The password used here is 'password'
// for example: docker run -d --name mysql-svc -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=test --memory=512m -p 3306:3306 --network=kuscia-exchange mysql:8.0
// go-sql-driver/mysql support MySQL (5.6+)
// postgres (9.6+)
// for example: docker run -d --name postgres-svc -e POSTGRES_PASSWORD=password -e POSTGRES_DB=test --memory=512m -p 5432:5432 --network=kuscia-exchange postgres:17.4
func init() {
	// get environment variables
	env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}
	user = env("MYSQL_TEST_USER", "root")
	pass = env("MYSQL_TEST_PASS", "password")
	prot = env("MYSQL_TEST_PROT", "tcp")
	addr = env("MYSQL_TEST_ADDR", "localhost:3306")
	dbname = env("MYSQL_TEST_DBNAME", "test")
	netAddr = fmt.Sprintf("%s(%s)", prot, addr)
	dsn = fmt.Sprintf("%s:%s@%s/%s?timeout=30s", user, pass, netAddr, dbname)
	c, err := net.Dial(prot, addr)
	if err == nil {
		availableMySQL = true
		c.Close()
	}

	pgHost = env("PGHOST", "localhost")
	pgPort = env("PGPORT", "5432")
	pgUser = env("PGUSER", "postgres")
	pgPassword = env("PGPASSWORD", "password")
	pgDatabase = env("PGDATABASE", "postgres")
	pgSslmode = env("PGSSLMODE", "disable")

	pgNetAddr = fmt.Sprintf("%s(%s:%s)", "tcp", pgHost, pgPort)
	pc, err := net.Dial("tcp", fmt.Sprintf("%s:%s", pgHost, pgPort))
	if err == nil {
		availablePostgreSQL = true
		pc.Close()
	}
}

func TestCheckDatastoreEndpoint_MySQL(t *testing.T) {

	if !availableMySQL {
		t.Skipf("MySQL server not running on %s", netAddr)
	}

	type args struct {
		datastoreEndpoint string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "empty datastoreEndpoint will use sqlite",
			args:    args{datastoreEndpoint: ""},
			wantErr: false,
		},
		{
			name:    "mysql datastoreEndpoint 1",
			args:    args{datastoreEndpoint: "mysql://root:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local"},
			wantErr: false,
		},
		{
			name:    "mysql datastoreEndpoint 2",
			args:    args{datastoreEndpoint: "mysql://root:password@tcp(127.0.0.1:3306)/test1?charset=utf8mb4&parseTime=True&loc=Local"},
			wantErr: true,
		},
		{
			name:    "mysql datastoreEndpoint 3",
			args:    args{datastoreEndpoint: "mysql://root:password1@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local"},
			wantErr: true,
		},
		{
			name:    "mysql datastoreEndpoint 4",
			args:    args{datastoreEndpoint: "mysql://root1:password@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local"},
			wantErr: true,
		},
		{
			name:    "mysql datastoreEndpoint 5",
			args:    args{datastoreEndpoint: "mysql://root:password@tcp(localhost:3306)/test?charset=utf8mb4&parseTime=True&loc=Local"},
			wantErr: false,
		},
		{
			name:    "mysql datastoreEndpoint 6",
			args:    args{datastoreEndpoint: "mysql://root1:password@tcp(mysql.scv:3306)/test?charset=utf8mb4&parseTime=True&loc=localdsad"},
			wantErr: true,
		},
		{
			name:    "mysql datastoreEndpoint 7",
			args:    args{datastoreEndpoint: "mysql://root:password@tcp(mysql.scv:3306)/test?charset=utf8mb4&parseTime=True&loc=Local"},
			wantErr: true,
		},
		{
			name:    "mysql datastoreEndpoint 8",
			args:    args{datastoreEndpoint: "mysql://qwesad@3dwq3e41:eqwe"},
			wantErr: true,
		},
		{
			name:    "postgres datastoreEndpoint 1",
			args:    args{datastoreEndpoint: "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"},
			wantErr: false,
		},
		{
			name:    "postgres datastoreEndpoint 2",
			args:    args{datastoreEndpoint: "postgresql://postgres:mysecretpassword@localhost:5432/postgres?sslmode=disable"},
			wantErr: false,
		},
		{
			name:    "unsupported datastoreEndpoint",
			args:    args{datastoreEndpoint: "sqlite://test.db"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckDatastoreEndpoint(tt.args.datastoreEndpoint); (err != nil) != tt.wantErr {
				t.Errorf("CheckDatastoreEndpoint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckDatastoreEndpoint_PostgreSQL(t *testing.T) {

	if !availablePostgreSQL {
		t.Skipf("PostgreSQL server not running on %s", pgNetAddr)
	}

	type args struct {
		datastoreEndpoint string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "empty datastoreEndpoint will use sqlite",
			args:    args{datastoreEndpoint: ""},
			wantErr: false,
		},
		{
			name:    "postgres datastoreEndpoint 1",
			args:    args{datastoreEndpoint: "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"},
			wantErr: false,
		},
		{
			name: "postgres datastoreEndpoint 2",
			args: args{datastoreEndpoint: "postgresql://postgres:password@localhost:5432/postgres"},
			// because of the `sslmode`, it will be error
			wantErr: true,
		},
		{
			name:    "postgres datastoreEndpoint 3",
			args:    args{datastoreEndpoint: "postgres://qwesad@3dwq3e41:eqwe"},
			wantErr: true,
		},
		{
			name:    "postgres datastoreEndpoint 4",
			args:    args{datastoreEndpoint: "postgresql://postgres:password@localhost:15432/postgres?sslmode=disable"},
			wantErr: true,
		},
		{
			name:    "postgres datastoreEndpoint 5",
			args:    args{datastoreEndpoint: "postgresql://postgres:password@localhost1:5432/postgres?sslmode=disable"},
			wantErr: true,
		},
		{
			name:    "unsupported datastoreEndpoint",
			args:    args{datastoreEndpoint: "sqlite://test.db"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckDatastoreEndpoint(tt.args.datastoreEndpoint); (err != nil) != tt.wantErr {
				t.Errorf("CheckDatastoreEndpoint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
