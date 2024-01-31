package metricexporter

import (
	"context"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

func getMetrics(url string) (string, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		nlog.Error("Error creating request:", err)
		return "", err
	}
	client := http.Client{
		Timeout: 100 * time.Millisecond,
	}
	response, err := client.Do(request)
	if err != nil {
		nlog.Error("Error sending request:", err)
		return "", err
	}
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		nlog.Error("Error reading response body:", err)
		return "", err
	}
	return string(responseBody), nil
}
func metricHandler(metricUrls map[string]string, w http.ResponseWriter) {
	metricsChan := make(chan string, len(metricUrls))
	var wg sync.WaitGroup

	for key, url := range metricUrls {
		wg.Add(1)
		go func(key string, url string) {
			defer wg.Done()

			metrics, err := getMetrics(url)
			if err == nil {
				metricsChan <- metrics
			} else {
				nlog.Warnf("metrics[%s] query failed", key)
				metricsChan <- "" // empty metrics
			}
		}(key, url)
	}

	go func() {
		wg.Wait()
		close(metricsChan)
	}()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	for metrics := range metricsChan {
		w.Write([]byte(metrics))
	}
}

func MetricExporter(ctx context.Context, metricURLs map[string]string, port string) {
	nlog.Info("Start to export metrics...")
	metricServer := http.NewServeMux()
	metricServer.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metricHandler(metricURLs, w)
	})

	go func() {
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
