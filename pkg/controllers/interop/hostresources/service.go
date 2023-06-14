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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	utilscommon "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

// runServiceWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runServiceWorker() {
	ctx := context.Background()
	queueName := generateQueueName(hostServicesQueueName, c.host, c.member)
	for queue.HandleQueueItem(ctx, queueName, c.servicesQueue, c.syncServiceHandler, maxRetries) {
	}
}

// handleAddedOrDeletedService is used to handle added or deleted service.
func (c *hostResourcesController) handleAddedOrDeletedService(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.servicesQueue)
}

// handleUpdatedService is used to handle updated service.
func (c *hostResourcesController) handleUpdatedService(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*corev1.Service)
	if !ok {
		nlog.Errorf("Object %#v is not a Service", oldObj)
		return
	}

	newPod, ok := newObj.(*corev1.Service)
	if !ok {
		nlog.Errorf("Object %#v is not a Service", newObj)
		return
	}

	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newPod, c.servicesQueue)
}

// Todo: Abstract into common interface
// syncServiceHandler is used to sync service between host and member cluster.
func (c *hostResourcesController) syncServiceHandler(ctx context.Context, key string) error {
	namespace, name, err := getNamespaceAndNameFromKey(key, c.member)
	if err != nil {
		nlog.Error(err)
		return nil
	}

	hService, err := c.hostServiceLister.Services(namespace).Get(name)
	if err != nil {
		// service is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Service %v may be deleted under host %v cluster, so delete the service under member cluster", key, c.host)
			if err = c.memberKubeClient.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			return nil
		}
		return err
	}

	hServiceCopy := hService.DeepCopy()
	hrv := hServiceCopy.ResourceVersion

	hExtractedService := utilscommon.ExtractService(hServiceCopy)
	hExtractedService.Labels[common.LabelResourceVersionUnderHostCluster] = hrv

	mService, err := c.memberServiceLister.Services(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Failed to get service %v under member cluster, create it...", key)
			if _, err = c.memberKubeClient.CoreV1().Services(namespace).Create(ctx, hExtractedService, metav1.CreateOptions{}); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					return nil
				}
				return fmt.Errorf("create service %v from host cluster failed, %v", key, err.Error())
			}
			return nil
		}
		return err
	}

	mServiceCopy := mService.DeepCopy()
	mrv := mServiceCopy.Labels[common.LabelResourceVersionUnderHostCluster]
	if !utilscommon.CompareResourceVersion(hrv, mrv) {
		nlog.Infof("Service %v resource version %v under host cluster is not greater than the value %v under member cluster, skip this event...", key, hrv, mrv)
		return nil
	}

	mExtractedService := utilscommon.ExtractService(mServiceCopy)
	if err = utilscommon.PatchService(ctx, c.memberKubeClient, mExtractedService, hExtractedService); err != nil {
		return fmt.Errorf("patch member service %v failed, %v", key, err.Error())
	}
	return nil
}
