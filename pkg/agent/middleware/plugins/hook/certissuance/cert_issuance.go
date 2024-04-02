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
	"bytes"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	defaultCertsDirName       = "certs"
	defaultContainerCertsPath = "./kuscia/certs"

	communicationRoleServer = "server"
	communicationRoleClient = "client"

	certsVolumeName = "certs"
)

func Register() {
	plugin.Register(common.PluginNameCertIssuance, &certIssuance{})
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
	if dependencies.AgentConfig.DomainCAKey == nil || dependencies.AgentConfig.DomainCACert == nil {
		nlog.Infof("Plugin cert-issuance will not be registered, signingCert=%v, signingKey=%v", dependencies.AgentConfig.DomainCACert, dependencies.AgentConfig.DomainCAKey)
		return nil
	}

	ci.signingCertFile = dependencies.AgentConfig.DomainCACertFile
	ci.caCert = dependencies.AgentConfig.DomainCACert
	ci.caKey = dependencies.AgentConfig.DomainCAKey
	ci.initialized = true

	hook.Register(common.PluginNameCertIssuance, ci)

	return nil
}

// CanExec implements the hook.Handler interface.
func (ci *certIssuance) CanExec(ctx hook.Context) bool {
	if !ci.initialized {
		return false
	}

	var pod *corev1.Pod
	switch ctx.Point() {
	case hook.PointGenerateContainerOptions:
		rObj, ok := ctx.(*hook.GenerateContainerOptionContext)
		if !ok {
			return false
		}
		pod = rObj.Pod
	case hook.PointK8sProviderSyncPod:
		syncPodCtx, ok := ctx.(*hook.K8sProviderSyncPodContext)
		if !ok {
			return false
		}
		pod = syncPodCtx.Pod
	default:
		return false
	}

	if pod == nil {
		nlog.Warnf("find pod == nil on certIssuance.CanExec, plugin will not exec")
		return false
	}

	if pod == nil || pod.Labels == nil ||
		(pod.Labels[common.LabelCommunicationRoleServer] != common.True &&
			pod.Labels[common.LabelCommunicationRoleClient] != common.True) {
		return false
	}

	return true
}

// ExecHook implements the hook.Handler interface.
// It renders the configuration template and writes the generated real configuration content to a new file/directory.
// The value of hostPath will be replaced by the new file/directory path.
func (ci *certIssuance) ExecHook(ctx hook.Context) (*hook.Result, error) {
	if !ci.initialized {
		return nil, fmt.Errorf("plugin cert-issuance is not initialized")
	}

	switch ctx.Point() {
	case hook.PointGenerateContainerOptions:
		gCtx, ok := ctx.(*hook.GenerateContainerOptionContext)
		if !ok {
			return nil, fmt.Errorf("invalid context type %T", ctx)
		}

		if err := ci.handleGenerateOptionContext(gCtx); err != nil {
			return nil, err
		}
	case hook.PointK8sProviderSyncPod:
		syncPodCtx, ok := ctx.(*hook.K8sProviderSyncPodContext)
		if !ok {
			return nil, fmt.Errorf("invalid context type %T", ctx)
		}

		if err := ci.handleSyncPodContext(syncPodCtx); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid point %v", ctx.Point())
	}

	return &hook.Result{}, nil
}

func (ci *certIssuance) handleSyncPodContext(ctx *hook.K8sProviderSyncPodContext) error {
	issueServerCert := ctx.Pod.Labels[common.LabelCommunicationRoleServer] == common.True
	issueClientCert := ctx.Pod.Labels[common.LabelCommunicationRoleClient] == common.True

	if !issueServerCert && !issueClientCert {
		nlog.Infof("No certificate needs to be issued to the pod %q", format.Pod(ctx.Pod))
		return nil
	}

	certsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "certs",
			Namespace: ctx.Pod.Namespace,
		},
		StringData: map[string]string{},
	}

	// issue server certificate.
	if issueServerCert {
		if err := ci.issueCertificateSecret(ctx, communicationRoleServer, certsSecret); err != nil {
			return err
		}
	}

	// issue client certificate.
	if issueClientCert {
		if err := ci.issueCertificateSecret(ctx, communicationRoleClient, certsSecret); err != nil {
			return err
		}
	}

	// inject ca.
	caCertData, err := os.ReadFile(ci.signingCertFile)
	if err != nil {
		return err
	}
	injectCertificateSecret(ctx.BkPod, certsSecret, "ca.crt", string(caCertData), common.EnvTrustedCAFile, filepath.Join(defaultContainerCertsPath, "ca.crt"))

	ctx.Secrets = append(ctx.Secrets, certsSecret)
	mountCertificateSecret(ctx.BkPod, certsSecret)

	nlog.Infof("Successfully issued certificate(server=%v,client=%v) for pod %q", issueServerCert, issueClientCert, format.Pod(ctx.Pod))
	return nil
}

