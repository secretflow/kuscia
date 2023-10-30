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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/secretflow/kuscia/pkg/transport/transerr"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/meta"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/signals"
)

var r *rand.Rand

func init() {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
}

type Opts struct {
	SessionCount  int
	TopicCount    int
	ConsumerCount int
	ProducerCount int
	Timeout       int

	Address string
	logCfg  *nlog.LogConfig
}

type TransClient struct {
	client *http.Client
	opts   *Opts
	codec  codec.Codec

	pushSucceedCount int64
	pushFailCount    int64

	popSucceedCount int64
	popFailCount    int64

	stopCh chan struct{}
}

// AddFlags adds flags for a specific server to the specified FlagSet.
func (o *Opts) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.SessionCount, "session-count", 10, "max concurrent session-count.")
	fs.IntVar(&o.TopicCount, "topic-count", 5, "max concurrent topic-count.")
	fs.IntVar(&o.ConsumerCount, "consumer-count", 5, "consumer worker count")
	fs.IntVar(&o.ProducerCount, "producer-count", 5, "producer worker count")
	fs.IntVar(&o.Timeout, "timeout", 2, "request timeout seconds")
	fs.StringVar(&o.Address, "address", "127.0.0.1:8080", "server address")

	o.logCfg = zlogwriter.InstallPFlags(fs)
}

func main() {
	opts := &Opts{}
	rootCmd := newCommand(opts)
	opts.AddFlags(rootCmd.Flags())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newCommand(opts *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "transClient",
		Long:         "transClient to test transport Server",
		Version:      meta.KusciaVersionString(),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			zLogWriter, err := zlogwriter.New(opts.logCfg)
			if err != nil {
				return err
			}
			nlog.Setup(nlog.SetWriter(zLogWriter))

			client := NewTransClient(opts)
			client.Start(signals.NewKusciaContextWithStopCh(signals.SetupSignalHandler()))
			return nil
		},
	}

	return cmd
}

func NewTransClient(opts *Opts) *TransClient {
	client := NewHTTPClient()
	tc := &TransClient{
		client:           client,
		opts:             opts,
		codec:            codec.NewProtoCodec(),
		pushSucceedCount: 0,
		pushFailCount:    0,
		popSucceedCount:  0,
		popFailCount:     0,
	}
	return tc
}

func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   90,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   time.Second * 120,
	}
}

func (tc *TransClient) Start(ctx context.Context) {
	for i := 0; i < tc.opts.ProducerCount; i++ {
		go runUntil(ctx.Done(), tc.producer)
	}

	for i := 0; i < tc.opts.ConsumerCount; i++ {
		go runUntil(ctx.Done(), tc.consumer)
	}

	go wait.Until(tc.stats, time.Minute, ctx.Done())

	<-ctx.Done()
}

func runUntil(stopCh <-chan struct{}, fn func()) {
	for true {
		select {
		case <-stopCh:
			return
		default:
			fn()
		}
	}
}

func (tc *TransClient) getRandomSid() string {
	return fmt.Sprintf("session-%d", r.Intn(tc.opts.SessionCount))
}

func (tc *TransClient) getRandomTopic() string {
	return fmt.Sprintf("topic-%d", r.Intn(tc.opts.TopicCount))
}

func (tc *TransClient) sendRequest(req *http.Request) (*codec.Outbound, error) {
	resp, err := tc.client.Do(req)
	if err != nil {
		nlog.Warnf("send http request fail:%v", err)
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}
	return tc.codec.UnMarshal(body)
}

func inc(count *int64) {
	atomic.AddInt64(count, 1)
}

func (tc *TransClient) producer() {
	msgLength := 256 * 1024

	content := common.GenerateRandomBytes(256*1024 + r.Intn(msgLength))
	path := fmt.Sprintf("http://%s/v1/interconn/chan/invoke", tc.opts.Address)

	req, _ := http.NewRequest("POST", path, bytes.NewBuffer(content))
	tc.setReqParams(req)

	outbound, _ := tc.sendRequest(req)
	if outbound != nil && outbound.Code == string(transerr.Success) {
		inc(&tc.pushSucceedCount)
	} else {
		inc(&tc.pushFailCount)
	}
}

func (tc *TransClient) setReqParams(req *http.Request) {
	sid := tc.getRandomSid()
	topic := tc.getRandomTopic()
	req.Header.Set(codec.PtpTopicID, topic)
	req.Header.Set(codec.PtpSessionID, sid)
	params := url.Values{}
	params.Add("timeout", fmt.Sprintf("%d", tc.opts.Timeout))
	req.URL.RawQuery = params.Encode()
}

func (tc *TransClient) consumer() {
	path := fmt.Sprintf("http://%s/v1/interconn/chan/pop", tc.opts.Address)
	req, _ := http.NewRequest("POST", path, bytes.NewBuffer(nil))
	tc.setReqParams(req)

	outbound, _ := tc.sendRequest(req)

	if outbound == nil || outbound.Code != string(transerr.Success) {
		inc(&tc.pushFailCount)
	}
	if outbound != nil && outbound.Payload != nil {
		inc(&tc.popSucceedCount)
	}
}

func (tc *TransClient) stats() {
	nlog.Infof("pushSucceed:%d pushFailed:%d popSucceed:%d popFailed:%d", tc.pushSucceedCount, tc.pushFailCount,
		tc.popSucceedCount, tc.popFailCount)
}
