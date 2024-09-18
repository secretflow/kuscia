// Copyright 2024 Ant Group Co., Ltd.
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

	bandwidth_limitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/bandwidth_limit/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	kusciacrypt "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_crypt/v3"
	headerdecorator "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_header_decorator/v3"
	kusciapoller "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_poller/v3"
	kusciareceiver "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_receiver/v3"
	kusciatoken "github.com/secretflow/kuscia-envoy/kuscia/api/filters/http/kuscia_token_auth/v3"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	GrpcHTTP1BridgeName        = "envoy.filters.http.grpc_http1_bridge"
	KusciaGressName            = "envoy.filters.http.kuscia_gress"
	RouterName                 = "envoy.filters.http.router"
	GrpcHTTP1ReverseBridgeName = "envoy.filters.http.grpc_http1_reverse_bridge"
	TokenAuthFilterName        = "envoy.filters.http.kuscia_token_auth"
	HeaderDecoratorFilterName  = "envoy.filters.http.kuscia_header_decorator"
	CryptFilterName            = "envoy.filters.http.kuscia_crypt"
	ReceiverFilterName         = "envoy.filters.http.kuscia_receiver"
	BandwidthLimitName         = "envoy.filters.http.bandwidth_limit"
	PollerFilterName           = "envoy.filters.http.kuscia_poller"
)

var (
	internalFilterPriority = map[string]int{
		GrpcHTTP1ReverseBridgeName: 0,
		KusciaGressName:            1,
		BandwidthLimitName:         2,
		CryptFilterName:            3,
		ReceiverFilterName:         4,
		PollerFilterName:           5,
		RouterName:                 6,
	}

	externalFilterPriority = map[string]int{
		GrpcHTTP1BridgeName:       0,
		KusciaGressName:           1,
		TokenAuthFilterName:       2,
		HeaderDecoratorFilterName: 3,
		CryptFilterName:           4,
		ReceiverFilterName:        5,
		RouterName:                6,
	}

	mutableFilters = map[string]bool{
		TokenAuthFilterName:       true,
		HeaderDecoratorFilterName: true,
		CryptFilterName:           true,
		ReceiverFilterName:        true,
		BandwidthLimitName:        true,
		PollerFilterName:          true,
	}

	// internal only filters config
	encryptRules      []*kusciacrypt.CryptRule // for outbound, on port 80
	pollAppendHeaders []*kusciapoller.Poller_SourceHeader
	virtualHostLimits map[string]map[string]*RouteLimitConfig

	// external only filers config
	decryptRules  []*kusciacrypt.CryptRule // for inbound, on port 1080
	appendHeaders []*headerdecorator.HeaderDecorator_SourceHeader
	sourceTokens  []*kusciatoken.TokenAuth_SourceToken

	// internal and external filters config
	receiverRules []*kusciareceiver.ReceiverRule

	internalFilterMap = map[string]protoreflect.ProtoMessage{}
	externalFilterMap = map[string]protoreflect.ProtoMessage{
		TokenAuthFilterName: &kusciatoken.TokenAuth{},
	}
)

func UpdateEncryptRules(rule *kusciacrypt.CryptRule, selfNamespace string, add bool) error {
	lock.Lock()
	defer lock.Unlock()

	encryptRules = updateCryptRules(rule, encryptRules, add)
	if len(encryptRules) == 0 {
		delete(internalFilterMap, CryptFilterName)
	} else {
		internalFilterMap[CryptFilterName] = &kusciacrypt.Crypt{
			SelfNamespace: selfNamespace,
			EncryptRules:  encryptRules,
		}
	}
	return updateHTTPFilters(internalFilterMap, InternalListener)
}

func UpdateDecryptRules(rule *kusciacrypt.CryptRule, selfNamespace string, add bool) error {
	lock.Lock()
	defer lock.Unlock()

	decryptRules = updateCryptRules(rule, decryptRules, add)
	if len(decryptRules) == 0 {
		delete(externalFilterMap, CryptFilterName)
	} else {
		externalFilterMap[CryptFilterName] = &kusciacrypt.Crypt{
			SelfNamespace: selfNamespace,
			DecryptRules:  decryptRules,
		}
	}
	return updateHTTPFilters(externalFilterMap, ExternalListener)
}

