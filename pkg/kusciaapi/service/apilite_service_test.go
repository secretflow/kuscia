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
package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	informers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	"github.com/stretchr/testify/assert"
)

type kusciaAPIDomainRoute struct {
	IDomainRouteService
	source      string
	destination string
}

type kusciaAPIDomainService struct {
	IDomainService
	domainID string
}

type kusciaAPIJobService struct {
	IJobService
	jobID string
	tasks []*kusciaapi.Task
}

type kusciaAPIServingService struct {
	IServingService
	servingID string
	parties   []*kusciaapi.ServingParty
}
type kusciaAPIDomainDataGrant struct {
	IDomainDataGrantService
	conf *config.KusciaAPIConfig
	data *kusciaapi.DomainDataGrantData
}

var kusciaAPIJS *kusciaAPIJobService

var kusciaAPIDS *kusciaAPIDomainService

var kusciaAPIDR *kusciaAPIDomainRoute

var kusciaAPISS *kusciaAPIServingService
var kusciaAPIDG *kusciaAPIDomainDataGrant

func TestServiceMain(t *testing.T) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	dir, err := os.MkdirTemp("", "TestCreateDomainDataGrant")
	assert.NoError(t, err)
	cafile := filepath.Join(dir, "ca.key")

	keyOut, err := os.OpenFile(cafile, os.O_CREATE|os.O_RDWR, 0644)
	assert.NoError(t, err)
	defer keyOut.Close()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)
	err = pem.Encode(keyOut, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	assert.NoError(t, err)
	kusciaAPIConfig := config.NewDefaultKusciaAPIConfig("")
	kusciaClient := kusciafake.NewSimpleClientset(makeMockAppImage("mockImageName"))
	kusciaAPIConfig.DomainKeyFile = cafile
	kusciaInformerFactory := informers.NewSharedInformerFactoryWithOptions(kusciaClient, 0)
	kusciaAPIConfig.KusciaClient = kusciaClient
	kusciaInformerFactory.Start(context.Background().Done())

	kusciaAPIDR = &kusciaAPIDomainRoute{
		source:              "alice",
		destination:         "bob",
		IDomainRouteService: NewDomainRouteService(kusciaAPIConfig),
	}

	kusciaAPIDS = &kusciaAPIDomainService{
		IDomainService: NewDomainService(kusciaAPIConfig),
		domainID:       "alice",
	}

	tasks := []*kusciaapi.Task{
		{
			TaskId:          "mockJobID-task1",
			Alias:           "task1",
			TaskInputConfig: "{}",
			AppImage:        "mockImageName",
			Parties: []*kusciaapi.Party{
				{
					DomainId: "alice",
				},
				{
					DomainId: "bob",
				},
			},
		},
	}
	kusciaAPIJS = &kusciaAPIJobService{
		IJobService: NewJobService(kusciaAPIConfig),
		jobID:       "test",
		tasks:       tasks,
	}

	replicas := int32(1)
	parties := []*kusciaapi.ServingParty{
		{
			AppImage: "mockImageName",
			Role:     "client",
			DomainId: "alice",
			Replicas: &replicas,
		},
	}
	kusciaAPISS = &kusciaAPIServingService{
		IServingService: NewServingService(kusciaAPIConfig),
		servingID:       "test",
		parties:         parties,
	}
	tm := metav1.Now().Add(30 * time.Second)
	data := &kusciaapi.DomainDataGrantData{
		DomaindatagrantId: "",
		Author:            "alice",
		DomaindataId:      "xxxxxx",
		GrantDomain:       "bob",
		Limit: &kusciaapi.GrantLimit{
			ExpirationTime: tm.UnixNano(),
			UseCount:       3,
			FlowId:         "bbbb",
			Componets:      []string{"mpc", "psi"},
			Initiator:      "alice",
			InputConfig:    "xxxxx",
		},
		Description: map[string]string{
			"test": "xxx",
		},
		Signature: "ssssss",
		DomainId:  "alice",
	}

	kusciaAPIDG = &kusciaAPIDomainDataGrant{
		IDomainDataGrantService: NewDomainDataGrantService(kusciaAPIConfig),
		conf:                    kusciaAPIConfig,
		data:                    data,
	}
}

func makeMockAppImage(name string) *v1alpha1.AppImage {
	replicas := int32(1)
	return &v1alpha1.AppImage{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.AppImageSpec{
			Image: v1alpha1.AppImageInfo{
				Name: "mock",
				Tag:  "latest",
			},
			DeployTemplates: []v1alpha1.DeployTemplate{
				{
					Name:     "mock",
					Replicas: &replicas,
					Spec: v1alpha1.PodSpec{
						Containers: []v1alpha1.Container{
							{
								Name: "mock",
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("100Mi"),
										corev1.ResourceCPU:    resource.MustParse("1"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("10Mi"),
										corev1.ResourceCPU:    resource.MustParse("10m"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
