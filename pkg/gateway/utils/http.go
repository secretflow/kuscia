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

package utils

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const KusciaEnvoyMsgHeaderKey = "Kuscia-Error-Message"

type HTTPParam struct {
	Method       string
	Path         string
	ClusterName  string
	KusciaSource string
	KusciaHost   string
	Headers      map[string]string
}

func ParseURL(url string) (string, string, uint32, string, error) {
	var protocol, hostPort, host, path string
	var port int
	var err error
	if strings.HasPrefix(url, "http://") {
		protocol = "http"
		hostPort = url[7:]
	} else if strings.HasPrefix(url, "https://") {
		protocol = "https"
		hostPort = url[8:]
	} else {
		return protocol, host, uint32(port), path, fmt.Errorf("invalid host: %s", url)
	}

	parts := strings.SplitN(hostPort, "/", 2)
	hostPort = parts[0]
	if len(parts) > 1 {
		path = "/" + parts[1]
	}

	fields := strings.Split(hostPort, ":")
	host = fields[0]
	if len(fields) == 2 {
		if port, err = strconv.Atoi(fields[1]); err != nil {
			return protocol, host, uint32(port), path, err
		}
	} else {
		if protocol == "http" {
			port = 80
		} else {
			port = 443
		}
	}

	return protocol, host, uint32(port), path, nil
}

func DoHTTPWithRetry(in interface{}, out interface{}, hp *HTTPParam, waitTime time.Duration, maxRetryTimes int) error {
	var err error
	for i := 0; i < maxRetryTimes; i++ {
		err = DoHTTP(in, out, hp)
		if err == nil {
			return nil
		}
		time.Sleep(waitTime)
	}
	return err
}

func DoHTTP(in interface{}, out interface{}, hp *HTTPParam) error {
	var req *http.Request
	var err error
	if hp.Method == http.MethodGet {
		req, err = http.NewRequest(http.MethodGet, InternalServer+hp.Path, nil)
		if err != nil {
			return fmt.Errorf("invalid request, detail -> %s", err.Error())
		}
	} else {
		inbody, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("invalid request, detail -> %s", err.Error())
		}
		req, err = http.NewRequest(hp.Method, InternalServer+hp.Path, bytes.NewBuffer(inbody))
		if err != nil {
			return fmt.Errorf("invalid request, detail -> %s", err.Error())
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(fmt.Sprintf("%s-Cluster", ServiceHandshake), hp.ClusterName)
	req.Header.Set("Kuscia-Source", hp.KusciaSource)
	req.Header.Set("kuscia-Host", hp.KusciaHost)
	for key, val := range hp.Headers {
		req.Header.Set(key, val)
	}
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request error, detail -> %s", err.Error())
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body error, detail -> %s", err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		if len(body) > 200 {
			body = body[:200]
		}
		return fmt.Errorf("response status code [%d], detail -> %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, out); err != nil {
		if len(body) > 200 {
			body = body[:200]
		}
		return fmt.Errorf("invalid response body, detail -> %s", string(body))
	}
	return nil
}

func ProbePeerEndpoint(endpointURL string) error {

	if endpointURL == "" {
		return fmt.Errorf("endpoint URL is empty")
	}

	req, err := http.NewRequest("GET", endpointURL, nil)
	if err != nil {
		return fmt.Errorf("endpoint URL is invalid: %v", err)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: time.Second * 5, Transport: tr}

	resp, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("sending request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && hasNonEmptyHeaderValue(resp.Header, KusciaEnvoyMsgHeaderKey) {
		return nil
	}
	nlog.Infof("response header: %+v", resp.Header)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unexpected status code: %d, read response body error, detail -> %s", resp.StatusCode, err.Error())
	}
	respbody := string(body)
	return fmt.Errorf("unexpected status code: %d, response body: %s", resp.StatusCode, respbody)

}

func hasNonEmptyHeaderValue(header http.Header, key string) bool {

	if values, ok := header[key]; ok {
		if len(values) == 0 {
			return false
		}
		return values[0] != ""
	}
	return false
}
