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

package clusters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/secretflow/kuscia/pkg/gateway/config"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

func TestMain(m *testing.M) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		nlog.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0755); err != nil {
		nlog.Fatal(err)
	}

	if err := paths.CopyDirectory("../../../etc/conf/domainroute", filepath.Join(dir, "conf")); err != nil {
		nlog.Fatal(err)
	}
	if err := paths.CreateIfNotExists(filepath.Join(dir, "nlogs"), 0755); err != nil {
		nlog.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		nlog.Fatal(err)
	}

	os.Setenv("NAMESPACE", "default")

	// start xds server
	instance := "test-instance"
	envoyNodeCluster := "kuscia-gateway-default"
	envoyNodeID := fmt.Sprintf("%s-%s", envoyNodeCluster, instance)

	xds.NewXdsServer(10000, envoyNodeID)
	config := &xds.InitConfig{
		Basedir:      "./conf/",
		XDSPort:      1054,
		ExternalPort: 1080,
		ExternalCert: nil,
		InternalCert: nil,
	}
	xds.InitSnapshot("default", "test-instance", config)

	os.Exit(m.Run())
}

func TestAddMasterClusters(t *testing.T) {
	err := AddMasterClusters(context.Background(), "alice", &config.MasterConfig{
		Master:    true,
		Namespace: "kuscia-master"})
	assert.NoError(t, err)
	vh, err := xds.QueryVirtualHost("handshake-virtual-host", xds.InternalRoute)
	assert.NoError(t, err)
	assert.NotNil(t, vh)
	assert.Contains(t, vh.Domains, "kuscia-handshake.master.svc")

	vh, err = xds.QueryVirtualHost("handshake-virtual-host", xds.ExternalRoute)
	assert.NoError(t, err)
	assert.NotNil(t, vh)
	assert.Contains(t, vh.Domains, "kuscia-handshake.master.svc")
}

func TestAddMasterProxyClusters(t *testing.T) {
	defer gock.Off()

	gock.New("http://127.0.0.1:80").
		Get("/handshake").
		MatchType("json").
		Reply(200).
		JSON(map[string]string{"namespace": "kuscia-test"})

	err := AddMasterProxyClusters(context.Background(), "alice", &config.MasterConfig{
		Master:    false,
		Namespace: "kuscia-master",
		MasterProxy: &config.ClusterConfig{
			Host:     "kuscia-master.svc",
			Port:     1080,
			Protocol: "http",
		}})
	assert.NoError(t, err)
}
