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

package diagnose

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseConfig(t *testing.T) {
	testcase := `{
		"task_input_config": "{\"HostName\":\"\",\"ServerPort\":0,\"domain_data_id\":\"diagnose-alice-bob-c2f33b77aebdb6ad7c11\",\"pass\":true,\"speed\":true,\"speed_thres\":0,\"rtt\":true,\"rtt_thres\":0,\"proxy_timeout\":false,\"proxy_timeout_thres\":0,\"size\":true,\"size_thres\":0,\"proxy_buffer\":true,\"kernel\":true}",
		"server_port": 1000,
		"domains": ["alice", "bob"],
		"endpoints": ["alice-endpoint", "bob-endpoint"],
		"self_index": 0
	  }`
	jobConfig, err := parseConfig([]byte(testcase))
	if err != nil {
		t.Errorf("parse failed: %v", err)
	}
	assert.Equal(t, jobConfig.PeerEndpoint, "bob-endpoint")
	assert.Equal(t, jobConfig.SelfEndpoint, "alice-endpoint")

}
