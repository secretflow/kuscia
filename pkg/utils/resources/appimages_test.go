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

package resources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func makeTestDeployTemplatesCase1() []kusciaapisv1alpha1.DeployTemplate {
	dts := []kusciaapisv1alpha1.DeployTemplate{
		{
			Name:     "abc",
			Role:     "server",
			Replicas: new(int32),
		},
		{
			Name:     "abc",
			Role:     "server,client",
			Replicas: new(int32),
		},
		{
			Name:     "abc",
			Role:     "",
			Replicas: new(int32),
		},
	}

	for i := range dts {
		*dts[i].Replicas = int32(i)
	}

	return dts
}

func makeTestDeployTemplatesCase2() []kusciaapisv1alpha1.DeployTemplate {
	dts := []kusciaapisv1alpha1.DeployTemplate{
		{
			Name:     "abc",
			Role:     "server",
			Replicas: new(int32),
		},
		{
			Name:     "abc",
			Role:     "server,client,",
			Replicas: new(int32),
		},
	}

	for i := range dts {
		*dts[i].Replicas = int32(i)
	}

	return dts
}

func Test_selectDeployTemplate(t *testing.T) {
	dts1 := makeTestDeployTemplatesCase1()
	dts2 := makeTestDeployTemplatesCase2()

	tests := []struct {
		templates    []kusciaapisv1alpha1.DeployTemplate
		role         string
		wantReplicas int32
		wantErr      bool
	}{
		{
			dts1,
			"server",
			0,
			false,
		},
		{
			dts1,
			"client",
			1,
			false,
		},
		{
			dts1,
			"",
			2,
			false,
		},
		{
			dts1,
			"not-exist",
			2,
			false,
		},
		{
			dts2,
			"",
			0,
			false,
		},
		{
			dts2,
			"not-exist",
			0,
			true,
		},
		{
			nil,
			"server",
			0,
			true,
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("TestCase %d", i), func(t *testing.T) {
			got, err := SelectDeployTemplate(tt.templates, tt.role)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantReplicas, *got.Replicas)
			}
		})
	}
}
