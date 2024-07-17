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

package network

import (
	"fmt"
	"net"
)

var BuiltinPortAllocator PortAllocator

type PortAllocator interface {
	Next() (int32, error)
}

type randomPortAllocator struct {
	minPort       int
	maxPort       int
	nextCheckPort int
}

func NewPortAllocator(hintMinPort, hintMaxPort int) PortAllocator {
	return &randomPortAllocator{
		minPort:       hintMinPort,
		maxPort:       hintMaxPort,
		nextCheckPort: hintMinPort,
	}
}

func (r *randomPortAllocator) Next() (int32, error) {
	for ; r.nextCheckPort < r.maxPort; r.nextCheckPort++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", r.nextCheckPort))
		if err == nil {
			defer listener.Close()
			r.nextCheckPort++
			return int32(r.nextCheckPort - 1), nil
		}
	}

	return -1, fmt.Errorf("not found a usable port in range[%d-%d)", r.minPort, r.maxPort)
}

func init() {
	BuiltinPortAllocator = NewPortAllocator(11111, 20000)
}
