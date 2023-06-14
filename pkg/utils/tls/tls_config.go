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

package tls

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
)

type CertConfig struct {
	Protocol   string
	CA         string
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
}

var c *CertConfig

func init() {
	c = &CertConfig{}
}

func InstallFlags(fs *flag.FlagSet) {
	if fs == nil {
		fs = flag.CommandLine
	}

	fs.StringVar(&c.Protocol, "protocol", "http", "Server protocol. support http and https protocol")
	fs.StringVar(&c.CA, "ca", "/home/admin/tls/ca.crt", "Server CA config")
	fs.StringVar(&c.ServerCert, "server-cert", "/home/admin/tls/server.crt", "Server cert config")
	fs.StringVar(&c.ServerKey, "server-key", "/home/admin/tls/server.key", "Server key config")
	fs.StringVar(&c.ClientCert, "client-cert", "/home/admin/tls/client.crt", "Client cert config")
	fs.StringVar(&c.ClientKey, "client-key", "/home/admin/tls/client.key", "Client key config")
}

func InstallPFlags(fs *pflag.FlagSet) {
	if fs == nil {
		fs = pflag.CommandLine
	}

	fs.StringVar(&c.Protocol, "protocol", "http", "Server protocol. support http and https protocol")
	fs.StringVar(&c.CA, "ca", "/home/admin/tls/ca.crt", "Server CA config")
	fs.StringVar(&c.ServerCert, "server-cert", "/home/admin/tls/server.crt", "Server cert config")
	fs.StringVar(&c.ServerKey, "server-key", "/home/admin/tls/server.key", "Server key config")
	fs.StringVar(&c.ClientCert, "client-cert", "/home/admin/tls/client.crt", "Client cert config")
	fs.StringVar(&c.ClientKey, "client-key", "/home/admin/tls/client.key", "Client key config")
}

func InitCertConfig(protocol, ca, serverCert, serverKey, clientCert, clientKey string) {
	c.Protocol = protocol
	c.CA = ca
	c.ServerCert = serverCert
	c.ServerKey = serverKey
	c.ClientCert = clientCert
	c.ClientKey = clientKey
}

func Enable() bool {
	return c.Protocol == "https"
}

func LoadServerTLSConfig() (*tls.Config, error) {
	if c.CA == "" && c.ServerCert == "" && c.ServerKey == "" {
		return nil, fmt.Errorf("load server tls config failure, CA|ServerCert|ServerKey can'c be empty")
	}

	caCertFile, err := LoadCertFile(c.CA)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertFile)

	certs, err := LoadTLSServerCertificate()
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: certs,
	}
	return config, nil
}

func LoadClientTLSConfig() (*tls.Config, error) {
	if c.CA == "" && c.ClientCert == "" && c.ClientKey == "" {
		return nil, fmt.Errorf("load client tls config failure, CA|ClientCert|ClientKey can'c be empty")
	}

	caCertFile, err := LoadCertFile(c.CA)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertFile)

	certificate, err := LoadTLSClientCertificate()
	if err != nil {
		return nil, fmt.Errorf("could not load client certificate: %v", err.Error())
	}
	config := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{certificate},
	}
	return config, nil
}

func LoadTLSServerCertificate() ([]tls.Certificate, error) {
	certPEMBlock, err := LoadCertFile(c.ServerCert)
	if err != nil {
		return nil, err
	}

	keyPEMBlock, err := LoadCertFile(c.ServerKey)
	if err != nil {
		return nil, err
	}

	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return nil, err
	}
	return certs, nil
}

func LoadTLSClientCertificate() (tls.Certificate, error) {
	certPEMBlock, err := LoadCertFile(c.ClientCert)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyPEMBlock, err := LoadCertFile(c.ClientKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return tls.Certificate{}, err
	}
	return cert, nil
}

func LoadCertFile(name string) ([]byte, error) {
	certContent, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("error reading %v certificate: %v", name, err.Error())
	}
	return certContent, nil
}

func SetHTTPTransport(client *http.Client) error {
	if client == nil {
		return fmt.Errorf("client is nil, please provide valid http client")
	}

	if Enable() {
		tlsConfig, err := LoadClientTLSConfig()
		if err != nil {
			return err
		}

		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		client.Transport = tr
	}
	return nil
}
