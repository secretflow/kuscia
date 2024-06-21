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

package modules

import (
	"context"
	"time"

	"github.com/secretflow/kuscia/pkg/interconn"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func NewInterConn(ctx context.Context, deps *Dependencies) (Module, error) {
	return interconn.NewServer(ctx, deps.Clients)
}

func RunInterConnWithDestroy(conf *Dependencies) {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := newShutdownHookEntry(1 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "interconn",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	RunInterConn(runCtx, cancel, conf, shutdownEntry)
}

func RunInterConn(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) {
	m, err := NewInterConn(ctx, conf)
	if err != nil {
		nlog.Error(err)
		cancel()
		return
	}

	go func() {
		defer func() {
			if shutdownEntry != nil {
				shutdownEntry.RunShutdown()
			}
		}()
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Fatalf("InterConn wait ready failed: %v", err)
	}
	nlog.Info("InterConn is ready")
}
