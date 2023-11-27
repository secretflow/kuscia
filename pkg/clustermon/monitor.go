package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"E2EMon/metric"
	"E2EMon/netmon"
	"E2EMon/parse"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// read the config file
	NetworkMetrics, AggregationMetrics, ClusterMetrics, MonitorPeriod, Prometheus, LocalFile := parse.ReadConfig("config.yaml")
	// initialize the output file
	var clusterOutput *os.File
	if LocalFile {
		var err error
		clusterOutput, err = os.OpenFile("./monitoringdata", os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 777)
		if err != nil {
			log.Fatalln("Fail to open the file", "monitoringdata", err)
		}
		defer func(clusterOutput *os.File) {
			err := clusterOutput.Close()
			if err != nil {
				log.Fatalln("Fail to close the file", "monitoringdata", err)
			}
		}(clusterOutput)
	}
	// get clusterName and clusterAddress
	clusterName, clusterAddress := parse.GetClusterAddress()
	localDomainName := parse.GetLocalDomainName()
	// get the cluster metrics to be monitored
	clusterMetrics := netmon.ConvertClusterMetrics(ClusterMetrics, clusterName)
	var MetricTypes = metric.NewMetricTypes()
	// register metrics for prometheus and initialize the calculation of change values
	var reg *prometheus.Registry 
	if Prometheus {
		reg = metric.ProduceMetrics(localDomainName, clusterName, clusterAddress, NetworkMetrics, ClusterMetrics, MetricTypes, AggregationMetrics)
	}
	lastClusterMetricValues := netmon.GetClusterMetricResults(localDomainName, clusterName, clusterAddress, clusterMetrics, AggregationMetrics, MonitorPeriod)

	fmt.Println("Start to monitor the cluster...")
	// monitor the cluster metrics
	ticker := time.NewTicker(time.Duration(MonitorPeriod) * time.Second)
	defer ticker.Stop()
	go func(ClusterMetrics []string, MetricTypes map[string]string, MonitorPeriods int, lastClusterMetricValues map[string]map[string]float64, Prometheus bool, LocalFile bool) {
		for range ticker.C {
			// get clusterName and clusterAddress
			clusterName, clusterAddress = parse.GetClusterAddress()
			// get the cluster metrics to be monitored
			clusterMetrics = netmon.ConvertClusterMetrics(ClusterMetrics, clusterName)
			// get cluster metrics
			currentClusterMetricValues := netmon.GetClusterMetricResults(localDomainName, clusterName, clusterAddress, clusterMetrics, AggregationMetrics, MonitorPeriods)
			// calculate the change values of cluster metrics
			lastClusterMetricValues[clusterName], currentClusterMetricValues[clusterName] = netmon.GetMetricChange(MetricTypes, lastClusterMetricValues[clusterName], currentClusterMetricValues[clusterName])
			for _, dstAddr := range clusterAddress {
				dstDomain := strings.Split(dstAddr, ":")[0]
				lastClusterMetricValues[dstDomain], currentClusterMetricValues[dstDomain] = netmon.GetMetricChange(MetricTypes, lastClusterMetricValues[dstDomain], currentClusterMetricValues[dstDomain])
			}
			// update cluster metrics in prometheus
			if Prometheus {
				metric.UpdateMetrics(currentClusterMetricValues, MetricTypes)
			}
			if LocalFile {
				netmon.LogClusterMetricResults(clusterOutput, currentClusterMetricValues)
			}
		}
	}(ClusterMetrics, MetricTypes, MonitorPeriod, lastClusterMetricValues, Prometheus, LocalFile)
	if Prometheus {
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
		log.Fatalln(http.ListenAndServe(ipAddresses[0].String()+":9091", nil))
	}
}
