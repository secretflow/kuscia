package netmon

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"E2EMon/parse"
)

// GetStatisticFromSs get the statistics of network flows from ss
func GetStatisticFromSs() []map[string]string {
	// execute ss comand
	cmd := exec.Command("ss", "-tio4n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Cannot get ss.")
	}
	// parse statistics from ss
	lines := strings.Split(string(output), "\n")
	var tcpStatisticList []map[string]string
	ssMetrics := make(map[string]string)
	ssMetrics["total_connections"] = "0"
	ssMetrics["retrans"] = "0"
	for idx, line := range lines {
		if idx < 1 {
			continue
		}
		fields := strings.Fields(line)
		if idx%2 == 1 {
			if len(fields) < 5 {
				continue
			}
			if len(fields) == 6 {
                                ssMetrics["retrans"] = strings.Split(strings.Split(fields[5], ",")[2], ")")[0]
                                ssMetrics["total_connections"] = "1"
                        }
			ssMetrics["localAddr"] = fields[3]
			ssMetrics["peerAddr"] = fields[4]
		} else {
			if len(fields) < 33 {
				continue
			}
			ssMetrics["rto"] = strings.Split(fields[4], ":")[1]
			rtt := strings.Split(fields[5], ":")[1]
			ssMetrics["rtt"] = strings.Split(rtt, "/")[0]
			ssMetrics["var_rtt"] = strings.Split(rtt, "/")[1]
			ssMetrics["delivery_rate"] = strings.Split(fields[27], "bps")[0]
			ssMetrics["bytes_send"] = strings.Split(fields[12], ":")[1]
			ssMetrics["bytes_received"] = strings.Split(fields[14], ":")[1]
			ssMetrics["min_rtt"] = strings.Split(fields[32], ":")[1]
			tcpStatisticList = append(tcpStatisticList, ssMetrics)
			ssMetrics = make(map[string]string)
			ssMetrics["total_connections"] = "0"
			ssMetrics["retrans"] = "0"
		}
	}
	return tcpStatisticList
}

// GetStatisticFromEnvoy get statistics of network flows from envoy
func GetStatisticFromEnvoy(metrics []string) map[string]float64 {
	// get statistics from envoy
	resp, err := http.Get("http://localhost:10000/stats?format=json")
	if err != nil {
		log.Fatalln("Fail to get the statistics from envoy", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("Fail to read the results of envoy ", err)
	}
	// parse data from envoy
	var statsData map[string]interface{}
	err = json.Unmarshal(body, &statsData)
	if err != nil {
		log.Fatalln("Fail to parse the results of envoy", err)
	}
	stats := make(map[string]float64)
	metricSet := make(map[string]bool)
	for _, metric := range metrics {
		metricSet[metric] = true
	}
	for _, metric := range statsData["stats"].([]interface{}) {
		if name, ok := metric.(map[string]interface{})["name"].(string); ok {
			if _, ok := metricSet[name]; ok {
				if value, ok := metric.(map[string]interface{})["value"].(float64); ok {
					stats[name] = value
				} else if str, ok := metric.(map[string]interface{})["value"].(string); ok {
					value, err := strconv.ParseFloat(str, 64)
					if err != nil {
						log.Fatalln(name, "Cannot convert ", str, "into type float64:", err)
					} else {
						stats[name] = value
					}
				}
			}
		}
	}
	return stats
}

// ConvertClusterMetrics convert cluster metrics for prometheus
func ConvertClusterMetrics(ClusterMetrics []string, clusterName string) []string {
	var clusterMetrics []string
	for _, metric := range ClusterMetrics {
		str := "cluster." + clusterName + "." + strings.ToLower(metric)
		clusterMetrics = append(clusterMetrics, str)
	}
	return clusterMetrics
}

// Filter filter network flows according to given five-tuple rules
func Filter(ssMetrics []map[string]string, srcIP string, dstIP string, srcPort string, dstPort string, protoNum string) []map[string]string {
	// parse the five-tuple rule, wildcard "*" means any field is included
	var results []map[string]string
	for _, metrics := range ssMetrics {
		src := strings.Split(metrics["localAddr"], ":")
		dst := strings.Split(metrics["peerAddr"], ":")
		if (srcIP != "*") && (src[0] != srcIP) {
			continue
		}
		if (dstIP != "*") && (dst[0] != dstIP) {
			continue
		}
		if (srcPort != "*") && (src[1] != srcPort) {
			continue
		}
		if (dstPort != "*") && (dst[1] != dstPort) {
			continue
		}
		if protoNum != "*" {
			continue
		}
		results = append(results, metrics)
	}
	return results
}

// Sum an aggregation function to sum up two network metrics
func Sum(metrics []map[string]string, key string) float64 {
	sum := 0.0
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			log.Fatalln("fail to parse float", metric[key],"key:",key)
		}
		sum += val
	}
	return sum
}

// Avg an aggregation function to calculate the average of two network metrics
func Avg(metrics []map[string]string, key string) float64 {
	return Sum(metrics, key) / float64(len(metrics))
}

// Max an aggregation function to calculate the maximum of two network metrics
func Max(metrics []map[string]string, key string) float64 {
	max := math.MaxFloat64 * (-1)
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			log.Fatalln("fail to parse float")
		}
		if val > max {
			max = val
		}
	}
	return max
}

