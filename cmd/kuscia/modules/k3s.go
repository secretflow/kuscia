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

package modules

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/datastore"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog/ljwriter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/process"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
)

type k3sModule struct {
	rootDir           string
	kubeconfigFile    string
	bindAddress       string
	listenPort        string
	dataDir           string
	datastoreEndpoint string
	clusterToken      string
	hostIP            string
	enableAudit       bool
	LogConfig         nlog.LogConfig
}

func (s *k3sModule) readyz(host string) error {

	// check k3s process
	if !process.CheckExists("k3s") {
		errMsg := "process [k3s] is not exists"
		nlog.Error(errMsg)
		return errors.New(errMsg)
	}

	cl := http.Client{}
	// check file exist
	serverCaFilePath := filepath.Join(s.dataDir, "server/tls/server-ca.crt")
	clientAdminCrtFilePath := filepath.Join(s.dataDir, "server/tls/client-admin.crt")
	clientAdminKeyFilePath := filepath.Join(s.dataDir, "server/tls/client-admin.key")

	if fileExistError := paths.CheckAllFileExist(serverCaFilePath, clientAdminCrtFilePath, clientAdminKeyFilePath); fileExistError != nil {
		err := fmt.Errorf("%s. Please check the k3s service is running successfully ", fileExistError)
		nlog.Error(err)
		return err
	}

	caCertFile, err := os.ReadFile(serverCaFilePath)
	if err != nil {
		nlog.Error(err)
		return err
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertFile) {
		msg := "caCertFile format error"
		nlog.Error(msg)
		return fmt.Errorf("%s", msg)
	}
	certPEMBlock, err := os.ReadFile(clientAdminCrtFilePath)
	if err != nil {
		nlog.Error(err)
		return err
	}

	keyPEMBlock, err := os.ReadFile(clientAdminKeyFilePath)
	if err != nil {
		nlog.Error(err)
		return err
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		nlog.Error(err)
		return err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:      caCertPool,
			Certificates: []tls.Certificate{cert},
		},
	}
	cl.Transport = tr
	req, err := http.NewRequest(http.MethodGet, host+"/readyz", nil)
	if err != nil {
		nlog.Errorf("NewRequest error:%s", err.Error())
		return err
	}
	resp, err := cl.Do(req)
	if err != nil {
		nlog.Errorf("Get ready err:%s", err.Error())
		return err
	}
	if resp == nil || resp.Body == nil {
		nlog.Error("resp must has body")
		return fmt.Errorf("resp must has body")
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		nlog.Error("ReadAll fail")
		return err
	}
	if string(respBytes) != "ok" {
		return errors.New("not ready")
	}
	return nil
}

func NewK3s(i *Dependencies) Module {
	var clusterToken string
	clusterToken = i.Master.ClusterToken
	if clusterToken == "" {
		clusterToken = fmt.Sprintf("%x", md5.Sum([]byte(i.DomainID)))
	}
	hostIP, err := network.GetHostIP()
	if err != nil {
		nlog.Fatal(err)
	}
	return &k3sModule{
		rootDir:           i.RootDir,
		kubeconfigFile:    i.KubeconfigFile,
		bindAddress:       "0.0.0.0",
		listenPort:        "6443",
		hostIP:            hostIP,
		dataDir:           filepath.Join(i.RootDir, k3sDataDirPrefix),
		enableAudit:       false,
		datastoreEndpoint: i.Master.DatastoreEndpoint,
		clusterToken:      clusterToken,
		LogConfig:         *i.LogConfig,
	}
}

