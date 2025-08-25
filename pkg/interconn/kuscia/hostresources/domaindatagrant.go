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

// runDomainDataGrantWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runDomainDataGrantWorker() {
	for queue.HandleQueueItem(context.Background(), c.domainDataGrantQueueName, c.domainDataGrantQueue,
		c.syncDomainDataGrantHandler, maxRetries) {
	}
}

// handleAddedDomainDataGrant is used to handle added domainDataGrant.
func (c *hostResourcesController) handleAddedDomainDataGrant(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.domainDataGrantQueue)
}

// handleUpdatedDomainDataGrant is used to handle updated domainDataGrant.
func (c *hostResourcesController) handleUpdatedDomainDataGrant(oldObj, newObj interface{}) {
	oldDdg, ok := oldObj.(*kusciaapisv1alpha1.DomainDataGrant)
	if !ok {
		nlog.Errorf("Object %#v is not a DomainDataGrant", oldObj)
		return
	}

	newDdg, ok := newObj.(*kusciaapisv1alpha1.DomainDataGrant)
	if !ok {
		nlog.Errorf("Object %#v is not a DomainDataGrant", newObj)
		return
	}

	if oldDdg.ResourceVersion == newDdg.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newDdg, c.domainDataGrantQueue)
}

// handleAddedDomainDataGrant is used to handle deleted domainDataGrant.
func (c *hostResourcesController) handleDeletedDomainDataGrant(obj interface{}) {
	domainDataGrant, ok := obj.(*kusciaapisv1alpha1.DomainDataGrant)
	if !ok {
		return
	}

	partyDomainIDs := ikcommon.GetInterConnKusciaPartyDomainIDs(domainDataGrant)
	if len(partyDomainIDs) == 0 {
		nlog.Warnf("Member domain id is empty from deleted domainDataGrant %v, skip processing it",
			ikcommon.GetObjectNamespaceName(domainDataGrant))
		return
	}

	for _, domainID := range partyDomainIDs {
		c.domainDataGrantQueue.Add(fmt.Sprintf("%v%v/%v",
			ikcommon.DeleteEventKeyPrefix, domainID, domainDataGrant.Name))
	}
}

// TODO: Abstract into common interface
// syncDomainDataGrantHandler is used to sync domainDataGrant between host and member cluster.
func (c *hostResourcesController) syncDomainDataGrantHandler(ctx context.Context, key string) error {
	key, deleteEvent := ikcommon.IsOriginalResourceDeleteEvent(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split domainDataGrant key %v, %v, skip processing it", key, err)
		return nil
	}

	if deleteEvent {
		return c.deleteDomainDataGrant(ctx, namespace, name)
	}

	hDomainDataGrant, err := c.hostDomainDataGrantLister.DomainDataGrants(namespace).Get(name)
	if err != nil {
		// DomainDataGrant is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Host %v domainDataGrant %v not found, delete member domainDataGrant", c.host, key)
			return c.deleteDomainDataGrant(ctx, namespace, name)
		}
		return err
	}

	partyDomainIDs := ikcommon.GetInterConnKusciaPartyDomainIDs(hDomainDataGrant)
	if partyDomainIDs == nil {
		nlog.Warnf("Failed to get party domain ids from host %v domainDataGrant %v, skip processing it",
			c.host, ikcommon.GetObjectNamespaceName(hDomainDataGrant))
		return nil
	}

	hDomainDataGrantCopy := hDomainDataGrant.DeepCopy()
	for _, domainID := range partyDomainIDs {
		mDomainDataGrant, err := c.memberDomainDataGrantLister.DomainDataGrants(domainID).Get(name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = c.createDomainDataGrant(ctx, hDomainDataGrantCopy, domainID)
			}
			return err
		}

		return c.updateDomainDataGrant(ctx, hDomainDataGrantCopy, mDomainDataGrant.DeepCopy())
	}

	return nil
}

func (c *hostResourcesController) deleteDomainDataGrant(ctx context.Context, namespace, name string) error {
	ddg, err := c.memberDomainDataGrantLister.DomainDataGrants(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if ddg.Annotations == nil || ddg.Annotations[common.InitiatorMasterDomainAnnotationKey] != c.host {
		return nil
	}

	nlog.Infof("Host %v domainDataGrant %v is deleted, so clean up member domainDataGrant %v", c.host, name,
		fmt.Sprintf("%v/%v", namespace, name))
	err = c.memberKusciaClient.KusciaV1alpha1().DomainDataGrants(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *hostResourcesController) createDomainDataGrant(ctx context.Context, hostDomainDataGrant *kusciaapisv1alpha1.DomainDataGrant, domainID string) error {
	initiator := ikcommon.GetObjectAnnotation(hostDomainDataGrant, common.InitiatorAnnotationKey)
	initiatorMasterDomainID, err := utilsres.GetMasterDomain(c.memberDomainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}

	domainDataGrant := utilsres.ExtractDomainDataGrantSpec(hostDomainDataGrant)
	domainDataGrant.Namespace = domainID
	domainDataGrant.Annotations[common.InitiatorMasterDomainAnnotationKey] = initiatorMasterDomainID
	domainDataGrant.Labels[common.LabelResourceVersionUnderHostCluster] = hostDomainDataGrant.ResourceVersion
	_, err = c.memberKusciaClient.KusciaV1alpha1().DomainDataGrants(domainDataGrant.Namespace).Create(ctx, domainDataGrant, metav1.CreateOptions{})
	return err
}

func (c *hostResourcesController) updateDomainDataGrant(ctx context.Context, hostDomainDataGrant, memberDomainDataGrant *kusciaapisv1alpha1.DomainDataGrant) error {
	hrv := hostDomainDataGrant.ResourceVersion
	mrv := memberDomainDataGrant.Labels[common.LabelResourceVersionUnderHostCluster]
	if !utilsres.CompareResourceVersion(hrv, mrv) {
		nlog.Infof("Host cluster %v domainDataGrant %v resource version %v is not greater than the domainDataGrant %v version %v under member cluster, skip this event...",
			c.host, ikcommon.GetObjectNamespaceName(hostDomainDataGrant), hrv, ikcommon.GetObjectNamespaceName(memberDomainDataGrant), mrv)
		return nil
	}

	hExtractedDomainDataGrantSpec := utilsres.ExtractDomainDataGrantSpec(hostDomainDataGrant)
	hExtractedDomainDataGrantSpec.Namespace = memberDomainDataGrant.Namespace
	hExtractedDomainDataGrantSpec.Annotations[common.InitiatorMasterDomainAnnotationKey] = memberDomainDataGrant.Annotations[common.InitiatorMasterDomainAnnotationKey]
	hExtractedDomainDataGrantSpec.Labels[common.LabelResourceVersionUnderHostCluster] = hrv
	mExtractedDomainDataGrantSpec := utilsres.ExtractDomainDataGrantSpec(memberDomainDataGrant)
	if err := utilsres.PatchDomainDataGrant(ctx, c.memberKusciaClient, mExtractedDomainDataGrantSpec, hExtractedDomainDataGrantSpec); err != nil {
		return fmt.Errorf("patch member domainDataGrant %v fail, %v", ikcommon.GetObjectNamespaceName(mExtractedDomainDataGrantSpec), err)
	}
	return nil
}
