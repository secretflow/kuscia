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

//nolint:dupl
package utils

import (
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func SetupPprof(debug bool, debugPort int, onlyControllers bool) {
	if debug {
		if debugPort <= 0 {
			debugPort = 28080
			if onlyControllers {
				debugPort = 28180
			}
		}

		if onlyControllers {
			nlog.Infof("Controllers debug port is %v", debugPort)
		} else {
			nlog.Infof("Common debug port is %v", debugPort)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		httpServer := &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", debugPort),
			Handler: mux,
		}
		go func() {
			if err := httpServer.ListenAndServe(); err != nil {
				nlog.Errorf("open debug mode fail:%+v", err)
			}
		}()
	}
}
