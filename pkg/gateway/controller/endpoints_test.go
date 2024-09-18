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
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"

	bandwidth_limitv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/bandwidth_limit/v3"
	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

type endpointController struct {
	*EndpointsController
	client kubernetes.Interface
}

func newEndpointControllerWithStop(stopCh <-chan struct{}) (*endpointController, error) {
	client := fake.NewSimpleClientset()

	informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
	serviceInformer := informerFactory.Core().V1().Services()
	endpointInformer := informerFactory.Core().V1().Endpoints()

	c, err := NewEndpointsController(false, client, serviceInformer, endpointInformer, "", nil)
	if err != nil {
		return nil, err
	}
	c.endpointsListerSynced = alwaysReady
	c.serviceListerSynced = alwaysReady

	informerFactory.Start(stopCh)
	// fix bug: informerFactory.Start in another coroutine, wait it started
	informerFactory.WaitForCacheSync(stopCh)

	ec := &endpointController{
		c,
		client,
	}
	return ec, err
}

func newEndpointController() (*endpointController, error) {
	return newEndpointControllerWithStop(wait.NeverStop)
}

type testCase struct {
	service              *v1.Service
	endpoints            *v1.Endpoints
	expectedAccessDomain string
	expectedExists       bool
	add                  bool
}

func (t *testCase) Name() string {
	if t.endpoints != nil {
		return t.endpoints.Name
	}
	if t.service != nil {
		return t.service.Name
	}
	panic("UNREACHABLE")
}

func (t *testCase) Namespace() string {
	if t.endpoints != nil {
		return t.endpoints.Namespace
	}
	if t.service != nil {
		return t.service.Namespace
	}
	panic("UNREACHABLE")
}

func (c *endpointController) runCase(t *testing.T, testCases []testCase) {
	ctx := context.Background()
	for _, tc := range testCases {
		if tc.add {
			if tc.service != nil {
				c.client.CoreV1().Services(tc.service.Namespace).Create(ctx, tc.service, metav1.CreateOptions{})
			}
			if tc.endpoints != nil {
				svr := constructServiceByEndpoints(tc.endpoints)
				c.client.CoreV1().Services(tc.endpoints.Namespace).Create(ctx, svr, metav1.CreateOptions{})
				c.client.CoreV1().Endpoints(tc.endpoints.Namespace).Create(ctx, tc.endpoints, metav1.CreateOptions{})
			}
		} else {
			if tc.service != nil {
				c.client.CoreV1().Services(tc.service.Namespace).Delete(ctx, tc.service.Name, metav1.DeleteOptions{})
			}
			if tc.endpoints != nil {
				c.client.CoreV1().Endpoints(tc.endpoints.Namespace).Delete(ctx, tc.endpoints.Name,
					metav1.DeleteOptions{})
				c.client.CoreV1().Services(tc.endpoints.Namespace).Delete(ctx, tc.endpoints.Name,
					metav1.DeleteOptions{})
			}
		}
		time.Sleep(400 * time.Millisecond)

		vhName1 := fmt.Sprintf("service-%s-internal", tc.Name())
		vhName2 := fmt.Sprintf("service-%s-external", tc.Name())
		internalVh, err1 := xds.QueryVirtualHost(vhName1, xds.InternalRoute)
		externalVh, err2 := xds.QueryVirtualHost(vhName2, xds.ExternalRoute)

		var vhs = []*routev3.VirtualHost{
			internalVh,
			externalVh,
		}
		if tc.add && tc.expectedExists {
			for i := 0; i <= 10; i++ {
				if err1 != nil || err2 != nil {
					time.Sleep(600 * time.Millisecond)
					_, err1 = xds.QueryVirtualHost(vhName1, xds.InternalRoute)
					_, err2 = xds.QueryVirtualHost(vhName2, xds.ExternalRoute)
				} else {
					break
				}
			}
			if err1 != nil || err2 != nil {
				t.Fatalf("Add service failed,err1: %v err2: %v", err1, err2)
			}
			if tc.expectedAccessDomain != "" {
				for _, vh := range vhs {
					n := len(vh.Routes[0].Match.Headers)
					assert.NotEqual(t, n, 0)
					assert.NotNil(t, vh.Routes[0].Match.Headers[n-1].GetStringMatch().GetSafeRegex().GetRegex())
					regex := vh.Routes[0].Match.Headers[n-1].GetStringMatch().
						GetSafeRegex().GetRegex()
					assert.Equal(t, tc.expectedAccessDomain, regex)
				}
			}
		}

		if (!tc.add || !tc.expectedExists) && (err1 == nil || err2 == nil) {
			for i := 0; i <= 5; i++ {
				if err1 == nil || err2 == nil {
					time.Sleep(600 * time.Millisecond)
					_, err1 = xds.QueryVirtualHost(vhName1, xds.InternalRoute)
					_, err2 = xds.QueryVirtualHost(vhName2, xds.ExternalRoute)
				} else {
					break
				}
			}
			t.Fatal("delete service failed")
		}
	}
}

