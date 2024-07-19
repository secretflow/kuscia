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

package metricexporter

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

func getMetrics(fullURL string) ([]byte, error) {
	request, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		nlog.Error("Error creating request:", err)
		return nil, err
	}
	client := http.Client{
		Timeout: 100 * time.Millisecond,
	}
	response, err := client.Do(request)
	if err != nil {
		nlog.Error("Error sending request:", err)
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		nlog.Error("Error reading response body:", err)
		return nil, err
	}
	return responseBody, nil
}

func BuildMetricURL(baseURL string, labels map[string]string) (string, error) {
	metricPath := "/" + labels["metric-path"]
	metricPort := labels["metric-port"]

	nlog.Infof("Base URL: %s", baseURL)
	nlog.Infof("Metric Path: %s", metricPath)
	nlog.Infof("Metric Port: %s", metricPort)

	u, err := url.Parse(baseURL)
	//!!!!!!这里是新加上的
	if err != nil {
		err = fmt.Errorf("Failed to parse base URL %s: URL is invalid or incorrectly formatted: %v", baseURL, err)
		return "", err
	}

	nlog.Infof("Parsed URL: %s", u.String())

	if metricPort != "" {
		u.Host = fmt.Sprintf("%s:%s", u.Hostname(), metricPort)
	}

	nlog.Infof("Updated Host: %s", u.Host)
	//!!!!!!!!!这里是新加上的
	if metricPath == "" {
		err = fmt.Errorf("Metric path is empty in labels for base URL %s", baseURL)
		return "", err
	}

	u.Path = metricPath

	fullURL := u.String()
	nlog.Infof("Constructed metric URL: %s", fullURL)

	//!!!!!!!!!这里是新加上的
	if fullURL == "" {
		err = fmt.Errorf("Constructed URL is empty after combining base URL %s with metric path %s", baseURL, metricPath)
		return "", err
	}

	return fullURL, nil
}

func metricHandler(fullURLs []string, w http.ResponseWriter) {
	nlog.Info("Entering metricHandler") //确认请求达到该函数
	metricsChan := make(chan []byte, len(fullURLs))
	var wg sync.WaitGroup

	for _, fullURL := range fullURLs {
		wg.Add(1)
		go func(fullURL string) {
			defer wg.Done()

			nlog.Infof("Fetching metrics from %s", fullURL)

			metrics, err := getMetrics(fullURL)
			if err == nil {
				nlog.Infof("Successfully fetched metrics from %s", fullURL)
				metricsChan <- metrics
			} else {
				nlog.Warnf("metrics query failed from %s: %v", fullURL, err)
				metricsChan <- nil // empty metrics
			}
		}(fullURL)
	}

	go func() {
		wg.Wait()
		close(metricsChan)
	}()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	for metrics := range metricsChan {
		if metrics != nil {
			w.Write(metrics)
		}
	}
}

func getAppMetricURL(labels map[string]string) (string, error) {
	// 直接从 labels 生成一个单独的 URL
	labelsURL, err := BuildMetricURL("http://localhost", labels)
	if err != nil {
		return "", fmt.Errorf("Error building URL from labels: %v", err)
	}
	return labelsURL, nil
}

func MetricExporter(ctx context.Context, metricURLs map[string]string, port string) {
	nlog.Infof("Start to export metrics on port %s...", port)

	// 构建所有的 metric URLs
	var fullURLs []string
	for _, baseURL := range metricURLs {
		fullURLs = append(fullURLs, baseURL) //保持原来的 metricURLs 不变
	}

	metricServer := http.NewServeMux()
	metricServer.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		nlog.Info("Received request for /metrics")
		metricHandler(fullURLs, w)
	})

	go func() {
		nlog.Infof("Starting metric server on port %s", port)
		if err := http.ListenAndServe("0.0.0.0:"+port, metricServer); err != nil {
			nlog.Error("Fail to start the metric exporterserver", err)
		}
	}()
	defer func() {
		close(ReadyChan)
		nlog.Info("Start to export metrics...")
	}()

	<-ctx.Done()
	nlog.Info("Stopping the metric exporter...")
}
