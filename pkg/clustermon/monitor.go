package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	//"strings"
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
		clusterOutput, err = os.OpenFile("./monitoring_data", os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 777)
		if err != nil {
			log.Fatalln("Fail to open the file", "monitoring_data", err)
		}
		defer func(clusterOutput *os.File) {
			err := clusterOutput.Close()
			if err != nil {
				log.Fatalln("Fail to close the file", "monitoring_data", err)
			}
		}(clusterOutput)
	}
	// get clusterName and clusterAddress
	clusterAddresses := parse.GetClusterAddress()
	localDomainName := parse.GetLocalDomainName()
	// get the cluster metrics to be monitored
	clusterMetrics := netmon.ConvertClusterMetrics(ClusterMetrics, clusterAddresses)
	var MetricTypes = metric.NewMetricTypes()
	// register metrics for prometheus and initialize the calculation of change values
	var reg *prometheus.Registry
	if Prometheus {
		reg = metric.ProduceMetrics(localDomainName, clusterAddresses, NetworkMetrics, ClusterMetrics, MetricTypes, AggregationMetrics)
	}
	lastClusterMetricValues := netmon.GetClusterMetricResults(localDomainName, clusterAddresses, clusterMetrics, AggregationMetrics, MonitorPeriod)
	fmt.Println("Start to monitor the cluster...")
	// monitor the cluster metrics
	ticker := time.NewTicker(time.Duration(MonitorPeriod) * time.Second)
	defer ticker.Stop()
	go func(ClusterMetrics []string, MetricTypes map[string]string, MonitorPeriods int, lastClusterMetricValues map[string]float64, Prometheus bool, LocalFile bool) {
		for range ticker.C {
			// get clusterName and clusterAddress
			clusterAddresses = parse.GetClusterAddress()
			// get the cluster metrics to be monitored
			clusterMetrics = netmon.ConvertClusterMetrics(ClusterMetrics, clusterAddresses)
			// get cluster metrics
			currentClusterMetricValues := netmon.GetClusterMetricResults(localDomainName, clusterAddresses, clusterMetrics, AggregationMetrics, MonitorPeriods)
			// calculate the change values of cluster metrics
			//for clusterName := range clusterAddresses {
			lastClusterMetricValues, currentClusterMetricValues = netmon.GetMetricChange(lastClusterMetricValues, currentClusterMetricValues)

			//}
			/*for _, dstAddr := range clusterAddresses[clusterName] {
				dstDomain := strings.Split(dstAddr, ":")[0]
				lastClusterMetricValues[dstDomain], currentClusterMetricValues[dstDomain] = netmon.GetMetricChange(lastClusterMetricValues[dstDomain], currentClusterMetricValues[dstDomain])
			}*/

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
		//goland:noinspection ALL
		ipAddresses, err := net.LookupIP("root-kuscia-lite-" + parse.GetLocalDomainName())
		if err != nil {
			log.Fatalln("Cannot find IP address:", err)
		}
		log.Fatalln(http.ListenAndServe(ipAddresses[0].String()+":9091", nil))
	}
}
