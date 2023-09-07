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

//nolint:dulp
package utils

import (
	"testing"
)

func TestGetInitConfig(t *testing.T) {
	defaultConfTest(RunModeMaster, t)
	defaultConfTest(RunModeLite, t)
	defaultConfTest(RunModeAutonomy, t)
}
func defaultConfTest(runMode string, t *testing.T) {
	conf := GetInitConfig("", "", runMode)
	if conf.RootDir != defaultRootDir {
		t.Errorf("%v GetInitConfig test failed: RootDir expected %v but got %v", runMode, defaultRootDir, conf.RootDir)
	}
	if conf.DomainID != defaultDomainID {
		t.Errorf("%v GetInitConfig test failed: DomainID expected %v but got %v", runMode, defaultDomainID, conf.DomainID)
	}
	if conf.ApiserverEndpoint != defaultEndpoint {
		t.Errorf("%v GetInitConfig test failed: ApiserverEndpoint expected %v but got %v", runMode, defaultEndpoint, conf.ApiserverEndpoint)
	}
}
