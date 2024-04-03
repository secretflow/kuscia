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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
)

const (
	DeleteEventKeyPrefix = "delete_event:"
)

func IsOriginalResourceDeleteEvent(key string) (string, bool) {
	deleted := false
	if strings.HasPrefix(key, DeleteEventKeyPrefix) {
		deleted = true
		key = strings.TrimPrefix(key, DeleteEventKeyPrefix)
	}
	return key, deleted
}

func SelfClusterIsInitiator(domainLister kuscialistersv1alpha1.DomainLister, obj metav1.Object) (bool, error) {
	annotations := obj.GetAnnotations()
	if len(annotations) == 0 || annotations[common.InitiatorAnnotationKey] == "" {
		return false, fmt.Errorf("annotation %v value is empty", common.InitiatorAnnotationKey)
	}

	switch annotations[common.SelfClusterAsInitiatorAnnotationKey] {
	case common.False:
		return false, nil
	case common.True:
		return true, nil
	default:
	}

	domain, err := domainLister.Get(annotations[common.InitiatorAnnotationKey])
	if err != nil {
		return false, fmt.Errorf("get domain %v fail, %v", obj.GetNamespace(), err)
	}

	if domain.Spec.Role != v1alpha1.Partner {
		return true, nil
	}
	return false, nil
}

func GetSelfClusterPartyDomainIDs(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if len(annotations) == 0 || annotations[common.InterConnSelfPartyAnnotationKey] == "" {
		return nil
	}

	return strings.Split(annotations[common.InterConnSelfPartyAnnotationKey], "_")
}

func GetInterConnKusciaPartyDomainIDs(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if len(annotations) == 0 || annotations[common.InterConnKusciaPartyAnnotationKey] == "" {
		return nil
	}

	return strings.Split(annotations[common.InterConnKusciaPartyAnnotationKey], "_")
}

func GetPartyMasterDomains(domainLister kuscialistersv1alpha1.DomainLister, obj metav1.Object) (map[string][]string, error) {
	partyDomainIDs := GetInterConnKusciaPartyDomainIDs(obj)
	if partyDomainIDs == nil {
		return nil, fmt.Errorf("annotation %v value is empty", common.InterConnKusciaPartyAnnotationKey)
	}

	masterDomains := make(map[string][]string)
	for _, domainID := range partyDomainIDs {
		domain, err := domainLister.Get(domainID)
		if err != nil {
			return nil, fmt.Errorf("get domain %v fail, %v", domainID, err)
		}

		if domain.Spec.Role == v1alpha1.Partner {
			if domain.Spec.MasterDomain != "" {
				masterDomains[domain.Spec.MasterDomain] = append(masterDomains[domain.Spec.MasterDomain], domainID)
			} else {
				masterDomains[domainID] = append(masterDomains[domainID], domainID)
			}
		}
	}

	return masterDomains, nil
}

func GetCurrentTime() *metav1.Time {
	now := metav1.Now()
	return &now
}

func GetObjectNamespaceName(obj metav1.Object) string {
	return fmt.Sprintf("%v/%v", obj.GetNamespace(), obj.GetName())
}

func GetObjectLabel(obj metav1.Object, key string) string {
	labels := obj.GetLabels()
	if len(labels) == 0 {
		return ""
	}
	return labels[key]
}

func GetObjectAnnotation(obj metav1.Object, key string) string {
	annotations := obj.GetAnnotations()
	if len(annotations) == 0 {
		return ""
	}
	return annotations[key]
}

func GetJobStage(stage string) v1alpha1.JobStage {
	switch stage {
	case string(v1alpha1.JobCreateStage):
		return v1alpha1.JobCreateStage
	case string(v1alpha1.JobStartStage):
		return v1alpha1.JobStartStage
	case string(v1alpha1.JobStopStage):
		return v1alpha1.JobStopStage
	case string(v1alpha1.JobCancelStage):
		return v1alpha1.JobCancelStage
	case string(v1alpha1.JobRestartStage):
		return v1alpha1.JobRestartStage
	case string(v1alpha1.JobSuspendStage):
		return v1alpha1.JobSuspendStage
	default:
		return ""
	}
}

func UpdateJobStage(job *v1alpha1.KusciaJob, jobSummary *v1alpha1.KusciaJobSummary) bool {
	if job.Status.Phase == v1alpha1.KusciaJobCancelled {
		return false
	}
	jobStageVersion := GetObjectLabel(job, common.LabelJobStageVersion)
	jobSummaryStageVersion := GetObjectLabel(jobSummary, common.LabelJobStageVersion)
	if jobSummaryStageVersion <= jobStageVersion {
		return false
	}

	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}
	job.Labels[common.LabelJobStageVersion] = jobSummaryStageVersion
	job.Labels[common.LabelJobStage] = string(jobSummary.Spec.Stage)
	job.Labels[common.LabelJobStageTrigger] = jobSummary.Spec.StageTrigger
	return true
}
