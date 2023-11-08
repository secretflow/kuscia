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
	"k8s.io/kubernetes/pkg/controller"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/gateway/xds"
)

type endpointController struct {
	*EndpointsController
	client kubernetes.Interface
}

func newEndpointController() (*endpointController, error) {
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

	informerFactory.Start(wait.NeverStop)
	ec := &endpointController{
		c,
		client,
	}
	return ec, err
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
	c, err := newEndpointController()
	if err != nil {
		t.Fatal(err)
	}

	go c.Run(1, make(<-chan struct{}))
	time.Sleep(300 * time.Millisecond)

	testCases := []testCase{
		{
			endpoints: &v1.Endpoints{
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
			},
			add:                  true,
			expectedAccessDomain: "(alice|bob)",
			expectedExists:       true,
		},
	}
	c.runCase(t, testCases)

	// delete
	testCases[0].add = false
	c.runCase(t, testCases)
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
