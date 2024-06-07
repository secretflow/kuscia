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

package kuscia

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientsetfake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/interconn/kuscia/hostresources"
	"github.com/secretflow/kuscia/pkg/utils/kubeconfig"
)

func TestHandleAddedorDeletedInteropConfig(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}

	ic := &kusciaapisv1alpha1.InteropConfig{ObjectMeta: metav1.ObjectMeta{Name: "ic"}}
	cc := c.(*Controller)
	cc.handleAddedorDeletedInteropConfig(ic)
	if cc.interopConfigQueue.Len() != 1 {
		t.Error("interop config queue length should be 1")
	}
}

func TestHandleUpdatedInteropConfig(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	ic1 := &kusciaapisv1alpha1.InteropConfig{ObjectMeta: metav1.ObjectMeta{Name: "ic1", ResourceVersion: "1"}}
	ic2 := &kusciaapisv1alpha1.InteropConfig{ObjectMeta: metav1.ObjectMeta{Name: "ic2", ResourceVersion: "2"}}

	tests := []struct {
		name   string
		oldObj interface{}
		newObj interface{}
		want   int
	}{
		{
			name:   "obj type is invalid",
			oldObj: "ic1",
			newObj: "ic2",
			want:   0,
		},
		{
			name:   "interop config is same",
			oldObj: ic1,
			newObj: ic1,
			want:   0,
		},
		{
			name:   "interop config is updated",
			oldObj: ic1,
			newObj: ic2,
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.handleUpdatedInteropConfig(tt.oldObj, tt.newObj)
			if cc.interopConfigQueue.Len() != tt.want {
				t.Errorf("got %v, want %v", cc.interopConfigQueue.Len(), tt.want)
			}
		})
	}
}

func TestRegisterInteropConfig(t *testing.T) {
	t.Parallel()
	hostKubeFakeClient := clientsetfake.NewSimpleClientset()
	hostKusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	hostresources.GetHostClient = func(token, masterURL string) (*kubeconfig.KubeClients, error) {
		return &kubeconfig.KubeClients{
			KubeClient:   hostKubeFakeClient,
			KusciaClient: hostKusciaFakeClient,
		}, nil
	}

	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	ic1 := &kusciaapisv1alpha1.InteropConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "ic1",
			ResourceVersion: "1",
		},
		Spec: kusciaapisv1alpha1.InteropConfigSpec{
			Host:    "ns2",
			Members: []string{"ns1"},
		},
	}

	updatedIc1 := &kusciaapisv1alpha1.InteropConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "ic1",
			ResourceVersion: "2",
		},
		Spec: kusciaapisv1alpha1.InteropConfigSpec{
			Host:    "ns2",
			Members: []string{"ns3", "ns4"},
		},
	}

	tests := []struct {
		name               string
		ic                 *kusciaapisv1alpha1.InteropConfig
		interopConfigInfos map[string]*interopConfigInfo
		wantMembers        int
	}{
		{
			name:        "register new interop config",
			ic:          ic1,
			wantMembers: 1,
		},
		{
			name: "interop config is updated",
			ic:   updatedIc1,
			interopConfigInfos: map[string]*interopConfigInfo{
				"ic1": {
					host:    "ns2",
					members: []string{"ns1"},
				},
			},
			wantMembers: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.interopConfigInfos != nil {
				cc.interopConfigInfos = tt.interopConfigInfos
			}
			cc.registerInteropConfig(tt.ic)
			icInfo := cc.getInteropConfigInfo(tt.ic.Name)
			if len(icInfo.members) != tt.wantMembers {
				t.Errorf("got: %v, want: %v", len(icInfo.members), tt.wantMembers)
			}
		})
	}
}

func TestDeregisterInteropConfig(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	tests := []struct {
		name               string
		key                string
		interopConfigInfos map[string]*interopConfigInfo
		wantMembers        int
	}{
		{
			name:        "interop config info is empty",
			wantMembers: 0,
		},
		{
			name: "interop config info only include one member",
			key:  "ic",
			interopConfigInfos: map[string]*interopConfigInfo{
				"ic": {
					host:    "ns1",
					members: []string{"ns2"},
				},
			},
			wantMembers: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.interopConfigInfos != nil {
				cc.interopConfigInfos = tt.interopConfigInfos
			}

			cc.deregisterInteropConfig(tt.key)
			icInfo := cc.getInteropConfigInfo(tt.key)
			got := 0
			if icInfo != nil {
				got = len(icInfo.members)
			}

			if got != tt.wantMembers {
				t.Errorf("got: %v, want: %v", got, tt.wantMembers)
			}
		})
	}
}

func TestGetInteropConfigInfo(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	tests := []struct {
		name               string
		key                string
		interopConfigInfos map[string]*interopConfigInfo
		wantNil            bool
	}{
		{
			name:    "key is not exist",
			key:     "ic1",
			wantNil: true,
		},
		{
			name: "key is exist",
			key:  "ic",
			interopConfigInfos: map[string]*interopConfigInfo{
				"ic": {
					host:    "ns1",
					members: []string{"ns2"},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.interopConfigInfos != nil {
				cc.interopConfigInfos = tt.interopConfigInfos
			}

			icInfo := cc.getInteropConfigInfo(tt.key)
			got := icInfo == nil
			if got != tt.wantNil {
				t.Errorf("got: %v, want: %v", got, tt.wantNil)
			}
		})
	}
}

func TestSetInteropConfigInfo(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	tests := []struct {
		name               string
		key                string
		interopConfigInfos *interopConfigInfo
	}{
		{
			name: "key is ic",
			key:  "ic",
			interopConfigInfos: &interopConfigInfo{
				host:    "ns1",
				members: []string{"ns2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc.setInteropConfigInfo(tt.key, tt.interopConfigInfos)
			icInfo := cc.getInteropConfigInfo(tt.key)
			if !reflect.DeepEqual(icInfo, tt.interopConfigInfos) {
				t.Error("value should be equal")
			}
		})
	}
}

func TestDeleteInteropConfigInfo(t *testing.T) {
	t.Parallel()
	kusciaFakeClient := kusciaclientsetfake.NewSimpleClientset()
	c := NewController(context.Background(), nil, kusciaFakeClient, nil)
	if c == nil {
		t.Error("new controller failed")
	}
	cc := c.(*Controller)

	tests := []struct {
		name               string
		key                string
		interopConfigInfos map[string]*interopConfigInfo
		wantNil            bool
	}{
		{
			name:    "key is not exist",
			key:     "ic1",
			wantNil: true,
		},
		{
			name: "key is exist",
			key:  "ic",
			interopConfigInfos: map[string]*interopConfigInfo{
				"ic": {
					host:    "ns1",
					members: []string{"ns2"},
				},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.interopConfigInfos != nil {
				cc.interopConfigInfos = tt.interopConfigInfos
			}

			cc.deleteInteropConfigInfo(tt.key)
			icInfo := cc.getInteropConfigInfo(tt.key)
			got := icInfo == nil
			if got != tt.wantNil {
				t.Errorf("got: %v, want: %v", got, tt.wantNil)
			}
		})
	}
}
