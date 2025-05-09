/*
 * Copyright 2024 Ant Group Co., Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package start

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/pkg/common"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

func Test_Start_Autonomy(t *testing.T) {
	autonomy := common.RunModeAutonomy
	domainKeyData, err := tls.GenerateKeyData()
	assert.NoError(t, err)
	kusciaConfig := &confloader.AutonomyKusciaConfig{
		CommonConfig: confloader.CommonConfig{
			Mode:          autonomy,
			DomainID:      "alice",
			DomainKeyData: domainKeyData,
			Protocol:      common.TLS,
		},
	}

	data, err := yaml.Marshal(kusciaConfig)
	assert.NoError(t, err)
	dir := t.TempDir()
	common.DefaultKusciaHomePath = func() string {
		return dir
	}
	
	filename := filepath.Join(dir, "kuscia.yaml")
	assert.NoError(t, os.WriteFile(filename, data, 600))
	defer os.Remove(filename)

	workDir := GetWorkDir()
	assert.NoError(t, paths.CopyDirectory(filepath.Join(workDir, "etc"), common.DefaultKusciaHomePath()+"/etc"))
	assert.NoError(t, paths.CopyDirectory(filepath.Join(workDir, "crds"), common.DefaultKusciaHomePath()+"/crds"))

	runCtx, cancle := context.WithCancel(context.Background())
	defer cancle()
	kusciaConf, _ := confloader.ReadConfig(filename)
	conf := modules.NewModuleRuntimeConfigs(runCtx, kusciaConf)
	conf.Clients = &kubeconfig.KubeClients{
		KubeClient:   clientsetfake.NewSimpleClientset(),
		KusciaClient: kusciaclientsetfake.NewSimpleClientset(),
	}
	defer conf.Close()
	mm := NewModuleManager()
	assert.True(t, mm.Regist("coredns", modules.NewCoreDNS, autonomy))

	errCh := make(chan error)

	go func() {
		errCh <- mm.Start(runCtx, autonomy, conf)
	}()

	select {
	case err := <-errCh:
		nlog.Errorf("Error occurred during start -> %v", err)
	case <-time.Tick(5 * time.Second):
		nlog.Info("all modules are running")
	}
	assert.NoError(t, paths.CopyFile(filepath.Join(conf.RootDir, common.TmpPrefix, "resolv.conf"), "/etc/resolv.conf"))
}

func GetWorkDir() string {
	_, filename, _, _ := runtime.Caller(0)
	nlog.Infof("path is %s", filename)
	return strings.SplitN(filename, "cmd", 2)[0]
}