func (ci *certIssuance) issueCertificateSecret(ctx *hook.K8sProviderSyncPodContext, role string, secret *corev1.Secret) error {
	certName := role + ".crt"
	keyName := role + ".key"

	var certBuf, keyBuf bytes.Buffer

	var envCertFile, envKeyFile string
	if role == communicationRoleServer {
		if err := ci.createServerCertificate([]string{"0.0.0.0"}, &certBuf, &keyBuf); err != nil {
			return fmt.Errorf("failed to create server certificate, detail-> %v", err)
		}
		envCertFile = common.EnvServerCertFile
		envKeyFile = common.EnvServerKeyFile
	} else {
		if err := ci.createClientCertificate(ctx.BkPod, &certBuf, &keyBuf); err != nil {
			return fmt.Errorf("failed to create client certificate, detail-> %v", err)
		}
		envCertFile = common.EnvClientCertFile
		envKeyFile = common.EnvClientKeyFile
	}

	ctrCertFile := filepath.Join(defaultContainerCertsPath, certName)
	ctrKeyFile := filepath.Join(defaultContainerCertsPath, keyName)

	injectCertificateSecret(ctx.BkPod, secret, certName, certBuf.String(), envCertFile, ctrCertFile)
	injectCertificateSecret(ctx.BkPod, secret, keyName, keyBuf.String(), envKeyFile, ctrKeyFile)

	return nil
}

func injectCertificateSecret(bkPod *corev1.Pod, secret *corev1.Secret, dataName, dataValue, envKey, containerPath string) {
	for i := range bkPod.Spec.Containers {
		c := &bkPod.Spec.Containers[i]
		c.Env = append(c.Env, corev1.EnvVar{
			Name:  envKey,
			Value: containerPath,
		})
	}

	secret.StringData[dataName] = dataValue
}

func mountCertificateSecret(bkPod *corev1.Pod, secret *corev1.Secret) {
	bkPod.Spec.Volumes = append(bkPod.Spec.Volumes, corev1.Volume{
		Name: certsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret.Name,
			},
		},
	})

	for i := range bkPod.Spec.Containers {
		c := &bkPod.Spec.Containers[i]
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      certsVolumeName,
			MountPath: filepath.Join(c.WorkingDir, defaultContainerCertsPath),
		})
	}
}

func (ci *certIssuance) handleGenerateOptionContext(ctx *hook.GenerateContainerOptionContext) error {
	issueServerCert := ctx.Pod.Labels[common.LabelCommunicationRoleServer] == common.True
	issueClientCert := ctx.Pod.Labels[common.LabelCommunicationRoleClient] == common.True

	if !issueServerCert && !issueClientCert {
		nlog.Infof("No certificate needs to be issued to the pod %q", format.Pod(ctx.Pod))
		return nil
	}

	hostCertsDir := filepath.Join(ctx.ContainerDir, defaultCertsDirName)
	if err := paths.EnsureDirectory(hostCertsDir, true); err != nil {
		return err
	}

	// issue server certificate.
	if issueServerCert {
		if err := ci.issueCertificateLocal(ctx, hostCertsDir, communicationRoleServer); err != nil {
			return err
		}
	}

	// issue client certificate.
	if issueClientCert {
		if err := ci.issueCertificateLocal(ctx, hostCertsDir, communicationRoleClient); err != nil {
			return err
		}
	}

	// inject ca file.
	signingCertFile := filepath.Join(hostCertsDir, "ca.crt")
	if err := paths.CopyFile(ci.signingCertFile, signingCertFile); err != nil {
		return fmt.Errorf("failed to copy signing certificate file %q to %q, detail-> %v", ci.signingCertFile, signingCertFile, err)
	}
	injectCertificateLocal(ctx, "trusted-ca", common.EnvTrustedCAFile, signingCertFile, filepath.Join(defaultContainerCertsPath, "ca.crt"))

	nlog.Infof("Successfully issued certificate(server=%v,client=%v) for container %q in pod %q", issueServerCert, issueClientCert, ctx.Container.Name, format.Pod(ctx.Pod))

	return nil
}

func (ci *certIssuance) issueCertificateLocal(obj *hook.GenerateContainerOptionContext, hostCertsDir, role string) error {
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

	injectCertificateLocal(obj, role+"-cert", envCertFile, hostCertFile, ctrCertFile)
	injectCertificateLocal(obj, role+"-key", envKeyFile, hostKeyFile, ctrKeyFile)

	return nil
}

func injectCertificateLocal(obj *hook.GenerateContainerOptionContext, name, envKey, hostPath, containerPath string) {
	obj.Opts.Envs = append(obj.Opts.Envs, container.EnvVar{
		Name:  envKey,
		Value: containerPath,
	})
	obj.Opts.Mounts = append(obj.Opts.Mounts, container.Mount{
		Name:          name,
		ContainerPath: filepath.Join(obj.Container.WorkingDir, containerPath),
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
