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

package kusciastorage

import (
	"github.com/secretflow/kuscia/pkg/kusciastorage/beans"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/signals"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
)

// RunKusciaStorage runs the kuscia storage service.
func RunKusciaStorage() error {
	appEngine := engine.New(&framework.AppConfig{
		Name:    "kuscia-storage",
		Usage:   "store large resource under k8s cluster",
		Version: meta.KusciaVersionString(),
	})

	err := beans.InjectBeans(appEngine)
	if err != nil {
		return err
	}

	return appEngine.RunCommand(signals.NewKusciaContextWithStopCh(signals.SetupSignalHandler()))
}
