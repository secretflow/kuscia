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

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

func TestCreateServing(t *testing.T) {
	res := kusciaAPISS.CreateServing(context.Background(), &kusciaapi.CreateServingRequest{
		ServingId:          kusciaAPISS.servingID,
		ServingInputConfig: "test",
		Initiator:          "alice",
		Parties:            kusciaAPISS.parties,
	})
	assert.Equal(t, res.Status.Code, int32(0))
}

func TestQueryServing(t *testing.T) {
	res := kusciaAPISS.QueryServing(context.Background(), &kusciaapi.QueryServingRequest{
		ServingId: kusciaAPISS.servingID,
	})
	assert.Equal(t, len(res.Data.Parties), len(kusciaAPISS.parties))
}

func TestBatchQueryServingStatus(t *testing.T) {
	res := kusciaAPISS.BatchQueryServingStatus(context.Background(), &kusciaapi.BatchQueryServingStatusRequest{
		ServingIds: []string{kusciaAPISS.servingID},
	})
	assert.Equal(t, len(res.Data.Servings), 1)
}

func TestUpdateServing(t *testing.T) {
	res := kusciaAPISS.UpdateServing(context.Background(), &kusciaapi.UpdateServingRequest{
		ServingId:          kusciaAPISS.servingID,
		ServingInputConfig: "123",
	})
	assert.Equal(t, res.Status.Code, int32(0))
}

func TestDeleteServing(t *testing.T) {
	res := kusciaAPISS.DeleteServing(context.Background(), &kusciaapi.DeleteServingRequest{
		ServingId: kusciaAPISS.servingID,
	})
	assert.Equal(t, res.Status.Code, int32(0))
}

