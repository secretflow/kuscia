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

//nolint:dulp
package modules

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

var (
	CertPrefix       = "etc/certs/"
	LogPrefix        = "var/logs/"
	StdoutPrefix     = "var/stdout/"
	ConfPrefix       = "etc/conf/"
	k3sDataDirPrefix = "var/k3s/"
)

type Dependencies struct {
	KusciaConfig
	EnvoyIP                string
	Clients                *kubeconfig.KubeClients
	ApiserverEndpoint      string
	KubeconfigFile         string
	ContainerdSock         string
	TransportConfigFile    string
	TransportPort          int
	InterConnSchedulerPort int
	IsMaster               bool
	KusciaKubeConfig       string
}

type KusciaConfig struct {
	RootDir        string                     `yaml:"rootDir,omitempty"`
	DomainID       string                     `yaml:"domainID,omitempty"`
	DomainKeyFile  string                     `yaml:"domainKeyFile,omitempty"`
	DomainCertFile string                     `yaml:"domainCertFile,omitempty"`
	CAKeyFile      string                     `yaml:"caKeyFile,omitempty"`
	CAFile         string                     `yaml:"caFile,omitempty"`
	Agent          KusciaAgentConfig          `yaml:"agent,omitempty"`
	Master         *kusciaconfig.MasterConfig `yaml:"master,omitempty"`
	ExternalTLS    *kusciaconfig.TLSConfig    `yaml:"externalTLS,omitempty"`
	IsMaster       bool
}

type KusciaAgentConfig struct {
	AllowPrivileged bool               `yaml:"allowPrivileged,omitempty"`
	Plugins         []config.PluginCfg `yaml:"plugins,omitempty"`
}

type Module interface {
	Run(ctx context.Context) error
	WaitReady(ctx context.Context) error
	Name() string
}

func EnsureCaKeyAndCert(conf *Dependencies) (*rsa.PrivateKey, *x509.Certificate, error) {
	if _, err := os.Stat(conf.CAKeyFile); os.IsNotExist(err) {
		caKey, caCertBytes, err := tls.CreateCA(conf.DomainID)
		if err != nil {
			return nil, nil, err
		}

		caKeyOut, err := os.Create(conf.CAKeyFile)
		if err != nil {
			return nil, nil, err
		}
		defer caKeyOut.Close()

		if err = pem.Encode(caKeyOut, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(caKey),
		}); err != nil {
			return nil, nil, err
		}

		caOut, err := os.Create(conf.CAFile)
		if err != nil {
			return nil, nil, err
		}
		defer caOut.Close()

		if err = pem.Encode(caOut, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caCertBytes,
		}); err != nil {
			return nil, nil, err
		}
		caCert, _ := x509.ParseCertificate(caCertBytes)
		return caKey, caCert, nil
	} else {
		caKey, err := utils.ParsePKCS1PrivateKey(conf.CAKeyFile)
		if err != nil {
			return nil, nil, err
		}
		caCert, err := utils.ParsePKCS1CertFromFile(conf.CAFile)
		if err != nil {
			return nil, nil, err
		}
		return caKey, caCert, nil
	}
}

func EnsureDomainKey(conf *Dependencies) error {
	if _, err := os.Stat(conf.DomainKeyFile); os.IsNotExist(err) {
		keyOut, err := os.OpenFile(conf.DomainKeyFile, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s, err: %v", conf.DomainKeyFile, err)
		}
		defer keyOut.Close()
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate rsa key, detail-> %v", err)
		}
		if err = pem.Encode(keyOut, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		}); err != nil {
			return fmt.Errorf("failed to encode key, detail-> %v", err)
		}
	}

	return nil
}

func RenderConfig(configPathTmpl, configPath string, s interface{}) error {
	configTmpl, err := template.ParseFiles(configPathTmpl)
	if err != nil {
		return err
	}
	var configContent bytes.Buffer
	if err := configTmpl.Execute(&configContent, s); err != nil {
		return err
	}

	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(configContent.String())
	if err != nil {
		return err
	}
	return nil
}

func EnsureDir(conf *Dependencies) error {
	if err := os.MkdirAll(filepath.Join(conf.RootDir, CertPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(conf.RootDir, LogPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(conf.RootDir, StdoutPrefix), 0755); err != nil {
		return err
	}
	return nil
}
