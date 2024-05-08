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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/proto/api/v1alpha1"
)

func mockHTTPServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := &v1alpha1.Status{
			Code: 200,
		}

		content := "test"

		var resp interface{}
		switch r.Method {
		case http.MethodGet:
			resp = &response{
				Status:  status,
				Content: content,
			}
		case http.MethodPost, http.MethodDelete:
			resp = &response{
				Status: status,
			}
		}

		br, _ := json.Marshal(resp)
		_, _ = io.WriteString(w, string(br))
	}))
}

func TestNewClient(t *testing.T) {
	testCases := []struct {
		name        string
		endpoint    string
		expectedErr bool
	}{
		{
			name:        "endpoint is empty",
			endpoint:    "",
			expectedErr: true,
		},
		{
			name:        "endpoint is localhost",
			endpoint:    "localhost",
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.endpoint)
			if tc.expectedErr {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestCreateResource(t *testing.T) {
	svr := mockHTTPServer()
	defer svr.Close()
	client, _ := New(svr.URL)

	type args struct {
		domain        string
		kind          string
		kindInstName  string
		resourceName  string
		normalReqBody *NormalRequestBody
		refReqBody    *ReferenceRequestBody
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr bool
	}{
		{
			name: "request body is empty",
			args: args{
				domain:       "alice",
				kind:         "pods",
				kindInstName: "test",
				resourceName: "config",
			},
			expectedErr: true,
		},
		{
			name: "get url failed",
			args: args{
				domain:       "alice",
				kindInstName: "test",
				resourceName: "config",
			},
			expectedErr: true,
		},
		{
			name: "create resource success",
			args: args{
				domain:       "alice",
				kind:         "pods",
				kindInstName: "test",
				resourceName: "config",
				normalReqBody: &NormalRequestBody{
					Content: "test",
				},
			},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := client.CreateResource(tc.args.domain, tc.args.kind, tc.args.kindInstName, tc.args.resourceName, tc.args.normalReqBody, tc.args.refReqBody)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetResource(t *testing.T) {
	svr := mockHTTPServer()
	defer svr.Close()
	client, _ := New(svr.URL)

	type args struct {
		domain       string
		kind         string
		kindInstName string
		resourceName string
	}

	testCases := []struct {
		name        string
		args        args
		expected    string
		expectedErr bool
	}{
		{
			name: "get url failed",
			args: args{
				domain:       "alice",
				kindInstName: "test",
				resourceName: "config",
			},
			expectedErr: true,
		},
		{
			name: "get resource success",
			args: args{
				domain:       "alice",
				kind:         "pods",
				kindInstName: "test",
				resourceName: "config",
			},
			expected:    "test",
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			get, err := client.GetResource(tc.args.domain, tc.args.kind, tc.args.kindInstName, tc.args.resourceName)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Equal(t, tc.expected, get)
				assert.Nil(t, err)
			}
		})
	}
}

func TestDeleteResource(t *testing.T) {
	svr := mockHTTPServer()
	defer svr.Close()
	client, _ := New(svr.URL)

	type args struct {
		domain       string
		kind         string
		kindInstName string
		resourceName string
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr bool
	}{
		{
			name: "get url failed",
			args: args{
				domain:       "alice",
				kindInstName: "test",
				resourceName: "config",
			},
			expectedErr: true,
		},
		{
			name: "delete resource success",
			args: args{
				domain:       "alice",
				kind:         "pods",
				kindInstName: "test",
				resourceName: "config",
			},
			expectedErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := client.DeleteResource(tc.args.domain, tc.args.kind, tc.args.kindInstName, tc.args.resourceName)
			if tc.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
