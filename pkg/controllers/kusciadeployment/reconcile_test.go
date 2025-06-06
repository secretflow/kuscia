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
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	clientsetfake "k8s.io/client-go/kubernetes/fake"

	kusciav1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func makeTestKusciaDeployment(name string, replicas int32, maxUnavailable, maxSurge int) *kusciav1alpha1.KusciaDeployment {
	mUnavailable := intstr.FromInt(maxUnavailable)
	mSurge := intstr.FromInt(maxSurge)
	return &kusciav1alpha1.KusciaDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kusciav1alpha1.KusciaDeploymentSpec{
			Initiator:   "alice",
			InputConfig: "test",
			Parties: []kusciav1alpha1.KusciaDeploymentParty{
				{
					DomainID:    "alice",
					AppImageRef: "sf-1",
					Template: kusciav1alpha1.KusciaDeploymentPartyTemplate{
						Replicas: &replicas,
						Strategy: &appsv1.DeploymentStrategy{
							Type: appsv1.RollingUpdateDeploymentStrategyType,
							RollingUpdate: &appsv1.RollingUpdateDeployment{
								MaxUnavailable: &mUnavailable,
								MaxSurge:       &mSurge,
							},
						},
					},
				},
				{
					DomainID:    "bob",
					AppImageRef: "sf-1",
					Template: kusciav1alpha1.KusciaDeploymentPartyTemplate{
						Replicas: &replicas,
						Strategy: &appsv1.DeploymentStrategy{
							Type: appsv1.RollingUpdateDeploymentStrategyType,
							RollingUpdate: &appsv1.RollingUpdateDeployment{
								MaxUnavailable: &mUnavailable,
								MaxSurge:       &mSurge,
							},
						},
					},
				},
			},
		},
	}
}

func makeTestDeployment(name, namespace, image string, replicas int32, maxUnavailable, maxSurge int) *appsv1.Deployment {
	mUnavailable := intstr.FromInt(maxUnavailable)
	mSurge := intstr.FromInt(maxSurge)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &mUnavailable,
					MaxSurge:       &mSurge,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "sf",
							Image: image,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestRefreshPartyDeploymentStatuses(t *testing.T) {
	kd := makeTestKusciaDeployment("kd-1", 1, 1, 1)
	kd.Status = kusciav1alpha1.KusciaDeploymentStatus{}

	partyKitInfos := []*PartyKitInfo{
		{
			kd:       kd,
			domainID: "alice",
			dkInfo: &DeploymentKitInfo{
				deploymentName: "kd-alice-1",
			},
		},
		{
			kd:       kd,
			domainID: "bob",
			dkInfo: &DeploymentKitInfo{
				deploymentName: "kd-bob-1",
			},
		},
	}

	aliceDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kd-alice-1",
			Namespace: "alice",
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          2,
			AvailableReplicas: 0,
		},
	}

	bobDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kd-bob-1",
			Namespace: "bob",
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          2,
			AvailableReplicas: 0,
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	depInformer := informerFactory.Apps().V1().Deployments()
	depInformer.Informer().GetStore().Add(aliceDep)
	depInformer.Informer().GetStore().Add(bobDep)
	c := &Controller{
		kubeClient:       kubeFakeClient,
		deploymentLister: depInformer.Lister(),
	}

	// alice: Replicas[2],AvailableReplicas[0]
	// bob:   Replicas[2],AvailableReplicas[0]
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhaseProgressing)

	// alice: Replicas[2],AvailableReplicas[1]
	// bob:   Replicas[2],AvailableReplicas[0]
	aliceDep.Status.AvailableReplicas = 1
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhaseProgressing)

	// alice: Replicas[2],AvailableReplicas[0]
	// bob:   Replicas[2],AvailableReplicas[1]
	aliceDep.Status.AvailableReplicas = 0
	bobDep.Status.AvailableReplicas = 1
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhaseProgressing)

	// alice: Replicas[2],AvailableReplicas[0]
	// bob:   Replicas[2],AvailableReplicas[2]
	aliceDep.Status.AvailableReplicas = 0
	bobDep.Status.AvailableReplicas = 2
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhaseProgressing)

	// alice: Replicas[2],AvailableReplicas[2]
	// bob:   Replicas[2],AvailableReplicas[0]
	aliceDep.Status.AvailableReplicas = 2
	bobDep.Status.AvailableReplicas = 0
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhaseProgressing)

	// alice: Replicas[2],AvailableReplicas[1]
	// bob:   Replicas[2],AvailableReplicas[1]
	aliceDep.Status.AvailableReplicas = 1
	bobDep.Status.AvailableReplicas = 1
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhasePartialAvailable)

	// alice: Replicas[2],AvailableReplicas[2]
	// bob:   Replicas[2],AvailableReplicas[1]
	aliceDep.Status.AvailableReplicas = 2
	bobDep.Status.AvailableReplicas = 1
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhasePartialAvailable)

	// alice: Replicas[2],AvailableReplicas[1]
	// bob:   Replicas[2],AvailableReplicas[2]
	aliceDep.Status.AvailableReplicas = 1
	bobDep.Status.AvailableReplicas = 2
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhasePartialAvailable)

	// alice: Replicas[2],AvailableReplicas[2]
	// bob:   Replicas[2],AvailableReplicas[2]
	aliceDep.Status.AvailableReplicas = 2
	bobDep.Status.AvailableReplicas = 2
	c.refreshPartyDeploymentStatuses(kd, partyKitInfos)
	assert.Equal(t, kd.Status.Phase, kusciav1alpha1.KusciaDeploymentPhaseAvailable)
}

