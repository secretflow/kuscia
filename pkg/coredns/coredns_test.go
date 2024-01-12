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

package coredns

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/signals"
)

func createTestGatewaySvc(kubeClient kubernetes.Interface, ns, name string) {
	testsvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{common.LabelLoadBalancer: string(common.DomainRouteLoadBalancer)},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "listen-port",
					Protocol: "TCP",
					Port:     35430,
				},
			},
			ClusterIP: "None",
			Type:      v1.ServiceTypeClusterIP,
		},
	}
	kubeClient.CoreV1().Services(ns).Create(context.Background(), testsvc, metav1.CreateOptions{})
}

func createTestSingleSvc(kubeClient kubernetes.Interface, ns, name string) {
	testsvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "listen-port",
					Protocol: "TCP",
					Port:     35430,
				},
			},
			ClusterIP: "None",
			Type:      v1.ServiceTypeClusterIP,
		},
	}
	kubeClient.CoreV1().Services(ns).Create(context.Background(), testsvc, metav1.CreateOptions{})
}

func createTestSvcWithEp(kubeClient kubernetes.Interface, ns, name string) {
	testsvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "listen-port",
					Protocol: "TCP",
					Port:     28971,
				},
			},
			ClusterIP: "None",
			Type:      v1.ServiceTypeClusterIP,
		},
	}

	kubeClient.CoreV1().Services(ns).Create(context.Background(), testsvc, metav1.CreateOptions{})
	testnodename := "testnodename"
	testep := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       "30.30.30.30",
						NodeName: &testnodename,
					},
				},
				Ports: []v1.EndpointPort{
					{
						Name:     "listen-port",
						Protocol: "TCP",
						Port:     28971,
					},
				},
			},
		},
	}
	kubeClient.CoreV1().Endpoints(ns).Create(context.Background(), testep, metav1.CreateOptions{})
}

func updateTestSvcWithEp(kubeClient kubernetes.Interface, ns, name string) {
	testsvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "listen-port",
					Protocol: "TCP",
					Port:     20000,
				},
			},
			ClusterIP: "None",
			Type:      v1.ServiceTypeClusterIP,
		},
	}

	kubeClient.CoreV1().Services(ns).Update(context.Background(), testsvc, metav1.UpdateOptions{})
	testnodename := "testnodename"
	testep := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       "30.30.30.31",
						NodeName: &testnodename,
					},
				},
				Ports: []v1.EndpointPort{
					{
						Name:     "listen-port",
						Protocol: "TCP",
						Port:     20000,
					},
				},
			},
		},
	}
	kubeClient.CoreV1().Endpoints(ns).Update(context.Background(), testep, metav1.UpdateOptions{})
}

func createTestExternalSvc(kubeClient kubernetes.Interface, ns, name string) {
	testsvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "listen-port",
					Protocol: "TCP",
					Port:     13542,
				},
			},
			ClusterIP:    "None",
			Type:         v1.ServiceTypeExternalName,
			ExternalName: "asfsafs.com",
		},
	}

	kubeClient.CoreV1().Services(ns).Create(context.Background(), testsvc, metav1.CreateOptions{})
}

func Test_startEndpointsController(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	ns := "test"
	n := &KusciaCoreDNS{Cache: cache.New(defaultExpiration, 0), Upstream: upstream.New(), Namespace: ns}
	ch := make(chan struct{}, 1)

	beginGoroutine := runtime.NumGoroutine()
	go func() {
		createTestGatewaySvc(kubeClient, ns, "testgatewaysvc")
		createTestSingleSvc(kubeClient, ns, "testsinglesvc")
		createTestSvcWithEp(kubeClient, ns, "testsvc")
		createTestExternalSvc(kubeClient, ns, "externalsvc")
		time.Sleep(time.Millisecond * 100)
		updateTestSvcWithEp(kubeClient, ns, "testsvc")
		time.Sleep(time.Millisecond * 100)
		kubeClient.CoreV1().Services(ns).Delete(context.Background(), "testsvc", metav1.DeleteOptions{})
		kubeClient.CoreV1().Endpoints(ns).Delete(context.Background(), "testsvc", metav1.DeleteOptions{})
		time.Sleep(time.Millisecond * 100)
		close(ch)
	}()
	startEndpointsController(signals.NewKusciaContextWithStopCh(ch), kubeClient, n, ns)
	time.Sleep(time.Second)
	assert.Equal(t, beginGoroutine, runtime.NumGoroutine())
}

