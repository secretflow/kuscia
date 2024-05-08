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

package rest

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/secretflow/kuscia/proto/api/v1alpha1"
)

const (
	httpPrefix  = "http://"
	httpsPrefix = "https://"
)

const (
	normalDomainRequestURL     = "/api/v1/namespaces/%s/%s/%s/%s"
	referenceDomainRequestURL  = normalDomainRequestURL + "/reference"
	normalClusterRequestURL    = "/api/v1/%s/%s/%s"
	referenceClusterRequestURL = normalClusterRequestURL + "/reference"
)

const (
	statusCodeForSuccess = int32(200)
)

// ReferenceRequestBody defines reference request body.
type ReferenceRequestBody struct {
	Domain       string `json:"domain,omitempty"`
	Kind         string `json:"kind"`
	KindInstName string `json:"kind_instance_name"`
	ResourceName string `json:"resource_name"`
}

// NormalRequestBody defines normal request body.
type NormalRequestBody struct {
	Content string `json:"content"`
}

// response defines response info of kuscia storage service.
type response struct {
	Status  *v1alpha1.Status `json:"status"`
	Content interface{}      `json:"content,omitempty"`
}

// urlMaker is used to build URL.
type urlMaker struct {
	scheme          string
	netLoc          string
	host            string
	serverAPIPrefix string
}

// init is used to initialize urlMaker.
func (um *urlMaker) init(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("endpoint can't be empty")
	}

	if strings.HasPrefix(endpoint, httpPrefix) {
		um.scheme = "http"
		um.netLoc = endpoint[len(httpPrefix):]
	} else if strings.HasPrefix(endpoint, httpsPrefix) {
		um.scheme = "https"
		um.netLoc = endpoint[len(httpsPrefix):]
	} else {
		um.scheme = "http"
		um.netLoc = endpoint
	}

	rawURL := um.scheme + "://" + um.netLoc
	parseURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	um.netLoc = parseURL.Host
	host, _, err := net.SplitHostPort(um.netLoc)
	if err != nil {
		host = um.netLoc
		if len(host) > 0 && host[0] == '[' && host[len(host)-1] == ']' {
			host = host[1 : len(host)-1]
		}
	}
	um.host = host
	return nil
}

// buildURL is used to get request url.
func (um *urlMaker) buildURL(domain, kind, kindInstName, resourceName string, isRefRequest bool) (string, error) {
	if kind == "" || kindInstName == "" || resourceName == "" {
		return "", fmt.Errorf("kind, kind instance name and resource name can't be empty")
	}

	urlPrefix := um.scheme + "://" + um.netLoc
	if um.serverAPIPrefix != "" {
		urlPrefix = urlPrefix + "/" + um.serverAPIPrefix
	}

	apiPath := ""
	switch isRefRequest {
	case true:
		if domain != "" {
			apiPath = fmt.Sprintf(referenceDomainRequestURL, domain, kind, kindInstName, resourceName)
		} else {
			apiPath = fmt.Sprintf(referenceClusterRequestURL, kind, kindInstName, resourceName)
		}
	default:
		if domain != "" {
			apiPath = fmt.Sprintf(normalDomainRequestURL, domain, kind, kindInstName, resourceName)
		} else {
			apiPath = fmt.Sprintf(normalClusterRequestURL, kind, kindInstName, resourceName)
		}
	}
	return urlPrefix + apiPath, nil
}

// getHost is used to get host.
func (um *urlMaker) getHost() string {
	return um.host
}
