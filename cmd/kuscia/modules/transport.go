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

package modules

import (
	"context"
	"fmt"
	"time"

	"github.com/secretflow/kuscia/pkg/transport/server/http"
	"github.com/secretflow/kuscia/pkg/utils/readyz"
)

type transportModule struct {
	moduleRuntimeBase
	rootDir    string
	configPath string
	address    string
}

func NewTransport(i *ModuleRuntimeConfigs) (Module, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", i.TransportPort)
	return &transportModule{
		moduleRuntimeBase: moduleRuntimeBase{
			name:         "transport",
			readyTimeout: 60 * time.Second,
			rdz:          readyz.NewTCPReadyZ(addr),
		},
		rootDir:    i.RootDir,
		configPath: i.TransportConfigFile,
		address:    addr,
	}, nil
}

func (t *transportModule) Run(ctx context.Context) error {
	return http.Run(ctx, t.configPath)
}