func getTestPod(ns, name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1.PodSpec{
			Hostname:  "testdomain",
			Subdomain: "testsubdomain",
		},
		Status: v1.PodStatus{
			PodIP: "30.30.30.30",
		},
	}
}
func Test_startPodController(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	ns := "test"
	n := &KusciaCoreDNS{Cache: cache.New(defaultExpiration, 0), Upstream: upstream.New(), Namespace: ns}
	ch := make(chan struct{}, 1)
	beginGoroutine := runtime.NumGoroutine()
	go func() {
		pod := getTestPod("test", "testpod")
		kubeClient.CoreV1().Pods("test").Create(context.Background(), pod, metav1.CreateOptions{})
		time.Sleep(time.Millisecond * 100)
		pod.Spec.Hostname = "testdomain2"
		pod.UID = "asfsdfsaf"
		kubeClient.CoreV1().Pods("test").Update(context.Background(), pod, metav1.UpdateOptions{})
		time.Sleep(time.Millisecond * 100)
		kubeClient.CoreV1().Pods("test").Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		time.Sleep(time.Millisecond * 100)
		close(ch)
	}()
	startPodController(signals.NewKusciaContextWithStopCh(ch), kubeClient, n, "test")
	time.Sleep(time.Second * 1)
	assert.Equal(t, runtime.NumGoroutine(), beginGoroutine)
}

func Test_kusciaParse_with_no_next(t *testing.T) {
	c := caddy.NewTestController("kuscia", "")
	_, err := KusciaParse(c, "test", "127.0.0.1")
	assert.NoError(t, err)
}

func Test_Name(t *testing.T) {
	n := &KusciaCoreDNS{}
	assert.Equal(t, n.Name(), "kuscia")
}

func Test_ServeDNS(t *testing.T) {
	ns := "test"
	m := &dns.Msg{Question: []dns.Question{{Name: "test"}}}
	n := &KusciaCoreDNS{Cache: cache.New(defaultExpiration, 0), Upstream: upstream.New(), Namespace: ns, Zones: []string{"test"}}
	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	_, err := n.ServeDNS(context.TODO(), rec, m)
	assert.NoError(t, err)
}

func Test_xfr(t *testing.T) {
	ns := "test"
	n := &KusciaCoreDNS{Cache: cache.New(defaultExpiration, 0), Upstream: upstream.New(), Namespace: ns, Zones: []string{"test"}}
	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	m := &dns.Msg{Question: []dns.Question{{Name: "test"}}}
	r := request.Request{W: rec, Req: m}
	assert.True(t, uint32(time.Now().Unix())-n.Serial(r) < 10)
	assert.Equal(t, n.MinTTL(r), uint32(30))
	code, err := n.Transfer(context.Background(), r)
	assert.NoError(t, err)
	assert.Equal(t, code, dns.RcodeServerFailure)
}

func Test_Records(t *testing.T) {
	ns := "test1"
	n := &KusciaCoreDNS{Cache: cache.New(defaultExpiration, 0), EnvoyIP: "127.0.0.1", Upstream: upstream.New(), Namespace: ns, Zones: []string{"alice.test.svc"}}
	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	m := &dns.Msg{Question: []dns.Question{{Name: "alice.test.svc."}}}
	r := request.Request{W: rec, Req: m}
	_, err := n.Reverse(context.Background(), r, true, plugin.Options{})
	assert.NoError(t, err)
	n.Namespace = "test"
	_, err = n.Reverse(context.Background(), r, true, plugin.Options{})
	assert.Equal(t, err.Error(), "key not found")
	n.Cache.Set("alice.test", []string{"127.0.0.1"}, defaultTTL*time.Second)
	_, err = n.Reverse(context.Background(), r, true, plugin.Options{})
	assert.NoError(t, err)
}
