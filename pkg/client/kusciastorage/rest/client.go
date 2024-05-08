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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Client defines a client which is used to access to kuscia storage service.
type Client struct {
	config     *config
	um         *urlMaker
	httpClient *http.Client
}

// New is used to create a client instance.
func New(endpoint string, options ...Option) (*Client, error) {
	conf := getDefaultConfig()

	for _, option := range options {
		option(conf)
	}

	um := &urlMaker{
		serverAPIPrefix: conf.serverAPIPrefix,
	}
	err := um.init(endpoint)
	if err != nil {
		return nil, err
	}

	client := &Client{
		config:     conf,
		um:         um,
		httpClient: &http.Client{},
	}

	client.init()
	return client, nil
}

// init is used to initialize client.
func (c *Client) init() {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   c.config.connectTimeout,
			KeepAlive: c.config.keepAliveTimeout,
		}).DialContext,
	}

	c.httpClient.Transport = transport
	c.httpClient.Timeout = c.config.readWriteTimeout
}

// CreateResource is used to create resource to kuscia storage service.
func (c *Client) CreateResource(
	domain, kind, kindInstName, resourceName string,
	normalReqBody *NormalRequestBody,
	refReqBody *ReferenceRequestBody) error {

	if (refReqBody == nil && normalReqBody == nil) || (refReqBody != nil && normalReqBody != nil) {
		return fmt.Errorf("reference request body or normal request body must exit")
	}

	var isRefRequest bool
	if refReqBody != nil {
		isRefRequest = true
	}

	host := c.um.getHost()
	url, err := c.um.buildURL(domain, kind, kindInstName, resourceName, isRefRequest)
	if err != nil {
		return err
	}

	var body []byte
	if refReqBody != nil {
		body, err = json.Marshal(refReqBody)
	} else {
		body, err = json.Marshal(normalReqBody)
	}
	if err != nil {
		return err
	}

	_, err = c.do(host, http.MethodPost, url, body)
	return err
}

// GetResource is used to get resource from kuscia storage service.
func (c *Client) GetResource(domain, kind, kindInstName, resourceName string) (string, error) {
	host := c.um.getHost()
	url, err := c.um.buildURL(domain, kind, kindInstName, resourceName, false)
	if err != nil {
		return "", err
	}

	resp, err := c.do(host, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	content, ok := resp.Content.(string)
	if !ok {
		return "", fmt.Errorf("response content can't convert to string type")
	}
	return content, nil
}

// DeleteResource is used to delete resource in the kuscia storage service
func (c *Client) DeleteResource(domain, kind, kindInstName, resourceName string) error {
	host := c.um.getHost()
	url, err := c.um.buildURL(domain, kind, kindInstName, resourceName, false)
	if err != nil {
		return err
	}

	_, err = c.do(host, http.MethodDelete, url, nil)
	return err
}

func (c *Client) do(host, method, url string, body []byte) (*response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Host = host
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	for i := 0; ; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil {
			break
		}

		if i >= c.config.retryTimes {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawRespBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rp := &response{}
	err = json.Unmarshal(rawRespBody, rp)
	if err != nil {
		return nil, err
	}

	if rp.Status != nil && rp.Status.Code != statusCodeForSuccess {
		return nil, fmt.Errorf("request failed, status code: %v, message: %q", rp.Status.Code, rp.Status.Message)
	}
	return rp, nil
}
