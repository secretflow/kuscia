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
	"fmt"
	"os"
)

type LogWriter interface {
	Infof(format string, args ...interface{})
	Info(args ...interface{})

	Debugf(format string, args ...interface{})
	Debug(args ...interface{})

	Warnf(format string, args ...interface{})
	Warn(args ...interface{})

	Errorf(format string, args ...interface{})
	Error(args ...interface{})

	Fatalf(format string, args ...interface{})
	Fatal(args ...interface{})

	Sync() error
}

type defaultLogWriter struct {
}

func (d *defaultLogWriter) Infof(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}
func (d *defaultLogWriter) Info(args ...interface{}) {
	fmt.Println(args...)
}

func (d *defaultLogWriter) Debugf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}
func (d *defaultLogWriter) Debug(args ...interface{}) {
	fmt.Println(args...)
}

func (d *defaultLogWriter) Warnf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}
func (d *defaultLogWriter) Warn(args ...interface{}) {
	fmt.Println(args...)
}

func (d *defaultLogWriter) Errorf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}
func (d *defaultLogWriter) Error(args ...interface{}) {
	fmt.Println(args...)
}

func (d *defaultLogWriter) Fatalf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (d *defaultLogWriter) Fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(1)
}

func (d *defaultLogWriter) Sync() error {
	return nil
}

func GetDefaultLogWriter() LogWriter {
	return &defaultLogWriter{}
}
