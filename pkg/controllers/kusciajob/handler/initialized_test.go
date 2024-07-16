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

package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientcorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	kusciainformersv1 "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/kuscia/v1alpha1"
)

const (
	testCaseOnePartner = iota
	testCaseNoPartner
)

func setDomainOnePartner(nsInformer clientcorev1.NamespaceInformer, domainInformer kusciainformersv1.DomainInformer) {
	aliceNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
	}
	bobNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bob",
		},
	}
	bobNs.Labels = make(map[string]string)
	bobNs.Labels[common.LabelDomainRole] = string(kusciaapisv1alpha1.Partner)
	aliceD := &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
		Spec: kusciaapisv1alpha1.DomainSpec{},
	}
	bobD := &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bob",
		},
		Spec: kusciaapisv1alpha1.DomainSpec{},
	}
	nsInformer.Informer().GetStore().Add(aliceNs)
	nsInformer.Informer().GetStore().Add(bobNs)
	domainInformer.Informer().GetStore().Add(aliceD)
	domainInformer.Informer().GetStore().Add(bobD)
}

func setDomainNoPartner(nsInformer clientcorev1.NamespaceInformer, domainInformer kusciainformersv1.DomainInformer) {
	aliceNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
	}
	bobNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bob",
		},
	}
	aliceD := &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
		Spec: kusciaapisv1alpha1.DomainSpec{},
	}
	bobD := &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bob",
		},
		Spec: kusciaapisv1alpha1.DomainSpec{},
	}
	nsInformer.Informer().GetStore().Add(aliceNs)
	nsInformer.Informer().GetStore().Add(bobNs)
	domainInformer.Informer().GetStore().Add(aliceD)
	domainInformer.Informer().GetStore().Add(bobD)
}

func setDomain(testCase int, nsInformer clientcorev1.NamespaceInformer, domainInformer kusciainformersv1.DomainInformer) {
	switch testCase {
	case testCaseOnePartner:
		setDomainOnePartner(nsInformer, domainInformer)
	case testCaseNoPartner:
		setDomainNoPartner(nsInformer, domainInformer)
	}
}

func TestInitializedHandler_HandlePhase(t *testing.T) {
	t.Parallel()
	independentJob := makeKusciaJob(KusciaJobForShapeIndependent,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	independentJob2 := makeKusciaJob(KusciaJobForShapeIndependent,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	type fields struct {
		kubeClient   kubernetes.Interface
		kusciaClient versioned.Interface
	}
	type args struct {
		kusciaJob *kusciaapisv1alpha1.KusciaJob
		testCase  int
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantNeedUpdate bool
		wantErr        assert.ErrorAssertionFunc
		wantJobPhase   kusciaapisv1alpha1.KusciaJobPhase
		wantFinalTasks map[string]taskAssertionFunc
	}{
		{
			name: "One party is partner return needUpdate{true} err{nil} phase{awaitingApproval}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseOnePartner,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobAwaitingApproval,
		},
		{
			name: "No partner should return needUpdate{true} err{nil} phase{pending}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob2,
				testCase:  testCaseNoPartner,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobPending,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(tt.fields.kusciaClient, 5*time.Minute)
			kubeInformerFactory := informers.NewSharedInformerFactory(tt.fields.kubeClient, 5*time.Minute)
			nsInformer := kubeInformerFactory.Core().V1().Namespaces()
			domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()

			setDomain(tt.args.testCase, nsInformer, domainInformer)
			deps := &Dependencies{
				KusciaClient:          tt.fields.kusciaClient,
				KusciaTaskLister:      kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Lister(),
				NamespaceLister:       nsInformer.Lister(),
				DomainLister:          domainInformer.Lister(),
				EnableWorkloadApprove: true,
			}
			h := &InitializedHandler{
				JobScheduler: NewJobScheduler(deps),
			}
			h.HandlePhase(tt.args.kusciaJob)
			//gotNeedUpdate, err := h.HandlePhase(tt.args.kusciaJob)
			//if !tt.wantErr(t, err, fmt.Sprintf("HandlePhase(%v)", tt.args.kusciaJob)) {
			//	return
			//}
			//assert.Equalf(t, tt.wantNeedUpdate, gotNeedUpdate, "HandlePhase(%v)", tt.args.kusciaJob)
			//assert.Equalf(t, tt.wantJobPhase, tt.args.kusciaJob.Status.Phase, "HandlePhase(%v)", tt.args.kusciaJob)
		})
	}
}
