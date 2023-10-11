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
	"E2EMon/sysmon"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// initialize the output file
	clusterOutput, err := os.OpenFile("./monitor_data", os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 777)
	if err != nil {
		log.Fatalln("Fail to open the file", "monitor_data", err)
	}
	defer func(clusterOutput *os.File) {
		err := clusterOutput.Close()
		if err != nil {
			log.Fatalln("Fail to close the file", "monitor_data", err)
		}
	}(clusterOutput)
	// read the config file
	NetworkMetrics, AggregationMetrics, ClusterMetrics, MonitorPeriods := parse.ReadConfig("config.yaml")
	// get clusterName and destinationAddress
	clusterName, destinationAddress := parse.GetDestinationAddress()
	// get the cluster metrics to be monitored
	clusterMetrics := netmon.ConvertClusterMetrics(ClusterMetrics, clusterName)

	var MetricTypes = metric_types.NewMetricTypes()
	// get the pid
	pid := fmt.Sprintf("%d", os.Getpid())
	// report to prometheus
	reg := metric_export.ProduceMetrics(clusterName, NetworkMetrics, ClusterMetrics, MetricTypes)
	clusterName = strings.Replace(clusterName, "-", "_", -1)

	// get cluster metric value to calculate metric change
	var clusterMetricValues = make(map[string]map[string]float64)
	for _, dstAddr := range destinationAddress {
		clusterMetricValues[strings.Split(dstAddr, ":")[0]] = make(map[string]float64)
	}
	// initialize the cluster metric value to calculate change
	for _, clusterMetricValue := range clusterMetricValues {
		clusterMetricValue = make(map[string]float64)
		for _, metric := range clusterMetrics {
			clusterMetricValue[metric] = 0
		}
	}
	fmt.Println("Start to monitor the cluster metrics...")
	// monitor the cluster metrics
	go func(ClusterMetrics []string, MetricTypes map[string]string, MonitorPeriods int) {
		for {
			// get clusterName and destinationAddress
			clusterName, destinationAddress := parse.GetDestinationAddress()
			// get the cluster metrics to be monitored
			clusterMetrics := netmon.ConvertClusterMetrics(ClusterMetrics, clusterName)
			// get cluster metrics
			clusterMetricResults := netmon.GetClusterMetricResults(clusterName, destinationAddress, clusterMetrics, AggregationMetrics) //netmon.Get_stats(cluMetrics)
			// calculate the change values of cluster metrics
			clusterMetricValues[clusterName], clusterMetricResults[clusterName] = netmon.GetMetricChange(clusterMetricValues[clusterName], clusterMetricResults[clusterName])
			for _, dstAddr := range destinationAddress {
				clusterMetricValues[dstAddr], clusterMetricResults[dstAddr] = netmon.GetMetricChange(clusterMetricValues[dstAddr], clusterMetricResults[dstAddr])
			}
			// update cluster metrics in prometheus
			metric_export.UpdateMetrics(clusterMetricResults, MetricTypes)
			// records the cluster metric results
			netmon.LogClusterMetricResults(clusterOutput, clusterMetricResults)
			time.Sleep(time.Duration(MonitorPeriods) * time.Second)
		}
	}(clusterMetrics, MetricTypes, MonitorPeriods)

	// get the usage of system resources
	sysOutput, err := os.OpenFile("./sysdata", os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 777)
	if err != nil {
		log.Fatalln("Cannot open the file of sysdata", err)
	}
	defer func(sysOutput *os.File) {
		err := sysOutput.Close()
		if err != nil {
			log.Fatalln("Cannot close the file of sysdata", err)
		}
	}(sysOutput)
	var initSystemResults []float64
	var systemResults []float64
	go func(pid string, MonitorPeriods int, sysOutput *os.File, initSystemResults []float64, systemResults []float64) {
		for {
			systemResults = systemResults[0:0]
			initSystemResults, systemResults = sysmon.GetSystemResourceUsage(pid, initSystemResults, systemResults)
			sysmon.LogSystemResourceUsage(sysOutput, systemResults)
			time.Sleep(time.Duration(MonitorPeriods) * time.Second)
		}
	}(pid, MonitorPeriods, sysOutput, initSystemResults, systemResults)

	// export to the prometheus
	http.Handle(
		"/metrics", promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			}),
	)
	ipAddresses, err := net.LookupIP("root-kuscia-lite-" + parse.GetDomainName())
	if err != nil {
		log.Fatalln("Cannot find IP address:", err)
	}
	log.Fatalln(http.ListenAndServe(ipAddresses[0].String()+":8080", nil))
}
