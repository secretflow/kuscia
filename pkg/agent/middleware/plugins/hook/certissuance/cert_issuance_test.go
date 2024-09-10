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
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

var testPods = []corev1.Pod{
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "test-namespace",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "default-container",
				},
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "test-namespace",
			Labels: map[string]string{
				common.LabelCommunicationRoleServer: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "default-container",
				},
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "test-namespace",
			Labels: map[string]string{
				common.LabelCommunicationRoleServer: "true",
				common.LabelCommunicationRoleClient: "true",
			},
			Annotations: map[string]string{
				kubetypes.ConfigSourceAnnotationKey: kubetypes.FileSource,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "default-container",
				},
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-3",
			Namespace: "test-namespace",
			Labels: map[string]string{
				common.LabelCommunicationRoleServer: "false",
			},
			Annotations: map[string]string{
				kubetypes.ConfigSourceAnnotationKey: kubetypes.FileSource,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "default-container",
				},
			},
		},
	},
}

func TestCertIssuanceWithGenerateOptCtx(t *testing.T) {
	rootDir := t.TempDir()
	certsDir := filepath.Join(rootDir, "certs")
	signingCertFile := filepath.Join(certsDir, "ca.crt")
	signingKeyFile := filepath.Join(certsDir, "ca.key")

	assert.NoError(t, paths.EnsureDirectory(certsDir, true))
	assert.NoError(t, tls.CreateCAFile("testca", signingCertFile, signingKeyFile))

	signingCert, err := tls.ParseCert(nil, signingCertFile)
	assert.NoError(t, err)
	signingKey, err := tls.ParseKey(nil, signingKeyFile)
	assert.NoError(t, err)

	dep := &plugin.Dependencies{
		AgentConfig: &config.AgentConfig{
			DomainCACertFile: signingCertFile,
			DomainCACert:     signingCert,
			DomainCAKey:      signingKey,
		},
	}

	ci := &certIssuance{}
	assert.NoError(t, ci.Init(context.Background(), dep, nil))

	tests := []struct {
		ctx                  *hook.GenerateContainerOptionContext
		canExec              bool
		issueServerCert      bool
		issueClientCert      bool
		injectCA             bool
		podGroupInClientCert string
	}{
		{
			ctx: &hook.GenerateContainerOptionContext{
				Pod:          &testPods[0],
				Container:    &testPods[0].Spec.Containers[0],
				Opts:         &pkgcontainer.RunContainerOptions{},
				ContainerDir: filepath.Join(rootDir, "pod-0"),
			},
			canExec: false,
		},
		{
			ctx: &hook.GenerateContainerOptionContext{
				Pod:          &testPods[1],
				Container:    &testPods[1].Spec.Containers[0],
				Opts:         &pkgcontainer.RunContainerOptions{},
				PodIPs:       []string{"192.168.1.1"},
				ContainerDir: filepath.Join(rootDir, "pod-1"),
			},
			canExec:              true,
			issueServerCert:      true,
			issueClientCert:      false,
			injectCA:             true,
			podGroupInClientCert: common.PodIdentityGroupExternal,
		},
		{
			ctx: &hook.GenerateContainerOptionContext{
				Pod:          &testPods[2],
				Container:    &testPods[2].Spec.Containers[0],
				Opts:         &pkgcontainer.RunContainerOptions{},
				PodIPs:       []string{"192.168.1.1"},
				ContainerDir: filepath.Join(rootDir, "pod-2"),
			},
			canExec:              true,
			issueServerCert:      true,
			issueClientCert:      true,
			injectCA:             true,
			podGroupInClientCert: common.PodIdentityGroupInternal,
		},
		{
			ctx: &hook.GenerateContainerOptionContext{
				Pod:          &testPods[3],
				Container:    &testPods[3].Spec.Containers[0],
				Opts:         &pkgcontainer.RunContainerOptions{},
				PodIPs:       []string{"192.168.1.1"},
				ContainerDir: filepath.Join(rootDir, "pod-3"),
			},
			canExec: false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Case-%d", i), func(t *testing.T) {
			assert.NoError(t, paths.EnsureDirectory(tt.ctx.ContainerDir, true))

			assert.Equal(t, tt.canExec, ci.CanExec(tt.ctx))
			if !tt.canExec {
				return
			}

			_, err := ci.ExecHook(tt.ctx)
			assert.NoError(t, err)

			var ctrServerCertFile, ctrServerKeyFile, ctrClientCertFile, ctrClientKeyFile string
			var hostServerCertFile, hostClientCertFile string
			for _, mount := range tt.ctx.Opts.Mounts {
				switch mount.Name {
				case "server-cert":
					ctrServerCertFile = mount.ContainerPath
					hostServerCertFile = mount.HostPath
				case "server-key":
					ctrServerKeyFile = mount.ContainerPath
				case "client-cert":
					ctrClientCertFile = mount.ContainerPath
					hostClientCertFile = mount.HostPath
				case "client-key":
					ctrClientKeyFile = mount.ContainerPath
				}
			}

			assert.Equal(t, tt.issueServerCert, ctrServerCertFile != "" && ctrServerKeyFile != "")
			assert.Equal(t, tt.issueClientCert, ctrClientCertFile != "" && ctrClientKeyFile != "")

			if tt.issueServerCert {
				cert, err := loadCertificateFile(hostServerCertFile)
				assert.NoError(t, err)
				assert.True(t, inIPAddresses(cert.IPAddresses, "127.0.0.1"))
				for _, ip := range tt.ctx.PodIPs {
					assert.True(t, inIPAddresses(cert.IPAddresses, ip))
				}
			}

			if tt.issueClientCert {
				cert, err := loadCertificateFile(hostClientCertFile)
				assert.NoError(t, err)
				assert.True(t, len(cert.Subject.OrganizationalUnit) > 0)
				assert.Equal(t, tt.podGroupInClientCert, cert.Subject.OrganizationalUnit[0])
			}

		})
	}
}

