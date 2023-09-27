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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

const (
	serviceHasSynced = "true"
)

// runServiceWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runServiceWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, serviceQueueName, c.serviceQueue, c.syncServiceHandler, maxRetries) {
	}
}

// handleAddedService is used to handle added service.
func (c *Controller) handleAddedService(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.serviceQueue)
}

// handleUpdatedService is used to handle updated service.
func (c *Controller) handleUpdatedService(oldObj, newObj interface{}) {
	oldSvc, ok := oldObj.(*corev1.Service)
	if !ok {
		nlog.Errorf("Object %#v is not a Service", oldObj)
		return
	}

	newSvc, ok := newObj.(*corev1.Service)
	if !ok {
		nlog.Errorf("Object %#v is not a Service", newObj)
		return
	}

	if oldSvc.ResourceVersion == newSvc.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newSvc, c.serviceQueue)
}

// handleDeletedService is used to handle deleted service.
func (c *Controller) handleDeletedService(obj interface{}) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return
	}

	host := svc.Labels[common.LabelInitiator]
	hra := c.hostResourceManager.GetHostResourceAccessor(host, svc.Namespace)
	if hra == nil {
		nlog.Warnf("Host resource accessor for service %v is empty of host/member %v/%v, skip this event...", svc.Name, host, svc.Namespace)
		return
	}

	hra.EnqueueService(fmt.Sprintf("%s/%s", svc.Namespace, svc.Name))
}

// Todo: Abstract into common interface
// syncServiceHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncServiceHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Split service key %v failed, %v", key, err.Error())
		return nil
	}

	mSvc, err := c.serviceLister.Services(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Warnf("Can't get service %v under member cluster, skip this event...", key)
			return nil
		}
		return err
	}

	host := mSvc.Labels[common.LabelInitiator]
	hra := c.hostResourceManager.GetHostResourceAccessor(host, namespace)
	if hra == nil {
		nlog.Warnf("Host resource accessor for service %v is empty of host/member %v/%v, skip this event", key, host, namespace)
		return nil
	}

	if !hra.HasSynced() {
		return fmt.Errorf("host %v resource accessor has not synced, retry", host)
	}

	hSvc, err := hra.HostServiceLister().Services(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Warnf("Service %v may be deleted under host %v cluster, skip this event...", key, host)
			return nil
		}
		nlog.Errorf("Get service %v under host %v cluster failed, %v", key, host, err.Error())
		return err
	}

	rvInLabel := mSvc.Labels[common.LabelResourceVersionUnderHostCluster]
	if utilsres.CompareResourceVersion(hSvc.ResourceVersion, rvInLabel) {
		return fmt.Errorf("service %v label resource version %v in member cluster is less than the value %v under host cluster, retry", key, rvInLabel, hSvc.ResourceVersion)
	}

	hSvcCopy := hSvc.DeepCopy()

	needUpdate := false
	extractedHSvcLabels := utilsres.ExtractServiceLabels(hSvcCopy)
	extractedHSvcLabelsCopy := extractedHSvcLabels.DeepCopy()
	if extractedHSvcLabels.Labels == nil || (extractedHSvcLabels.Labels != nil && extractedHSvcLabels.Labels[common.LabelHasSynced] != serviceHasSynced) {
		needUpdate = true
		if extractedHSvcLabels.Labels == nil {
			extractedHSvcLabels.Labels = make(map[string]string)
		}

		extractedHSvcLabelsCopy = extractedHSvcLabels.DeepCopy()
		extractedHSvcLabelsCopy.Labels[common.LabelHasSynced] = serviceHasSynced

	}

	if mSvc.Annotations != nil && mSvc.Annotations[common.ReadyTimeAnnotationKey] != " " {
		if hSvcCopy.Annotations == nil || (hSvcCopy.Annotations[common.ReadyTimeAnnotationKey] != mSvc.Annotations[common.ReadyTimeAnnotationKey]) {
			needUpdate = true
			extractedHSvcLabelsCopy.Annotations = map[string]string{
				common.ReadyTimeAnnotationKey: mSvc.Annotations[common.ReadyTimeAnnotationKey],
			}
		}
	}

	if needUpdate {
		nlog.Infof("Patch member service %v/%v labels and annotations to host %v cluster", extractedHSvcLabels.Namespace, extractedHSvcLabels.Name, host)
		if err = utilsres.PatchService(ctx, hra.HostKubeClient(), extractedHSvcLabels, extractedHSvcLabelsCopy); err != nil {
			return fmt.Errorf("patch service %v label %v under host cluster failed, %v", key, common.LabelHasSynced, err.Error())
		}
	}

	return nil
}
