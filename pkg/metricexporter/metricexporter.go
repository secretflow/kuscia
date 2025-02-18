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
	"strconv"
	"sync"
	"time"

	"github.com/secretflow/kuscia/pkg/agent/pod"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan  = make(chan struct{})
	podManager pod.Manager
)

func ListPodMetricUrls(podManager pod.Manager) (map[string]string, error) {

	if podManager == nil {
		return nil, fmt.Errorf("podManager is not initialized")
	}

	metricUrls := map[string]string{}
	pods := podManager.GetPods()

	for _, pod := range pods {
		metricPath := ""
		metricPort := ""

		if val, ok := pod.Annotations[common.MetricPathAnnotationKey]; ok {
			metricPath = val
		}

		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == pod.Annotations[common.MetricPortAnnotationKey] {
					metricPort = strconv.Itoa(int(port.ContainerPort))
					break
				}
			}
		}

		if metricPath != "" && metricPort != "" {
			if metricPort == "80" {
				metricUrls[pod.Name] = fmt.Sprintf("http://%s/%s", pod.Status.PodIP, metricPath)
			} else {
				metricUrls[pod.Name] = fmt.Sprintf("http://%s:%s/%s", pod.Status.PodIP, metricPort, metricPath)
			}
		}
	}
	return metricUrls, nil
}

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
func metricHandler(metricURLs map[string]string, w http.ResponseWriter) {
	metricsChan := make(chan []byte, len(metricURLs))
	var wg sync.WaitGroup

	for _, fullURL := range metricURLs {
		wg.Add(1)
		go func(fullURL string) {
			defer wg.Done()

			metrics, err := getMetrics(fullURL)
			if err == nil {
				metricsChan <- metrics
			} else {
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
			_, _ = w.Write(metrics)
		}
	}
}

func combine(map1, map2 map[string]string) map[string]string {
	for k, v := range map2 {
		map1[k] = v
	}
	return map1
}

func MetricExporter(ctx context.Context, metricURLs map[string]string, port int) {
	nlog.Infof("Start to export metrics on port %d...", port)

	if podManager != nil {
		podMetrics, err := ListPodMetricUrls(podManager)
		if err != nil {
			nlog.Errorf("Error retrieving pod metrics: %v", err)
		} else {
			metricURLs = combine(metricURLs, podMetrics)
		}
	} else {
		nlog.Warn("podManager is nil, skipping ListPodMetricUrls call")
	}

	metricServer := http.NewServeMux()
	metricServer.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricHandler(metricURLs, w)
	})

	go func() {
		nlog.Infof("Starting metric server on port %d", port)

		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), metricServer); err != nil {
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
