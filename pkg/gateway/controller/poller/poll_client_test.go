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

package poller

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func runTestReceiverServer(t *testing.T) {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Transfer-Encoding", "chunked")
		flusher.Flush()

		<-req.Context().Done()
		t.Log("Http request context done")
	})

	assert.NoError(t, http.ListenAndServe(":12345", nil))
}

func createTestPollConnection(index int) *PollConnection {
	dialer := &net.Dialer{
		Timeout: time.Millisecond * 500,
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
	}
	pc := NewPollConnection(index, client, "test", "test.alice.svc")
	pc.receiverAddress = "127.0.0.1:12345"
	pc.forceReconnectIntervalBase = time.Millisecond * 1000
	pc.forceReconnectIntervalMaxJitter = 1
	pc.requestTimeout = time.Millisecond * 2000
	pc.pollRetryInterval = time.Millisecond * 1
	return pc
}

func TestPollConnection_Start(t *testing.T) {
	go runTestReceiverServer(t)
	time.Sleep(time.Second)

	t.Run("Connected", func(t *testing.T) {
		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			pc := createTestPollConnection(1)
			pc.Start(ctx)
			close(done)
		}()

		select {
		case <-done:
			break
		case <-time.After(time.Millisecond * 1800):
			t.Fatal("timeout")
		}
	})

	t.Run("Canceled", func(t *testing.T) {
		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			pc := createTestPollConnection(2)
			pc.Start(ctx)
			close(done)
		}()

		cancel()
		select {
		case <-done:
			break
		case <-time.After(time.Millisecond * 1800):
			t.Fatal("timeout")
		}
	})
}
