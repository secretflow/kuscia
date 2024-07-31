// Copyright 2024 Ant Group Co., Ltd.
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

package runtime

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type KernelParam struct {
	Key         string
	Description string
	Value       string
	IsMatch     bool
}

type Kernel interface {
	List() []*KernelParam
}

type osKernel struct {
	params []*KernelParam
}

func CurrentKernel() Kernel {
	k := &osKernel{}

	needGreaterThanOrEqualToDefaultValue, needLessThanOrEuqalToDefaultValue := false, true

	k.params = append(k.params,
		// maximum size of the SYN backlog queue in a TCP connection. (value >= 2048)
		NewFileKernalParam("tcp_max_syn_backlog", "/proc/sys/net/ipv4/tcp_max_syn_backlog", 2048, needGreaterThanOrEqualToDefaultValue),
		// wait accept queue size. (value >= 2048)
		NewFileKernalParam("somaxconn", "/proc/sys/net/core/somaxconn", 2048, needGreaterThanOrEqualToDefaultValue),
		// tcp retry times, 5 --> 25.6s-51.2s, 15 --> 924.6s-1044.6s.  (value <= 5)
		NewFileKernalParam("tcp_retries2", "/proc/sys/net/ipv4/tcp_retries2", 5, needLessThanOrEuqalToDefaultValue),
		// controls the behavior of TCP slow start when a connection remains idle for a certain period of time. set to 0, make keep-alive connection send quickly
		// (value <= 0)
		NewFileKernalParam("tcp_slow_start_after_idle", "/proc/sys/net/ipv4/tcp_slow_start_after_idle", 0, needLessThanOrEuqalToDefaultValue),
		// client reuse TIME_WAIT socket. 1 --> enable. (value >= 1)
		NewFileKernalParam("tcp_tw_reuse", "/proc/sys/net/ipv4/tcp_tw_reuse", 1, needGreaterThanOrEqualToDefaultValue),
		// max open files. (value >= 102400)
		NewFileKernalParam("file-max", "/proc/sys/fs/file-max", 102400, needGreaterThanOrEqualToDefaultValue),
	)

	return k
}

func (k *osKernel) List() []*KernelParam {
	return k.params
}

func errorParams(key, fileName string, err error) *KernelParam {
	return &KernelParam{
		Key:         key,
		Description: fmt.Sprintf("Query from %s failed with %s", fileName, err.Error()),
		Value:       "Unknown",
		IsMatch:     false,
	}
}

func NewFileKernalParam(key, fileName string, defaultValue int, defaultIsMaxValue bool) *KernelParam {
	c, err := os.ReadFile(fileName)
	if err != nil {
		nlog.Warnf("Read file(%s) failed with error: %s", fileName, err.Error())
		return errorParams(key, fileName, err)
	}

	value := strings.Trim(string(c), "\n")

	intVal, err := strconv.Atoi(value)
	if err != nil {
		nlog.Warnf("File(%s) content(%s) is not int, error=%s", fileName, value, err.Error())
		return errorParams(key, fileName, err)
	}

	return &KernelParam{
		Key:         key,
		Description: fmt.Sprintf("Default[%d], File[%s]", defaultValue, fileName),
		Value:       value,
		IsMatch:     (defaultIsMaxValue && intVal <= defaultValue) || (!defaultIsMaxValue && intVal >= defaultValue),
	}
}
