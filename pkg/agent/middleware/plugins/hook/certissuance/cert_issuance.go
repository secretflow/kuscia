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

package certissuance

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

const (
	pluginNameCertIssuance = "cert-issuance"

	defaultCertsDirName       = "certs"
	defaultContainerCertsPath = "/etc/kuscia/certs"

	communicationRoleServer = "server"
	communicationRoleClient = "client"
)

func Register() {
	plugin.Register(pluginNameCertIssuance, &certIssuance{})
}

type certIssuance struct {
	signingCertFile string
	caCert          *x509.Certificate
	caKey           *rsa.PrivateKey
	initialized     bool
}

// Type implements the plugin.Plugin interface.
func (ci *certIssuance) Type() string {
	return hook.PluginType
}

// Init implements the plugin.Plugin interface.
func (ci *certIssuance) Init(dependencies *plugin.Dependencies, cfg *config.PluginCfg) error {
	signingKeyFile := dependencies.AgentConfig.DomainCAKeyFile
	signingCertFile := dependencies.AgentConfig.DomainCAFile

	if signingKeyFile == "" || signingCertFile == "" {
		nlog.Infof("Plugin cert-issuance will not be registered, signingCertFile=%v, signingKeyFile=%v", signingCertFile, signingKeyFile)
		return nil
	}

	cert, key, err := tls.LoadX509KeyPair(signingCertFile, signingKeyFile)
	if err != nil {
		return err
	}

	ci.signingCertFile = signingCertFile
	ci.caCert = cert
	ci.caKey = key
	ci.initialized = true

	hook.Register(pluginNameCertIssuance, ci)

	return nil
}

// CanExec implements the hook.Handler interface.
func (ci *certIssuance) CanExec(obj interface{}, point hook.Point) bool {
	if !ci.initialized {
		return false
	}

	if point != hook.PointGenerateRunContainerOptions {
		return false
	}

	rObj, ok := obj.(*hook.RunContainerOptionsObj)
	if !ok {
		return false
	}

	if rObj.Pod.Labels[common.LabelCommunicationRoleServer] != common.True && rObj.Pod.Labels[common.LabelCommunicationRoleClient] != common.True {
		return false
	}

	return true
}

// ExecHook implements the hook.Handler interface.
// It renders the configuration template and writes the generated real configuration content to a new file/directory.
// The value of hostPath will be replaced by the new file/directory path.
func (ci *certIssuance) ExecHook(obj interface{}, point hook.Point) (*hook.Result, error) {
	if !ci.initialized {
		return nil, fmt.Errorf("plugin cert-issuance is not initialized")
	}

	rObj, ok := obj.(*hook.RunContainerOptionsObj)
	if !ok {
		return nil, fmt.Errorf("can't convert object to pod")
	}

	issueServerCert := rObj.Pod.Labels[common.LabelCommunicationRoleServer] == common.True
	issueClientCert := rObj.Pod.Labels[common.LabelCommunicationRoleClient] == common.True

	if !issueServerCert && !issueClientCert {
		nlog.Infof("No certificate needs to be issued to the pod %q", format.Pod(rObj.Pod))
		return &hook.Result{}, nil
	}

	hostCertsDir := filepath.Join(rObj.ContainerDir, defaultCertsDirName)
	if err := paths.EnsureDirectory(hostCertsDir, true); err != nil {
		return nil, err
	}

	// issue server certificate.
	if issueServerCert {
		if err := ci.issueCertificate(rObj, hostCertsDir, communicationRoleServer); err != nil {
			return nil, err
		}
	}

	// issue client certificate.
	if issueClientCert {
		if err := ci.issueCertificate(rObj, hostCertsDir, communicationRoleClient); err != nil {
			return nil, err
		}
	}

	// inject ca file.
	injectCertificate(rObj, "trusted-ca", common.EnvTrustedCAFile, ci.signingCertFile, filepath.Join(defaultContainerCertsPath, "ca.crt"))

	nlog.Infof("Successfully issued certificate(server=%v,client=%v) for container %q in pod %q", issueServerCert, issueClientCert, rObj.Container.Name, format.Pod(rObj.Pod))

	return &hook.Result{}, nil
}

func (ci *certIssuance) issueCertificate(obj *hook.RunContainerOptionsObj, hostCertsDir, role string) error {
	certName := role + ".crt"
	keyName := role + ".key"

	hostCertFile := filepath.Join(hostCertsDir, certName)
	certOut, err := os.OpenFile(hostCertFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file %q, detail-> %v", hostCertFile, err)
	}
	defer certOut.Close()

	hostKeyFile := filepath.Join(hostCertsDir, keyName)
	keyOut, err := os.OpenFile(hostKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open file %q, detail-> %v", hostKeyFile, err)
	}
	defer keyOut.Close()

	var envCertFile, envKeyFile string
	if role == communicationRoleServer {
		if err = ci.createServerCertificate(obj.PodIPs, certOut, keyOut); err != nil {
			return fmt.Errorf("failed to create server certificate, detail-> %v", err)
		}
		envCertFile = common.EnvServerCertFile
		envKeyFile = common.EnvServerKeyFile
	} else {
		if err = ci.createClientCertificate(obj.Pod, certOut, keyOut); err != nil {
			return fmt.Errorf("failed to create client certificate, detail-> %v", err)
		}
		envCertFile = common.EnvClientCertFile
		envKeyFile = common.EnvClientKeyFile
	}

	ctrCertFile := filepath.Join(defaultContainerCertsPath, certName)
	ctrKeyFile := filepath.Join(defaultContainerCertsPath, keyName)

	injectCertificate(obj, role+"-cert", envCertFile, hostCertFile, ctrCertFile)
	injectCertificate(obj, role+"-key", envKeyFile, hostKeyFile, ctrKeyFile)

	return nil
}

func injectCertificate(obj *hook.RunContainerOptionsObj, name, envKey, hostPath, containerPath string) {
	obj.Opts.Envs = append(obj.Opts.Envs, container.EnvVar{
		Name:  envKey,
		Value: containerPath,
	})
	obj.Opts.Mounts = append(obj.Opts.Mounts, container.Mount{
		Name:          name,
		ContainerPath: containerPath,
		HostPath:      hostPath,
		ReadOnly:      true,
	})
}

func (ci *certIssuance) createServerCertificate(podIPs []string, certOut, keyOut io.Writer) error {
	var netIPs []net.IP
	for _, podIP := range podIPs {
		netIPs = append(netIPs, net.ParseIP(podIP))
	}

	netIPs = append(netIPs, net.IPv4(127, 0, 0, 1))

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject:      pkix.Name{CommonName: netIPs[0].String()},
		IPAddresses:  netIPs,
		DNSNames:     []string{"*.svc"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	return tls.GenerateX509KeyPair(ci.caCert, ci.caKey, cert, certOut, keyOut)
}

func (ci *certIssuance) createClientCertificate(pod *corev1.Pod, certOut, keyOut io.Writer) error {
	var podGroup string

	if pod.Annotations[kubetypes.ConfigSourceAnnotationKey] == kubetypes.FileSource {
		podGroup = common.PodIdentityGroupInternal
	} else {
		podGroup = common.PodIdentityGroupExternal
	}

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject:      pkix.Name{OrganizationalUnit: []string{podGroup}, CommonName: pod.Name},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	return tls.GenerateX509KeyPair(ci.caCert, ci.caKey, cert, certOut, keyOut)
}
