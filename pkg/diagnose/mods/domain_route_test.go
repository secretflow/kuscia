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
	"errors"
	"testing"

	"github.com/secretflow/kuscia/pkg/diagnose/app/netstat"
	"github.com/secretflow/kuscia/pkg/diagnose/utils"
	"github.com/stretchr/testify/assert"
	"github.com/xhd2015/xgo/runtime/mock"
)

func TestDomainRouteMod(t *testing.T) {
	netParam := &netstat.NetworkParam{
		Bidirection: true,
	}

	conf := &DiagnoseConfig{
		Source:       "alice",
		Destination:  "bob",
		NetworkParam: netParam,
	}

	reporter := utils.NewReporter("")
	mod := NewDomainRouteMod(reporter, nil, conf)
	drMod := mod.(*DomainRouteMod)

	mock.Patch(drMod.crdMod.Run, func(ctx context.Context) error {
		return nil
	})

	mock.Patch(drMod.taskGroup.Start, func(ctx context.Context) ([]*netstat.TaskOutput, error) {
		return nil, nil
	})

	err := mod.Run(context.Background())
	assert.Nil(t, err)
}

func TestDomainRouteModFail(t *testing.T) {
	netParam := &netstat.NetworkParam{
		Bidirection: true,
	}

	conf := &DiagnoseConfig{
		Source:       "alice",
		Destination:  "bob",
		NetworkParam: netParam,
	}

	reporter := utils.NewReporter("")
	mod := NewDomainRouteMod(reporter, nil, conf)
	drMod := mod.(*DomainRouteMod)

	mock.Patch(drMod.crdMod.Run, func(ctx context.Context) error {
		return nil
	})

	mock.Patch(drMod.taskGroup.Start, func(ctx context.Context) ([]*netstat.TaskOutput, error) {
		return nil, errors.New("")
	})

	err := mod.Run(context.Background())
	assert.NotNil(t, err)
}
