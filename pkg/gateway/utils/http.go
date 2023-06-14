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
	"strconv"
	"strings"
)

func ParseURL(url string) (string, string, uint32, error) {
	var protocol, hostPort, host string
	var port int
	var err error
	if strings.HasPrefix(url, "http://") {
		protocol = "http"
		hostPort = url[7:]
	} else if strings.HasPrefix(url, "https://") {
		protocol = "https"
		hostPort = url[8:]
	} else {
		return protocol, host, uint32(port), fmt.Errorf("invalid host: %s", url)
	}

	fields := strings.Split(hostPort, ":")
	host = fields[0]
	if len(fields) == 2 {
		if port, err = strconv.Atoi(fields[1]); err != nil {
			return protocol, host, uint32(port), err
		}
	} else {
		if protocol == "http" {
			port = 80
		} else {
			port = 443
		}
	}

	return protocol, host, uint32(port), nil
}
