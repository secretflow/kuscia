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
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type WhitelistChecker interface {
	Check(address string, ports []uint32) bool
}

type WhitelistItem struct {
	// address may be an ip 127.0.0.1, a CIDR 10.0.0.1/24, or an explicit hostname gw.abc.com
	Address string `yaml:"address,omitempty"`
	// optional config for allow ports, none means all ports are acceptable
	Ports []uint32 `yaml:"ports,omitempty"`

	ip    net.IP
	ipnet *net.IPNet
}

type Whitelist struct {
	Items []WhitelistItem `yaml:"items,omitempty"`
}

// Init a new whitelist checker from given file path, returns nil if any error occurs.
func NewWhitelistChecker(whitelist string) (*Whitelist, error) {
	if whitelist == "" {
		return nil, nil
	}

	f, err := os.OpenFile(whitelist, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open whitelist file: %s", whitelist)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	wl := &Whitelist{}
	if err := yaml.Unmarshal(data, wl); err != nil {
		return nil, err
	}

	for i := range wl.Items {
		wl.Items[i].ip = net.ParseIP(wl.Items[i].Address)
		_, wl.Items[i].ipnet, _ = net.ParseCIDR(wl.Items[i].Address)
	}
	return wl, nil
}

func (c *Whitelist) Check(address string, ports []uint32) bool {
	target := net.ParseIP(address)
	for _, item := range c.Items {
		if target != nil {
			if item.ip.Equal(target) {
				return portsContains(item.Ports, ports)
			}
			if item.ipnet != nil && item.ipnet.Contains(target) {
				return portsContains(item.Ports, ports)
			}
		} else if strings.Compare(address, item.Address) == 0 {
			return portsContains(item.Ports, ports)
		}
	}

	return false
}

func portsContains(s, e []uint32) bool {
	if len(s) == 0 {
		// no ports field means all ports are acceptable
		return true
	}

	for _, x := range e {
		exist := false
		for _, y := range s {
			if x == y {
				exist = true
				break
			}
		}
		if !exist {
			return false
		}
	}
	return true
}
