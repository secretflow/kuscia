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

package mods

import (
	"context"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey"
	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/stretchr/testify/assert"
)

func TestNetworkMod(t *testing.T) {
	netParam := &netstat.NetworkParam{
		Bidirection: true,
	}

	conf := &DiagnoseConfig{
		Source:       "alice",
		Destination:  "bob",
		NetworkParam: netParam,
	}

	reporter := utils.NewReporter("")
	mod := NewNetworkMod(reporter, nil, conf)
	netMod := mod.(*NetworkMod)

	patch1 := gomonkey.ApplyMethod(reflect.TypeOf(netMod.cdrMod), "Run", func(_ *DomainRouteMod, ctx context.Context) error {
		return nil
	})
	defer patch1.Reset()
	err := mod.Run(context.Background())
	assert.Nil(t, err)
}
