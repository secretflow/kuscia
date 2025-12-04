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

package modules

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func Test_RunConfigManager(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := mockModuleRuntimeConfig(t)
	m, err := NewConfManager(config)
	assert.NoError(t, err)
	assert.NoError(t, runModule(runCtx, m))
}

func Test_MetricExporter(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := mockModuleRuntimeConfig(t)
	m, err := NewMetricExporter(config)
	assert.NoError(t, err)
	assert.NoError(t, runModule(runCtx, m))
}

func runModule(ctx context.Context, m Module) error {
	var err error
	errCh := make(chan error)
	readyCh := make(chan struct{})
	go func() {
		err := m.Run(ctx)
		errCh <- err
	}()

	go func() {
		if err := m.WaitReady(ctx); err != nil {
			errCh <- err
		} else {
			close(readyCh)
		}
	}()

	select {
	case err = <-errCh:
		nlog.Errorf("start module [%s] err -> %v", m.Name(), err)
	case <-readyCh:
		nlog.Infof("start module [%s] successfully", m.Name())
	}
	return err
}
