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

package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciascheme "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/scheme"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func TestCancelledHandler_HandlePhase(t *testing.T) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(nlog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{Interface: kubefake.NewSimpleClientset().CoreV1().Events("default")})
	assert.NoError(t, kusciascheme.AddToScheme(scheme.Scheme))
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "kuscia-job-controller"})
	kubeFakeClient := kubefake.NewSimpleClientset()
	kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeFakeClient, 0)

	nsInformer := kubeInformersFactory.Core().V1().Namespaces()
	type fields struct {
		recorder record.EventRecorder
	}
	type args struct {
		kusciaJob *kusciaapisv1alpha1.KusciaJob
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "BestEffort mode task{a,[a->b],[a->c],[c->d]} maxParallelism{1} and succeeded{a,b,c,d} should return needUpdate{true} err{nil}",
			fields: fields{
				recorder: recorder,
			},
			args: args{
				kusciaJob: makeKusciaJob(KusciaJobForShapeTree,
					kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil),
			},
			want:    false,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kusciaClient := kusciafake.NewSimpleClientset()
			kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 5*time.Minute)
			deps := &Dependencies{
				KusciaClient:          kusciaClient,
				KusciaTaskLister:      kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Lister(),
				NamespaceLister:       nsInformer.Lister(),
				DomainLister:          kusciaInformerFactory.Kuscia().V1alpha1().Domains().Lister(),
				EnableWorkloadApprove: true,
			}
			s := NewCancelledHandler(deps)
			got, err := s.HandlePhase(tt.args.kusciaJob)
			if !tt.wantErr(t, err, fmt.Sprintf("HandlePhase(%v)", tt.args.kusciaJob)) {
				return
			}
			assert.Equalf(t, tt.want, got, "HandlePhase(%v)", tt.args.kusciaJob)
		})
	}
}
