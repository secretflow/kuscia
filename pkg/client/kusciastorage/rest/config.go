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

package rest

import (
	"time"
)

// Option is an alias of config func.
type Option func(c *config)

// config defines client configuration.
type config struct {
	serverAPIPrefix  string
	retryTimes       int
	connectTimeout   time.Duration
	readWriteTimeout time.Duration
	keepAliveTimeout time.Duration
}

// getDefaultConfig gets the default configurations.
func getDefaultConfig() *config {
	c := &config{}

	c.retryTimes = 3
	c.connectTimeout = 30 * time.Second
	c.readWriteTimeout = 300 * time.Second
	c.keepAliveTimeout = 60 * time.Second

	return c
}

// SetServerAPIPrefix sets the server api prefix.
func SetServerAPIPrefix(prefix string) Option {
	return func(c *config) {
		c.serverAPIPrefix = prefix
	}
}

// SetRetryTimes sets the retry times for failure request.
func SetRetryTimes(retryTimes int) Option {
	return func(c *config) {
		c.retryTimes = retryTimes
	}
}

// SetConnectTimeout sets the client connect timeout.
func SetConnectTimeout(timeout int) Option {
	return func(c *config) {
		c.connectTimeout = time.Duration(timeout) * time.Second
	}
}

// SetReadWriteTimeout sets the client read and write timeout.
func SetReadWriteTimeout(timeout int) Option {
	return func(c *config) {
		c.readWriteTimeout = time.Duration(timeout) * time.Second
	}
}

// SetKeepAliveTimeout sets the http client keep alive timeout.
func SetKeepAliveTimeout(timeout int) Option {
	return func(c *config) {
		c.keepAliveTimeout = time.Duration(timeout) * time.Second
	}
}
