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

type NLog struct {
	logWriter LogWriter
	formatter Formatter
	ctx       context.Context
}

func (n *NLog) WithCtx(ctx context.Context) *NLog {
	ret := &NLog{ctx: ctx, logWriter: n.logWriter, formatter: n.formatter}
	if ctx == nil {
		ret.ctx = context.Background()
	}
	return ret
}

func (n *NLog) Infof(format string, args ...interface{}) {
	n.logWriter.Info(n.formatter.Format(n.ctx, fmt.Sprintf(format, args...)))
}

func (n *NLog) Info(args ...interface{}) {
	n.logWriter.Info(n.formatter.Format(n.ctx, fmt.Sprint(args...)))
}

func (n *NLog) Debugf(format string, args ...interface{}) {
	n.logWriter.Debug(n.formatter.Format(n.ctx, fmt.Sprintf(format, args...)))
}

func (n *NLog) Debug(args ...interface{}) {
	n.logWriter.Debug(n.formatter.Format(n.ctx, fmt.Sprint(args...)))
}

func (n *NLog) Warnf(format string, args ...interface{}) {
	n.logWriter.Warn(n.formatter.Format(n.ctx, fmt.Sprintf(format, args...)))
}

func (n *NLog) Warn(args ...interface{}) {
	n.logWriter.Warn(n.formatter.Format(n.ctx, fmt.Sprint(args...)))
}

func (n *NLog) Errorf(format string, args ...interface{}) {
	n.logWriter.Error(n.formatter.Format(n.ctx, fmt.Sprintf(format, args...)))
}

func (n *NLog) Error(args ...interface{}) {
	n.logWriter.Error(n.formatter.Format(n.ctx, fmt.Sprint(args...)))
}

func (n *NLog) Fatalf(format string, args ...interface{}) {
	n.logWriter.Fatal(n.formatter.Format(n.ctx, fmt.Sprintf(format, args...)))
}

func (n *NLog) Fatal(args ...interface{}) {
	n.logWriter.Fatal(n.formatter.Format(n.ctx, fmt.Sprint(args...)))
}

func NewNLog(ops ...Option) *NLog {
	n := &NLog{logWriter: GetDefaultLogWriter(), formatter: NewDefaultFormatter(), ctx: context.Background()}
	for _, o := range ops {
		if o != nil {
			o.apply(n)
		}
	}
	return n
}
