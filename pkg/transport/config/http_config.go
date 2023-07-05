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

package config

type ServerConfig struct {
	Port           int   `yaml:"port,omitempty"`
	ReadTimeout    int   `yaml:"readTimeout,omitempty"`
	WriteTimeout   int   `yaml:"writeTimeout,omitempty"`
	IdleTimeout    int   `yaml:"idleTimeout,omitempty"`
	ReqBodyMaxSize int64 `yaml:"reqBodyMaxSize,omitempty"`
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Port:           8081,
		ReadTimeout:    300,
		WriteTimeout:   300,
		IdleTimeout:    60,
		ReqBodyMaxSize: 134217728,
	}
}
