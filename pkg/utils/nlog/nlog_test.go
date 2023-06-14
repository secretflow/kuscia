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

package nlog

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
)

func logTest(ctx context.Context, log *NLog) {
	log.WithCtx(ctx).Info("test log info")
	log.WithCtx(ctx).Infof("%s", "test log info")

	log.WithCtx(ctx).Debug("test log debug")
	log.WithCtx(ctx).Debugf("%s", "test log debug")

	log.WithCtx(ctx).Error("test log error")
	log.WithCtx(ctx).Errorf("%s", "test log error")

	log.WithCtx(ctx).Warn("test log warn")
	log.WithCtx(ctx).Warnf("%s", "test log warn")
}

func TestNewNLogWithNilOption(t *testing.T) {
	log := NewNLog(nil)
	assert.True(t, log != nil)
	ctx := context.Background()

	logTest(ctx, log)
}

func TestNewNLogWithNilContext(t *testing.T) {
	ctx := context.Background()
	logger, err := zlogwriter.New(&zlogwriter.LogConfig{LogPath: filepath.Join(t.TempDir(), "context.log")})
	assert.NoError(t, err)
	log := NewNLog(SetWriter(logger), SetFormatter(NewDefaultFormatter()))
	assert.True(t, log != nil)
	logTest(ctx, log)
}

type testContext struct {
	context.Context
	values map[interface{}]interface{}
}

func (t *testContext) Value(key interface{}) interface{} {
	return t.values[key]
}

type testVerticalBarFormatter struct {
}

func (t *testVerticalBarFormatter) Format(ctx context.Context, log string) string {
	if ctx == nil {
		return log
	}
	c, ok := ctx.(*testContext)
	if !ok {
		return log
	}
	traceID := c.values["trace_id"]
	tag := c.values["tags"]
	return fmt.Sprintf("%v | %v | %s ", traceID, tag, log)
}

type testSpaceFormatter struct {
}

func (t *testSpaceFormatter) Format(ctx context.Context, log string) string {
	if ctx == nil {
		return log
	}

	traceID := ctx.Value("trace_id")
	tag := ctx.Value("tags")
	return fmt.Sprintf("%v %v %s ", traceID, tag, log)
}

func TestNewNLogWithVerticalBarFormatter(t *testing.T) {
	ctx := &testContext{Context: context.Background(), values: map[interface{}]interface{}{"trace_id": "test_trace", "tags": "alice bob"}}
	logger, err := zlogwriter.New(&zlogwriter.LogConfig{LogPath: filepath.Join(t.TempDir(), "context.log")})
	assert.NoError(t, err)
	log := NewNLog(SetWriter(logger), SetFormatter(&testVerticalBarFormatter{}))
	assert.True(t, log != nil)
	logTest(ctx, log)
}

func TestNewNLogWithSpaceFormatter(t *testing.T) {
	ctx := &testContext{Context: context.Background(), values: map[interface{}]interface{}{"trace_id": 33, "tags": 44}}
	logger, err := zlogwriter.New(&zlogwriter.LogConfig{LogPath: filepath.Join(t.TempDir(), "context.log")})
	assert.NoError(t, err)
	log := NewNLog(SetWriter(logger), SetFormatter(&testSpaceFormatter{}))
	assert.True(t, log != nil)
	logTest(ctx, log)
}

func TestWithCtx(t *testing.T) {
	log := WithCtx(context.Background())
	assert.True(t, log != nil)
	ctx := context.Background()
	logTest(ctx, log)
}

func TestDefaultLogger(t *testing.T) {
	log := DefaultLogger()
	assert.True(t, log != nil)
	ctx := context.Background()
	logTest(ctx, log)
}
