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

package coredns

import (
	"context"
	"time"

	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Serial implements the Transferer interface.
func (e *KusciaCoreDNS) Serial(state request.Request) uint32 {
	return uint32(time.Now().Unix())
}

// MinTTL implements the Transferer interface.
func (e *KusciaCoreDNS) MinTTL(state request.Request) uint32 {
	return 30
}

// Transfer implements the Transferer interface.
func (e *KusciaCoreDNS) Transfer(ctx context.Context, state request.Request) (int, error) {
	return dns.RcodeServerFailure, nil
}