func constructServiceByEndpoints(endpoints *v1.Endpoints) *v1.Service {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        endpoints.Name,
			Namespace:   endpoints.Namespace,
			Annotations: endpoints.Annotations,
		},
		Spec: v1.ServiceSpec{
			ClusterIP: "None",
			Selector: map[string]string{
				"app": "foo",
			},
			Ports: []v1.ServicePort{
				{Name: "http", Port: 80},
			},
		},
		Status: v1.ServiceStatus{},
	}
	return service
}

func TestEndpoints(t *testing.T) {
	stopCh := make(chan struct{})
	c, err := newEndpointControllerWithStop(stopCh)
	assert.Nil(t, err)

	endpoints := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpointsA",
			Namespace: "default",
			Annotations: map[string]string{
				common.AccessDomainAnnotationKey: "alice,bob",
			},
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{IP: FakeServerIP},
				},
				Ports: []v1.EndpointPort{
					{Port: FakeServerPort, Name: "http"},
				},
			},
		},
	}

	ctx := context.Background()

	assert.True(t, cache.WaitForCacheSync(stopCh, c.serviceListerSynced, c.endpointsListerSynced))

	svr := constructServiceByEndpoints(endpoints)
	s1, err := c.client.CoreV1().Services(endpoints.Namespace).Create(ctx, svr, metav1.CreateOptions{})
	assert.NotNil(t, s1)
	assert.NoError(t, err)
	e1, err := c.client.CoreV1().Endpoints(endpoints.Namespace).Create(ctx, endpoints, metav1.CreateOptions{})
	assert.NotNil(t, e1)
	assert.NoError(t, err)

	assert.NoError(t, wait.PollImmediate(20*time.Millisecond, 60*time.Second, func() (bool, error) {
		return c.queue.Len() >= 1, nil
	}))

	for c.queue.Len() > 0 {
		assert.True(t, queue.HandleQueueItem(ctx, endpointsQueueName, c.queue, c.syncHandler, maxRetries))
	}

	vhName1 := fmt.Sprintf("service-%s-internal", endpoints.Name)
	vhName2 := fmt.Sprintf("service-%s-external", endpoints.Name)
	internalVh, err1 := xds.QueryVirtualHost(vhName1, xds.InternalRoute)
	externalVh, err2 := xds.QueryVirtualHost(vhName2, xds.ExternalRoute)

	assert.NoError(t, err1)
	assert.NoError(t, err2)

	var vhs = []*routev3.VirtualHost{
		internalVh,
		externalVh,
	}

	for _, vh := range vhs {
		n := len(vh.Routes[0].Match.Headers)
		assert.NotEqual(t, n, 0)
		assert.NotNil(t, vh.Routes[0].Match.Headers[n-1].GetStringMatch().GetSafeRegex().GetRegex())
		regex := vh.Routes[0].Match.Headers[n-1].GetStringMatch().
			GetSafeRegex().GetRegex()
		assert.Equal(t, "(alice|bob)", regex)
	}

	assert.NoError(t, c.client.CoreV1().Endpoints(endpoints.Namespace).Delete(ctx, endpoints.Name,
		metav1.DeleteOptions{}))
	assert.NoError(t, c.client.CoreV1().Services(endpoints.Namespace).Delete(ctx, endpoints.Name,
		metav1.DeleteOptions{}))

	assert.NoError(t, wait.PollImmediate(20*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		return c.queue.Len() >= 1, nil
	}))

	assert.True(t, queue.HandleQueueItem(ctx, endpointsQueueName, c.queue, c.syncHandler, maxRetries))

	_, err1 = xds.QueryVirtualHost(vhName1, xds.InternalRoute)
	_, err2 = xds.QueryVirtualHost(vhName2, xds.ExternalRoute)

	assert.Error(t, err1)
	assert.Error(t, err2)

	c.queue.ShutDown()

	close(stopCh)
}

func TestService(t *testing.T) {
	c, err := newEndpointController()
	if err != nil {
		t.Fatal(err)
	}

	go c.Run(1, make(<-chan struct{}))
	time.Sleep(200 * time.Millisecond)

	testCases := []testCase{
		{
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "serviceA",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "http", Port: 80},
					},
					ClusterIP: "None",
					Selector: map[string]string{
						"app": "foo",
					},
				},
			},
			add:                  true,
			expectedAccessDomain: "",
			expectedExists:       false,
		},
		{
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "serviceB",
					Namespace: "default",
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "http", Port: FakeServerPort},
					},
					Type:         v1.ServiceTypeExternalName,
					ExternalName: FakeServerIP,
				},
			},
			add:                  true,
			expectedAccessDomain: "",
			expectedExists:       true,
		},
	}

	c.runCase(t, testCases)

	// delete
	testCases[0].add = false
	testCases[1].add = false
	c.runCase(t, testCases)
}

