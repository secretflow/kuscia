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

import (
	"crypto/tls"

	"github.com/spf13/pflag"

	ktls "github.com/secretflow/kuscia/pkg/utils/tls"
)

// TLSConfig defines tls config info.
type TLSConfig struct {
	EnableTLS      bool
	CAPath         string
	ServerCertPath string
	ServerKeyPath  string
	ClientCertPath string
	ClientKeyPath  string
}

// InstallPFlags used to install pflags.
func (c *TLSConfig) InstallPFlags(beanName string, fs *pflag.FlagSet) {
	if fs == nil {
		fs = pflag.CommandLine
	}

	fs.BoolVar(&c.EnableTLS, beanName+"-enable-tls", false, beanName+": enable tls feature")
	fs.StringVar(&c.CAPath, beanName+"-ca", "/home/admin/tls/ca.crt", beanName+": ca file path")
	fs.StringVar(&c.ServerCertPath, beanName+"-server-cert", "/home/admin/tls/server.crt", beanName+": server cert file path")
	fs.StringVar(&c.ServerKeyPath, beanName+"-server-key", "/home/admin/tls/server.key", beanName+": server key file path")
	fs.StringVar(&c.ClientCertPath, beanName+"-client-cert", "/home/admin/tls/client.crt", beanName+": client cert file path")
	fs.StringVar(&c.ClientKeyPath, beanName+"-client-key", "/home/admin/tls/client.key", beanName+": client key file path")
}

// InitTLSConfig used to init tls config.
func (c *TLSConfig) InitTLSConfig(enableTLS bool, caPath, serverCertPath, serverKeyPath, clientCertPath, clientKeyPath string) {
	c.EnableTLS = enableTLS
	if caPath != "" {
		c.CAPath = caPath
	}

	if serverCertPath != "" {
		c.ServerCertPath = serverCertPath
	}

	if serverKeyPath != "" {
		c.ServerKeyPath = serverKeyPath
	}

	if clientCertPath != "" {
		c.ClientCertPath = clientCertPath
	}

	if clientKeyPath != "" {
		c.ClientKeyPath = clientKeyPath
	}
}

// TLSEnabled checks if tls is enabled.
func (c *TLSConfig) TLSEnabled() bool {
	return c.EnableTLS
}

// LoadServerTLSConfig loads server tls config.
func (c *TLSConfig) LoadServerTLSConfig() (*tls.Config, error) {
	return ktls.BuildServerTLSConfigFromPath(c.CAPath, c.ServerCertPath, c.ServerKeyPath)
}
