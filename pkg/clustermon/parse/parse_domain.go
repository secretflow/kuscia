package parse

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	//"fmt"
	"net/http"
	"strconv"
	"strings"
)

// GetDomainName get the name of a local domain
func GetDomainName() string {
	resp, err := http.Get("http://localhost:1054/handshake")
	if err != nil {
		log.Fatal(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var namespaceId map[string]string
	err = json.Unmarshal(body, &namespaceId)
	if err != nil {
		log.Fatal(err)
	}
	return namespaceId["namespace"]
	// return "root-kuscia-lite-" + namespaceId["namespace"]
}

// GetIpFromDomain get a list of IP addresses from a local domain name
func GetIpFromDomain(localDomainName string) []string {
	ipAddresses, err := net.LookupIP(localDomainName)
	var ipAddr []string
	if err != nil {
		log.Fatalln("Cannot find IP address:", err)
	}
	for _, ip := range ipAddresses {
		ipAddr = append(ipAddr, ip.String())
	}
	return ipAddr
}

// GetDestinationAddress get the address and port of a remote domain connected by a local domain
func GetDestinationAddress() (string, []string) {
	// get the results of config_dump
	resp, err := http.Get("http://localhost:10000/config_dump?format=json")
	if err != nil {
		log.Fatalln("Fail to get the results of config_dump", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)
	// parse the results of config_dump
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("Fail to parse the results of config_dump", err)
	}
	var configDumpData map[string]interface{}
	err = json.Unmarshal(body, &configDumpData)
	if err != nil {
		log.Fatalln("Fail to parse the results of config_dump", err)
	}
	//fmt.Println(configDumpData)
	configData := configDumpData["configs"].([]interface{})
	configData1 := configData[1].(map[string]interface{})
	configData2 := configData1["dynamic_active_clusters"].([]interface{})
	domainName := GetDomainName()
	var domainID = 0
	for id, cluster := range configData2 {
		clusterName := cluster.(map[string]interface{})["cluster"].(map[string]interface{})["load_assignment"].(map[string]interface{})["cluster_name"].(string)
		if strings.HasPrefix(clusterName, domainName+"-to-") {
			domainID = id
			break
		}
	}
	configData3 := configData2[domainID].(map[string]interface{})
	configData4 := configData3["cluster"].(map[string]interface{})
	configData5 := configData4["load_assignment"].(map[string]interface{})
	clusterName := configData5["cluster_name"].(string)

	endpoints := configData5["endpoints"].([]interface{})
	var destinationDomain []string
	for _, lbEndpoints := range endpoints {
		for _, lbEndPoint := range lbEndpoints.(map[string]interface{})["lb_endpoints"].([]interface{}) {
			socketAddress := lbEndPoint.(map[string]interface{})["endpoint"].(map[string]interface{})["address"].(map[string]interface{})["socket_address"].(map[string]interface{})
			address := socketAddress["address"].(string)
			portValue := int(socketAddress["port_value"].(float64))
			destinationDomain = append(destinationDomain, address+":"+strconv.Itoa(portValue))
		}
	}
	return clusterName, destinationDomain
}