func (s *k3sModule) Run(ctx context.Context) error {
	args := []string{
		"server",
		"-v=5",
		"-d=" + s.dataDir,
		"-o=" + s.kubeconfigFile,
		"--disable-agent",
		"--bind-address=" + s.bindAddress,
		"--https-listen-port=" + s.listenPort,
		"--node-ip=" + s.hostIP,
		"--disable-cloud-controller",
		"--disable-network-policy",
		"--disable-scheduler",
		"--flannel-backend=none",
		"--disable=traefik",
		"--disable=coredns",
		"--disable=servicelb",
		"--disable=local-storage",
		"--disable=metrics-server",
		"--kube-apiserver-arg=event-ttl=10m",
	}
	if s.datastoreEndpoint != "" {
		args = append(args, "--datastore-endpoint="+s.datastoreEndpoint)
	}
	if s.clusterToken != "" {
		args = append(args, "--token="+s.clusterToken)
	}
	if s.enableAudit {
		args = append(args,
			"--kube-apiserver-arg=audit-log-path="+filepath.Join(s.rootDir, pkgcom.LogPrefix, "k3s-audit.log"),
			"--kube-apiserver-arg=audit-policy-file="+filepath.Join(s.rootDir, pkgcom.ConfPrefix, "k3s/k3s-audit-policy.yaml"),
			"--kube-apiserver-arg=audit-log-maxbackup=10",
			"--kube-apiserver-arg=audit-log-maxsize=300",
		)
	}

	sp := supervisor.NewSupervisor("k3s", nil, -1)
	s.LogConfig.LogPath = filepath.Join(s.rootDir, pkgcom.LogPrefix, "k3s.log")
	lj, _ := ljwriter.New(&s.LogConfig)
	n := nlog.NewNLog(nlog.SetWriter(lj))

	return sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.CommandContext(ctx, filepath.Join(s.rootDir, "bin/k3s"), args...)
		cmd.Stderr = n
		cmd.Stdout = n

		envs := os.Environ()
		envs = append(envs, "CATTLE_NEW_SIGNED_CERT_EXPIRATION_DAYS=3650")
		cmd.Env = envs
		return &ModuleCMD{
			cmd:   cmd,
			score: &k3sOOMScore,
		}
	})
}

func (s *k3sModule) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	tickerReady := time.NewTicker(time.Second)
	defer tickerReady.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tickerReady.C:
			if nil == s.readyz("https://127.0.0.1:"+s.listenPort) {
				return nil
			}
		case <-ticker.C:
			return fmt.Errorf("wait k3s ready timeout")
		}
	}
}

func (s *k3sModule) Name() string {
	return "k3s"
}

func RunK3sWithDestroy(conf *Dependencies) error {
	runCtx, cancel := context.WithCancel(context.Background())
	shutdownEntry := newShutdownHookEntry(1 * time.Second)
	conf.RegisterDestroyFunc(DestroyFunc{
		Name:              "k3s",
		DestroyCh:         runCtx.Done(),
		DestroyFn:         cancel,
		ShutdownHookEntry: shutdownEntry,
	})
	return RunK3s(runCtx, cancel, conf, shutdownEntry)
}

func RunK3s(ctx context.Context, cancel context.CancelFunc, conf *Dependencies, shutdownEntry *shutdownHookEntry) error {
	// check DatastoreEndpoint
	if err := datastore.CheckDatastoreEndpoint(conf.Master.DatastoreEndpoint); err != nil {
		nlog.Error(err)
		cancel()
		return err
	}

	m := NewK3s(conf)
	go func() {
		defer func() {
			if shutdownEntry != nil {
				shutdownEntry.RunShutdown()
			}
		}()
		if err := m.Run(ctx); err != nil {
			nlog.Error(err)
			cancel()
		}
	}()
	if err := m.WaitReady(ctx); err != nil {
		nlog.Error(err)
		cancel()
	} else {
		if err = applyCRD(conf); err != nil {
			nlog.Error(err)
			cancel()
		}
		if err = applyKusciaResources(conf); err != nil {
			nlog.Error(err)
			cancel()
		}
		if err = genKusciaKubeConfig(conf); err != nil {
			nlog.Error(err)
			cancel()
		}
		nlog.Info("k3s is ready")
	}

	return nil
}

func applyCRD(conf *Dependencies) error {
	dirPath := filepath.Join(conf.RootDir, "crds/v1alpha1")
	dirs, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	sw := sync.WaitGroup{}
	for _, dir := range dirs {
		if dir.IsDir() {
			continue
		}
		file := filepath.Join(dirPath, dir.Name())
		sw.Add(1)
		go func(f string) {
			applyFile(conf, f)
			sw.Done()
		}(file)
	}
	sw.Wait()
	return nil
}

