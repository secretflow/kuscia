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

package ljwriter

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

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

type Writer struct {
	logLevel Level
	logger   *lumberjack.Logger
}

// New creates a new writer with config.
func New(config *nlog.LogConfig) (*Writer, error) {
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}
	if _, ok := levelMap[strings.ToLower(config.LogLevel)]; !ok {
		return nil, fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	return &Writer{
		logger: &lumberjack.Logger{
			Filename:   config.LogPath,
			MaxSize:    config.MaxFileSizeMB, // megabytes
			MaxBackups: config.MaxFiles,
			Compress:   config.Compress,
		},
		logLevel: levelMap[config.LogLevel],
	}, nil
}

func (w *Writer) Infof(format string, args ...interface{}) {
	if w.logLevel <= InfoLevel {
		w.logger.Write([]byte(fmt.Sprintf(format, args...)))
	}
}

func (w *Writer) Info(args ...interface{}) {
	if w.logLevel <= InfoLevel {
		w.logger.Write([]byte(fmt.Sprint(args...)))
	}
}

func (w *Writer) Debugf(format string, args ...interface{}) {
	if w.logLevel <= DebugLevel {
		w.logger.Write([]byte(fmt.Sprintf(format, args...)))
	}
}

func (w *Writer) Debug(args ...interface{}) {
	if w.logLevel <= DebugLevel {
		w.logger.Write([]byte(fmt.Sprint(args...)))
	}
}

func (w *Writer) Warnf(format string, args ...interface{}) {
	if w.logLevel <= WarnLevel {
		w.logger.Write([]byte(fmt.Sprintf(format, args...)))
	}
}
func (w *Writer) Warn(args ...interface{}) {
	if w.logLevel <= WarnLevel {
		w.logger.Write([]byte(fmt.Sprint(args...)))
	}
}

func (w *Writer) Errorf(format string, args ...interface{}) {
	if w.logLevel <= ErrorLevel {
		w.logger.Write([]byte(fmt.Sprintf(format, args...)))
	}
}

func (w *Writer) Error(args ...interface{}) {
	if w.logLevel <= ErrorLevel {
		w.logger.Write([]byte(fmt.Sprint(args...)))
	}
}

func (w *Writer) Fatalf(format string, args ...interface{}) {
	if w.logLevel <= FatalLevel {
		w.logger.Write([]byte(fmt.Sprintf(format, args...)))
	}
	os.Exit(1)
}

func (w *Writer) Fatal(args ...interface{}) {
	if w.logLevel <= FatalLevel {
		w.logger.Write([]byte(fmt.Sprint(args...)))
	}
	os.Exit(1)
}

func (w *Writer) Sync() error {
	return nil
}

func (w *Writer) Write(p []byte) (int, error) {
	return w.logger.Write(p)
}
