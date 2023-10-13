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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/secretflow/kuscia/pkg/agent/config"
	cmconf "github.com/secretflow/kuscia/pkg/confmanager/config"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/logs"
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
	EnableContainerd       bool
	LogConfig              *nlog.LogConfig
}

type KusciaConfig struct {
	RootDir        string                     `yaml:"rootDir,omitempty"`
	DomainID       string                     `yaml:"domainID,omitempty"`
	DomainKeyFile  string                     `yaml:"domainKeyFile,omitempty"`
	DomainCertFile string                     `yaml:"domainCertFile,omitempty"`
	CAKeyFile      string                     `yaml:"caKeyFile,omitempty"`
	CACertFile     string                     `yaml:"caFile,omitempty"`
	Agent          KusciaAgentConfig          `yaml:"agent,omitempty"`
	Master         *kusciaconfig.MasterConfig `yaml:"master,omitempty"`
	ConfManager    cmconf.ConfManagerConfig   `yaml:"confManager,omitempty"`
	ExternalTLS    *kusciaconfig.TLSConfig    `yaml:"externalTLS,omitempty"`
	IsMaster       bool
}

type KusciaAgentConfig struct {
	config.AgentConfig `yaml:",inline"`
}

type Module interface {
	Run(ctx context.Context) error
	WaitReady(ctx context.Context) error
	Name() string
}

func EnsureCaKeyAndCert(conf *Dependencies) (*rsa.PrivateKey, *x509.Certificate, error) {
	if _, err := os.Stat(conf.CAKeyFile); os.IsNotExist(err) {
		caKey, caCertBytes, err := tlsutils.CreateCA(conf.DomainID)
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

		caOut, err := os.Create(conf.CACertFile)
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
		caKey, err := tlsutils.ParsePKCS1PrivateKey(conf.CAKeyFile)
		if err != nil {
			return nil, nil, err
		}
		caCert, err := tlsutils.ParsePKCS1CertFromFile(conf.CACertFile)
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

func EnsureDomainCert(conf *Dependencies) error {
	caKey, err := tlsutils.ParsePKCS1PrivateKey(conf.CAKeyFile)
	if err != nil {
		return err
	}
	caCert, err := tlsutils.ParsePKCS1CertFromFile(conf.CACertFile)
	if err != nil {
		return err
	}
	domainKey, err := tlsutils.ParsePKCS1PrivateKey(conf.DomainKeyFile)
	if err != nil {
		return err
	}
	domainCrt := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               caCert.Subject,
		PublicKeyAlgorithm:    caCert.PublicKeyAlgorithm,
		PublicKey:             domainKey.PublicKey,
		NotBefore:             caCert.NotBefore,
		NotAfter:              caCert.NotAfter,
		ExtKeyUsage:           caCert.ExtKeyUsage,
		KeyUsage:              caCert.KeyUsage,
		BasicConstraintsValid: caCert.BasicConstraintsValid,
	}
	domainCrtRaw, err := x509.CreateCertificate(rand.Reader, domainCrt, caCert, &domainKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	certOut, err := os.Create(conf.DomainCertFile)
	if err != nil {
		return err
	}
	defer certOut.Close()

	return pem.Encode(certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: domainCrtRaw,
	})
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

func CreateDefaultDomain(ctx context.Context, conf *Dependencies) error {
	certStr, err := os.ReadFile(conf.DomainCertFile)
	if err != nil {
		return err
	}
	_, err = conf.Clients.KusciaClient.KusciaV1alpha1().Domains().Create(ctx, &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: conf.DomainID,
		},
		Spec: kusciaapisv1alpha1.DomainSpec{
			Cert: base64.StdEncoding.EncodeToString(certStr),
		}}, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}

func InitLogs(logConfig *nlog.LogConfig) error {
	zlog, err := zlogwriter.New(logConfig)
	if err != nil {
		return err
	}
	nlog.Setup(nlog.SetWriter(zlog))
	logs.Setup(nlog.SetWriter(zlog))
	klog.SetOutput(nlog.DefaultLogger())
	klog.LogToStderr(false)
	return nil
}
