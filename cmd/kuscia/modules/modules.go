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
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync/atomic"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/secretbackend"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/logs"
)

var (
	CertPrefix                    = "etc/certs/"
	LogPrefix                     = "var/logs/"
	StdoutPrefix                  = "var/stdout/"
	ConfPrefix                    = "etc/conf/"
	k3sDataDirPrefix              = "var/k3s/"
	defaultInterConnSchedulerPort = 8084
	defaultEndpointForLite        = "http://apiserver.master.svc"
)

type Dependencies struct {
	confloader.KusciaConfig
	CAKey                   *rsa.PrivateKey
	CACert                  *x509.Certificate
	DomainKey               *rsa.PrivateKey
	DomainCert              *x509.Certificate
	Clients                 *kubeconfig.KubeClients
	ApiserverEndpoint       string
	KubeconfigFile          string
	ContainerdSock          string
	TransportConfigFile     string
	TransportPort           int
	InterConnSchedulerPort  int
	KusciaKubeConfig        string
	EnableContainerd        bool
	SecretBackendHolder     *secretbackend.Holder
	DomainCertByMasterValue atomic.Value // the value is <*x509.Certificate>
	LogConfig               *nlog.LogConfig
}

func (d *Dependencies) MakeClients() {
	clients, err := kubeconfig.CreateClientSetsFromKubeconfig(d.KubeconfigFile, d.ApiserverEndpoint)
	if err != nil {
		nlog.Fatal(err)
	}
	d.Clients = clients
}

type Module interface {
	Run(ctx context.Context) error
	WaitReady(ctx context.Context) error
	Name() string
}

func LoadCaDomainKeyAndCert(dependencies *Dependencies) error {
	var err error
	if dependencies.CAKey, err = tlsutils.ParseKey([]byte(dependencies.CAKeyData),
		dependencies.CAKeyFile); err != nil {
		nlog.Errorf("load key failed: key: %t, file: %s", len(dependencies.CAKeyData) == 0, dependencies.CAKeyFile)
		return err
	}

	if dependencies.CACertFile == "" {
		nlog.Errorf("load cert failed: ca cert must be file, ca file should not be empty")
		return err
	}

	if dependencies.CACert, err = tlsutils.ParseCert(nil,
		dependencies.CACertFile); err != nil {
		nlog.Errorf("load cert failed: file: %s", dependencies.CACertFile)
		return err
	}

	if dependencies.DomainKey, err = tlsutils.ParseKey([]byte(dependencies.DomainKeyData),
		dependencies.DomainKeyFile); err != nil {
		nlog.Errorf("load key failed: key: %t, file: %s", len(dependencies.CAKeyData) == 0, dependencies.DomainKeyFile)
		return err
	}

	return nil
}

func EnsureDomainCert(conf *Dependencies) error {
	caKey := conf.CAKey
	caCert := conf.CACert
	domainKey := conf.DomainKey
	domainCrt := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: conf.DomainID},
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
	if conf.DomainCert, err = x509.ParseCertificate(domainCrtRaw); err != nil {
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

func InitDependencies(ctx context.Context, kusciaConf confloader.KusciaConfig) *Dependencies {
	dependencies := &Dependencies{
		KusciaConfig: kusciaConf,
	}

	// run config loader
	dependencies.SecretBackendHolder = secretbackend.NewHolder()
	for _, sbc := range dependencies.SecretBackends {
		if err := dependencies.SecretBackendHolder.Init(sbc.Name, sbc.Driver, sbc.Params); err != nil {
			nlog.Fatalf("Init secret backend name=%s params=%+v failed: %s", sbc.Name, sbc.Params, err)
		}
	}
	configLoaders, err := confloader.NewConfigLoaderChain(ctx, dependencies.KusciaConfig.ConfLoaders, dependencies.SecretBackendHolder)
	if err != nil {
		nlog.Fatalf("Init config loader failed: %s", err)
	}
	if err = configLoaders.Load(ctx, &dependencies.KusciaConfig); err != nil {
		nlog.Errorf("Load config by configloader failed: %s", err)
	}

	nlog.Debugf("After config loader handle, kuscia config is %+v", dependencies.KusciaConfig)

	// make runtime dir
	err = EnsureDir(dependencies)
	if err != nil {
		nlog.Fatal(err)
	}

	// for master and autonomy, get k3s backend config.
	if dependencies.RunMode == common.RunModeMaster || dependencies.RunMode == common.RunModeAutonomy {
		dependencies.ApiserverEndpoint = dependencies.Master.APIServer.Endpoint
		dependencies.KubeconfigFile = dependencies.Master.APIServer.KubeConfig
		dependencies.KusciaKubeConfig = filepath.Join(dependencies.RootDir, "etc/kuscia.kubeconfig")
		dependencies.InterConnSchedulerPort = defaultInterConnSchedulerPort
	}
	// for autonomy and lite
	if dependencies.RunMode == common.RunModeAutonomy || dependencies.RunMode == common.RunModeLite {
		dependencies.ContainerdSock = filepath.Join(dependencies.RootDir, "containerd/run/containerd.sock")
		dependencies.TransportConfigFile = filepath.Join(dependencies.RootDir, "etc/conf/transport/transport.yaml")
		dependencies.TransportPort, err = GetTransportPort(dependencies.TransportConfigFile)
		if err != nil {
			nlog.Fatal(err)
		}
		dependencies.EnableContainerd = true
		if dependencies.Agent.Provider.Runtime == config.K8sRuntime {
			dependencies.EnableContainerd = false
		}
	}
	if dependencies.RunMode == common.RunModeLite {
		dependencies.ApiserverEndpoint = defaultEndpointForLite
	}

	return dependencies
}
