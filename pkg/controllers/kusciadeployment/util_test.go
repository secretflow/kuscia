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

package kusciadeployment

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	proto "github.com/secretflow/kuscia/proto/api/v1alpha1/appconfig"
)

func TestFillPartyClusterDefine(t *testing.T) {
	tests := []struct {
		name    string
		kitInfo *PartyKitInfo
		parties []*proto.Party
		wantErr bool
	}{
		{
			name: "can't find the party",
			kitInfo: &PartyKitInfo{
				domainID: "alice",
				role:     "server",
			},
			parties: []*proto.Party{
				{
					Name:     "bob",
					Role:     "client",
					Services: nil,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fillPartyClusterDefine(tt.kitInfo, tt.parties)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestFillClusterDefine(t *testing.T) {
	tests := []struct {
		name          string
		dkInfo        *DeploymentKitInfo
		parties       []*proto.Party
		partyIndex    int
		endpointIndex int
		want          *proto.ClusterDefine
	}{
		{
			name: "parties is empty",
			dkInfo: &DeploymentKitInfo{
				deploymentName: "deploy-1",
			},
			partyIndex:    0,
			endpointIndex: 0,
			want: &proto.ClusterDefine{
				Parties:         nil,
				SelfPartyIdx:    0,
				SelfEndpointIdx: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fillClusterDefine(tt.dkInfo, tt.parties, tt.partyIndex, tt.endpointIndex)
			assert.Equal(t, tt.want, tt.dkInfo.clusterDef)
		})
	}
}

func TestFillAllocatedPorts(t *testing.T) {
	tests := []struct {
		name   string
		dkInfo *DeploymentKitInfo
		want   *proto.AllocatedPorts
	}{
		{
			name: "ports is empty",
			dkInfo: &DeploymentKitInfo{
				deploymentName: "deploy-1",
				ports:          nil,
				allocatedPorts: nil,
			},
			want: &proto.AllocatedPorts{
				Ports: make([]*proto.Port, 0),
			},
		},
		{
			name: "ports is not empty",
			dkInfo: &DeploymentKitInfo{
				deploymentName: "deploy-1",
				ports: NamedPorts{
					"domain": kusciaapisv1alpha1.ContainerPort{
						Name:     "domain",
						Port:     8080,
						Protocol: "HTTP",
						Scope:    kusciaapisv1alpha1.ScopeDomain,
					},
				},
				allocatedPorts: nil,
			},
			want: &proto.AllocatedPorts{
				Ports: []*proto.Port{
					{
						Name:     "domain",
						Port:     8080,
						Scope:    string(kusciaapisv1alpha1.ScopeDomain),
						Protocol: "HTTP",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fillAllocatedPorts(tt.dkInfo)
			assert.Equal(t, tt.want, tt.dkInfo.allocatedPorts)
		})
	}
}

func TestGenerateClusterDefineParty(t *testing.T) {
	kusciaClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	aliceDomain := &kusciaapisv1alpha1.Domain{
		ObjectMeta: v1.ObjectMeta{
			Name: "alice",
		},
	}
	bobDomain := &kusciaapisv1alpha1.Domain{
		ObjectMeta: v1.ObjectMeta{
			Name: "bob",
		},
		Spec: kusciaapisv1alpha1.DomainSpec{Role: kusciaapisv1alpha1.Partner},
	}
	domainInformer.Informer().GetStore().Add(aliceDomain)
	domainInformer.Informer().GetStore().Add(bobDomain)
	c := &Controller{
		kusciaClient: kusciaClient,
		domainLister: domainInformer.Lister(),
	}

	tests := []struct {
		name    string
		kitInfo *PartyKitInfo
		want    *proto.Party
	}{
		{
			name: "serviced ports is empty",
			kitInfo: &PartyKitInfo{
				domainID: "alice",
				role:     "client",
			},
			want: &proto.Party{
				Name:     "alice",
				Role:     "client",
				Services: nil,
			},
		},
		{
			name: "serviced ports is not empty",
			kitInfo: &PartyKitInfo{
				domainID:      "alice",
				role:          "client",
				servicedPorts: []string{"domain", "cluster"},
				dkInfo: &DeploymentKitInfo{
					deploymentName: "deploy-1",
					portService:    generatePortServices("deploy-1", []string{"domain", "cluster"}),
					ports: NamedPorts{
						"domain": kusciaapisv1alpha1.ContainerPort{
							Name:     "domain",
							Port:     8080,
							Protocol: "HTTP",
							Scope:    kusciaapisv1alpha1.ScopeDomain,
						},
						"cluster": kusciaapisv1alpha1.ContainerPort{
							Name:     "cluster",
							Port:     8081,
							Protocol: "HTTP",
							Scope:    kusciaapisv1alpha1.ScopeCluster,
						},
					},
				},
			},
			want: &proto.Party{
				Name: "alice",
				Role: "client",
				Services: []*proto.Service{
					{
						PortName:  "domain",
						Endpoints: []string{"deploy-1-domain.alice.svc:8080"},
					},
					{
						PortName:  "cluster",
						Endpoints: []string{"deploy-1-cluster.alice.svc"},
					},
				},
			},
		},
		{
			name: "partner party serviced ports is not empty",
			kitInfo: &PartyKitInfo{
				domainID:      "bob",
				role:          "client",
				servicedPorts: []string{"domain", "cluster"},
				dkInfo: &DeploymentKitInfo{
					deploymentName: "deploy-1",
					portService:    generatePortServices("deploy-1", []string{"domain", "cluster"}),
					ports: NamedPorts{
						"domain": kusciaapisv1alpha1.ContainerPort{
							Name:     "domain",
							Port:     8080,
							Protocol: "HTTP",
							Scope:    kusciaapisv1alpha1.ScopeDomain,
						},
						"cluster": kusciaapisv1alpha1.ContainerPort{
							Name:     "cluster",
							Port:     8081,
							Protocol: "HTTP",
							Scope:    kusciaapisv1alpha1.ScopeCluster,
						},
					},
				},
			},
			want: &proto.Party{
				Name: "bob",
				Role: "client",
				Services: []*proto.Service{
					{
						PortName:  "cluster",
						Endpoints: []string{"deploy-1-cluster.bob.svc"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := c.generateClusterDefineParty(tt.kitInfo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMergeContainersPorts(t *testing.T) {
	tests := []struct {
		name       string
		containers []kusciaapisv1alpha1.Container
		want       NamedPorts
	}{
		{
			name: "containers is empty",
			want: NamedPorts{},
		},
		{
			name: "container ports is empty",
			containers: []kusciaapisv1alpha1.Container{
				{
					Name: "test",
				},
			},
			want: NamedPorts{},
		},
		{
			name: "container ports is not empty",
			containers: []kusciaapisv1alpha1.Container{
				{
					Name: "test",
					Ports: []kusciaapisv1alpha1.ContainerPort{
						{
							Name:     "domain",
							Port:     8080,
							Protocol: "HTTP",
							Scope:    "domain",
						},
					},
				},
			},
			want: NamedPorts{"domain": kusciaapisv1alpha1.ContainerPort{
				Name:     "domain",
				Port:     8080,
				Protocol: "HTTP",
				Scope:    "domain",
			}},
		},
		{
			name: "container ports with repeated port info",
			containers: []kusciaapisv1alpha1.Container{
				{
					Name: "test",
					Ports: []kusciaapisv1alpha1.ContainerPort{
						{
							Name:     "domain",
							Port:     8080,
							Protocol: "HTTP",
							Scope:    "domain",
						},
						{
							Name:     "domain",
							Port:     8080,
							Protocol: "HTTP",
							Scope:    "domain",
						},
					},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := mergeContainersPorts(tt.containers)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateServicedPorts(t *testing.T) {
	tests := []struct {
		name       string
		namedPorts NamedPorts
		want       []string
	}{
		{
			name: "named ports are empty",
		},
		{
			name: "named ports with local port",
			namedPorts: NamedPorts{
				"domain": kusciaapisv1alpha1.ContainerPort{
					Name:  "local",
					Port:  8000,
					Scope: kusciaapisv1alpha1.ScopeLocal,
				},
			},
		},
		{
			name: "named ports with domain port",
			namedPorts: NamedPorts{
				"domain": kusciaapisv1alpha1.ContainerPort{
					Name:  "domain",
					Port:  8000,
					Scope: kusciaapisv1alpha1.ScopeDomain,
				},
			},
			want: []string{"domain"},
		},
		{
			name: "named ports with cluster port",
			namedPorts: NamedPorts{
				"domain": kusciaapisv1alpha1.ContainerPort{
					Name:  "cluster",
					Port:  8000,
					Scope: kusciaapisv1alpha1.ScopeCluster,
				},
			},
			want: []string{"cluster"},
		},
		{
			name: "named ports with local, domain and cluster port",
			namedPorts: NamedPorts{
				"local": kusciaapisv1alpha1.ContainerPort{
					Name:  "local",
					Port:  8000,
					Scope: "local",
				},
				"domain": kusciaapisv1alpha1.ContainerPort{
					Name:  "domain",
					Port:  8001,
					Scope: kusciaapisv1alpha1.ScopeDomain,
				},
				"cluster": kusciaapisv1alpha1.ContainerPort{
					Name:  "cluster",
					Port:  8002,
					Scope: kusciaapisv1alpha1.ScopeCluster,
				},
			},
			want: []string{"cluster", "domain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateServicedPorts(tt.namedPorts)
			sort.Strings(got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGeneratePortServices(t *testing.T) {
	tests := []struct {
		name           string
		deploymentName string
		servicedPorts  []string
		want           PortService
	}{
		{
			name:           "serviced ports is empty",
			deploymentName: "dm-1",
			want:           PortService{},
		},
		{
			name:           "serviced ports is not empty",
			deploymentName: "dm-1",
			servicedPorts:  []string{"domain", "cluster"},
			want:           PortService{"domain": "dm-1-domain", "cluster": "dm-1-cluster"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generatePortServices(tt.deploymentName, tt.servicedPorts)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGeneratePortAccessDomains(t *testing.T) {
	parties := []kusciaapisv1alpha1.KusciaDeploymentParty{
		{
			DomainID: "domain-a",
			Role:     "server",
		},
		{
			DomainID: "domain-b",
			Role:     "client",
		},
		{
			DomainID: "domain-c",
			Role:     "client",
		},
	}

	ports := NamedPorts{
		"port-10000": kusciaapisv1alpha1.ContainerPort{
			Name:     "port-10000",
			Port:     10000,
			Protocol: "HTTP",
			Scope:    kusciaapisv1alpha1.ScopeCluster,
		},
		"port-10001": kusciaapisv1alpha1.ContainerPort{
			Name:     "port-10001",
			Port:     10001,
			Protocol: "HTTP",
			Scope:    kusciaapisv1alpha1.ScopeDomain,
		},
	}

	tests := []struct {
		name                  string
		parties               []kusciaapisv1alpha1.KusciaDeploymentParty
		networkPolicy         *kusciaapisv1alpha1.NetworkPolicy
		wantPortAccessDomains map[string][]string
	}{
		{
			name:    "domain-b,domain-c can access all port, domain-a only can access one port",
			parties: parties,
			networkPolicy: &kusciaapisv1alpha1.NetworkPolicy{
				Ingresses: []kusciaapisv1alpha1.Ingress{
					{
						From: kusciaapisv1alpha1.IngressFrom{
							Roles: []string{"client"},
						},
						Ports: []kusciaapisv1alpha1.IngressPort{
							{
								Port: "port-10000",
							},
							{
								Port: "port-10001",
							},
						},
					},
					{
						From: kusciaapisv1alpha1.IngressFrom{
							Roles: []string{"client", "server"},
						},
						Ports: []kusciaapisv1alpha1.IngressPort{
							{
								Port: "port-10002",
							},
						},
					},
				},
			},

			wantPortAccessDomains: map[string][]string{
				"port-10000": {"domain-b", "domain-c"},
				"port-10001": {"domain-b", "domain-c"},
				"port-10002": {"domain-a", "domain-b", "domain-c"},
			},
		},
		{
			name:          "domain-a, domain-b and domain-c can access cluster port",
			parties:       parties,
			networkPolicy: nil,
			wantPortAccessDomains: map[string][]string{
				"port-10000": {"domain-a", "domain-b", "domain-c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessDomains := generatePortAccessDomains(tt.parties, tt.networkPolicy, ports)
			accessDomainsConverted := map[string][]string{}
			for port, domains := range accessDomains {
				domainSlice := strings.Split(domains, ",")
				sort.Strings(domainSlice)
				accessDomainsConverted[port] = domainSlice
			}
			assert.Equal(t, tt.wantPortAccessDomains, accessDomainsConverted)
		})
	}
}

func TestGenerateConfigMapName(t *testing.T) {
	tests := []struct {
		name           string
		deploymentName string
		want           string
	}{
		{
			name:           "want configmap name: cm-1-configtemplate",
			deploymentName: "cm-1",
			want:           "cm-1-configtemplate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateConfigMapName(tt.deploymentName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateDeploymentName(t *testing.T) {
	tests := []struct {
		name   string
		kdName string
		role   string
		want   string
	}{
		{
			name:   "party role is empty",
			kdName: "kd-1",
			want:   "kd-1",
		},
		{
			name:   "party role is not empty",
			kdName: "kd-1",
			role:   "host",
			want:   "kd-1-host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateDeploymentName(tt.kdName, tt.role)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandleError(t *testing.T) {
	ctx := context.Background()

	kd1 := makeTestKusciaDeployment("kd-1", 2, 1, 1)
	kd2 := makeTestKusciaDeployment("kd-2", 2, 1, 1)
	kd2.Status.Phase = kusciaapisv1alpha1.KusciaDeploymentPhaseFailed

	kusciaClient := kusciafake.NewSimpleClientset(kd2)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	kdInformer := kusciaInformerFactory.Kuscia().V1alpha1().KusciaDeployments()
	kdInformer.Informer().GetStore().Add(kd1)
	c := &Controller{
		kusciaClient: kusciaClient,
	}

	tests := []struct {
		name          string
		partyKitInfos map[string]*PartyKitInfo
		preKdStatus   *kusciaapisv1alpha1.KusciaDeploymentStatus
		kd            *kusciaapisv1alpha1.KusciaDeployment
		wantErr       bool
	}{
		{
			name:    "kd status is not failed",
			kd:      kd1,
			wantErr: false,
		},
		{
			name: "kd status changed to failed",
			preKdStatus: &kusciaapisv1alpha1.KusciaDeploymentStatus{
				Phase: kusciaapisv1alpha1.KusciaDeploymentPhaseProgressing,
			},
			kd:      kd2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.handleError(ctx, tt.partyKitInfos, tt.preKdStatus, tt.kd, nil)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}