func UpdateAppendHeaders(header *headerdecorator.HeaderDecorator_SourceHeader, add bool) error {
	lock.Lock()
	defer lock.Unlock()

	appendHeaders = updateAppendHeaders(header, appendHeaders, add)
	if len(appendHeaders) == 0 {
		delete(externalFilterMap, HeaderDecoratorFilterName)
	} else {
		externalFilterMap[HeaderDecoratorFilterName] = &headerdecorator.HeaderDecorator{
			AppendHeaders: appendHeaders,
		}
	}
	return updateHTTPFilters(externalFilterMap, ExternalListener)
}

func UpdateSourceTokens(token *kusciatoken.TokenAuth_SourceToken, add bool) error {
	lock.Lock()
	defer lock.Unlock()

	sourceTokens = updateSourceTokens(token, sourceTokens, add)
	// always keep token auth filter
	externalFilterMap[TokenAuthFilterName] = &kusciatoken.TokenAuth{
		SourceTokenList: sourceTokens,
	}
	return updateHTTPFilters(externalFilterMap, ExternalListener)
}

func UpdateReceiverRules(newRule *kusciareceiver.ReceiverRule, selfNamespace string, add bool) error {
	lock.Lock()
	defer lock.Unlock()

	receiverRules = updateReceiverRules(newRule, receiverRules, add)
	if len(receiverRules) == 0 {
		delete(internalFilterMap, ReceiverFilterName)
		delete(externalFilterMap, ReceiverFilterName)
	} else {
		internalFilterMap[ReceiverFilterName] = &kusciareceiver.Receiver{
			SelfNamespace: selfNamespace,
			Rules:         receiverRules,
		}
		externalFilterMap[ReceiverFilterName] = &kusciareceiver.Receiver{
			SelfNamespace: selfNamespace,
			Rules:         receiverRules,
		}
	}
	if err := updateHTTPFilters(internalFilterMap, InternalListener); err != nil {
		return err
	}
	if err := updateHTTPFilters(externalFilterMap, ExternalListener); err != nil {
		return err
	}
	return nil
}

func UpdatePollAppendHeaders(header *kusciapoller.Poller_SourceHeader, add bool) error {
	lock.Lock()
	defer lock.Unlock()

	pollAppendHeaders = updatePollerHeaders(header, pollAppendHeaders, add)
	if len(pollAppendHeaders) == 0 {
		delete(internalFilterMap, PollerFilterName)
	} else {
		internalFilterMap[PollerFilterName] = &kusciapoller.Poller{
			AppendHeaders: pollAppendHeaders,
		}
	}
	return updateHTTPFilters(internalFilterMap, InternalListener)
}

func UpdateBandwidthLimit(vhName string, taskID string, serviceName string, limitKbps *int64, add bool) error {
	nlog.Infof("add or update virtual host limit, vhName: %s, taskId: %s, serviceName: %s, limitKbps: %+v", vhName, taskID, serviceName, limitKbps)
	lock.Lock()
	defer lock.Unlock()

	if add {
		updateVirtualHostLimit(vhName, taskID, serviceName, *limitKbps)
		if _, ok := internalFilterMap[BandwidthLimitName]; !ok {
			internalFilterMap[BandwidthLimitName] = &bandwidth_limitv3.BandwidthLimit{
				StatPrefix: "kuscia_bandwidth_limit",
			}
		}
	} else {
		deleteVirtualHostLimit(vhName, taskID, serviceName)
		if len(virtualHostLimits) == 0 {
			delete(internalFilterMap, BandwidthLimitName)
		}
	}
	return updateHTTPFilters(internalFilterMap, InternalListener)
}

func updateVirtualHostLimit(vhName string, taskID string, serviceName string, limitKbps int64) {
	cfgs, ok := virtualHostLimits[vhName]
	if !ok {
		virtualHostLimits[vhName] = map[string]*RouteLimitConfig{taskID: {Services: []string{serviceName}, LimitKbps: limitKbps}}
		return
	}
	cfg, ok := cfgs[taskID]
	if !ok {
		cfgs[taskID] = &RouteLimitConfig{Services: []string{serviceName}, LimitKbps: limitKbps}
		return
	}
	if !contains(cfg.Services, serviceName) {
		cfg.Services = append(cfg.Services, serviceName)
	}
	cfg.LimitKbps = limitKbps
}

