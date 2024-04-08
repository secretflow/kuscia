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
package hostresources

import (
	"context"
	"fmt"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runJobWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runJobWorker() {
	for queue.HandleQueueItem(context.Background(), c.jobQueueName, c.jobQueue, c.syncJobHandler, maxRetries) {
	}
}

// handleAddedJob is used to handle added job.
func (c *hostResourcesController) handleAddedJob(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.jobQueue)
}

// handleUpdatedJob is used to handle updated job.
func (c *hostResourcesController) handleUpdatedJob(oldObj, newObj interface{}) {
	oldJob, ok := oldObj.(*kusciaapisv1alpha1.KusciaJob)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJob", oldObj)
		return
	}

	newJob, ok := newObj.(*kusciaapisv1alpha1.KusciaJob)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaJob", newObj)
		return
	}

	if oldJob.ResourceVersion == newJob.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newJob, c.jobQueue)
}

// handleDeletedJob is used to handle deleted job.
func (c *hostResourcesController) handleDeletedJob(obj interface{}) {
	job, ok := obj.(*kusciaapisv1alpha1.KusciaJob)
	if !ok {
		return
	}

	c.jobQueue.Add(fmt.Sprintf("%v%v/%v",
		ikcommon.DeleteEventKeyPrefix, common.KusciaCrossDomain, job.Name))
}

// TODO: Abstract into common interface
// syncJobHandler is used to sync job between host and member cluster.
func (c *hostResourcesController) syncJobHandler(ctx context.Context, key string) error {
	key, deleteEvent := ikcommon.IsOriginalResourceDeleteEvent(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split job key %v, %v, skip processing it", key, err)
		return nil
	}

	if deleteEvent {
		return c.deleteJob(ctx, namespace, name)
	}

	hJob, err := c.hostJobLister.KusciaJobs(namespace).Get(name)
	if err != nil {
		// Job is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Job %v may be deleted under host %v cluster, skip processing it", key, c.host)
			return nil
		}
		return err
	}
	return c.createJob(ctx, hJob)
}

func (c *hostResourcesController) deleteJob(ctx context.Context, namespace, name string) error {
	kj, err := c.memberJobLister.KusciaJobs(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if kj.Annotations == nil || kj.Annotations[common.InitiatorMasterDomainAnnotationKey] != c.host {
		return nil
	}

	nlog.Infof("Host %v job %v is deleted, so clean up member job %v", c.host, name, fmt.Sprintf("%v/%v", namespace, name))
	err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *hostResourcesController) createJob(ctx context.Context, hostJob *kusciaapisv1alpha1.KusciaJob) error {
	job, err := c.memberJobLister.KusciaJobs(common.KusciaCrossDomain).Get(hostJob.Name)
	if job != nil {
		nlog.Infof("Job %v already exists under member cluster, skip creating it", hostJob.Name)
		return nil
	}

	initiator := ikcommon.GetObjectAnnotation(hostJob, common.InitiatorAnnotationKey)
	initiatorMasterDomainID, err := utilsres.GetMasterDomain(c.memberDomainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}

	kj := &kusciaapisv1alpha1.KusciaJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostJob.Name,
			Namespace: common.KusciaCrossDomain,
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:               initiator,
				common.SelfClusterAsInitiatorAnnotationKey:  common.False,
				common.InitiatorMasterDomainAnnotationKey:   initiatorMasterDomainID,
				common.InterConnKusciaPartyAnnotationKey:    ikcommon.GetObjectAnnotation(hostJob, common.InterConnKusciaPartyAnnotationKey),
				common.KusciaPartyMasterDomainAnnotationKey: hostJob.Namespace,
			},
		},
		Spec: *hostJob.Spec.DeepCopy(),
	}

	for k, v := range hostJob.Labels {
		if strings.Contains(k, common.JobCustomFieldsLabelPrefix) {
			if kj.Labels == nil {
				kj.Labels = make(map[string]string)
			}
			kj.Labels[k] = v
		}
	}

	_, err = c.memberKusciaClient.KusciaV1alpha1().KusciaJobs(kj.Namespace).Create(ctx, kj, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
