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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURLMakerInit(t *testing.T) {
	um := &urlMaker{}
	testCases := []struct {
		name        string
		endpoint    string
		expectedErr bool
	}{
		{
			name:        "empty endpoint",
			endpoint:    "",
			expectedErr: true,
		},
		{
			name:        "http endpoint",
			endpoint:    "http://localhost:8080",
			expectedErr: false,
		},
		{
			name:        "https endpoint",
			endpoint:    "https://localhost/kuscia-storage",
			expectedErr: false,
		},
		{
			name:        "endpoint without scheme",
			endpoint:    "127.0.0.1",
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := um.init(tc.endpoint)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestURLMakerGetURL(t *testing.T) {
	type args struct {
		domain       string
		kind         string
		kindInstName string
		resourceName string
		isRefRequest bool
	}

	testCases := []struct {
		um          *urlMaker
		name        string
		args        args
		apiPath     string
		expectedErr bool
	}{
		{
			name: "empty kind",
			um: &urlMaker{
				scheme: "http",
				netLoc: "localhost",
			},
			args: args{
				domain:       "alice",
				kindInstName: "test",
				resourceName: "config",
			},
			expectedErr: true,
		},
		{
			name: "empty kind instance name",
			um: &urlMaker{
				scheme: "http",
				netLoc: "localhost",
			},
			args: args{
				domain:       "alice",
				kind:         "pods",
				resourceName: "config",
			},
			expectedErr: true,
		},
		{
			name: "empty resource name",
			um: &urlMaker{
				scheme: "http",
				netLoc: "localhost",
			},
			args: args{
				domain:       "alice",
				kind:         "pods",
				kindInstName: "test",
			},
			expectedErr: true,
		},
		{
			name: "url for domain resource",
			um: &urlMaker{
				scheme: "http",
				netLoc: "localhost",
			},
			args: args{
				domain:       "alice",
				kind:         "pods",
				kindInstName: "test",
				resourceName: "config",
			},
			apiPath:     normalDomainRequestURL,
			expectedErr: false,
		},
		{
			name: "url for reference domain resource",
			um: &urlMaker{
				scheme: "http",
				netLoc: "localhost",
			},
			args: args{
				domain:       "alice",
				kind:         "pods",
				kindInstName: "test",
				resourceName: "config",
				isRefRequest: true,
			},
			apiPath:     referenceDomainRequestURL,
			expectedErr: false,
		},
		{
			name: "url for cluster resource",
			um: &urlMaker{
				scheme: "http",
				netLoc: "localhost",
			},
			args: args{
				kind:         "appimages",
				kindInstName: "test",
				resourceName: "config",
			},
			apiPath:     normalClusterRequestURL,
			expectedErr: false,
		},
		{
			name: "url for reference cluster resource",
			um: &urlMaker{
				scheme: "http",
				netLoc: "localhost",
			},
			args: args{
				kind:         "appimages",
				kindInstName: "test",
				resourceName: "config",
				isRefRequest: true,
			},
			apiPath:     referenceClusterRequestURL,
			expectedErr: false,
		},
		{
			name: "set server api prefix for norma request",
			um: &urlMaker{
				scheme:          "http",
				netLoc:          "localhost",
				serverAPIPrefix: "kuscia-storage",
			},
			args: args{
				kind:         "appimages",
				kindInstName: "test",
				resourceName: "config",
			},
			apiPath:     normalClusterRequestURL,
			expectedErr: false,
		},
		{
			name: "set server api prefix for reference request",
			um: &urlMaker{
				scheme:          "http",
				netLoc:          "localhost",
				serverAPIPrefix: "kuscia-storage",
			},
			args: args{
				kind:         "appimages",
				kindInstName: "test",
				resourceName: "config",
				isRefRequest: true,
			},
			apiPath:     referenceClusterRequestURL,
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			get, err := tc.um.buildURL(tc.args.domain, tc.args.kind, tc.args.kindInstName, tc.args.resourceName, tc.args.isRefRequest)
			urlPrefix := tc.um.scheme + "://" + tc.um.netLoc
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				expected := fmt.Sprintf(tc.apiPath, tc.args.domain, tc.args.kind, tc.args.kindInstName, tc.args.resourceName)
				if tc.args.domain == "" {
					expected = fmt.Sprintf(tc.apiPath, tc.args.kind, tc.args.kindInstName, tc.args.resourceName)
				}

				if tc.um.serverAPIPrefix != "" {
					expected = "/" + tc.um.serverAPIPrefix + expected
				}
				expected = urlPrefix + expected

				assert.Nil(t, err)
				assert.Equal(t, expected, get)
			}
		})
	}
}

func TestURLMakerGetHost(t *testing.T) {
	um := &urlMaker{
		host: "localhost",
	}

	testCases := []struct {
		name     string
		expected string
	}{
		{
			name:     "host name is localhost",
			expected: "localhost",
		},
	}

	for _, tc := range testCases {
		get := um.getHost()
		assert.Equal(t, tc.expected, get)
	}
}
