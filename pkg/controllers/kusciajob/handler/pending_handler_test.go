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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestPendingHandler_HandlePhase(t *testing.T) {
	independentJob := makeKusciaJob(KusciaJobForShapeIndependent,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)
	linearJob := makeKusciaJob(KusciaJobForShapeTree,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)

	type fields struct {
		kubeClient   kubernetes.Interface
		kusciaClient versioned.Interface
	}
	type args struct {
		kusciaJob *kusciaapisv1alpha1.KusciaJob
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
			name: "BestEffort mode task{a,b,c,d} maxParallelism{2} maxParallelism=2 should return needUpdate{true} err{nil} phase{}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   "",
		},
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} maxParallelism{2} should return needUpdate{true} err{nil} phase{}",
			fields: fields{
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: linearJob,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(tt.fields.kusciaClient, 5*time.Minute)
			kubeInformerFactory := informers.NewSharedInformerFactory(tt.fields.kubeClient, 5*time.Minute)
			nsInformer := kubeInformerFactory.Core().V1().Namespaces()
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
			nsInformer.Informer().GetStore().Add(aliceNs)
			nsInformer.Informer().GetStore().Add(bobNs)

			h := &PendingHandler{
				jobScheduler: NewJobScheduler(tt.fields.kusciaClient, nsInformer.Lister(), kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Lister()),
			}
			stopCh := make(<-chan struct{})
			go kusciaInformerFactory.Start(stopCh)
			cache.WaitForCacheSync(wait.NeverStop, kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Informer().HasSynced)

			gotNeedUpdate, err := h.HandlePhase(tt.args.kusciaJob)
			if !tt.wantErr(t, err, fmt.Sprintf("HandlePhase(%v)", tt.args.kusciaJob)) {
				return
			}
			assert.Equalf(t, tt.wantNeedUpdate, gotNeedUpdate, "HandlePhase(%v)", tt.args.kusciaJob)
			assert.Equalf(t, tt.wantJobPhase, tt.args.kusciaJob.Status.Phase, "HandlePhase(%v)", tt.args.kusciaJob)
		})
	}
}
