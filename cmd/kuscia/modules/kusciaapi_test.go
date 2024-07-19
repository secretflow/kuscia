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
func Test_RunKusciaAPI(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	dependency := mockDependency(t)
	_ = RunKusciaAPI(runCtx, cancel, dependency, nil)
	cancel()
	runCtx.Done()
}

func Test_RunKusciaAPIWithTLS(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	dependency := mockDependency(t)
	dependency.KusciaAPI.HTTPPort = 8010
	dependency.KusciaAPI.GRPCPort = 8011
	dependency.KusciaAPI.HTTPInternalPort = 8012
	dependency.Protocol = common.TLS
	RunKusciaAPI(runCtx, cancel, dependency, nil)
	cancel()
}

func Test_RunKusciaAPIWithMTLS(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	dependency := mockDependency(t)
	dependency.KusciaAPI.HTTPPort = 8020
	dependency.KusciaAPI.GRPCPort = 8021
	dependency.KusciaAPI.HTTPInternalPort = 8022
	dependency.Protocol = common.MTLS
	RunKusciaAPI(runCtx, cancel, dependency, nil)
	cancel()
}

func Test_RunKusciaAPIWithNOTLS(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	dependency := mockDependency(t)
	dependency.KusciaAPI.HTTPPort = 8030
	dependency.KusciaAPI.GRPCPort = 8031
	dependency.KusciaAPI.HTTPInternalPort = 8032
	dependency.Protocol = common.NOTLS
	RunKusciaAPI(runCtx, cancel, dependency, nil)
	cancel()
}
*/