// Min an aggregation function to calculate the minimum of two network metrics
func Min(metrics []map[string]string, key string) float64 {
	min := math.MaxFloat64
	for _, metric := range metrics {
		val, err := strconv.ParseFloat(metric[key], 64)
		if err != nil {
			log.Fatalln("fail to parse float")
		}
		if val < min {
			min = val
		}
	}
	return min
}

// Rate an aggregation function to calculate the rate of a network metric betweem to metrics
func Rate(metric1 float64, metric2 float64) float64 {
	if metric2 == 0.0{
		return 0
	}
	return metric1 / metric2
}

// Alert an alert function that reports whether a metric value exceeds a given threshold
func Alert(metric float64, threshold float64) bool {
	return metric > threshold
}

// AggregateStatistics aggregate statistics using an aggregation function
func AggregateStatistics(clusterResults map[string]float64, networkResults []map[string]string, aggregationMetrics map[string]string, dstDomain string, MonitorPeriods int) map[string]float64 {
	if(len(networkResults) == 0){
		return clusterResults
	}
	for metric := range networkResults[0] {
		aggFunc := aggregationMetrics[metric]
		if metric != "localAddr" && metric != "peerAddr" {
			if metric == "retrans" {
				threshold := 0.0
				clusterResults["cluster." + dstDomain + "." + "retran_rate"] = Rate(Sum(networkResults, "retrans") - threshold, Sum(networkResults, "total_connections"))
				continue
			}
			if aggFunc == "sum" {
				clusterResults["cluster." + dstDomain + "." + metric] = Sum(networkResults, metric)
			} else if aggFunc == "avg" {
				clusterResults["cluster." + dstDomain + "." + metric] = Avg(networkResults, metric)
			} else if aggFunc == "max" {
				clusterResults["cluster." + dstDomain + "." + metric] = Max(networkResults, metric)
			} else if aggFunc == "min" {
				clusterResults["cluster." + dstDomain + "." + metric] = Min(networkResults, metric)
			}
			if metric == "bytes_send" || metric == "bytes_received" {
				clusterResults["cluster." + dstDomain + "." + metric] = Rate(clusterResults["cluster." + dstDomain + "." + metric], float64(MonitorPeriods))
			}
		}
	}
	return clusterResults
}

// GetClusterMetricResults Get the results of cluster statistics after filtering
func GetClusterMetricResults(clusterName string, remoteAddress []string, clusterMetrics []string, AggregationMetrics map[string]string, MonitorPeriods int) map[string]map[string]float64 {
	//_, addrToPort := parse.GetRemoteAddrAndPort()
	// get the statistics from SS
	ssMetrics := GetStatisticFromSs()
	// get the statistics from envoy
	clusterStatistics := GetStatisticFromEnvoy(clusterMetrics)
	// get the source domain name
	localDomain := parse.GetLocalDomainName()
	// get the source/destination IP from domain names
	sourceIP := parse.GetIpFromDomain("root-kuscia-lite-" + localDomain)
	destinationIP := make(map[string][]string)
	for _, remoteDomainName := range remoteAddress {
		destinationIP[remoteDomainName] = parse.GetIpFromDomain(strings.Split(remoteDomainName, ":")[0])
	}
	// group metrics by the domain name
	clusterResults := make(map[string]map[string]float64)
	clusterResults[clusterName] = make(map[string]float64)
	for metric, value := range clusterStatistics {
		clusterResults[clusterName][metric] = value
	}
	for _, dstAddr := range remoteAddress {
		// get the connection name
		dstDomain := strings.Split(dstAddr, ":")[0]
		clusterResults[dstDomain] = make(map[string]float64)
		var networkResults []map[string]string
		dstPort := strings.Split(dstAddr, ":")[1]
		for _, srcIP := range sourceIP {
			for _, dstIP := range destinationIP[dstAddr] {
				networkResult := Filter(ssMetrics, srcIP, dstIP, "*", dstPort, "*")
				networkResults = append(networkResults, networkResult...)
			}
		}
		clusterResults[dstDomain] = AggregateStatistics(clusterResults[dstDomain], networkResults, AggregationMetrics, dstDomain, MonitorPeriods)
	}
	return clusterResults
}

// GetMetricChange get the change values of metrics
func GetMetricChange(MetricTypes map[string]string, lastMetricValues map[string]float64, currentMetricValues map[string]float64) (map[string]float64, map[string]float64) {
	for metric, value := range currentMetricValues {
		currentMetricValues[metric] = currentMetricValues[metric] - lastMetricValues[metric]
		if(currentMetricValues[metric] < 0){
			currentMetricValues[metric] = 0
		}
		lastMetricValues[metric] = value

	}
	return lastMetricValues, currentMetricValues
}

// LogClusterMetricResults write the results of cluster statistics into a file
func LogClusterMetricResults(clusterOutput *os.File, clusterResults map[string]map[string]float64) {
	for clusterName, clusterResult := range clusterResults {
		clusterOutput.WriteString(clusterName + "\n")
		for metric, val := range clusterResult {
			clusterOutput.WriteString(metric + ":" + fmt.Sprintf("%f", val) + ";\n")
		}
		// clusterOutput.WriteString("\n")
	}
}