func TestRefreshPartyDeploymentStatus(t *testing.T) {
	CreationTimestamp := metav1.Now()
	tests := []struct {
		name                    string
		partyDeploymentStatuses map[string]map[string]*kusciav1alpha1.KusciaDeploymentPartyStatus
		role                    string
		deployment              *appsv1.Deployment
		want                    bool
	}{
		{
			name:                    "party status does not exist in partyDeploymentStatuses",
			partyDeploymentStatuses: map[string]map[string]*kusciav1alpha1.KusciaDeploymentPartyStatus{},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy",
					Namespace: "alice",
				},
				Status: appsv1.DeploymentStatus{
					Replicas:            1,
					UpdatedReplicas:     1,
					ReadyReplicas:       1,
					AvailableReplicas:   1,
					UnavailableReplicas: 0,
				},
			},
			want: true,
		},
		{
			name: "deployment status does not exist in partyDeploymentStatuses",
			partyDeploymentStatuses: map[string]map[string]*kusciav1alpha1.KusciaDeploymentPartyStatus{
				"alice": {},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy",
					Namespace: "alice",
				},
				Status: appsv1.DeploymentStatus{
					Replicas:            1,
					UpdatedReplicas:     1,
					ReadyReplicas:       1,
					AvailableReplicas:   1,
					UnavailableReplicas: 0,
				},
			},
			want: true,
		},
		{
			name: "deployment status is different with the status in partyDeploymentStatuses",
			partyDeploymentStatuses: map[string]map[string]*kusciav1alpha1.KusciaDeploymentPartyStatus{
				"alice": {
					"deploy": &kusciav1alpha1.KusciaDeploymentPartyStatus{},
				},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy",
					Namespace: "alice",
				},
				Status: appsv1.DeploymentStatus{
					Replicas:            1,
					UpdatedReplicas:     1,
					ReadyReplicas:       1,
					AvailableReplicas:   1,
					UnavailableReplicas: 0,
				},
			},
			want: true,
		},
		{
			name: "deployment status is same as the status in partyDeploymentStatuses",
			partyDeploymentStatuses: map[string]map[string]*kusciav1alpha1.KusciaDeploymentPartyStatus{
				"alice": {
					"deploy": &kusciav1alpha1.KusciaDeploymentPartyStatus{
						Phase:               kusciav1alpha1.KusciaDeploymentPhaseAvailable,
						Replicas:            1,
						UpdatedReplicas:     1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
						CreationTimestamp:   &CreationTimestamp,
					},
				},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: CreationTimestamp,
					Name:              "deploy",
					Namespace:         "alice",
				},
				Status: appsv1.DeploymentStatus{
					Replicas:            1,
					UpdatedReplicas:     1,
					ReadyReplicas:       1,
					AvailableReplicas:   1,
					UnavailableReplicas: 0,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := refreshPartyDeploymentStatus(tt.partyDeploymentStatuses, tt.deployment, tt.role)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSyncService(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	partyKitInfo := &PartyKitInfo{
		kd:       kd,
		domainID: "alice",
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
			ports: NamedPorts{
				"domain": kusciav1alpha1.ContainerPort{
					Name:  "domain",
					Port:  8080,
					Scope: kusciav1alpha1.ScopeDomain,
				},
			},
			portService: PortService{"domain": "kd-svc-1"},
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	svcInformer := informerFactory.Core().V1().Services()
	c := &Controller{
		kubeClient:    kubeFakeClient,
		serviceLister: svcInformer.Lister(),
	}

	err := c.syncService(context.Background(), []*PartyKitInfo{partyKitInfo})
	assert.NoError(t, err)
}

func TestCreateService(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	partyKitInfo := &PartyKitInfo{
		kd:       kd,
		domainID: "alice",
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
			ports: NamedPorts{
				"domain": kusciav1alpha1.ContainerPort{
					Name:  "domain",
					Port:  8080,
					Scope: kusciav1alpha1.ScopeDomain,
				},
			},
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	c := &Controller{
		kubeClient: kubeFakeClient,
	}

	tests := []struct {
		name        string
		kitInfo     *PartyKitInfo
		portName    string
		serviceName string
		wantErr     bool
	}{
		{
			name:        "port is not found",
			kitInfo:     partyKitInfo,
			portName:    "cluster",
			serviceName: "kd-svc-1",
			wantErr:     true,
		},
		{
			name:        "port is found",
			kitInfo:     partyKitInfo,
			portName:    "domain",
			serviceName: "kd-svc-1",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.createService(context.Background(), tt.kitInfo, tt.portName, tt.serviceName)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestGenerateService(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	partyKitInfo := &PartyKitInfo{
		kd:       kd,
		domainID: "alice",
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
		},
	}

	port := kusciav1alpha1.ContainerPort{
		Name:  "domain",
		Port:  8080,
		Scope: "domain",
	}

	svc, err := generateService(partyKitInfo, "kd-1-svc", port)
	assert.NotNil(t, svc)
	assert.NoError(t, err)
}

func TestSyncConfigMap(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	partyKitInfo := &PartyKitInfo{
		kd:                    kd,
		domainID:              "alice",
		configTemplatesCMName: "alice-cm",
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	cmInformer := informerFactory.Core().V1().ConfigMaps()
	c := &Controller{
		kubeClient:      kubeFakeClient,
		configMapLister: cmInformer.Lister(),
	}

	err := c.syncConfigMap(context.Background(), []*PartyKitInfo{partyKitInfo})
	assert.NoError(t, err)
}

func TestCreateConfigMap(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	partyKitInfo := &PartyKitInfo{
		kd:                    kd,
		domainID:              "alice",
		configTemplatesCMName: "alice-cm",
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
		},
	}

	kubeFakeClient := clientsetfake.NewSimpleClientset()
	c := &Controller{
		kubeClient: kubeFakeClient,
	}

	err := c.createConfigMap(context.Background(), partyKitInfo)
	assert.NoError(t, err)
}

func TestGenerateConfigMap(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	partyKitInfo := &PartyKitInfo{
		kd:                    kd,
		domainID:              "alice",
		configTemplatesCMName: "alice-cm",
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
		},
	}

	cm := generateConfigMap(partyKitInfo)
	assert.Equal(t, partyKitInfo.configTemplatesCMName, cm.Name)
	assert.Equal(t, partyKitInfo.domainID, cm.Namespace)
}

func TestSyncDeployment(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := informerFactory.Core().V1().Namespaces()
	deployInformer := informerFactory.Apps().V1().Deployments()

	aliceNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "alice"},
	}
	nsInformer.Informer().GetStore().Add(aliceNs)

	partyKitInfo := &PartyKitInfo{
		kd:       kd,
		domainID: "alice",
		deployTemplate: &kusciav1alpha1.KusciaDeploymentPartyTemplate{
			Spec: kusciav1alpha1.PodSpec{
				Containers: []kusciav1alpha1.Container{
					{
						Name: "sf",
					},
				},
			},
		},
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
			image:          "sf-2",
		},
	}

	c := &Controller{
		kubeClient:       kubeFakeClient,
		deploymentLister: deployInformer.Lister(),
		namespaceLister:  nsInformer.Lister(),
	}

	err := c.syncDeployment(context.Background(), []*PartyKitInfo{partyKitInfo})
	assert.NoError(t, err)
}

func TestGenerateDeployment(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	kubeFakeClient := clientsetfake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	nsInformer := informerFactory.Core().V1().Namespaces()

	aliceNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "alice"},
	}
	bobNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "bob"},
	}

	nsInformer.Informer().GetStore().Add(aliceNs)
	nsInformer.Informer().GetStore().Add(bobNs)

	partyKitInfo := &PartyKitInfo{
		kd:       kd,
		domainID: "alice",
		deployTemplate: &kusciav1alpha1.KusciaDeploymentPartyTemplate{
			Spec: kusciav1alpha1.PodSpec{
				Containers: []kusciav1alpha1.Container{
					{
						Name: "sf",
					},
				},
			},
		},
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
			image:          "sf-2",
		},
	}

	c := &Controller{
		kubeClient:      kubeFakeClient,
		namespaceLister: nsInformer.Lister(),
	}

	deployment, err := c.generateDeployment(partyKitInfo)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)
}

func TestUpdateDeployment(t *testing.T) {
	kd := makeTestKusciaDeployment("kd", 1, 1, 1)
	d1 := makeTestDeployment("kd-1", "alice", "sf-1", 2, 2, 2)
	kubeFakeClient := clientsetfake.NewSimpleClientset(d1)
	informerFactory := informers.NewSharedInformerFactory(kubeFakeClient, 0)
	deployInformer := informerFactory.Apps().V1().Deployments()
	deployInformer.Informer().GetStore().Add(d1)

	partyKitInfo := &PartyKitInfo{
		kd:       kd,
		domainID: "alice",
		deployTemplate: &kusciav1alpha1.KusciaDeploymentPartyTemplate{
			Spec: kusciav1alpha1.PodSpec{
				Containers: []kusciav1alpha1.Container{
					{
						Name: "sf",
					},
				},
			},
		},
		dkInfo: &DeploymentKitInfo{
			deploymentName: "kd-1",
			image:          "sf-2",
		},
	}

	c := &Controller{
		kubeClient:       kubeFakeClient,
		deploymentLister: deployInformer.Lister(),
	}

	err := c.updateDeployment(context.Background(), partyKitInfo)
	assert.NoError(t, err)
}
