package netexporter

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/netexporter/netmetrics"
	"github.com/secretflow/kuscia/pkg/netexporter/parse"
	"github.com/secretflow/kuscia/pkg/netexporter/promexporter"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	ReadyChan = make(chan struct{})
)

func NetExporter(ctx context.Context, runMode pkgcom.RunModeType, ExportPeriod uint) {
	// read the config
	NetworkMetrics, AggregationMetrics, ClusterMetrics := parse.LoadMetricConfig()
	clusterAddresses := parse.GetClusterAddress()
	localDomainName := parse.GetLocalDomainName()
	// get the cluster metrics to be exported
	clusterMetrics := netmetrics.ConvertClusterMetrics(ClusterMetrics, clusterAddresses)
	var MetricTypes = promexporter.NewMetricTypes()
	// register metrics for prometheus and initialize the calculation of change values
	var reg *prometheus.Registry
	reg = promexporter.ProduceMetrics(localDomainName, clusterAddresses, NetworkMetrics, ClusterMetrics, MetricTypes, AggregationMetrics)
	lastClusterMetricValues := netmetrics.GetClusterMetricResults(runMode, localDomainName, clusterAddresses, clusterMetrics, AggregationMetrics, ExportPeriod)
	nlog.Info("Start to export metrics...")
	close(ReadyChan)
	// export the cluster metrics
	ticker := time.NewTicker(time.Duration(ExportPeriod) * time.Second)
	defer ticker.Stop()
	go func(runMode pkgcom.RunModeType, ClusterMetrics []string, MetricTypes map[string]string, ExportPeriods uint, lastClusterMetricValues map[string]float64) {
		for range ticker.C {
			// get clusterName and clusterAddress
			clusterAddresses = parse.GetClusterAddress()
			// get the cluster metrics to be exported
			clusterMetrics = netmetrics.ConvertClusterMetrics(ClusterMetrics, clusterAddresses)
			// get cluster metrics
			currentClusterMetricValues := netmetrics.GetClusterMetricResults(runMode, localDomainName, clusterAddresses, clusterMetrics, AggregationMetrics, ExportPeriods)
			// calculate the change values of cluster metrics
			lastClusterMetricValues, currentClusterMetricValues = netmetrics.GetMetricChange(lastClusterMetricValues, currentClusterMetricValues)
			// update cluster metrics in prometheus
			promexporter.UpdateMetrics(currentClusterMetricValues, MetricTypes)
		}
	}(runMode, ClusterMetrics, MetricTypes, ExportPeriod, lastClusterMetricValues)
	// export to the prometheus
	http.Handle(
		"/netmetrics", promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			}),
	)
	nlog.Error(http.ListenAndServe("0.0.0.0:9092", nil))
	<-ctx.Done()
	nlog.Info("Stopping the metric exporter...")
}