func genKusciaKubeConfig(conf *Dependencies) error {
	c := &kusciaConfig{
		serverCertFile:         filepath.Join(conf.RootDir, k3sDataDirPrefix, "server/tls/server-ca.crt"),
		clientKeyFile:          filepath.Join(conf.RootDir, k3sDataDirPrefix, "server/tls/client-ca.key"),
		clientCertFile:         filepath.Join(conf.RootDir, k3sDataDirPrefix, "server/tls/client-ca.crt"),
		clusterRoleFile:        filepath.Join(conf.RootDir, pkgcom.ConfPrefix, "kuscia-clusterrole.yaml"),
		clusterRoleBindingFile: filepath.Join(conf.RootDir, pkgcom.ConfPrefix, "kuscia-clusterrolebinding.yaml"),
		kubeConfigTmplFile:     filepath.Join(conf.RootDir, pkgcom.ConfPrefix, "kuscia.kubeconfig.tmpl"),
		kubeConfig:             conf.KusciaKubeConfig,
	}

	// generate kuscia client certs
	serverCert, err := tlsutils.LoadCertFile(c.serverCertFile)
	if err != nil {
		return err
	}
	nlog.Info("load serverCertFile successfully")
	rootCert, rootKey, err := tlsutils.LoadX509EcKeyPair(c.clientCertFile, c.clientKeyFile)
	if err != nil {
		return err
	}
	nlog.Info("load rootCert, rootKey successfully")
	certTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject: pkix.Name{
			CommonName: "kuscia",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	var certOut, keyOut bytes.Buffer
	if err := tlsutils.GenerateX509KeyPair(rootCert, rootKey, certTmpl, &certOut, &keyOut); err != nil {
		return err
	}
	nlog.Info("generate certOut, keyOut successfully")
	// generate kuscia kubeconfig
	s := struct {
		ServerCert string
		Endpoint   string
		KusciaCert string
		KusciaKey  string
	}{
		ServerCert: base64.StdEncoding.EncodeToString(serverCert),
		Endpoint:   conf.ApiserverEndpoint,
		KusciaCert: base64.StdEncoding.EncodeToString(certOut.Bytes()),
		KusciaKey:  base64.StdEncoding.EncodeToString(keyOut.Bytes()),
	}
	if err := common.RenderConfig(c.kubeConfigTmplFile, c.kubeConfig, s); err != nil {
		return err
	}
	nlog.Info("generate kuscia kubeconfig successfully")
	// apply kuscia clusterRole and clusterRoleBinding
	roleFiles := []string{
		c.clusterRoleFile,
		c.clusterRoleBindingFile,
	}
	sw := sync.WaitGroup{}
	for _, file := range roleFiles {
		sw.Add(1)
		go func(f string) {
			applyFile(conf, f)
			sw.Done()
		}(file)
	}
	sw.Wait()
	return nil
}

func applyKusciaResources(conf *Dependencies) error {
	// apply kuscia clusterRole
	resourceFiles := []string{
		filepath.Join(conf.RootDir, pkgcom.ConfPrefix, "domain-cluster-res.yaml"),
	}
	sw := sync.WaitGroup{}
	for _, file := range resourceFiles {
		sw.Add(1)
		go func(f string) {
			applyFile(conf, f)
			sw.Done()
		}(file)
	}
	sw.Wait()
	return nil
}

func applyFile(conf *Dependencies, file string) error {
	cmd := exec.Command(filepath.Join(conf.RootDir, "bin/kubectl"), "--kubeconfig", conf.KubeconfigFile, "apply", "-f", file)
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		nlog.Fatalf("apply %s err:%s", file, err.Error())
		return err
	}
	nlog.Infof("apply %s", file)
	return nil
}

type kusciaConfig struct {
	serverCertFile         string
	clientKeyFile          string
	clientCertFile         string
	clusterRoleFile        string
	clusterRoleBindingFile string
	kubeConfigTmplFile     string
	kubeConfig             string
}
