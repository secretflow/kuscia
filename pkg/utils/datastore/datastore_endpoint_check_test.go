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
	user      string
	pass      string
	prot      string
	addr      string
	dbname    string
	dsn       string
	netAddr   string
	available bool
)

// See https://github.com/go-sql-driver/mysql/wiki/Testing
// The password used here is 'password'
// for example: docker run -d --name mysql-svc -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=test --memory=512m -p 3306:3306 --network=kuscia-exchange mysql:8.0
// go-sql-driver/mysql support MySQL (5.6+)
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
		available = true
		c.Close()
	}
}

func TestCheckDatastoreEndpoint(t *testing.T) {

	if !available {
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
