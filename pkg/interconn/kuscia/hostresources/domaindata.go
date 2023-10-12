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

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// runPodWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runDomainDataWorker() {
	ctx := context.Background()
	queueName := generateQueueName(hostDomainDataQueueName, c.host, c.member)
	for queue.HandleQueueItem(ctx, queueName, c.domainDataQueue, c.syncDomainDataHandler, maxRetries) {
	}
}

// handleAddedOrDeletedDomainData is used to handle added or deleted pod.
func (c *hostResourcesController) handleAddedOrDeletedDomainData(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.domainDataQueue)
}

// handleUpdatedDomainData is used to handle updated pod.
func (c *hostResourcesController) handleUpdatedDomainData(oldObj, newObj interface{}) {
	oldDdg, ok := oldObj.(*kusciaapisv1alpha1.DomainData)
	if !ok {
		nlog.Errorf("Object %#v is not a DomainData", oldObj)
		return
	}

	newDdg, ok := newObj.(*kusciaapisv1alpha1.DomainData)
	if !ok {
		nlog.Errorf("Object %#v is not a DomainData", newObj)
		return
	}

	if oldDdg.ResourceVersion == newDdg.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newDdg, c.domainDataQueue)
}

// Todo: Abstract into common interface
// syncDomainDataHandler is used to sync DomainData between host and member cluster.
func (c *hostResourcesController) syncDomainDataHandler(ctx context.Context, key string) error {
	namespace, name, err := getNamespaceAndNameFromKey(key, c.member)
	if err != nil {
		nlog.Error(err)
		return nil
	}

	hDdg, err := c.hostDomainDataLister.DomainDatas(namespace).Get(name)
	if err != nil {
		// DomainDataGrant is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("DomainData %v may be deleted under host %v cluster, so delete the DomainData under member cluster", key, c.host)
			if err = c.memberKusciaClient.KusciaV1alpha1().DomainDatas(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			return nil
		}
		return err
	}

	hDomainDataCopy := hDdg.DeepCopy()
	hrv := hDomainDataCopy.ResourceVersion

	hExtractedDomainDataSpec := utilsres.ExtractDomainDataSpec(hDomainDataCopy)
	hExtractedDomainDataSpec.Labels[common.LabelResourceVersionUnderHostCluster] = hrv

	mDomainDataGrant, err := c.memberDomainDataLister.DomainDatas(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Failed to get DomainData %v under member cluster, create it...", key)
			if _, err = c.memberKusciaClient.KusciaV1alpha1().DomainDatas(namespace).Create(ctx, hExtractedDomainDataSpec, metav1.CreateOptions{}); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					return nil
				}
				return fmt.Errorf("create DomainData %v from host cluster failed, %v", key, err.Error())
			}
			return nil
		}
		return err
	}

	mDomainDataCopy := mDomainDataGrant.DeepCopy()
	mrv := mDomainDataCopy.Labels[common.LabelResourceVersionUnderHostCluster]
	if !utilsres.CompareResourceVersion(hrv, mrv) {
		nlog.Infof("DomainData %v resource version %v under host cluster is not greater than the value %v under member cluster, skip this event...", key, hrv, mrv)
		return nil
	}

	mExtractedDomainDataSpec := utilsres.ExtractDomainDataSpec(mDomainDataCopy)
	if err = utilsres.PatchDomainData(ctx, c.memberKusciaClient, mExtractedDomainDataSpec, hExtractedDomainDataSpec); err != nil {
		return fmt.Errorf("patch member DomainData %v failed, %v", key, err.Error())
	}
	return nil
}
