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
)

var defaultLogger *NLog

func init() {
	defaultLogger = &NLog{
		logWriter: GetDefaultLogWriter(),
		ctx:       context.Background(),
		formatter: NewDefaultFormatter(),
	}
}

func Setup(ops ...Option) {
	defaultLogger = NewNLog(ops...)
}

func DefaultLogger() *NLog {
	return defaultLogger
}

func WithCtx(ctx context.Context) *NLog {
	return defaultLogger.WithCtx(ctx)
}

func Infof(format string, args ...interface{}) {
	defaultLogger.logWriter.Info(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprintf(format, args...)))
}

func Info(args ...interface{}) {
	defaultLogger.logWriter.Info(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprint(args...)))
}

func Debugf(format string, args ...interface{}) {
	defaultLogger.logWriter.Debug(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprintf(format, args...)))
}

func Debug(args ...interface{}) {
	defaultLogger.logWriter.Debug(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprint(args...)))
}

func Warnf(format string, args ...interface{}) {
	defaultLogger.logWriter.Warn(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprintf(format, args...)))
}

func Warn(args ...interface{}) {
	defaultLogger.logWriter.Warn(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprint(args...)))
}

func Errorf(format string, args ...interface{}) {
	defaultLogger.logWriter.Error(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprintf(format, args...)))
}

func Error(args ...interface{}) {
	defaultLogger.logWriter.Error(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprint(args...)))
}

func Fatalf(format string, args ...interface{}) {
	defaultLogger.logWriter.Fatal(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprintf(format, args...)))
}

func Fatal(args ...interface{}) {
	defaultLogger.logWriter.Fatal(defaultLogger.formatter.Format(defaultLogger.ctx, fmt.Sprint(args...)))
}

func Write(p []byte) (int, error) {
	return defaultLogger.logWriter.Write(p)
}

func Sync() error {
	return defaultLogger.logWriter.Sync()
}