func TestValidateCreateServingRequest(t *testing.T) {
	tests := []struct {
		name              string
		expectedInitiator string
		req               *kusciaapi.CreateServingRequest
		wantErr           bool
	}{
		{
			name:    "request serving id is invalid",
			req:     &kusciaapi.CreateServingRequest{},
			wantErr: true,
		},
		{
			name:              "request initiator doesn't match",
			expectedInitiator: "alice",
			req: &kusciaapi.CreateServingRequest{
				ServingId: "test",
				Initiator: "bob",
			},
			wantErr: true,
		},
		{
			name:              "request parties is empty",
			expectedInitiator: "alice",
			req: &kusciaapi.CreateServingRequest{
				ServingId: "test",
				Initiator: "alice",
			},
			wantErr: true,
		},
		{
			name:              "request party appimage is empty",
			expectedInitiator: "alice",
			req: &kusciaapi.CreateServingRequest{
				ServingId: "test",
				Initiator: "alice",
				Parties: []*kusciaapi.ServingParty{{
					DomainId: "alice",
				}},
			},
			wantErr: true,
		},
		{
			name:              "request party service name prefix doesn't match regex",
			expectedInitiator: "alice",
			req: &kusciaapi.CreateServingRequest{
				ServingId: "test",
				Initiator: "alice",
				Parties: []*kusciaapi.ServingParty{{
					DomainId:          "alice",
					AppImage:          "test",
					ServiceNamePrefix: "00Abd",
				}},
			},
			wantErr: true,
		},
		{
			name:              "request party service name prefix length is invalid",
			expectedInitiator: "alice",
			req: &kusciaapi.CreateServingRequest{
				ServingId: "test",
				Initiator: "alice",
				Parties: []*kusciaapi.ServingParty{{
					DomainId:          "alice",
					AppImage:          "test",
					ServiceNamePrefix: "svc-11111111-11111111-11111111-11111111-11111111-11111111",
				}},
			},
			wantErr: true,
		},
		{
			name:              "request is valid",
			expectedInitiator: "alice",
			req: &kusciaapi.CreateServingRequest{
				ServingId: "test",
				Initiator: "alice",
				Parties: []*kusciaapi.ServingParty{{
					DomainId:          "alice",
					AppImage:          "test",
					ServiceNamePrefix: "s-11111111-11111111-11111111-11111111-11111111-1",
				}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateCreateServingRequest(tt.expectedInitiator, tt.req)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestValidateBatchQueryServingStatusRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *kusciaapi.BatchQueryServingStatusRequest
		wantErr bool
	}{
		{
			name:    "request data is empty",
			req:     &kusciaapi.BatchQueryServingStatusRequest{},
			wantErr: true,
		},
		{
			name: "request data serving id is empty",
			req: &kusciaapi.BatchQueryServingStatusRequest{
				ServingIds: []string{},
			},
			wantErr: true,
		},
		{
			name: "request is valid",
			req: &kusciaapi.BatchQueryServingStatusRequest{
				ServingIds: []string{"test"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateBatchQueryServingStatusRequest(tt.req)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestAuthenticateServingRequest(t *testing.T) {
	parentCtx := context.WithValue(context.Background(), constants.SourceDomainKey, "alice")
	tests := []struct {
		name    string
		ctx     context.Context
		parties []*kusciaapi.ServingParty
		wantErr bool
	}{
		{
			name:    "request from master",
			ctx:     context.WithValue(parentCtx, constants.AuthRole, constants.AuthRoleMaster),
			parties: nil,
			wantErr: false,
		},
		{
			name:    "request from domain and domain doesn't exist in party",
			ctx:     context.WithValue(parentCtx, constants.AuthRole, constants.AuthRoleDomain),
			parties: nil,
			wantErr: true,
		},
		{
			name: "request from domain and domain exist in party",
			ctx:  context.WithValue(parentCtx, constants.AuthRole, constants.AuthRoleDomain),
			parties: []*kusciaapi.ServingParty{{
				DomainId: "alice",
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authenticateServingRequest(tt.ctx, tt.parties)
			assert.Equal(t, tt.wantErr, got != nil)
		})
	}
}

func TestSelfAsParticipant(t *testing.T) {
	parentCtx := context.WithValue(context.Background(), constants.SourceDomainKey, "alice")
	tests := []struct {
		name string
		ctx  context.Context
		kd   *v1alpha1.KusciaDeployment
		want bool
	}{
		{
			name: "request from master",
			ctx:  context.WithValue(parentCtx, constants.AuthRole, constants.AuthRoleMaster),
			kd:   nil,
			want: true,
		},
		{
			name: "request from domain and domain doesn't exist in party",
			ctx:  context.WithValue(parentCtx, constants.AuthRole, constants.AuthRoleDomain),
			kd: &v1alpha1.KusciaDeployment{
				Spec: v1alpha1.KusciaDeploymentSpec{
					Parties: []v1alpha1.KusciaDeploymentParty{{
						DomainID: "bob",
					}},
				},
			},
			want: false,
		},
		{
			name: "request from domain and domain exist in party",
			ctx:  context.WithValue(parentCtx, constants.AuthRole, constants.AuthRoleDomain),
			kd: &v1alpha1.KusciaDeployment{
				Spec: v1alpha1.KusciaDeploymentSpec{
					Parties: []v1alpha1.KusciaDeploymentParty{{
						DomainID: "alice",
					}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selfAsParticipant(tt.ctx, tt.kd)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildKusciaDeployment(t *testing.T) {
	replicas := int32(2)
	parties := []*kusciaapi.ServingParty{{
		DomainId: "alice",
		AppImage: "mockImageName",
		Replicas: &replicas,
		UpdateStrategy: &kusciaapi.UpdateStrategy{
			Type: "Recreate",
		},
		Resources: []*kusciaapi.Resource{{
			MinCpu:    "1",
			MaxCpu:    "1",
			MinMemory: "1Gi",
			MaxMemory: "1Gi",
		}},
	}}

	s := kusciaAPISS.IServingService.(*servingService)
	req := &kusciaapi.CreateServingRequest{
		ServingId: "test",
		Parties:   parties,
	}

	got, err := s.buildKusciaDeployment(context.Background(), req)
	assert.Equal(t, true, got != nil)
	assert.Equal(t, nil, err)
}

func TestBuildKusciaDeploymentPartyStrategy(t *testing.T) {
	tests := []struct {
		name  string
		party *kusciaapi.ServingParty
		want  *appsv1.DeploymentStrategy
	}{
		{
			name: "party input strategy type is invalid",
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
				UpdateStrategy: &kusciaapi.UpdateStrategy{
					Type:           "invalid_type",
					MaxUnavailable: "30%",
					MaxSurge:       "40%",
				},
			},
			want: nil,
		},
		{
			name: "party input strategy is empty and return default strategy",
			party: &kusciaapi.ServingParty{
				AppImage:       "mockImageName",
				DomainId:       "alice",
				UpdateStrategy: nil,
			},
			want: &appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   1,
						StrVal: "25%",
					},
					MaxSurge: &intstr.IntOrString{
						Type:   1,
						StrVal: "25%",
					},
				},
			},
		},
		{
			name: "party input strategy type is Recreate",
			party: &kusciaapi.ServingParty{
				AppImage:       "mockImageName",
				DomainId:       "alice",
				UpdateStrategy: &kusciaapi.UpdateStrategy{Type: recreateDeploymentStrategyType},
			},
			want: &appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
		{
			name: "party input strategy type is RollingUpdate",
			party: &kusciaapi.ServingParty{
				AppImage: "mockImage",
				DomainId: "alice",
				UpdateStrategy: &kusciaapi.UpdateStrategy{
					Type:           rollingUpdateDeploymentStrategyType,
					MaxUnavailable: "30%",
					MaxSurge:       "40%",
				},
			},
			want: &appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   1,
						StrVal: "30%",
					},
					MaxSurge: &intstr.IntOrString{
						Type:   1,
						StrVal: "40%",
					},
				},
			},
		},
	}

	s := servingService{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.buildKusciaDeploymentPartyStrategy(tt.party)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildKusciaDeploymentPartyResources(t *testing.T) {
	tests := []struct {
		name  string
		party *kusciaapi.ServingParty
		want  []v1alpha1.Container
	}{
		{
			name: "party resources is empty",
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
			},
			want: nil,
		},
		{
			name: "party resources container name is empty",
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
				Resources: []*kusciaapi.Resource{
					{
						ContainerName: "",
						MinCpu:        "20m",
						MaxCpu:        "2",
						MinMemory:     "20Mi",
						MaxMemory:     "2Gi",
					},
				},
			},
			want: []v1alpha1.Container{
				{
					Name: "mock",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("2"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("20Mi"),
							corev1.ResourceCPU:    resource.MustParse("20m"),
						},
					},
				},
			},
		},
		{
			name: "party resources container name is not empty and not exist",
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
				Resources: []*kusciaapi.Resource{
					{
						ContainerName: "mock-1",
						MinCpu:        "20m",
						MaxCpu:        "2",
						MinMemory:     "20Mi",
						MaxMemory:     "2Gi",
					},
				},
			},
			want: nil,
		},
		{
			name: "party resources container name is not empty and exist",
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
				Resources: []*kusciaapi.Resource{
					{
						ContainerName: "mock",
						MinCpu:        "20m",
						MaxCpu:        "2",
						MinMemory:     "20Mi",
						MaxMemory:     "2Gi",
					},
				},
			},
			want: []v1alpha1.Container{
				{
					Name: "mock",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("2"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("20Mi"),
							corev1.ResourceCPU:    resource.MustParse("20m"),
						},
					},
				},
			},
		},
	}

	kusciaClient := kusciafake.NewSimpleClientset(makeMockAppImage("mockImageName"))
	s := servingService{kusciaClient: kusciaClient}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.buildKusciaDeploymentPartyContainers(context.Background(), tt.party)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildServingResources(t *testing.T) {
	kd := &v1alpha1.KusciaDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "serving-1",
		},
		Spec: v1alpha1.KusciaDeploymentSpec{},
	}

	tests := []struct {
		name          string
		kdParty       *v1alpha1.KusciaDeploymentParty
		partyTemplate *v1alpha1.DeployTemplate
		want          []*kusciaapi.Resource
	}{
		{
			name: "kd party containers is empty",
			kdParty: &v1alpha1.KusciaDeploymentParty{
				DomainID: "alice",
				Template: v1alpha1.KusciaDeploymentPartyTemplate{},
			},
			partyTemplate: &v1alpha1.DeployTemplate{
				Name: "tpl",
				Spec: v1alpha1.PodSpec{
					Containers: []v1alpha1.Container{
						{
							Name: "mock",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2Gi"),
									corev1.ResourceCPU:    resource.MustParse("2"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("20Mi"),
									corev1.ResourceCPU:    resource.MustParse("20m"),
								},
							},
						},
					},
				},
			},
			want: []*kusciaapi.Resource{
				{
					ContainerName: "mock",
					MinCpu:        "20m",
					MaxCpu:        "2",
					MinMemory:     "20Mi",
					MaxMemory:     "2Gi",
				},
			},
		},
		{
			name: "kd party containers is not empty",
			kdParty: &v1alpha1.KusciaDeploymentParty{
				DomainID: "alice",
				Template: v1alpha1.KusciaDeploymentPartyTemplate{
					Spec: v1alpha1.PodSpec{
						Containers: []v1alpha1.Container{
							{
								Name: "mock",
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("2Gi"),
										corev1.ResourceCPU:    resource.MustParse("2"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("20Mi"),
										corev1.ResourceCPU:    resource.MustParse("20m"),
									},
								},
							},
						},
					},
				},
			},
			partyTemplate: &v1alpha1.DeployTemplate{
				Name: "tpl",
				Spec: v1alpha1.PodSpec{
					Containers: []v1alpha1.Container{
						{
							Name: "mock",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
									corev1.ResourceCPU:    resource.MustParse("1"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("20Mi"),
									corev1.ResourceCPU:    resource.MustParse("20m"),
								},
							},
						},
					},
				},
			},
			want: []*kusciaapi.Resource{
				{
					ContainerName: "mock",
					MinCpu:        "20m",
					MaxCpu:        "2",
					MinMemory:     "20Mi",
					MaxMemory:     "2Gi",
				},
			},
		},
	}

	s := servingService{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.buildServingResources(context.Background(), kd, tt.kdParty, tt.partyTemplate)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildServingStatusDetail(t *testing.T) {
	tests := []struct {
		name string
		kd   *v1alpha1.KusciaDeployment
		want *kusciaapi.ServingStatusDetail
	}{
		{
			name: "kd party deployment status is empty",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kd",
				},
				Status: v1alpha1.KusciaDeploymentStatus{},
			},
			want: nil,
		},
		{
			name: "kd party deployment status is not empty",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kd",
				},
				Status: v1alpha1.KusciaDeploymentStatus{
					Phase:            v1alpha1.KusciaDeploymentPhaseAvailable,
					TotalParties:     2,
					AvailableParties: 2,
					PartyDeploymentStatuses: map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{
						"alice": {
							"kd-deploy-1": {
								Replicas:            1,
								UpdatedReplicas:     1,
								AvailableReplicas:   1,
								UnavailableReplicas: 0,
								Conditions: []appsv1.DeploymentCondition{
									{
										Type:   appsv1.DeploymentAvailable,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
						"bob": {
							"kd-deploy-1": {
								Replicas:            2,
								UpdatedReplicas:     2,
								AvailableReplicas:   2,
								UnavailableReplicas: 0,
								Conditions: []appsv1.DeploymentCondition{
									{
										Type:   appsv1.DeploymentAvailable,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
				},
			},
			want: &kusciaapi.ServingStatusDetail{
				State:            "Available",
				TotalParties:     2,
				AvailableParties: 2,
				PartyStatuses: []*kusciaapi.PartyServingStatus{
					{
						DomainId:            "alice",
						State:               "Available",
						Replicas:            1,
						AvailableReplicas:   1,
						UnavailableReplicas: 0,
						UpdatedReplicas:     1,
					},
					{
						DomainId:            "bob",
						State:               "Available",
						Replicas:            2,
						AvailableReplicas:   2,
						UnavailableReplicas: 0,
						UpdatedReplicas:     2,
					},
				},
			},
		},
	}

	kubeClient := kubefake.NewSimpleClientset()
	s := servingService{kubeClient: kubeClient}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.buildServingStatusDetail(context.Background(), tt.kd)
			if tt.want == nil {
				assert.Equal(t, tt.want, got)
			} else {
				assert.Equal(t, tt.want.State, got.State)
				assert.Equal(t, tt.want.TotalParties, got.TotalParties)
				assert.Equal(t, tt.want.AvailableParties, got.AvailableParties)
			}
		})
	}
}

func TestUpdateKusciaDeploymentParty(t *testing.T) {
	replicas := int32(1)
	replicasTwo := int32(2)
	tests := []struct {
		name      string
		servingID string
		kd        *v1alpha1.KusciaDeployment
		party     *kusciaapi.ServingParty
		want      interface{}
	}{
		{
			name:      "party is empty",
			servingID: "serving-1",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "serving-1",
				},
				Spec: v1alpha1.KusciaDeploymentSpec{},
			},
			party: nil,
			want:  false,
		},
		{
			name:      "party alice appimage is updated",
			servingID: "serving-1",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "serving-1",
				},
				Spec: v1alpha1.KusciaDeploymentSpec{
					Initiator:   "alice",
					InputConfig: "123456",
					Parties: []v1alpha1.KusciaDeploymentParty{
						{
							DomainID:    "alice",
							AppImageRef: "mockImageName",
							Template: v1alpha1.KusciaDeploymentPartyTemplate{
								Replicas: &replicas,
							},
						},
						{
							DomainID: "bob",
						},
					},
				},
			},
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName2",
				DomainId: "alice",
				Replicas: &replicas,
			},
			want: true,
		},
		{
			name:      "party alice appimage is updated, but can not find the appimage",
			servingID: "serving-1",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "serving-1",
				},
				Spec: v1alpha1.KusciaDeploymentSpec{
					Initiator:   "alice",
					InputConfig: "123456",
					Parties: []v1alpha1.KusciaDeploymentParty{
						{
							DomainID:    "alice",
							AppImageRef: "mockImageName",
							Template: v1alpha1.KusciaDeploymentPartyTemplate{
								Replicas: &replicas,
							},
						},
						{
							DomainID: "bob",
						},
					},
				},
			},
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName3",
				DomainId: "alice",
				Replicas: &replicas,
			},
			want: false,
		},
		{
			name:      "party alice replicas is updated",
			servingID: "serving-1",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "serving-1",
				},
				Spec: v1alpha1.KusciaDeploymentSpec{
					Initiator:   "alice",
					InputConfig: "123456",
					Parties: []v1alpha1.KusciaDeploymentParty{
						{
							DomainID:    "alice",
							AppImageRef: "mockImageName",
							Template: v1alpha1.KusciaDeploymentPartyTemplate{
								Replicas: &replicas,
							},
						},
						{
							DomainID: "bob",
						},
					},
				},
			},
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
				Replicas: &replicasTwo,
			},
			want: true,
		},
		{
			name:      "party alice strategy is updated",
			servingID: "serving-1",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "serving-1",
				},
				Spec: v1alpha1.KusciaDeploymentSpec{
					Initiator:   "alice",
					InputConfig: "123456",
					Parties: []v1alpha1.KusciaDeploymentParty{
						{
							DomainID:    "alice",
							AppImageRef: "mockImageName",
							Template: v1alpha1.KusciaDeploymentPartyTemplate{
								Replicas: &replicas,
								Strategy: &appsv1.DeploymentStrategy{
									Type: appsv1.RecreateDeploymentStrategyType,
								},
							},
						},
						{
							DomainID: "bob",
						},
					},
				},
			},
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
				Replicas: &replicas,
				UpdateStrategy: &kusciaapi.UpdateStrategy{
					Type:           "RollingUpdate",
					MaxSurge:       "30%",
					MaxUnavailable: "30%",
				},
			},
			want: true,
		},
		{
			name:      "party alice resources is updated",
			servingID: "serving-1",
			kd: &v1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "serving-1",
				},
				Spec: v1alpha1.KusciaDeploymentSpec{
					Initiator:   "alice",
					InputConfig: "123456",
					Parties: []v1alpha1.KusciaDeploymentParty{
						{
							DomainID:    "alice",
							AppImageRef: "mockImageName",
							Template: v1alpha1.KusciaDeploymentPartyTemplate{
								Replicas: &replicas,
								Strategy: &appsv1.DeploymentStrategy{
									Type: appsv1.RollingUpdateDeploymentStrategyType,
									RollingUpdate: &appsv1.RollingUpdateDeployment{
										MaxUnavailable: &intstr.IntOrString{
											Type:   1,
											StrVal: "30%",
										},
										MaxSurge: &intstr.IntOrString{
											Type:   1,
											StrVal: "30%",
										},
									},
								},
							},
						},
						{
							DomainID: "bob",
						},
					},
				},
			},
			party: &kusciaapi.ServingParty{
				AppImage: "mockImageName",
				DomainId: "alice",
				Replicas: &replicas,
				UpdateStrategy: &kusciaapi.UpdateStrategy{
					Type:           "RollingUpdate",
					MaxSurge:       "30%",
					MaxUnavailable: "30%",
				},
				Resources: []*kusciaapi.Resource{
					{
						ContainerName: "",
						MinCpu:        "1",
						MaxCpu:        "1",
						MinMemory:     "1Gi",
						MaxMemory:     "1Gi",
					},
				},
			},
			want: true,
		},
	}

	kusciaClient := kusciafake.NewSimpleClientset(makeMockAppImage("mockImageName"), makeMockAppImage("mockImageName2"))
	s := servingService{kusciaClient: kusciaClient}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.updateKusciaDeploymentParty(context.Background(), tt.kd, tt.party)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetAffinityMode(t *testing.T) {
	s := servingService{}
	tests := []struct {
		name     string
		mode     string
		expected string
	}{
		{
			name:     "empty mode should default to anti-affinity",
			mode:     "",
			expected: "anti-affinity",
		},
		{
			name:     "none mode",
			mode:     "none",
			expected: "none",
		},
		{
			name:     "affinity mode",
			mode:     "affinity",
			expected: "affinity",
		},
		{
			name:     "anti-affinity mode",
			mode:     "anti-affinity",
			expected: "anti-affinity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.getAffinityMode(tt.mode)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBuildAffinityForMode(t *testing.T) {
	s := servingService{}
	servingID := "test-serving"

	tests := []struct {
		name     string
		mode     string
		expected *corev1.Affinity
	}{
		{
			name:     "none mode should return nil",
			mode:     "none",
			expected: nil,
		},
		{
			name: "affinity mode should return PodAffinity",
			mode: "affinity",
			expected: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kuscia.secretflow/kd-name": servingID,
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
		},
		{
			name: "anti-affinity mode should return PodAntiAffinity",
			mode: "anti-affinity",
			expected: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kuscia.secretflow/kd-name": servingID,
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
		},
		{
			name: "unknown mode should default to anti-affinity",
			mode: "unknown",
			expected: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kuscia.secretflow/kd-name": servingID,
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.buildAffinityForMode(tt.mode, servingID)
			if tt.expected == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				if tt.mode == "affinity" {
					assert.NotNil(t, got.PodAffinity)
					assert.Nil(t, got.PodAntiAffinity)
					assert.Equal(t, tt.expected.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight, got.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight)
				} else {
					assert.NotNil(t, got.PodAntiAffinity)
					assert.Nil(t, got.PodAffinity)
					assert.Equal(t, tt.expected.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight, got.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight)
				}
			}
		})
	}
}

