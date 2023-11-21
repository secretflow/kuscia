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
	clusterOutput, err := os.OpenFile("./../exp/exp3/monitoringdata", os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 777)
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
	NetworkMetrics, AggregationMetrics, ClusterMetrics, MonitorPeriods := parse.ReadConfig("config_test.yaml")
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
        		_, destinationAddress := parse.GetDestinationAddress()
			var clusterNames []string
			var clusterMetrics []string
			clusterNames = append(clusterNames, "alice-to-bob-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob1-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob2-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob3-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob4-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob5-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob6-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob7-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob8-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob9-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob10-HTTP")
			clusterNames = append(clusterNames, "alice-to-bob11-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob12-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob13-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob14-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob15-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob16-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob17-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob18-HTTP")
                        clusterNames = append(clusterNames, "alice-to-bob19-HTTP")
			for _, clusterName := range clusterNames{
				clusterMetric := netmon.ConvertClusterMetrics(ClusterMetrics, clusterName)
				for _, metric := range clusterMetric{
					clusterMetrics = append(clusterMetrics, metric)
				}
			}
			clusterMetricResults := netmon.GetClusterMetricResults(clusterNames, destinationAddress, clusterMetrics, AggregationMetrics)
			for _, clusterName := range clusterNames{
				// get cluster metrics
				//clusterMetricResults := netmon.GetClusterMetricResults(clusterName, destinationAddress, clusterMetrics, AggregationMetrics)
				// calculate the change values of cluster metrics
				clusterMetricValues[clusterName], clusterMetricResults[clusterName] = netmon.GetMetricChange(clusterMetricValues[clusterName], clusterMetricResults[clusterName])
				for _, dstAddr := range destinationAddress {
					clusterMetricValues[dstAddr], clusterMetricResults[dstAddr] = netmon.GetMetricChange(clusterMetricValues[dstAddr], clusterMetricResults[dstAddr])
				}
				// update cluster metrics in prometheus
				// metric_export.UpdateMetrics(clusterMetricResults, MetricTypes)
				// records the cluster metric results
			}
			metric_export.UpdateMetrics(clusterMetricResults, MetricTypes)
			netmon.LogClusterMetricResults(clusterOutput, clusterMetricResults)
			time.Sleep(time.Duration(MonitorPeriods) * time.Second)
		}
	}(ClusterMetrics, MetricTypes, MonitorPeriods)

	// get the usage of system resources
	sysOutput, err := os.OpenFile("./../exp/exp3/sysdata", os.O_CREATE|os.O_RDWR|os.O_TRUNC|os.O_APPEND, 777)
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
