// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logutils

import (
	"fmt"
	"os"
	"sync"
)

// ReopenableLogger is only designed for container logging purpose
// when container runtime is in runp mode.
// ReopenableLogger will work along with exec.Cmd.Stdout/Stderr and ContainerLogManager,
// and will perform reopen when CRI interface ReopenContainerLog is called.
type ReopenableLogger struct {
	fileName string
	file     *os.File
	mu       sync.Mutex
}

func NewReopenableLogger(fileName string) *ReopenableLogger {
	return &ReopenableLogger{
		fileName: fileName,
		file:     nil,
		mu:       sync.Mutex{},
	}
}

// Write into file, if nil open a new file
func (l *ReopenableLogger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		if err = l.openNew(); err != nil {
			return 0, err
		}
	}
	return l.file.Write(p)
}

// Close file to ensure data is all written.
func (l *ReopenableLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.close()
	if err != nil {
		return err
	}
	l.file = nil
	return nil
}

// call ReopenFile within CRI interface ReopenContainerLog
func (l *ReopenableLogger) ReopenFile() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return fmt.Errorf("file %s is not opened", l.fileName)
	}
	err := l.close()
	if err != nil {
		// close error, stop open new file
		return err
	}
	l.file = nil
	return l.openNew()
}

func (l *ReopenableLogger) close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// assume that specified file does not exist,
// if it exists, it will be truncated
func (l *ReopenableLogger) openNew() error {
	file, err := os.OpenFile(l.fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	l.file = file
	return nil
}
