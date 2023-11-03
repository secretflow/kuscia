package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"E2EMon/metric_export"
	"E2EMon/metric_types"
	"E2EMon/netmon"
	"E2EMon/parse"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// initialize the output file
	clusterOutput, err := os.OpenFile("./monitoringdata", os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 777)
	if err != nil {
		log.Fatalln("Fail to open the file", "monitoringdata", err)
	}
	defer func(clusterOutput *os.File) {
		err := clusterOutput.Close()
		if err != nil {
			log.Fatalln("Fail to close the file", "monitoringdata", err)
		}
	}(clusterOutput)
	// read the config file
	NetworkMetrics, AggregationMetrics, ClusterMetrics, MonitorPeriods := parse.ReadConfig("config.yaml")
	// get clusterName and clusterAddress
	clusterName, clusterAddress := parse.GetClusterAddress()
	//localDomain := "root-kuscia-lite-" + parse.GetLocalDomainName()
	// get the cluster metrics to be monitored
	clusterMetrics := netmon.ConvertClusterMetrics(ClusterMetrics, clusterName)

	var MetricTypes = metric_types.NewMetricTypes()
	// register metrics for prometheus and initialize the calculation of change values
	reg := metric_export.ProduceMetrics(clusterName, clusterAddress, NetworkMetrics, ClusterMetrics, MetricTypes)
	lastClusterMetricValues := netmon.GetClusterMetricResults(clusterName, clusterAddress, clusterMetrics, AggregationMetrics, MonitorPeriods)
	fmt.Println("Start to monitor the cluster...")
	// monitor the cluster metrics
	go func(ClusterMetrics []string, MetricTypes map[string]string, MonitorPeriods int, lastClusterMetricValues map[string]map[string]float64) {
		for {
			// get clusterName and clusterAddress
			clusterName, clusterAddress := parse.GetClusterAddress()
			// get the cluster metrics to be monitored
			clusterMetrics := netmon.ConvertClusterMetrics(ClusterMetrics, clusterName)
			// get cluster metrics
			currentClusterMetricValues := netmon.GetClusterMetricResults(clusterName, clusterAddress, clusterMetrics, AggregationMetrics, MonitorPeriods)
			// calculate the change values of cluster metrics
			lastClusterMetricValues[clusterName], currentClusterMetricValues[clusterName] = netmon.GetMetricChange(MetricTypes, lastClusterMetricValues[clusterName], currentClusterMetricValues[clusterName])
			for _, dstAddr := range clusterAddress {
				dstDomain := strings.Split(dstAddr, ":")[0]
				lastClusterMetricValues[dstDomain], currentClusterMetricValues[dstDomain] = netmon.GetMetricChange(MetricTypes, lastClusterMetricValues[dstDomain], currentClusterMetricValues[dstDomain])
			}
			// update cluster metrics in prometheus
			metric_export.UpdateMetrics(currentClusterMetricValues, MetricTypes)
			netmon.LogClusterMetricResults(clusterOutput, currentClusterMetricValues)
			time.Sleep(time.Duration(MonitorPeriods) * time.Second)
		}
	}(ClusterMetrics, MetricTypes, MonitorPeriods, lastClusterMetricValues)

	// export to the prometheus
	http.Handle(
		"/metrics", promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			}),
	)
	ipAddresses, err := net.LookupIP("root-kuscia-lite-" + parse.GetLocalDomainName())
	if err != nil {
		log.Fatalln("Cannot find IP address:", err)
	}
	log.Fatalln(http.ListenAndServe(ipAddresses[0].String()+":8080", nil))
}
