package ssexporter

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/ssexporter/parse"
	"github.com/secretflow/kuscia/pkg/ssexporter/promexporter"
	"github.com/secretflow/kuscia/pkg/ssexporter/ssmetrics"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

func SsExporter(ctx context.Context, runMode pkgcom.RunModeType, ExportPeriod uint) {
	// read the config
	SsMetrics, AggregationMetrics := parse.LoadMetricConfig()
	clusterAddresses := parse.GetClusterAddress()
	localDomainName := parse.GetLocalDomainName()
	var MetricTypes = promexporter.NewMetricTypes()
	// register metrics for prometheus and initialize the calculation of change values
	var reg *prometheus.Registry
	reg = promexporter.ProduceMetrics(localDomainName, clusterAddresses, SsMetrics, MetricTypes, AggregationMetrics)
	ssMetrics := ssmetrics.ConvertClusterMetrics(SsMetrics, clusterAddresses)
	lastClusterMetricValues := ssmetrics.GetSsMetricResults(runMode, localDomainName, clusterAddresses, AggregationMetrics, ExportPeriod)
	nlog.Info("Start to export metrics...")
	close(ReadyChan)
	// export the cluster metrics
	ticker := time.NewTicker(time.Duration(ExportPeriod) * time.Second)
	defer ticker.Stop()
	go func(runMode pkgcom.RunModeType, NetMetrics []string, MetricTypes map[string]string, ExportPeriods uint, lastClusterMetricValues map[string]float64) {
		for range ticker.C {
			// get clusterName and clusterAddress
			clusterAddresses = parse.GetClusterAddress()
			// get cluster metrics
			currentClusterMetricValues := ssmetrics.GetSsMetricResults(runMode, localDomainName, clusterAddresses, AggregationMetrics, ExportPeriods)
			// calculate the change values of cluster metrics
			lastClusterMetricValues, currentClusterMetricValues = ssmetrics.GetMetricChange(lastClusterMetricValues, currentClusterMetricValues)
			// update cluster metrics in prometheus
			promexporter.UpdateMetrics(currentClusterMetricValues, MetricTypes)
		}
	}(runMode, ssMetrics, MetricTypes, ExportPeriod, lastClusterMetricValues)
	// export to the prometheus
	http.Handle(
		"/ssmetrics", promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			}),
	)
	nlog.Error(http.ListenAndServe("0.0.0.0:9092", nil))
	<-ctx.Done()
	nlog.Info("Stopping the metric exporter...")
}