func deleteVirtualHostLimit(vhName string, taskID string, serviceName string) {
	cfgs, ok := virtualHostLimits[vhName]
	if !ok {
		return
	}
	if cfg, ok := cfgs[taskID]; ok {
		for i, s := range cfg.Services {
			if s == serviceName {
				cfg.Services = append(cfg.Services[:i], cfg.Services[i+1:]...)
				break
			}
		}
		if len(cfg.Services) == 0 {
			delete(cfgs, taskID)
		}
	}
	if len(cfgs) == 0 {
		delete(virtualHostLimits, vhName)
	}
}

func contains(services []string, serviceName string) bool {
	for _, s := range services {
		if s == serviceName {
			return true
		}
	}
	return false
}

func updateCryptRules(r *kusciacrypt.CryptRule, rules []*kusciacrypt.CryptRule,
	add bool) []*kusciacrypt.CryptRule {
	for i, rule := range rules {
		if rule.Source == r.Source && rule.Destination == r.Destination {
			if add {
				rules[i] = r
				return rules
			}
			rules = append(rules[:i], rules[i+1:]...)
			return rules
		}
	}
	if add {
		rules = append(rules, r)
	}
	return rules
}

func updateAppendHeaders(header *headerdecorator.HeaderDecorator_SourceHeader,
	headers []*headerdecorator.HeaderDecorator_SourceHeader, add bool) []*headerdecorator.HeaderDecorator_SourceHeader {
	found := false
	for i, h := range headers {
		if header.Source == h.Source {
			if add {
				headers[i] = header
			} else {
				headers = append(headers[:i], headers[i+1:]...)
			}
			found = true
			break
		}
	}
	if add && !found {
		headers = append(headers, header)
	}
	return headers
}

func updateSourceTokens(token *kusciatoken.TokenAuth_SourceToken,
	tokens []*kusciatoken.TokenAuth_SourceToken, add bool) []*kusciatoken.TokenAuth_SourceToken {
	found := false
	for i, t := range tokens {
		if token.Source == t.Source {
			if add {
				tokens[i] = token
			} else {
				tokens = append(tokens[:i], tokens[i+1:]...)
			}
			found = true
			break
		}
	}
	if add && !found {
		tokens = append(tokens, token)
	}
	return tokens
}

func updateReceiverRules(newRule *kusciareceiver.ReceiverRule, config []*kusciareceiver.ReceiverRule,
	add bool) []*kusciareceiver.ReceiverRule {
	for i, rule := range config {
		if rule.Source == newRule.Source && rule.Destination == newRule.Destination && rule.Svc == newRule.Svc {
			if add {
				config[i] = newRule
				return config
			}
			config = append(config[:i], config[i+1:]...)
			return config
		}
	}
	if add {
		config = append(config, newRule)
	}
	return config
}

func updatePollerHeaders(header *kusciapoller.Poller_SourceHeader,
	headers []*kusciapoller.Poller_SourceHeader, add bool) []*kusciapoller.Poller_SourceHeader {
	found := false
	for i, h := range headers {
		if header.Source == h.Source {
			if add {
				headers[i] = header
			} else {
				headers = append(headers[:i], headers[i+1:]...)
			}
			found = true
			break
		}
	}
	if add && !found {
		headers = append(headers, header)
	}
	return headers
}

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
		dic: internalFilterPriority,
	}

	HTTPFilters.filters = append(HTTPFilters.filters, f...)
	sort.Sort(HTTPFilters)
	return HTTPFilters.filters
}

func sortExternalFilters(f []*hcm.HttpFilter) []*hcm.HttpFilter {
	HTTPFilters := HTTPFilters{
		dic: externalFilterPriority,
	}

	HTTPFilters.filters = append(HTTPFilters.filters, f...)
	sort.Sort(HTTPFilters)
	return HTTPFilters.filters
}
