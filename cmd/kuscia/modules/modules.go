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
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	"github.com/secretflow/kuscia/pkg/utils/process"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/logs"
)

var (
	k3sDataDirPrefix              = "var/k3s/"
	kusciaLogPath                 = "var/logs/kuscia.log"
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
	SsExportPort            string
	KtExportPort            string
	NodeExportPort          string
	MetricExportPort        string
	KusciaKubeConfig        string
	EnableContainerd        bool
	SecretBackendHolder     *secretbackend.Holder
	Image                   *confloader.ImageConfig
	DomainCertByMasterValue atomic.Value // the value is <*x509.Certificate>
	LogConfig               *nlog.LogConfig
	Logrorate               confloader.LogrotateConfig
	destroyFuncs            []DestroyFunc
	stopCh                  chan struct{}
	stopped                 int32 // 0: not stopped 1: stopped
	lock                    sync.Mutex
}

type DestroyFunc struct {
	Name              string
	DestroyFn         func()
	DestroyCh         <-chan struct{}
	ShutdownHookEntry *shutdownHookEntry
}

func (d *Dependencies) Destroy() {
	d.lock.Lock()
	defer d.lock.Unlock()
	nlog.Infof("Begin to destroy modules......")
	size := len(d.destroyFuncs)
	if size > 0 {
		for i := size - 1; i >= 0; i-- {
			destroy := d.destroyFuncs[i]
			nlog.Infof("Module [%s] begin to destroy", destroy.Name)
			destroy.DestroyFn()
			shutdownEntry := destroy.ShutdownHookEntry
			if shutdownEntry != nil {
				nlog.Infof("module[%s] shutdown entry found", destroy.Name)
				shutdownEntry.WaitDown()
			}
		}
	}
	nlog.Infof("End to destroy modules......")
}

func (d *Dependencies) RegisterDestroyFunc(destroyFunc DestroyFunc) {
	d.lock.Lock()
	defer d.lock.Unlock()
	nlog.Infof("Add destroyFunc [%s]", destroyFunc.Name)
	d.destroyFuncs = append(d.destroyFuncs, destroyFunc)
}

func (d *Dependencies) Stop() <-chan struct{} {
	d.lock.Lock()
	defer d.lock.Unlock()
	for _, destroy := range d.destroyFuncs {
		go func(f DestroyFunc) {
			<-f.DestroyCh
			nlog.Errorf("Module[%s] receive stop", f.Name)
			if atomic.CompareAndSwapInt32(&d.stopped, 0, 1) {
				close(d.stopCh)
			}
		}(destroy)
	}
	return d.stopCh
}

func (d *Dependencies) WaitAllModulesDone(stop <-chan struct{}) {
	select {
	// receive signal exit
	case <-stop:
		nlog.Infof("receive signal exit....")
		// receive module exit
	case <-d.Stop():
		nlog.Infof("receive module exit....")
	}
	d.Destroy()
}

type shutdownHookEntry struct {
	lock    sync.Mutex
	closed  bool
	closeCh chan struct{}
	timeOut time.Duration
}

func NewShutdownHookEntry(timeout time.Duration) *shutdownHookEntry {
	return &shutdownHookEntry{
		timeOut: timeout,
		closeCh: make(chan struct{}),
	}
}

func (s *shutdownHookEntry) RunShutdown() {
	s.lock.Lock()
	defer s.lock.Unlock()
	if !s.closed {
		close(s.closeCh)
		s.closed = true
	}
}

func (s *shutdownHookEntry) WaitDown() {
	timeout := s.timeOut
	timeoutTicker := time.NewTicker(timeout)
	defer timeoutTicker.Stop()
	select {
	case <-timeoutTicker.C:
		nlog.Warnf("exceed timeout: %s", timeout.String())
	case <-s.closeCh:
		nlog.Infof("shut down with closeCh close")
	}
}

func (d *Dependencies) MakeClients() {
	clients, err := kubeconfig.CreateClientSetsFromKubeconfig(d.KubeconfigFile, d.ApiserverEndpoint)
	if err != nil {
		nlog.Fatal(err)
	}
	d.Clients = clients
}

func (d *Dependencies) Close() {
	if d.SecretBackendHolder != nil {
		d.SecretBackendHolder.CloseAll()
	}
}

type Module interface {
	Run(ctx context.Context) error
	WaitReady(ctx context.Context) error
	Name() string
}

func (d *Dependencies) LoadCaDomainKeyAndCert() error {
	var err error
	config := d.KusciaConfig

	if d.CAKey, err = tlsutils.ParseEncodedKey(config.CAKeyData, config.CAKeyFile); err != nil {
		nlog.Errorf("load key failed: key: %t, file: %s", len(config.CAKeyData) == 0, config.CAKeyFile)
		return err
	}

	if config.CACertFile == "" {
		nlog.Errorf("load cert failed: ca cert must be file, ca file should not be empty")
		return err
	}

	if d.CACert, err = tlsutils.ParseCertWithGenerated(d.CAKey, config.DomainID, nil, config.CACertFile); err != nil {
		nlog.Errorf("load cert failed: file: %s", d.CACertFile)
		return err
	}

	if d.DomainKey, err = tlsutils.ParseEncodedKey(config.DomainKeyData, config.DomainKeyFile); err != nil {
		nlog.Errorf("load key failed: key: %t, file: %s", len(config.CAKeyData) == 0, config.DomainKeyFile)
		return err
	}

	if config.DomainCertFile == "" {
		nlog.Errorf("load cert failed: ca cert must be file, ca file should not be empty")
		return err
	}

	if d.DomainCert, err = tlsutils.ParseCertWithGenerated(d.DomainKey, d.DomainID, nil, config.DomainCertFile); err != nil {
		nlog.Errorf("load cert failed: file: %s", d.DomainCertFile)
		return err
	}

	return nil
}

