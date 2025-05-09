package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/secretflow/kuscia/pkg/transport/codec"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestAddFlags(t *testing.T) {
	opts := &Opts{}
	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)

	opts.AddFlags(flagSet)

	assert.NotNil(t, opts.logCfg)
	assert.NotNil(t, flagSet.Lookup("session-count"))
	assert.NotNil(t, flagSet.Lookup("topic-count"))
	assert.NotNil(t, flagSet.Lookup("consumer-count"))
	assert.NotNil(t, flagSet.Lookup("producer-count"))
	assert.NotNil(t, flagSet.Lookup("timeout"))
	assert.NotNil(t, flagSet.Lookup("address"))
}

func TestNewTransClient(t *testing.T) {
	opts := &Opts{
		SessionCount:  10,
		TopicCount:    5,
		ConsumerCount: 3,
		ProducerCount: 3,
		Timeout:       2,
		Address:       "127.0.0.1:8080",
	}

	client := NewTransClient(opts)
	assert.NotNil(t, client)
	assert.Equal(t, opts, client.opts)
	assert.NotNil(t, client.client)
	assert.Equal(t, int64(0), client.pushSucceedCount)
	assert.Equal(t, int64(0), client.pushFailCount)
	assert.Equal(t, int64(0), client.popSucceedCount)
	assert.Equal(t, int64(0), client.popFailCount)
}

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient()
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
	assert.Equal(t, 120*time.Second, client.Timeout)
}

func TestGetRandomSid(t *testing.T) {
	opts := &Opts{SessionCount: 5}
	client := NewTransClient(opts)

	sid := client.getRandomSid()
	assert.Contains(t, sid, "session-")
}

func TestGetRandomTopic(t *testing.T) {
	opts := &Opts{TopicCount: 5}
	client := NewTransClient(opts)

	topic := client.getRandomTopic()
	assert.Contains(t, topic, "topic-")
}

func TestSetReqParams(t *testing.T) {
	opts := &Opts{SessionCount: 5, TopicCount: 5, Timeout: 10}
	client := NewTransClient(opts)

	req, _ := http.NewRequest("POST", "http://example.com", nil)
	client.setReqParams(req)

	assert.NotEmpty(t, req.Header.Get(codec.PtpTopicID))
	assert.NotEmpty(t, req.Header.Get(codec.PtpSessionID))
	values, err := url.ParseQuery(req.URL.RawQuery)
	assert.NoError(t, err)
	assert.Equal(t, "10", values.Get("timeout"))
}
