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
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pkgcom "github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/datastore"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/ljwriter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/process"
	"github.com/secretflow/kuscia/pkg/utils/supervisor"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
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
	conf              *ModuleRuntimeConfigs

	// ready
	readyCh    chan struct{}
	readyError error
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

func NewK3s(i *ModuleRuntimeConfigs) (Module, error) {
	var clusterToken string
	clusterToken = i.Master.ClusterToken
	if clusterToken == "" {
		clusterToken = fmt.Sprintf("%x", md5.Sum([]byte(i.DomainID)))
	}
	hostIP, err := network.GetHostIP()
	if err != nil {
		nlog.Fatal(err)
	}
	// check DatastoreEndpoint
	if err := datastore.CheckDatastoreEndpoint(i.Master.DatastoreEndpoint); err != nil {
		nlog.Errorf("k3s check datastore endpoint failed with: %s", err.Error())
		return nil, err
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
		conf:              i,
		readyCh:           make(chan struct{}),
	}, nil
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
	s.LogConfig.LogPath = buildK3sLogPath(s.rootDir)
	lj, _ := ljwriter.New(&s.LogConfig)
	n := nlog.NewNLog(nlog.SetWriter(lj))

	go func() {
		s.readyError = s.startCheckReady(ctx)
		if s.readyError == nil {
			s.readyError = s.initKusciaEnvAfterReady(ctx)
		}
		close(s.readyCh)
		nlog.Infof("close k3s ready chan")
	}()

	err := sp.Run(ctx, func(ctx context.Context) supervisor.Cmd {
		cmd := exec.Command(filepath.Join(s.rootDir, "bin/k3s"), args...)
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

	if err != nil {
		readK3sLog(s.LogConfig.LogPath, 10)
	}

	return err
}

func (s *k3sModule) WaitReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.readyCh:
		nlog.Infof("k3s is ready now")
		return s.readyError
	}
}

func (s *k3sModule) Name() string {
	return "k3s"
}

func (s *k3sModule) startCheckReady(ctx context.Context) error {
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

func (s *k3sModule) initKusciaEnvAfterReady(ctx context.Context) error {
	if err := applyCRD(s.conf); err != nil {
		nlog.Errorf("init failed after k3s started with err: %s", err.Error())
		return err
	}
	if err := applyKusciaResources(s.conf); err != nil {
		nlog.Errorf("init failed after k3s started with err: %s", err.Error())
		return err
	}
	if err := genKusciaKubeConfig(s.conf); err != nil {
		nlog.Errorf("init failed after k3s started with err: %s", err.Error())
		return err
	}

	clients, err := kubeconfig.CreateClientSetsFromKubeconfig(s.conf.KubeconfigFile, s.conf.ApiserverEndpoint)
	if err != nil {
		nlog.Errorf("init failed after k3s started with err: %s", err.Error())
		return err
	}

	s.conf.Clients = clients

	if err := createDefaultDomain(ctx, s.conf); err != nil {
		nlog.Errorf("init failed after k3s started with err: %s", err.Error())
		return err
	}
	nlog.Infof("registed namespace %s without error", s.conf.DomainID)

	if err := createCrossNamespace(ctx, s.conf); err != nil {
		nlog.Errorf("init failed after k3s started with err: %s", err.Error())
		return err
	}

	nlog.Infof("K3s init done")

	return nil

}

func (s *k3sModule) readyz(host string) error {
	// check k3s process
	if !process.CheckExists("k3s") {
		errMsg := "process [k3s] is not exists"
		nlog.Error(errMsg)
		return errors.New(errMsg)
	}

	// check file exist
	serverCaFilePath := filepath.Join(s.dataDir, "server/tls/server-ca.crt")
	clientAdminCrtFilePath := filepath.Join(s.dataDir, "server/tls/client-admin.crt")
	clientAdminKeyFilePath := filepath.Join(s.dataDir, "server/tls/client-admin.key")

	if fileExistError := paths.CheckAllFileExist(serverCaFilePath, clientAdminCrtFilePath, clientAdminKeyFilePath); fileExistError != nil {
		err := fmt.Errorf("%s. waiting k3s service starting", fileExistError)
		nlog.Warn(err)
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

	cl := http.Client{}
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

func applyCRD(conf *ModuleRuntimeConfigs) error {
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

func genKusciaKubeConfig(conf *ModuleRuntimeConfigs) error {
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

func applyKusciaResources(conf *ModuleRuntimeConfigs) error {
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

func applyFile(conf *ModuleRuntimeConfigs, file string) error {
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

func createDefaultDomain(ctx context.Context, conf *ModuleRuntimeConfigs) error {
	nlog.Infof("try to regist namespace: %s", conf.DomainID)
	certRaw, err := os.ReadFile(conf.DomainCertFile)
	if err != nil {
		return err
	}

	certStr := base64.StdEncoding.EncodeToString(certRaw)
	_, err = conf.Clients.KusciaClient.KusciaV1alpha1().Domains().Create(ctx, &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: conf.DomainID,
		},
		Spec: kusciaapisv1alpha1.DomainSpec{
			Cert: certStr,
		}}, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		dm, err := conf.Clients.KusciaClient.KusciaV1alpha1().Domains().Get(ctx, conf.DomainID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if dm.Spec.Cert != certStr {
			nlog.Warnf("domain %s cert is not match, will update", conf.DomainID)
			dm.Spec.Cert = certStr
			_, err = conf.Clients.KusciaClient.KusciaV1alpha1().Domains().Update(ctx, dm, metav1.UpdateOptions{})
			return err
		}
		return nil
	}

	return err
}

func createCrossNamespace(ctx context.Context, conf *ModuleRuntimeConfigs) error {
	if _, err := conf.Clients.KubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkgcom.KusciaCrossDomain,
		},
	}, metav1.CreateOptions{}); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

// readK3sLog read the last N lines of k3s log
func readK3sLog(k3sLogFilePath string, nLastLine int) {
	lines, readFileErr := readLastNLinesAsString(k3sLogFilePath, nLastLine)
	if readFileErr != nil {
		nlog.Errorf("[k3s] read k3s file error: %v", readFileErr)
	} else {
		nlog.Errorf("[k3s] Log content: %s [k3s] Log end", lines)
	}
}

// readLastNLinesAsString read the last N lines of a file as string
func readLastNLinesAsString(filePath string, n int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	fileSize := stat.Size()

	buf := make([]byte, 0)
	newlineChar := byte('\n')

	for offset := int64(1); offset <= fileSize; offset++ {
		_, err = file.Seek(-offset, io.SeekEnd)
		if err != nil {
			return "", err
		}

		b := make([]byte, 1)
		_, err = file.Read(b)
		if err != nil {
			return "", err
		}

		if b[0] == newlineChar {
			if len(buf) > 0 { // Avoid appending empty lines at the end of file
				n--
			}
			if n == 0 {
				break
			}
		}
		buf = append([]byte{b[0]}, buf...)
	}

	return string(buf), nil
}

// buildK3sLogPath build the absolute path of k3s log
func buildK3sLogPath(rootDir string) string {
	return filepath.Join(rootDir, pkgcom.LogPrefix, "k3s.log")
}
