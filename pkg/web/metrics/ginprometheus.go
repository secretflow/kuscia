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

package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

/*
RequestCounterURLLabelMappingFn is a function which can be supplied to the middleware to control
the cardinality of the request counter's "url" label, which might be required in some contexts.
For instance, if for a "/customer/:name" route you don't want to generate a time series for every
possible customer name, you could use this function:

	func(c *gin.Context) string {
		url := c.Request.URL.Path
		for _, p := range c.Params {
			if p.Key == "name" {
				url = strings.Replace(url, p.Value, ":name", 1)
				break
			}
		}
		return url
	}

which would map "/customer/alice" and "/customer/bob" to their template "/customer/:name".
*/
type RequestCounterURLLabelMappingFn func(c *gin.Context) string

type ginMetrics struct {
	reqCnt       *prometheus.CounterVec
	reqDur       *prometheus.HistogramVec
	reqSz, resSz prometheus.Summary
}

func registerMetricsWithDefault(subsystem string) (*ginMetrics, error) {
	//  Default Metrics
	//	counter, counter_vec, gauge, gauge_vec,
	//	histogram, histogram_vec, summary, summary_vec
	m := ginMetrics{}
	m.reqCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "How many HTTP requests processed, partitioned by status code and HTTP method.",
		},
		[]string{"code", "method", "handler", "host", "url"},
	)
	if err := prometheus.Register(m.reqCnt); err != nil {
		return nil, fmt.Errorf("%s_requests_total could not be registered in Prometheus, Error:(%v)", subsystem, err)
	}

	m.reqDur = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "The HTTP request latencies in seconds.",
		},
		[]string{"code", "method", "url"},
	)
	if err := prometheus.Register(m.reqDur); err != nil {
		return nil, fmt.Errorf("%s_request_duration_seconds could not be registered in Prometheus, Error:(%v)", subsystem, err)
	}

	m.resSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "response_size_bytes",
			Help:      "The HTTP response sizes in bytes.",
		},
	)
	if err := prometheus.Register(m.resSz); err != nil {
		return nil, fmt.Errorf("%s_response_size_bytes could not be registered in Prometheus, Error:(%v)", subsystem, err)
	}
	m.reqSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "request_size_bytes",
			Help:      "The HTTP request sizes in bytes.",
		},
	)
	if err := prometheus.Register(m.reqSz); err != nil {
		return nil, fmt.Errorf("%s_request_size_bytes could not be registered in Prometheus, Error:(%v)", subsystem, err)
	}

	return &m, nil
}

// RegistGin adds the middleware to a gin engine.(without use engine as a metrics server)
func RegistGin(subsystem string, e *gin.Engine) error {
	gm, err := registerMetricsWithDefault(subsystem)
	if err != nil {
		return err
	}
	e.Use(handlerFunc(gm, ""))
	return nil
}

// RegistGinWithRouter adds the middleware and metrics path to a gin engine.
func RegistGinWithRouter(subsystem string, e *gin.Engine, metricsPath string) error {
	gm, err := registerMetricsWithDefault(subsystem)
	if err != nil {
		return err
	}
	e.Use(handlerFunc(gm, metricsPath))
	e.GET(metricsPath, prometheusHandler())
	return nil
}

// RegistGinWithRouterAuth adds the middleware and metrics path to a gin engine with BasicAuth.
func RegistGinWithRouterAuth(subsystem string, e *gin.Engine, metricspath string, accounts gin.Accounts) error {
	gm, err := registerMetricsWithDefault(subsystem)
	if err != nil {
		return err
	}
	e.Use(handlerFunc(gm, metricspath))
	e.GET(metricspath, gin.BasicAuth(accounts), prometheusHandler())
	return nil
}

// HandlerFunc defines handler function for middleware
func handlerFunc(m *ginMetrics, metricsPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer c.Next()
		if c.Request.URL.Path == metricsPath {
			return
		}

		start := time.Now()
		reqSz := computeApproximateRequestSize(c.Request)

		status := strconv.Itoa(c.Writer.Status())
		elapsed := float64(time.Since(start)) / float64(time.Second)
		resSz := float64(c.Writer.Size())

		m.reqDur.WithLabelValues(status, c.Request.Method, c.Request.URL.String()).Observe(elapsed)
		m.reqCnt.WithLabelValues(status, c.Request.Method, c.HandlerName(), c.Request.Host, c.Request.URL.String()).Inc()
		m.reqSz.Observe(float64(reqSz))
		m.resSz.Observe(resSz)
	}
}

func prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// From https://github.com/DanielHeckrath/gin-prometheus/blob/master/gin_prometheus.go
func computeApproximateRequestSize(r *http.Request) int {
	s := 0
	if r.URL != nil {
		s = len(r.URL.Path)
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	return s
}
