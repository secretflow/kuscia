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
	"flag"
	"os"
	"time"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LogConfig struct {
	LogLevel string
	LogPath  string

	// MaxFileSizeMB is the maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxFileSizeMB int

	// MaxFiles is the maximum number of old log files to retain.  The default
	// is to retain all old log files
	MaxFiles int
}

// Writer is the zap.SugaredLogger implementation of nlog.LogWriter interface.
type Writer struct {
	*zap.SugaredLogger
	atomicLevel zap.AtomicLevel
}

type WriterWrapper struct {
	writeFunc func(args ...interface{})
}

// InstallFlags defines log flags with flag.FlagSet.
func InstallFlags(flagset *flag.FlagSet) *LogConfig {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	var c LogConfig
	flagset.StringVar(&c.LogLevel, "log.level", "INFO", "Logs of this level or above will be output")
	flagset.StringVar(&c.LogPath, "log.path", "", "Also output logs to this file, empty means only output to stdout")
	flagset.IntVar(&c.MaxFileSizeMB, "log.file_size", 512, "Maximum size in megabytes of the log file before it gets rotated")
	flagset.IntVar(&c.MaxFiles, "log.max_files", 10, "Maximum number of old log files to retain")
	return &c
}

// InstallPFlags defines log flags with pflag.FlagSet.
func InstallPFlags(flagset *pflag.FlagSet) *LogConfig {
	if flagset == nil {
		flagset = pflag.CommandLine
	}

	var c LogConfig
	flagset.StringVar(&c.LogLevel, "log.level", "INFO", "Logs of this level or above will be output")
	flagset.StringVar(&c.LogPath, "log.path", "", "Also output logs to this file, empty means only output to stdout")
	flagset.IntVar(&c.MaxFileSizeMB, "log.file_size", 512, "Maximum size in megabytes of the log file before it gets rotated")
	flagset.IntVar(&c.MaxFiles, "log.max_files", 10, "Maximum number of old log files to retain")
	return &c
}

// New creates a new writer with config.
func New(config *LogConfig) (*Writer, error) {
	if config == nil {
		config = &LogConfig{
			LogLevel: "Debug",
		}
	}
	atomicLevel := zap.NewAtomicLevel()

	log, err := newZapLogger(config, &zapcore.EncoderConfig{
		ConsoleSeparator: " ",

		LevelKey:   "Level",
		TimeKey:    "Timestamp",
		MessageKey: "Message",
		CallerKey:  "Caller",

		EncodeLevel:  zapcore.CapitalLevelEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
		EncodeTime: func(time time.Time, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendString(time.Format("2006-01-02 15:04:05.000"))
		},
	}, atomicLevel)
	if err != nil {
		return nil, err
	}

	writer := &Writer{
		log,
		atomicLevel,
	}

	return writer, nil
}

// newZapLogger creates a *zap.SugaredLogger object.
func newZapLogger(config *LogConfig, encoderConfig *zapcore.EncoderConfig, atomicLevel zap.AtomicLevel) (*zap.SugaredLogger, error) {
	syncer := zapcore.AddSync(os.Stdout)
	if config.LogPath != "" {
		syncer = zapcore.NewMultiWriteSyncer(syncer, zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.LogPath,
			MaxSize:    config.MaxFileSizeMB, // megabytes
			MaxBackups: config.MaxFiles,
		}))
	}

	if err := changeLogLevel(atomicLevel, config.LogLevel); err != nil {
		return nil, err
	}

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(*encoderConfig), syncer, atomicLevel)
	zap.ReplaceGlobals(zap.New(core).WithOptions(zap.AddCaller()))

	return zap.L().WithOptions(zap.AddCallerSkip(1)).Sugar(), nil
}

func changeLogLevel(atomicLevel zap.AtomicLevel, newLevel string) error {
	var level zapcore.Level
	if err := level.Set(newLevel); err != nil {
		return err
	}
	atomicLevel.SetLevel(level)
	return nil
}

// ChangeLogLevel changes the log level on the fly.
// choose from DEBUG, INFO, WARN, ERROR, FATAL.
func (w *Writer) ChangeLogLevel(newLevel string) error {
	return changeLogLevel(w.atomicLevel, newLevel)
}

// Flush flushes any buffered log entries.
func (w *Writer) Flush() error {
	return w.Sync()
}
