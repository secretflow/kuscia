package metricexporter

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

func getMetrics(buffer *bytes.Buffer, url string) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		nlog.Error("Error creating request:", err)
		return
	}
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		nlog.Error("Error sending request:", err)
		return
	}
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		nlog.Error("Error reading response body:", err)
		return
	}
	buffer.Write(responseBody)
}
func metricHandler(w http.ResponseWriter, r *http.Request) {
	nodeExporterUrl := "http://0.0.0.0:9100/metrics"
	netExporterUrl := "http://0.0.0.0:9092/netmetrics"
	_, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	var buffer bytes.Buffer
	getMetrics(&buffer, nodeExporterUrl)
	getMetrics(&buffer, netExporterUrl)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write(buffer.Bytes())
}

func MetricExporter(ctx context.Context, runMode pkgcom.RunModeType, exportPeriod uint) {
	nlog.Info("Start to export metrics...")
	metricServer := http.NewServeMux()
	metricServer.HandleFunc("/metrics", metricHandler)
	close(ReadyChan)
	nlog.Error(http.ListenAndServe("0.0.0.0:9091", metricServer))
	<-ctx.Done()
	nlog.Info("Stopping the metric exporter...")
}
