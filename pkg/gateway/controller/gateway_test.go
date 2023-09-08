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

package controller

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	core "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/gateway/utils"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/network"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

var (
	alwaysReady = func() bool { return true }
)

func TestMain(m *testing.M) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		nlog.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0755); err != nil {
		nlog.Fatal(err)
	}

	if err := paths.CopyDirectory("../../../etc/conf/domainroute", filepath.Join(dir, "conf")); err != nil {
		nlog.Fatal(err)
	}
	if err := paths.CreateIfNotExists(filepath.Join(dir, "nlogs"), 0755); err != nil {
		nlog.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		nlog.Fatal(err)
	}

	os.Setenv("NAMESPACE", "default")

	// start xds server
	instance := "test-instance"
	envoyNodeCluster := "kuscia-gateway-default"
	envoyNodeID := fmt.Sprintf("%s-%s", envoyNodeCluster, instance)

	xds.NewXdsServer(10000, envoyNodeID)
	config := &xds.InitConfig{
		Basedir:      "./conf/",
		XDSPort:      1054,
		ExternalPort: 1080,
		ExternalCert: nil,
		InternalCert: nil,
	}
	xds.InitSnapshot("default", "test-instance", config)

	os.Exit(m.Run())
}

type fixture struct {
	t testing.TB

	namespace string
	hostname  string
	address   string

	prikey *rsa.PrivateKey
	pubkey string

	client *fake.Clientset
	// Objects to put in the store.
	drLister []*kusciaapisv1alpha1.Gateway

	// Actions expected to happen on the client. Objects from here are also
	// preloaded into NewSimpleFake.
	actions []core.Action
	objects []runtime.Object
}

func newFixture(t testing.TB) *fixture {
	f := &fixture{}
	f.t = t

	f.namespace = "default"
	hostname, err := os.Hostname()
	if err != nil {
		f.t.Fatal(err)
	}
	f.hostname = strings.ToLower(hostname)
	address, err := network.GetHostIP()
	if err != nil {
		f.t.Fatal(err)
	}
	f.address = address

	// parse rsa private key
	prikey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		f.t.Fatal(err)
	}
	f.prikey = prikey

	pubPemData := utils.EncodePKCS1PublicKey(prikey)
	f.pubkey = string(pubPemData)

	f.objects = []runtime.Object{}
	return f
}

func (f *fixture) newController() (*GatewayController, error) {
	f.client = fake.NewSimpleClientset(f.objects...)

	kusciaInformerFactory := informers.NewSharedInformerFactory(f.client, controller.NoResyncPeriodFunc())
	kusciaInformerFactory.Start(wait.NeverStop)

	gatewayInformer := kusciaInformerFactory.Kuscia().V1alpha1().Gateways()

	c, err := NewGatewayController(f.namespace, f.prikey, f.client, gatewayInformer)
	if err != nil {
		return nil, err
	}
	c.gatewayListerSynced = alwaysReady
	for _, gw := range f.drLister {
		kusciaInformerFactory.Kuscia().V1alpha1().Gateways().Informer().GetIndexer().Add(gw)
	}

	return c, nil
}

func (f *fixture) newGateway() *kusciaapisv1alpha1.Gateway {
	return &kusciaapisv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.hostname,
			Namespace: f.namespace,
		},
		Status: kusciaapisv1alpha1.GatewayStatus{
			Address: f.address,
			UpTime: metav1.Time{
				Time: time.Now(),
			},
			HeartbeatTime: metav1.Time{
				Time: time.Now(),
			},
			PublicKey: f.pubkey,
		},
	}
}

func (f *fixture) doSync() {
	c, err := f.newController()
	if err != nil {
		f.t.Fatal(err)
	}

	err = c.syncHandler()
	// Update only creates missing objects for a few resource types
	// https://github.com/kubernetes/client-go/issues/479
	if err != nil && !errors.IsNotFound(err) {
		f.t.Fatal(err)
	}

	actions := f.client.Actions()
	if len(f.actions) != len(f.client.Actions()) {
		f.t.Fatalf("Unexpected actions size, want: %d, got: %d", len(f.actions), len(f.client.Actions()))
	}

	for i, expectedAction := range f.actions {
		action := actions[i]
		if !(expectedAction.Matches(action.GetVerb(), action.GetResource().Resource) && action.GetSubresource() == expectedAction.GetSubresource()) {
			f.t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expectedAction, action)
			continue
		}
	}
}

func TestGatewayCreate(t *testing.T) {
	f := newFixture(t)

	gw := f.newGateway()
	f.actions = append(f.actions, core.NewCreateAction(
		schema.GroupVersionResource{
			Group:    "kuscia.secretflow",
			Version:  "v1",
			Resource: "gateways",
		},
		gw.Namespace,
		gw,
	))

	f.doSync()
}

func TestGatewayUpdate(t *testing.T) {
	f := newFixture(t)

	gw := f.newGateway()
	f.drLister = append(f.drLister, gw)
	f.objects = append(f.objects, gw)
	f.actions = append(f.actions, core.NewUpdateSubresourceAction(
		schema.GroupVersionResource{
			Group:    "kuscia.secretflow",
			Version:  "v1",
			Resource: "gateways",
		},
		"status",
		gw.Namespace,
		gw,
	))

	f.doSync()
}
