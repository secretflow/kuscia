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
