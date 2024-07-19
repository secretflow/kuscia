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

package modules

/*
func Test_RunDomainRoute(t *testing.T) {
	tmpDir := t.TempDir()
	path, err := os.Getwd()
	assert.NoError(t, err)
	workDir := strings.SplitN(path, "cmd", 2)[0]
	nlog.Infof("work dir is: %s", workDir)
	assert.NoError(t, err)
	dependency := mockDependency(t)
	dependency.RootDir = workDir
	dependency.Master = kusciaconfig.MasterConfig{
		APIServer: &kusciaconfig.APIServerConfig{
			KubeConfig: filepath.Join(tmpDir, "etc/kubeconfig"),
			Endpoint:   "https://127.0.0.1:6443",
		},
		KusciaAPI: &kusciaconfig.ServiceConfig{
			Endpoint: "http://127.0.0.1:8092",
		},
	}
	dependency.Clients = &kubeconfig.KubeClients{
		KubeClient:   clientsetfake.NewSimpleClientset(),
		KusciaClient: kusciaclientsetfake.NewSimpleClientset(),
		Kubeconfig: &restclient.Config{
			Host: "https://127.0.0.1:6443",
		},
	}
	runCtx, cancel := context.WithCancel(context.Background())
	RunDomainRoute(runCtx, cancel, dependency, nil)
}
*/
