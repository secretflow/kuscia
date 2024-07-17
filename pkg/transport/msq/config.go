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

package msq

import "fmt"

const (
	cleanStep            = 100
	sidPqInitialCapacity = 5

	minTotalByteSizeLimit      = 1024 * 1024 * 100
	minPerSessionByteSizeLimit = 1024 * 1024

	minDeadSessionIDExpireSeconds = 600
	maxDeadSessionIDExpireSeconds = 3600

	minSessionExpireSeconds = 120
	maxSessionExpireSeconds = 1800

	minNormalizeActiveSeconds = 1
	maxNormalizeActiveSeconds = 10

	minCleanIntervalSeconds = 30
	maxCleanIntervalSeconds = 600
)

type Config struct {
	TotalByteSizeLimit      uint64 `yaml:"totalByteSizeLimit,omitempty"`
	PerSessionByteSizeLimit uint64 `yaml:"perSessionByteSizeLimit,omitempty"`
	TopicQueueCapacity      int    `yaml:"topicQueueCapacity,omitempty"`

	DeadSessionIDExpireSeconds int64 `yaml:"deadSessionIDExpireSeconds,omitempty"`
	SessionExpireSeconds       int64 `yaml:"sessionExpireSeconds,omitempty"`
	NormalizeActiveSeconds     int64 `yaml:"normalizeActiveSeconds,omitempty"`

	CleanIntervalSeconds int64 `yaml:"cleanIntervalSeconds,omitempty"`
}

func DefaultMsgConfig() *Config {
	return &Config{
		TotalByteSizeLimit:         1024 * 1024 * 1024 * 16,
		PerSessionByteSizeLimit:    1024 * 1024 * 60,
		TopicQueueCapacity:         5,
		DeadSessionIDExpireSeconds: 1800,
		SessionExpireSeconds:       600,
		NormalizeActiveSeconds:     4,
		CleanIntervalSeconds:       120,
	}
}

func (c *Config) Check() error {
	if c.TotalByteSizeLimit < minTotalByteSizeLimit {
		return fmt.Errorf("TotalByteSizeLimit(%d) of msq should greater than %d", c.TotalByteSizeLimit,
			minTotalByteSizeLimit)
	}

	if c.PerSessionByteSizeLimit > c.TotalByteSizeLimit {
		return fmt.Errorf("PerSessionByteSizeLimit(%d) of msq should less than TotalByteSizeLimit(%d)",
			c.PerSessionByteSizeLimit, c.TotalByteSizeLimit)
	}

	if c.PerSessionByteSizeLimit < minPerSessionByteSizeLimit {
		return fmt.Errorf("PerSessionByteSizeLimit(%d) of msq should greater than %d",
			c.PerSessionByteSizeLimit, minPerSessionByteSizeLimit)
	}

	adjustInt64(&c.DeadSessionIDExpireSeconds, minDeadSessionIDExpireSeconds, maxDeadSessionIDExpireSeconds)
	adjustInt64(&c.SessionExpireSeconds, minSessionExpireSeconds, maxSessionExpireSeconds)
	adjustInt64(&c.NormalizeActiveSeconds, minNormalizeActiveSeconds, maxNormalizeActiveSeconds)
	adjustInt64(&c.CleanIntervalSeconds, minCleanIntervalSeconds, maxCleanIntervalSeconds)
	return nil
}

func adjustInt64(v *int64, min, max int64) {
	if *v < min {
		*v = min
	}

	if *v > max {
		*v = max
	}
}
