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

package parse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadMetricConfig test LoadMetricConfig
func TestLoadMetricConfig(t *testing.T) {
	testCases := []struct {
		name            string
		expectedMetrics []string
		expectedAgg     map[string]string
	}{
		{
			name:            "Check LoadMetricConfig output",
			expectedMetrics: []string{"rtt", "retrans", "total_connections", "retran_rate"},
			expectedAgg: map[string]string{
				"rtt":               "avg",
				"retrans":           "sum",
				"total_connections": "sum",
				"retran_rate":       "rate",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics, agg := LoadMetricConfig()
			require.Equal(t, tc.expectedMetrics, metrics, "Metrics do not match expected values")
			require.Equal(t, tc.expectedAgg, agg, "Aggregated metrics do not match expected values")
		})
	}
}
