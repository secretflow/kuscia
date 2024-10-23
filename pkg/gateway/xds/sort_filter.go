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

package xds

import (
	"sort"

	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
)

var (
	internalHTTPFilters = map[string]int{
		"envoy.filters.http.grpc_http1_reverse_bridge": 0,
		"envoy.filters.http.kuscia_gress":              1,
		"envoy.filters.http.kuscia_crypt":              2,
		"envoy.filters.http.kuscia_receiver":           3,
		"envoy.filters.http.kuscia_poller":             4,
		"envoy.filters.http.router":                    5,
	}

	externalHTTPFilters = map[string]int{
		"envoy.filters.http.grpc_http1_bridge":       0,
		"envoy.filters.http.kuscia_gress":            1,
		"envoy.filters.http.kuscia_token_auth":       2,
		"envoy.filters.http.kuscia_header_decorator": 3,
		"envoy.filters.http.kuscia_crypt":            4,
		"envoy.filters.http.kuscia_receiver":         5,
		"envoy.filters.http.router":                  6,
	}
)

type HTTPFilters struct {
	filters []*hcm.HttpFilter
	dic     map[string]int
}

func (f HTTPFilters) Len() int {
	return len(f.filters)
}

func (f HTTPFilters) Less(i, j int) bool {
	return f.dic[f.filters[i].Name] < f.dic[f.filters[j].Name]
}

func (f HTTPFilters) Swap(i, j int) {
	f.filters[i], f.filters[j] = f.filters[j], f.filters[i]
}

func sortInternalFilters(f []*hcm.HttpFilter) []*hcm.HttpFilter {
	HTTPFilters := HTTPFilters{
		dic: internalHTTPFilters,
	}

	HTTPFilters.filters = append(HTTPFilters.filters, f...)
	sort.Sort(HTTPFilters)
	return HTTPFilters.filters
}

func sortExternalFilters(f []*hcm.HttpFilter) []*hcm.HttpFilter {
	HTTPFilters := HTTPFilters{
		dic: externalHTTPFilters,
	}

	HTTPFilters.filters = append(HTTPFilters.filters, f...)
	sort.Sort(HTTPFilters)
	return HTTPFilters.filters
}
