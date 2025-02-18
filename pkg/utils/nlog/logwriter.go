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
	"strings"
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
	Write(p []byte) (int, error)

	ChangeLogLevel(newLevel string) error
}

// A Level is a logging priority. Higher levels are more important.
type Level int

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production.
	DebugLevel Level = iota
	// InfoLevel is the default logging priority.
	InfoLevel
	// WarnLevel logs are more important than Info, but don't need individual
	// human review.
	WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs.
	ErrorLevel
	// DPanicLevel logs are particularly important errors. In development the
	// logger panics after writing the message.
	DPanicLevel
	// PanicLevel logs a message, then panics.
	PanicLevel
	// FatalLevel logs a message, then calls os.Exit(1).
	FatalLevel

	_minLevel = DebugLevel
	_maxLevel = FatalLevel

	// InvalidLevel is an invalid value for Level.
	//
	// Core implementations may panic if they see messages of this level.
	InvalidLevel = _maxLevel + 1
)

var levelMap = map[string]Level{
	"debug": DebugLevel,
	// InfoLevel is the default logging priority.
	"info": InfoLevel,
	// WarnLevel logs are more important than Info, but don't need individual
	// human review.
	"warn": WarnLevel,
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs.
	"error": ErrorLevel,
	// DPanicLevel logs are particularly important errors. In development the
	// logger panics after writing the message.
	"dpanic": DPanicLevel,
	// PanicLevel logs a message, then panics.
	"panic": PanicLevel,
	// FatalLevel logs a message, then calls os.Exit(1).
	"fatal": FatalLevel,
}

type defaultLogWriter struct {
	logLevel Level
}

func (d *defaultLogWriter) Infof(format string, args ...interface{}) {
	if d.logLevel <= InfoLevel {
		fmt.Println(fmt.Sprintf(format, args...))
	}
}
func (d *defaultLogWriter) Info(args ...interface{}) {
	if d.logLevel <= InfoLevel {
		fmt.Println(args...)
	}
}

func (d *defaultLogWriter) Debugf(format string, args ...interface{}) {
	if d.logLevel <= DebugLevel {
		fmt.Println(fmt.Sprintf(format, args...))
	}
}
func (d *defaultLogWriter) Debug(args ...interface{}) {
	if d.logLevel <= DebugLevel {
		fmt.Println(args...)
	}
}

func (d *defaultLogWriter) Warnf(format string, args ...interface{}) {
	if d.logLevel <= WarnLevel {
		fmt.Println(fmt.Sprintf(format, args...))
	}
}
func (d *defaultLogWriter) Warn(args ...interface{}) {
	if d.logLevel <= WarnLevel {
		fmt.Println(args...)
	}
}

func (d *defaultLogWriter) Errorf(format string, args ...interface{}) {
	if d.logLevel <= ErrorLevel {
		fmt.Println(fmt.Sprintf(format, args...))
	}
}
func (d *defaultLogWriter) Error(args ...interface{}) {
	if d.logLevel <= ErrorLevel {
		fmt.Println(args...)
	}
}

func (d *defaultLogWriter) Fatalf(format string, args ...interface{}) {
	if d.logLevel <= FatalLevel {
		fmt.Println(fmt.Sprintf(format, args...))
	}
	os.Exit(1)
}

func (d *defaultLogWriter) Fatal(args ...interface{}) {
	if d.logLevel <= FatalLevel {
		fmt.Println(args...)
	}
	os.Exit(1)
}

func (d *defaultLogWriter) Sync() error {
	return nil
}

func (d *defaultLogWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (d *defaultLogWriter) ChangeLogLevel(newLevel string) error {
	logLevel, ok := levelMap[strings.ToLower(newLevel)]
	if !ok {
		return fmt.Errorf("invalid log level: %s", newLevel)
	}
	d.logLevel = logLevel
	return nil
}

func GetDefaultLogWriter() LogWriter {
	return &defaultLogWriter{
		logLevel: InfoLevel,
	}
}