func TestInvalidProtocol(t *testing.T) {
	c, err := newEndpointController()
	if err != nil {
		t.Fatal(err)
	}

	go c.Run(1, make(<-chan struct{}))
	time.Sleep(200 * time.Millisecond)

	testCases := []testCase{
		{
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "serviceA",
					Namespace: "default",
					Annotations: map[string]string{
						common.ProtocolAnnotationKey: "foo",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{Name: "http", Port: 80},
					},
					ClusterIP: "None",
					Type:      v1.ServiceTypeExternalName,
					Selector: map[string]string{
						"app": "foo",
					},
				},
			},
			add:            true,
			expectedExists: false,
		},
		{
			endpoints: &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpointsB",
					Namespace: "default",
					Annotations: map[string]string{
						common.ProtocolAnnotationKey: "foo",
					},
				},
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{
							{IP: "127.0.0.1"},
						},
						Ports: []v1.EndpointPort{
							{Port: FakeServerPort, Name: "http"},
						},
					},
				},
			},
			add:            true,
			expectedExists: false,
		},
	}

	c.runCase(t, testCases)

	// delete
	testCases[0].add = false
	testCases[1].add = false
	c.runCase(t, testCases)
}

func TestBandwidthLimit(t *testing.T) {
	ns := "alice"
	dc := newDomainRouteTestInfo(ns, 1054)
	stopCh := make(chan struct{})
	go dc.Run(context.Background(), 1, stopCh)

	ec, err := newEndpointController()
	if err != nil {
		t.Fatal(err)
	}
	go ec.Run(1, make(<-chan struct{}))

	time.Sleep(200 * time.Millisecond)

	dr := &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alice-bob",
			Namespace: ns,
		},
		Spec: kusciaapisv1alpha1.DomainRouteSpec{
			Source:            ns,
			Destination:       "bob",
			InterConnProtocol: kusciaapisv1alpha1.InterConnKuscia,
			Endpoint: kusciaapisv1alpha1.DomainEndpoint{
				Host: EnvoyServerIP,
				Ports: []kusciaapisv1alpha1.DomainPort{
					{
						Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						Port:     ExternalServerPort,
					},
				},
			},
			AuthenticationType: kusciaapisv1alpha1.DomainAuthenticationToken,
			TokenConfig: &kusciaapisv1alpha1.TokenConfig{
				TokenGenMethod:       kusciaapisv1alpha1.TokenGenMethodRSA,
				SourcePublicKey:      base64.StdEncoding.EncodeToString(pubPemData),
				DestinationPublicKey: base64.StdEncoding.EncodeToString(pubPemData),
			},
		},
		Status: kusciaapisv1alpha1.DomainRouteStatus{
			TokenStatus: kusciaapisv1alpha1.DomainRouteTokenStatus{
				Tokens: []kusciaapisv1alpha1.DomainRouteToken{
					{
						Token: fakeRevisionToken,
					},
				},
			},
		},
	}

	dc.client.KusciaV1alpha1().DomainRoutes(dr.Namespace).Create(context.Background(), dr, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "serviceA",
			Namespace: "alice",
			Annotations: map[string]string{
				fmt.Sprintf("%s%s", common.TaskBandwidthLimitAnnotationPrefix, "bob"): "100",
				common.TaskIDAnnotationKey: "hello-world",
			},
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeExternalName,
			Ports: []v1.ServicePort{
				{Name: "http", Port: 80},
			},
			ClusterIP: "None",
			Selector: map[string]string{
				"app": "foo",
			},
		},
	}

	ec.kubeClient.CoreV1().Services(svc.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	time.Sleep(200 * time.Millisecond)

	vhName := fmt.Sprintf("%s-to-%s", dr.Spec.Source, dr.Spec.Destination)
	vh, err := xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.NoError(t, err)
	routes := vh.GetRoutes()

	assert.Equal(t, "hello-world-bandwidth-limit", routes[0].Name)
	limit := bandwidth_limitv3.BandwidthLimit{}
	err = routes[0].GetTypedPerFilterConfig()["envoy.filters.http.bandwidth_limit"].UnmarshalTo(&limit)
	assert.NoError(t, err)
	assert.Equal(t, uint64(100), limit.LimitKbps.Value)

	f, err := xds.GetHTTPFilterConfig(xds.BandwidthLimitName, xds.InternalListener)
	assert.NoError(t, err)
	assert.NotNil(t, f)

	ec.kubeClient.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
	time.Sleep(200 * time.Millisecond)

	vh, err = xds.QueryVirtualHost(vhName, xds.InternalRoute)
	assert.NoError(t, err)
	routes = vh.GetRoutes()

	assert.Equal(t, routes[0].Name, "default")

	f, err = xds.GetHTTPFilterConfig(xds.BandwidthLimitName, xds.InternalListener)
	assert.Error(t, err)
	assert.Nil(t, f)
}
