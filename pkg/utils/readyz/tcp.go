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

package readyz

import (
	"context"
	"fmt"
	"net"
	"time"
)

type TCPReadyZ struct {
	host string
}

func NewTCPReadyZ(host string) ReadyZ {
	return &TCPReadyZ{
		host: host,
	}
}

func (tr *TCPReadyZ) IsReady(ctx context.Context) error {
	conn, err := net.DialTimeout("tcp", tr.host, time.Second)
	if err != nil {
		return fmt.Errorf("check host(%s) start fail, err: %s", tr.host, err.Error())
	}
	conn.Close()
	return nil
}
