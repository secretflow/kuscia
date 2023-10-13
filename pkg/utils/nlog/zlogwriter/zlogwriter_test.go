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

package zlogwriter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func logTest(ctx context.Context, log *nlog.NLog) {
	log.WithCtx(ctx).Info("test log info")
	log.WithCtx(ctx).Infof("%s", "test log info")

	log.WithCtx(ctx).Debug("test log debug")
	log.WithCtx(ctx).Debugf("%s", "test log debug")

	log.WithCtx(ctx).Error("test log error")
	log.WithCtx(ctx).Errorf("%s", "test log error")

	log.WithCtx(ctx).Warn("test log warn")
	log.WithCtx(ctx).Warnf("%s", "test log warn")
}

func TestNewNLogWithNilContext(t *testing.T) {
	ctx := context.Background()
	logger, err := New(&nlog.LogConfig{LogPath: filepath.Join(t.TempDir(), "context.log")})
	assert.NoError(t, err)
	log := nlog.NewNLog(nlog.SetWriter(logger), nlog.SetFormatter(nlog.NewDefaultFormatter()))
	assert.True(t, log != nil)
	logTest(ctx, log)
}
