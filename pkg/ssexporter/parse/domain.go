// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package parse configures files and domain files
package parse

import (
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	jsoniter "github.com/json-iterator/go"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// GetIPFromDomain get a list of IP addresses from a local domain name
func GetIPFromDomain(domainName string) (ipList []string) {
	if IsIP(domainName) {
		ipList = append(ipList, domainName)
		return
	}
	ipAddresses, err := net.LookupIP(domainName)
	if err != nil {
		nlog.Warnf("Cannot find IP address: %s", err.Error())
		return
	}
	for _, ip := range ipAddresses {
		ipList = append(ipList, ip.String())
	}
	return
}

func IsIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip != nil
}

// GetClusterAddress get the address and port of a remote domain connected by a local domain
func GetClusterAddress(domainID string) (map[string][]string, error) {
	endpointAddresses := make(map[string][]string)
	// get the results of config_dump
	resp, err := http.Get("http://localhost:10000/config_dump?resource=dynamic_active_clusters")
	if err != nil {
		nlog.Warnf("Fail to get the results of config_dump %s", err.Error())
		return endpointAddresses, err
	}
	defer resp.Body.Close()
	// parse the results of config_dump
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		nlog.Error("Fail to parse the results of config_dump", err)
		return endpointAddresses, err
	}
	res := make(map[string]interface{})
	if err := jsoniter.Unmarshal(body, &res); err != nil {
		return endpointAddresses, err
	}
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
