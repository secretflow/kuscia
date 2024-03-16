// Package parse configures files and domain files
package parse

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// GetIPFromDomain get a list of IP addresses from a local domain name
func GetIPFromDomain(localDomainName string) []string {
	ipAddresses, err := net.LookupIP(localDomainName)
	var ipAddr []string
	if err != nil {
		nlog.Error("Cannot find IP address:", err)
	}
	for _, ip := range ipAddresses {
		ipAddr = append(ipAddr, ip.String())
	}
	return ipAddr
}

// GetClusterAddress get the address and port of a remote domain connected by a local domain
func GetClusterAddress(domainID string) (map[string][]string, error) {
	endpointAddresses := make(map[string][]string)
	// get the results of config_dump
	resp, err := http.Get("http://localhost:10000/config_dump?resource=dynamic_active_clusters")
	if err != nil {
		nlog.Error("Fail to get the results of config_dump", err)
		return endpointAddresses, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)
	// parse the results of config_dump
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		nlog.Error("Fail to parse the results of config_dump", err)
		return endpointAddresses, err
	}
	res := make(map[string]interface{})
	err = jsoniter.Unmarshal(body, &res)
	configs := jsoniter.Get(body, "configs")

	for i := 0; ; i++ {
		x := configs.Get(i)
		if x.Size() == 0 {
			break
		}
		loadAssignment := x.Get("cluster", "load_assignment")
		clusterName := loadAssignment.Get("cluster_name").ToString()

		if !strings.HasPrefix(clusterName, domainID+"-to-") {
			break
		}
		endpoints := loadAssignment.Get("endpoints")

		for j := 0; ; j++ {
			lbEndpoints := endpoints.Get(j, "lb_endpoints")
			if lbEndpoints.Size() == 0 {
				break
			}
			for k := 0; ; k++ {
				endpoint := lbEndpoints.Get(k)
				if endpoint.Size() == 0 {
					break
				}
				socketAddress := endpoint.Get("endpoint", "address", "socket_address")
				address := socketAddress.Get("address")
				portValue := socketAddress.Get("port_value")
				endpointAddress := address.ToString() + ":" + portValue.ToString()
				endpointAddresses[clusterName] = append(endpointAddresses[clusterName], endpointAddress)
			}
		}
	}
	return endpointAddresses, nil
}
