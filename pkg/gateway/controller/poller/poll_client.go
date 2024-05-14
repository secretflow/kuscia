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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	defaultRequestTimeout                  = time.Second * 300
	defaultReceiverTimeoutSeconds          = 300
	defaultForceReconnectIntervalBase      = time.Second * 270
	defaultForceReconnectIntervalMaxJitter = 10 * 1000
	defaultPollRetryInterval               = time.Second * 10
)

type PollConnection struct {
	connID          string
	client          *http.Client
	receiverAddress string
	serviceName     string
	connected       chan struct{}
	disconnected    chan struct{}
	cancel          context.CancelFunc

	receiverTimeoutSeconds          int
	requestTimeout                  time.Duration
	forceReconnectIntervalBase      time.Duration
	forceReconnectIntervalMaxJitter int
	pollRetryInterval               time.Duration
}

func NewPollConnection(index int, client *http.Client, receiverDomain, serviceName string) *PollConnection {
	return &PollConnection{
		connID:                          fmt.Sprintf("%s:%s:%d", serviceName, receiverDomain, index),
		client:                          client,
		receiverAddress:                 fmt.Sprintf("%s.%s.svc", common.ReceiverServiceName, receiverDomain),
		serviceName:                     serviceName,
		connected:                       make(chan struct{}),
		disconnected:                    make(chan struct{}),
		receiverTimeoutSeconds:          defaultReceiverTimeoutSeconds,
		requestTimeout:                  defaultRequestTimeout,
		forceReconnectIntervalBase:      defaultForceReconnectIntervalBase,
		forceReconnectIntervalMaxJitter: defaultForceReconnectIntervalMaxJitter,
		pollRetryInterval:               defaultPollRetryInterval,
	}
}

func (conn *PollConnection) Start(ctx context.Context) {
	go func() {
		defer close(conn.disconnected)
		if err := conn.connect(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				nlog.Infof("Poll connection %q canceled", conn.connID)
			} else if errors.Is(err, io.EOF) {
				nlog.Infof("Poll connection %q closed", conn.connID)
			} else if errors.Is(err, context.DeadlineExceeded) {
				nlog.Infof("Poll connection %q context deadline exceeded", conn.connID)
			} else if errors.Is(err, io.ErrUnexpectedEOF) {
				nlog.Warnf("Poll connection %q encountered Unexpected EOF", conn.connID)
			} else {
				nlog.Errorf("Poll connection %q error, detail-> %v", conn.connID, err)
				time.Sleep(conn.pollRetryInterval)
			}
		}
	}()

	select {
	case <-conn.disconnected: // connection disconnected
		nlog.Infof("Poll connection %q disconnected", conn.connID)
		return
	case <-conn.connected:
	}

	reconnectDuration := conn.forceReconnectIntervalBase + time.Duration(rand.Intn(conn.forceReconnectIntervalMaxJitter))*time.Millisecond
	select {
	case <-conn.disconnected: // connection disconnected
		nlog.Infof("Poll connection %q disconnected", conn.connID)
		return
	case <-time.After(reconnectDuration):
		nlog.Infof("Poll connection %q lasted for %v, prepare to reconnect", conn.connID, reconnectDuration)
		return
	}
}

func (conn *PollConnection) Stop() {
	if conn.cancel != nil {
		conn.cancel()
	}
}

func (conn *PollConnection) connect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, conn.requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/poll?service=%s&timeout=%ds",
		conn.receiverAddress, conn.serviceName, conn.receiverTimeoutSeconds), nil)
	if err != nil {
		return err
	}

	resp, err := conn.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}

	close(conn.connected)

	nlog.Infof("Poll connection %q succeed, read body....", conn.connID)

	_, err = io.ReadAll(resp.Body)
	return err
}

type PollClient struct {
	stopCh         chan struct{}
	client         *http.Client
	serviceName    string
	receiverDomain string
}

func newPollClient(client *http.Client, serviceName, receiverDomain string) *PollClient {
	return &PollClient{
		client:         client,
		serviceName:    serviceName,
		stopCh:         make(chan struct{}),
		receiverDomain: receiverDomain,
	}
}

func (pc *PollClient) Start() {
	go pc.pollReceiver()
}

func (pc *PollClient) pollReceiver() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	index := 0

	for {
		select {
		case <-pc.stopCh:
			nlog.Infof("Stop poll connection %v", buildPollerKey(pc.serviceName, pc.receiverDomain))
			return
		default:
			conn := NewPollConnection(index, pc.client, pc.receiverDomain, pc.serviceName)
			conn.Start(ctx)
			index++
			if index > 999999 {
				index = 0
			}
		}
	}
}

func (pc *PollClient) Stop() {
	close(pc.stopCh)
}