func TestBuildKusciaDeploymentWithAffinityMode(t *testing.T) {
	replicas := int32(2)
	parties := []*kusciaapi.ServingParty{{
		DomainId: "alice",
		AppImage: "mockImageName",
		Replicas: &replicas,
	}}

	kusciaClient := kusciafake.NewSimpleClientset(makeMockAppImage("mockImageName"))
	s := servingService{kusciaClient: kusciaClient}

	tests := []struct {
		name         string
		affinityMode string
		expectAffinity bool
	}{
		{
			name:          "affinity_mode is none, should not set affinity in party template",
			affinityMode:  "none",
			expectAffinity: false,
		},
		{
			name:          "affinity_mode is affinity, should set PodAffinity",
			affinityMode:  "affinity",
			expectAffinity: true,
		},
		{
			name:          "affinity_mode is anti-affinity, should set PodAntiAffinity",
			affinityMode:  "anti-affinity",
			expectAffinity: true,
		},
		{
			name:          "affinity_mode is empty, should default to anti-affinity",
			affinityMode:  "",
			expectAffinity: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &kusciaapi.CreateServingRequest{
				ServingId:    "test-serving",
				Initiator:    "alice",
				AffinityMode: tt.affinityMode,
				Parties:      parties,
			}

			kd, err := s.buildKusciaDeployment(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, kd)

			if tt.expectAffinity {
				assert.NotNil(t, kd.Spec.Parties[0].Template.Spec.Affinity)
				if tt.affinityMode == "affinity" || tt.affinityMode == "" {
					// Empty mode defaults to anti-affinity
					if tt.affinityMode == "affinity" {
						assert.NotNil(t, kd.Spec.Parties[0].Template.Spec.Affinity.PodAffinity)
						assert.Nil(t, kd.Spec.Parties[0].Template.Spec.Affinity.PodAntiAffinity)
					} else {
						assert.NotNil(t, kd.Spec.Parties[0].Template.Spec.Affinity.PodAntiAffinity)
					}
				}
			} else {
				// When affinity_mode is "none", affinity should be nil
				assert.Nil(t, kd.Spec.Parties[0].Template.Spec.Affinity)
			}
		})
	}
}