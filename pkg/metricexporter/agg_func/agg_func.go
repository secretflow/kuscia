package agg_func

import (
	"math"
	"strconv"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// Sum an aggregation function to sum up two network metrics
func Sum(metrics []map[string]string, key string) (float64, error) {
	sum := 0.0
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Warnf("fail to parse float: %s, key: %s, value: %f", metric[key], key, val)
			return sum, err
		}
		sum += val
	}
	return sum, nil
}

// Avg an aggregation function to calculate the average of two network metrics
func Avg(metrics []map[string]string, key string) (float64, error) {
	sum, err := Sum(metrics, key)
	if err != nil {
		nlog.Warnf("Fail to get the sum of kt metrics, err: %v", err)
		return sum, err
	}
	return sum / float64(len(metrics)), nil
}

// Max an aggregation function to calculate the maximum of two network metrics
func Max(metrics []map[string]string, key string) (float64, error) {
	max := math.MaxFloat64 * (-1)
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Warn("fail to parse float")
			return max, err
		}
		if val > max {
			max = val
		}
	}
	return max, nil
}

// Min an aggregation function to calculate the minimum of two network metrics
func Min(metrics []map[string]string, key string) (float64, error) {
	min := math.MaxFloat64
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			nlog.Warn("fail to parse float")
			return min, err
		}
		if val < min {
			min = val
		}
	}
	return min, nil
}

// Rate an aggregation function to calculate the rate of a network metric between to metrics
func Rate(metric1 float64, metric2 float64) float64 {
	if metric2 == 0.0 {
		return 0
	}
	return metric1 / metric2
}