func TestCertIssuanceWithSyncPodContext(t *testing.T) {
	rootDir := t.TempDir()
	certsDir := filepath.Join(rootDir, "certs")
	signingCertFile := filepath.Join(certsDir, "ca.crt")
	signingKeyFile := filepath.Join(certsDir, "ca.key")
	assert.NoError(t, paths.EnsureDirectory(certsDir, true))
	assert.NoError(t, tls.CreateCAFile("testca", signingCertFile, signingKeyFile))

	signingCert, err := tls.ParseCert(nil, signingCertFile)
	assert.NoError(t, err)
	signingKey, err := tls.ParseKey(nil, signingKeyFile)
	assert.NoError(t, err)

	dep := &plugin.Dependencies{
		AgentConfig: &config.AgentConfig{
			DomainCACert:     signingCert,
			DomainCAKey:      signingKey,
			DomainCACertFile: signingCertFile,
		},
	}

	ci := &certIssuance{}
	assert.NoError(t, ci.Init(context.Background(), dep, nil))

	tests := []struct {
		ctx                  *hook.K8sProviderSyncPodContext
		canExec              bool
		issueServerCert      bool
		issueClientCert      bool
		injectCA             bool
		podGroupInClientCert string
	}{
		{
			ctx: &hook.K8sProviderSyncPodContext{
				Pod: &testPods[0],
			},
			canExec: false,
		},
		{
			ctx: &hook.K8sProviderSyncPodContext{
				Pod: &testPods[1],
			},
			canExec:              true,
			issueServerCert:      true,
			issueClientCert:      false,
			injectCA:             true,
			podGroupInClientCert: common.PodIdentityGroupExternal,
		},
		{
			ctx: &hook.K8sProviderSyncPodContext{
				Pod: &testPods[2],
			},
			canExec:              true,
			issueServerCert:      true,
			issueClientCert:      true,
			injectCA:             true,
			podGroupInClientCert: common.PodIdentityGroupInternal,
		},
		{
			ctx: &hook.K8sProviderSyncPodContext{
				Pod: &testPods[3],
			},
			canExec: false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Case-%d", i), func(t *testing.T) {
			tt.ctx.BkPod = tt.ctx.Pod.DeepCopy()
			tt.ctx.BkPod.Namespace = "bk-namespace"

			assert.Equal(t, tt.canExec, ci.CanExec(tt.ctx))
			if !tt.canExec {
				return
			}

			_, err := ci.ExecHook(tt.ctx)
			assert.NoError(t, err)

			ctr := tt.ctx.BkPod.Spec.Containers[0]
			envs := map[string]string{}
			for _, env := range ctr.Env {
				envs[env.Name] = env.Value
			}

			assert.Equal(t, 1, len(tt.ctx.Secrets))
			certsSecret := tt.ctx.Secrets[0]
			ctrServerCertData := certsSecret.StringData["server.crt"]
			ctrServerKeyData := certsSecret.StringData["server.key"]
			if tt.issueServerCert {
				assert.True(t, ctrServerCertData != "" && ctrServerKeyData != "")
				assert.Equal(t, filepath.Join(defaultContainerCertsPath, "server.crt"), envs[common.EnvServerCertFile])
				assert.Equal(t, filepath.Join(defaultContainerCertsPath, "server.key"), envs[common.EnvServerKeyFile])
				cert, err := loadCertificate([]byte(ctrServerCertData))
				assert.NoError(t, err)
				assert.True(t, inIPAddresses(cert.IPAddresses, "127.0.0.1"))

			}

			ctrClientCertData := certsSecret.StringData["client.crt"]
			ctrClientKeyData := certsSecret.StringData["client.key"]
			if tt.issueClientCert {
				assert.True(t, ctrClientCertData != "" && ctrClientKeyData != "")
				assert.Equal(t, filepath.Join(defaultContainerCertsPath, "client.crt"), envs[common.EnvClientCertFile])
				assert.Equal(t, filepath.Join(defaultContainerCertsPath, "client.key"), envs[common.EnvClientKeyFile])

				cert, err := loadCertificate([]byte(ctrClientCertData))
				assert.NoError(t, err)
				assert.True(t, len(cert.Subject.OrganizationalUnit) > 0)
				assert.Equal(t, tt.podGroupInClientCert, cert.Subject.OrganizationalUnit[0])
			}

			ctrCAData := certsSecret.StringData["ca.crt"]
			assert.Equal(t, tt.injectCA, ctrCAData != "")
			assert.Equal(t, filepath.Join(defaultContainerCertsPath, "ca.crt"), envs[common.EnvTrustedCAFile])

			assert.Equal(t, 1, len(ctr.VolumeMounts))
			assert.Equal(t, certsVolumeName, ctr.VolumeMounts[0].Name)
		})
	}
}

func inIPAddresses(ipAddresses []net.IP, ip string) bool {
	for _, address := range ipAddresses {
		if address.String() == ip {
			return true
		}
	}
	return false
}

func loadCertificateFile(certFile string) (*x509.Certificate, error) {
	certContent, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}

	return loadCertificate(certContent)
}

func loadCertificate(data []byte) (*x509.Certificate, error) {
	certBlock, _ := pem.Decode(data)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}
