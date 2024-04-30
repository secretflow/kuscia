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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

func TestIsOriginalResourceDeleteEvent(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "key is deleted event",
			key:  DeleteEventKeyPrefix + "task-1",
			want: true,
		},
		{
			name: "key is not deleted event",
			key:  "task-1",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := IsOriginalResourceDeleteEvent(tt.key)
			if got != tt.want {
				assert.Equal(t, got, tt.want)
			}
		})
	}
}

func TestSelfClusterIsInitiator(t *testing.T) {
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	alice := &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{Name: "alice"},
	}
	domainInformer.Informer().GetStore().Add(alice)
	domainLister := domainInformer.Lister()

	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "task-1",
			Namespace:   common.KusciaCrossDomain,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	// labels is empty, should return false
	got, _ := SelfClusterIsInitiator(domainLister, kt)
	assert.False(t, got)

	// label LabelSelfClusterAsInitiator is false, should return false
	kt.Annotations[common.InitiatorAnnotationKey] = "alice"
	kt.Annotations[common.SelfClusterAsInitiatorAnnotationKey] = "false"
	got, _ = SelfClusterIsInitiator(domainLister, kt)
	assert.False(t, got)

	// label LabelSelfClusterAsInitiator is true, should return false
	kt.Annotations[common.SelfClusterAsInitiatorAnnotationKey] = "true"
	got, _ = SelfClusterIsInitiator(domainLister, kt)
	assert.True(t, got)

	// domain not found, should return false
	delete(kt.Annotations, common.SelfClusterAsInitiatorAnnotationKey)
	kt.Annotations[common.InitiatorAnnotationKey] = "bob"
	got, _ = SelfClusterIsInitiator(domainLister, kt)
	assert.False(t, got)

	// domain is initiator, should return true
	delete(kt.Annotations, common.SelfClusterAsInitiatorAnnotationKey)
	kt.Annotations[common.InitiatorAnnotationKey] = "alice"
	got, _ = SelfClusterIsInitiator(domainLister, kt)
	assert.True(t, got)

	// domain is partner, should return true
	delete(kt.Annotations, common.SelfClusterAsInitiatorAnnotationKey)
	kt.Annotations[common.InitiatorAnnotationKey] = "alice"
	alice.Spec.Role = kusciaapisv1alpha1.Partner
	got, _ = SelfClusterIsInitiator(domainLister, kt)
	assert.False(t, got)
}

func TestGetSelfClusterPartyDomainIDs(t *testing.T) {
	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "task-1",
			Namespace:   common.KusciaCrossDomain,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	// labels is empty, should return nil
	got := GetSelfClusterPartyDomainIDs(kt)
	assert.Nil(t, got)

	// labels is not empty, should return not nil
	kt.Annotations[common.InterConnSelfPartyAnnotationKey] = "alice"
	got = GetSelfClusterPartyDomainIDs(kt)
	assert.NotNil(t, got)
}

func TestGetInterConnKusciaPartyDomainIDs(t *testing.T) {
	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "task-1",
			Namespace:   common.KusciaCrossDomain,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	// labels is empty, should return nil
	got := GetInterConnKusciaPartyDomainIDs(kt)
	assert.Nil(t, got)

	// labels is not empty, should return not nil
	kt.Annotations[common.InterConnKusciaPartyAnnotationKey] = "alice"
	got = GetInterConnKusciaPartyDomainIDs(kt)
	assert.NotNil(t, got)
}

func TestGetPartyMasterDomains(t *testing.T) {
	kusciaFakeClient := kusciafake.NewSimpleClientset()
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaFakeClient, 0)
	domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
	alice := &kusciaapisv1alpha1.Domain{
		ObjectMeta: metav1.ObjectMeta{Name: "alice"},
	}
	domainInformer.Informer().GetStore().Add(alice)
	domainLister := domainInformer.Lister()

	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "task-1",
			Namespace:   common.KusciaCrossDomain,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	// partyDomainIDs is empty, should return nil
	got, _ := GetPartyMasterDomains(domainLister, kt)
	assert.Nil(t, got)

	// partyDomainIDs is not empty but doesn't include partner, should return empty
	kt.Annotations[common.InterConnKusciaPartyAnnotationKey] = "alice"
	got, _ = GetPartyMasterDomains(domainLister, kt)
	assert.Equal(t, 0, len(got))

	// partyDomainIDs is not empty and include partner,should return not empty
	kt.Annotations[common.InterConnKusciaPartyAnnotationKey] = "alice"
	alice.Spec.Role = kusciaapisv1alpha1.Partner
	got, _ = GetPartyMasterDomains(domainLister, kt)
	assert.Equal(t, 1, len(got))
}

