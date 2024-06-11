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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestResourceFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{
			name: "obj is kuscia deployment",
			obj: &kusciaapisv1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "cross-domain",
				},
			},
			want: false,
		},
		{
			name: "obj is kuscia job",
			obj: &kusciaapisv1alpha1.KusciaJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "cross-domain",
				},
			},
			want: false,
		},
		{
			name: "obj is kuscia task",
			obj: &kusciaapisv1alpha1.KusciaTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "cross-domain",
				},
			},
			want: false,
		},
		{
			name: "obj is task resource",
			obj: &kusciaapisv1alpha1.TaskResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj is job summary",
			obj: &kusciaapisv1alpha1.KusciaJobSummary{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj is task summary",
			obj: &kusciaapisv1alpha1.KusciaTaskSummary{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj is deployment summary",
			obj: &kusciaapisv1alpha1.KusciaTaskSummary{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj is str",
			obj:  "str",
			want: false,
		},
	}

	c := &Controller{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.resourceFilter(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterJob(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		obj  metav1.Object
		want bool
	}{
		{
			name: "obj namespace is not cross-domain",
			obj: &kusciaapisv1alpha1.KusciaJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj match the condition",
			obj: &kusciaapisv1alpha1.KusciaJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: common.KusciaCrossDomain,
					Annotations: map[string]string{
						common.InitiatorAnnotationKey:            "alice",
						common.InterConnKusciaPartyAnnotationKey: "bob",
						common.InterConnSelfPartyAnnotationKey:   "alice",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterJob(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterDeployment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		obj  metav1.Object
		want bool
	}{
		{
			name: "obj doesn't match the condition",
			obj: &kusciaapisv1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj match the condition",
			obj: &kusciaapisv1alpha1.KusciaDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: common.KusciaCrossDomain,
					Annotations: map[string]string{
						common.InitiatorAnnotationKey:            "alice",
						common.InterConnKusciaPartyAnnotationKey: "bob",
						common.InterConnSelfPartyAnnotationKey:   "alice",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterDeployment(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterTask(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		obj  metav1.Object
		want bool
	}{
		{
			name: "obj namespace is not cross-domain",
			obj: &kusciaapisv1alpha1.KusciaTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj doesn't match the condition",
			obj: &kusciaapisv1alpha1.KusciaTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: common.KusciaCrossDomain,
					Annotations: map[string]string{
						common.InitiatorAnnotationKey:            "alice",
						common.InterConnKusciaPartyAnnotationKey: "bob",
						common.InterConnSelfPartyAnnotationKey:   "alice",
					},
				},
			},
			want: false,
		},
		{
			name: "obj match the condition",
			obj: &kusciaapisv1alpha1.KusciaJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: common.KusciaCrossDomain,
					Annotations: map[string]string{
						common.TaskAliasAnnotationKey:            "test",
						common.JobIDAnnotationKey:                "job-1",
						common.InitiatorAnnotationKey:            "alice",
						common.InterConnKusciaPartyAnnotationKey: "bob",
						common.InterConnSelfPartyAnnotationKey:   "alice",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterTask(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterTaskResource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		obj  metav1.Object
		want bool
	}{
		{
			name: "obj labels is empty",
			obj: &kusciaapisv1alpha1.TaskResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
				},
			},
			want: false,
		},
		{
			name: "obj interconn protocol type is not kuscia",
			obj: &kusciaapisv1alpha1.TaskResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
					Labels: map[string]string{
						common.LabelInterConnProtocolType: "bfia",
					},
				},
			},
			want: false,
		},
		{
			name: "obj annotations is empty",
			obj: &kusciaapisv1alpha1.TaskResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
					Labels: map[string]string{
						common.LabelInterConnProtocolType: "kuscia",
					},
				},
			},
			want: false,
		},
		{
			name: "obj labels and annotations is match",
			obj: &kusciaapisv1alpha1.TaskResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "alice",
					Labels: map[string]string{
						common.LabelInterConnProtocolType: "kuscia",
					},
					Annotations: map[string]string{
						common.InitiatorAnnotationKey: "alice",
						common.TaskIDAnnotationKey:    "test",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterTaskResource(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterResourceSummary(t *testing.T) {
	t.Parallel()
	now := metav1.Now()
	tests := []struct {
		name string
		obj  metav1.Object
		want bool
	}{
		{
			name: "obj deletion timestamp is not empty",
			obj: &kusciaapisv1alpha1.KusciaJobSummary{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test",
					Namespace:         "alice",
					DeletionTimestamp: &now,
				},
			},
			want: false,
		},
		{
			name: "obj match the condition",
			obj: &kusciaapisv1alpha1.KusciaJobSummary{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: common.KusciaCrossDomain,
					Annotations: map[string]string{
						common.InitiatorAnnotationKey:            "alice",
						common.InterConnKusciaPartyAnnotationKey: "bob",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterResourceSummary(tt.obj)
			assert.Equal(t, tt.want, got)
		})
	}
}
