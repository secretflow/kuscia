// Copyright 2024 Ant Group Co., Ltd.
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

package clusterdomainroute

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/controllers"
	kusciav1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
)

func TestController_syncServiceToken(t *testing.T) {
	domains := []*kusciav1alpha1.Domain{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "alice",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bob",
			},
			Spec: kusciav1alpha1.DomainSpec{
				Role: kusciav1alpha1.Partner,
				InterConnProtocols: []kusciav1alpha1.InterConnProtocolType{
					kusciav1alpha1.InterConnKuscia,
				},
			},
		},
	}

	cdrs := []*kusciav1alpha1.ClusterDomainRoute{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "alice-bob",
			},
			Spec: kusciav1alpha1.ClusterDomainRouteSpec{
				DomainRouteSpec: kusciav1alpha1.DomainRouteSpec{
					Source:      "alice",
					Destination: "bob",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bob-alice",
			},
			Spec: kusciav1alpha1.ClusterDomainRouteSpec{
				DomainRouteSpec: kusciav1alpha1.DomainRouteSpec{
					Source:      "bob",
					Destination: "alice",
				},
			},
		},
	}

	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset(domains[0], domains[1], cdrs[0], cdrs[1])

	curDir, err := os.Getwd()
	assert.NoError(t, err)
	rootDir := filepath.Dir(filepath.Dir(filepath.Dir(curDir)))
	ic := NewController(context.Background(), controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
		RootDir:      rootDir,
	})
	c := ic.(*controller)
	c.kusciaInformerFactory.Start(c.ctx.Done())
	cache.WaitForCacheSync(c.ctx.Done(), c.domainListerSynced)

	hasUpdate, err := c.syncServiceToken(c.ctx, cdrs[0])
	assert.NoError(t, err)
	assert.False(t, false)

	hasUpdate, err = c.syncServiceToken(c.ctx, cdrs[1])
	assert.ErrorContains(t, err, "serviceaccounts \"\" not found")
	assert.False(t, hasUpdate)

	kubeClient.Fake.PrependReactor("create", "serviceaccounts/token", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authenticationv1.TokenRequest{Status: authenticationv1.TokenRequestStatus{Token: "abc"}}, nil
	})
	hasUpdate, err = c.syncServiceToken(c.ctx, cdrs[1])
	assert.NoError(t, err)
	assert.True(t, hasUpdate)

	cdr, err := kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Get(c.ctx, "bob-alice", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(cdr.Spec.RequestHeadersToAdd))
	assert.Equal(t, "Bearer abc", cdr.Spec.RequestHeadersToAdd[common.AuthorizationHeaderName])

	hasUpdate, err = c.syncServiceToken(c.ctx, cdr)
	assert.NoError(t, err)
	assert.False(t, hasUpdate)
}