func TestGetCurrentTime(t *testing.T) {
	got := GetCurrentTime()
	assert.NotNil(t, got)
}

func TestGetObjectNamespaceName(t *testing.T) {
	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-1",
			Namespace: common.KusciaCrossDomain,
			Labels:    map[string]string{},
		},
	}
	got := GetObjectNamespaceName(kt)
	assert.Equal(t, common.KusciaCrossDomain+"/task-1", got)
}

func TestGetObjectLabel(t *testing.T) {
	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "task-1",
			Namespace: common.KusciaCrossDomain,
			Labels:    map[string]string{},
		},
	}
	// labels is empty, should return empty string
	got := GetObjectLabel(kt, common.LabelJobStage)
	assert.Equal(t, "", got)

	// labels is not empty, should return not empty string
	kt.Labels[common.LabelJobStage] = "start"
	got = GetObjectLabel(kt, common.LabelJobStage)
	assert.Equal(t, "start", got)
}

func TestGetObjectAnnotation(t *testing.T) {
	kt := &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "task-1",
			Namespace:   common.KusciaCrossDomain,
			Annotations: map[string]string{},
		},
	}
	// labels is empty, should return empty string
	got := GetObjectAnnotation(kt, common.JobIDAnnotationKey)
	assert.Equal(t, "", got)

	// labels is not empty, should return not empty string
	kt.Annotations[common.JobIDAnnotationKey] = "job-1"
	got = GetObjectAnnotation(kt, common.JobIDAnnotationKey)
	assert.Equal(t, "job-1", got)
}

func TestGetJobStage(t *testing.T) {
	tests := []struct {
		name  string
		stage string
		want  kusciaapisv1alpha1.JobStage
	}{
		{
			name:  "stage is create",
			stage: string(kusciaapisv1alpha1.JobCreateStage),
			want:  kusciaapisv1alpha1.JobCreateStage,
		},
		{
			name:  "stage is start",
			stage: string(kusciaapisv1alpha1.JobStartStage),
			want:  kusciaapisv1alpha1.JobStartStage,
		},
		{
			name:  "stage is stop",
			stage: string(kusciaapisv1alpha1.JobStopStage),
			want:  kusciaapisv1alpha1.JobStopStage,
		},
		{
			name:  "stage is cancel",
			stage: string(kusciaapisv1alpha1.JobCancelStage),
			want:  kusciaapisv1alpha1.JobCancelStage,
		},
		{
			name:  "stage is restart",
			stage: string(kusciaapisv1alpha1.JobRestartStage),
			want:  kusciaapisv1alpha1.JobRestartStage,
		},
		{
			name:  "stage is unknown",
			stage: "unknown",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetJobStage(tt.stage)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUpdateJobStage(t *testing.T) {
	kj := &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-1",
			Namespace: common.KusciaCrossDomain,
			Labels:    map[string]string{},
		},
	}

	kjs := &kusciaapisv1alpha1.KusciaJobSummary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-1",
			Namespace: "alice",
			Labels:    map[string]string{},
		},
	}

	// phase is cancelled: return false
	kjs.Spec.Stage = kusciaapisv1alpha1.JobCancelStage
	kj.Status.Phase = kusciaapisv1alpha1.KusciaJobCancelled
	got := UpdateJobStage(kj, kjs)
	assert.Equal(t, false, got)
	// no version: return false
	kjs.Spec.Stage = kusciaapisv1alpha1.JobStartStage
	kj.Status.Phase = kusciaapisv1alpha1.KusciaJobRunning
	got = UpdateJobStage(kj, kjs)
	assert.Equal(t, false, got)
	// job summary update stage
	kjs.Spec.Stage = kusciaapisv1alpha1.JobStartStage
	kjs.Labels[common.LabelJobStageVersion] = "1"
	got = UpdateJobStage(kj, kjs)
	assert.Equal(t, true, got)
}
