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

//nolint:dupl
package hostresources

import (
	"context"
	"fmt"

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

// runDomainDataWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runDomainDataWorker() {
	for queue.HandleQueueItem(context.Background(), c.domainDataQueueName, c.domainDataQueue, c.syncDomainDataHandler, maxRetries) {
	}
}

// handleAddedDomainData is used to handle added domain data.
func (c *hostResourcesController) handleAddedDomainData(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.domainDataQueue)
}

// handleUpdatedDomainData is used to handle updated domain data.
func (c *hostResourcesController) handleUpdatedDomainData(oldObj, newObj interface{}) {
	oldDd, ok := oldObj.(*kusciaapisv1alpha1.DomainData)
	if !ok {
		nlog.Errorf("Object %#v is not a DomainData", oldObj)
		return
	}

	newDd, ok := newObj.(*kusciaapisv1alpha1.DomainData)
	if !ok {
		nlog.Errorf("Object %#v is not a DomainData", newObj)
		return
	}

	if oldDd.ResourceVersion == newDd.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newDd, c.domainDataQueue)
}

// handleDeletedDomainData is used to handle deleted domain data.
func (c *hostResourcesController) handleDeletedDomainData(obj interface{}) {
	domainData, ok := obj.(*kusciaapisv1alpha1.DomainData)
	if !ok {
		return
	}

	partyDomainIDs := ikcommon.GetInterConnKusciaPartyDomainIDs(domainData)
	if len(partyDomainIDs) == 0 {
		nlog.Warnf("Member domain id is empty from deleted domainData %v, skip processing it",
			ikcommon.GetObjectNamespaceName(domainData))
		return
	}

	for _, domainID := range partyDomainIDs {
		c.domainDataQueue.Add(fmt.Sprintf("%v%v/%v",
			ikcommon.DeleteEventKeyPrefix, domainID, domainData.Name))
	}
}

// TODO: Abstract into common interface
// syncDomainDataHandler is used to sync DomainData between host and member cluster.
func (c *hostResourcesController) syncDomainDataHandler(ctx context.Context, key string) error {
	key, deleteEvent := ikcommon.IsOriginalResourceDeleteEvent(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split domainData key %v, %v, skip processing it", key, err)
		return nil
	}

	if deleteEvent {
		return c.deleteDomainData(ctx, namespace, name)
	}

	hDomainData, err := c.hostDomainDataLister.DomainDatas(namespace).Get(name)
	if err != nil {
		// DomainData is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Host %v domainData %v not found, delete member domainData", c.host, key)
			// Delete member domainData
			return c.deleteDomainData(ctx, namespace, name)
		}
		return err
	}

	partyDomainIDs := ikcommon.GetInterConnKusciaPartyDomainIDs(hDomainData)
	if partyDomainIDs == nil {
		nlog.Warnf("Failed to get party domain ids from host %v domainData %v, skip processing it",
			c.host, ikcommon.GetObjectNamespaceName(hDomainData))
		return nil
	}

	hDomainDataCopy := hDomainData.DeepCopy()
	for _, domainID := range partyDomainIDs {
		mDomainData, err := c.memberDomainDataLister.DomainDatas(domainID).Get(name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = c.createDomainData(ctx, hDomainDataCopy, domainID)
			}
			return err
		}

		return c.updateDomainData(ctx, hDomainDataCopy, mDomainData.DeepCopy())
	}
	return nil
}

func (c *hostResourcesController) deleteDomainData(ctx context.Context, namespace, name string) error {
	dd, err := c.memberDomainDataLister.DomainDatas(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if dd.Annotations == nil || dd.Annotations[common.InitiatorMasterDomainAnnotationKey] != c.host {
		return nil
	}

	nlog.Infof("Host %v domainData %v is deleted, so clean up member domainData %v", c.host, name, fmt.Sprintf("%v/%v", namespace, name))
	err = c.memberKusciaClient.KusciaV1alpha1().DomainDatas(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *hostResourcesController) createDomainData(ctx context.Context, hostDomainData *kusciaapisv1alpha1.DomainData, domainID string) error {
	initiator := ikcommon.GetObjectAnnotation(hostDomainData, common.InitiatorAnnotationKey)
	initiatorMasterDomainID, err := utilsres.GetMasterDomain(c.memberDomainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}

	domainData := utilsres.ExtractDomainDataSpec(hostDomainData)
	domainData.Namespace = domainID
	domainData.Annotations[common.InitiatorMasterDomainAnnotationKey] = initiatorMasterDomainID
	domainData.Labels[common.LabelResourceVersionUnderHostCluster] = hostDomainData.ResourceVersion
	_, err = c.memberKusciaClient.KusciaV1alpha1().DomainDatas(domainData.Namespace).Create(ctx, domainData, metav1.CreateOptions{})
	return err
}

func (c *hostResourcesController) updateDomainData(ctx context.Context, hostDomainData, memberDomainData *kusciaapisv1alpha1.DomainData) error {
	hrv := hostDomainData.ResourceVersion
	mrv := memberDomainData.Labels[common.LabelResourceVersionUnderHostCluster]
	if !utilsres.CompareResourceVersion(hrv, mrv) {
		nlog.Infof("Host cluster %v domainData %v resource version %v is not greater than the version %v under member cluster %v, skip this event...",
			c.host, ikcommon.GetObjectNamespaceName(hostDomainData), hrv, mrv, ikcommon.GetObjectNamespaceName(memberDomainData))
		return nil
	}

	hExtractedDomainDataSpec := utilsres.ExtractDomainDataSpec(hostDomainData)
	hExtractedDomainDataSpec.Namespace = memberDomainData.Namespace
	hExtractedDomainDataSpec.Annotations[common.InitiatorMasterDomainAnnotationKey] = memberDomainData.Annotations[common.InitiatorMasterDomainAnnotationKey]
	hExtractedDomainDataSpec.Labels[common.LabelResourceVersionUnderHostCluster] = hrv
	mExtractedDomainDataSpec := utilsres.ExtractDomainDataSpec(memberDomainData)
	if err := utilsres.PatchDomainData(ctx, c.memberKusciaClient, mExtractedDomainDataSpec, hExtractedDomainDataSpec); err != nil {
		return fmt.Errorf("patch member domainData %v fail, %v", ikcommon.GetObjectNamespaceName(mExtractedDomainDataSpec), err)
	}

	return nil
}