func EnsureDir(conf *Dependencies) error {
	if err := os.MkdirAll(filepath.Join(conf.RootDir, common.CertPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(conf.RootDir, common.LogPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(conf.RootDir, common.StdoutPrefix), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(conf.RootDir, common.TmpPrefix), 0755); err != nil {
		return err
	}
	return nil
}

func CreateDefaultDomain(ctx context.Context, conf *Dependencies) error {
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

func CreateCrossNamespace(ctx context.Context, conf *Dependencies) error {
	if _, err := conf.Clients.KubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.KusciaCrossDomain,
		},
	}, metav1.CreateOptions{}); err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
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

func initLoggerConfig(kusciaConf confloader.KusciaConfig, logPath string) *nlog.LogConfig {
	return &nlog.LogConfig{
		LogPath:       logPath,
		LogLevel:      kusciaConf.LogLevel,
		MaxFileSizeMB: kusciaConf.Logrotate.MaxFileSizeMB,
		MaxFiles:      kusciaConf.Logrotate.MaxFiles,
		Compress:      true,
	}
}

func InitDependencies(ctx context.Context, kusciaConf confloader.KusciaConfig) *Dependencies {
	dependencies := &Dependencies{
		KusciaConfig: kusciaConf,
	}
	dependencies.stopCh = make(chan struct{})
	// init log
	logConfig := initLoggerConfig(kusciaConf, kusciaLogPath)
	if err := InitLogs(logConfig); err != nil {
		nlog.Fatal(err)
	}

	nlog.Debugf("Read kuscia config: %+v", kusciaConf)
	dependencies.LogConfig = logConfig
	dependencies.Logrorate = kusciaConf.Logrotate
	dependencies.Image = &kusciaConf.Image
	// run config loader
	dependencies.SecretBackendHolder = secretbackend.NewHolder()
	nlog.Info("Start to init all secret backends ... ")
	for _, sbc := range dependencies.SecretBackends {
		if err := dependencies.SecretBackendHolder.Init(sbc.Name, sbc.Driver, sbc.Params); err != nil {
			nlog.Fatalf("Init secret backend name=%s params=%+v failed: %s", sbc.Name, sbc.Params, err)
		}
	}
	if len(dependencies.SecretBackends) == 0 {
		nlog.Warnf("Init all secret backend but no provider found, creating default mem type")
		if err := dependencies.SecretBackendHolder.Init(common.DefaultSecretBackendName, common.DefaultSecretBackendType, map[string]any{}); err != nil {
			nlog.Fatalf("Init default secret backend failed: %s", err)
		}
	}
	nlog.Info("Finish Initializing all secret backends")

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
		dependencies.EnableContainerd = false
		if dependencies.Agent.Provider.Runtime == config.ContainerRuntime {
			dependencies.EnableContainerd = true
		}
	}
	if dependencies.RunMode == common.RunModeLite {
		dependencies.ApiserverEndpoint = defaultEndpointForLite
	}

	// init certs
	if err = dependencies.LoadCaDomainKeyAndCert(); err != nil {
		nlog.Fatal(err)
	}

	// init exporter ports
	dependencies.KtExportPort = "9093"
	dependencies.SsExportPort = "9092"
	dependencies.NodeExportPort = "9100"
	dependencies.MetricExportPort = "9091"
	return dependencies
}

type ModuleCMD struct {
	cmd   *exec.Cmd
	score *int
}

func (c *ModuleCMD) Start() error {
	return c.cmd.Start()
}

func (c *ModuleCMD) Wait() error {
	return c.cmd.Wait()
}

func (c *ModuleCMD) Stop() error {
	if c.cmd != nil && c.cmd.Process != nil {
		// Windows doesn't support Interrupt
		if runtime.GOOS == "windows" {
			return c.cmd.Process.Signal(os.Kill)
		}
		return c.cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

func (c *ModuleCMD) Pid() int {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return 0
}

func (c *ModuleCMD) SetOOMScore() error {
	if c.score == nil {
		return nil
	}
	return process.SetOOMScore(c.Pid(), *c.score)
}

func SetKusciaOOMScore() {
	if err := process.SetOOMScore(1, initProcessOOMScore); err != nil {
		nlog.Warnf("Set init process oom_score_adj failed, %v, skip setting it", err)
	}

	if err := process.SetOOMScore(os.Getpid(), kusciaOOMScore); err != nil {
		nlog.Warnf("Set kuscia controllers process oom_score_adj failed, %v, skip setting it", err)
	}
}
