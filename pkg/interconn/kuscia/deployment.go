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
package kuscia

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runDeploymentWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runDeploymentWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, deploymentQueueName, c.deploymentQueue, c.syncDeploymentHandler, maxRetries) {
	}
}

// handleAddedDeployment is used to handle added kuscia deployment.
func (c *Controller) handleAddedDeployment(obj interface{}) {
	kd, ok := obj.(*v1alpha1.KusciaDeployment)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeployment", obj)
		return
	}
	if kd.Namespace != common.KusciaCrossDomain {
		return
	}
	queue.EnqueueObjectWithKey(obj, c.deploymentQueue)
}

// handleUpdatedDeployment is used to handle updated kuscia deployment.
func (c *Controller) handleUpdatedDeployment(oldObj, newObj interface{}) {
	oldKd, ok := oldObj.(*v1alpha1.KusciaDeployment)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeployment", oldObj)
		return
	}
	if oldKd.Namespace != common.KusciaCrossDomain {
		return
	}

	newKd, ok := newObj.(*v1alpha1.KusciaDeployment)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeployment", newObj)
		return
	}

	if oldKd.ResourceVersion == newKd.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newKd, c.deploymentQueue)
}

// handleDeletedDeployment is used to handle deleted kuscia deployment.
func (c *Controller) handleDeletedDeployment(obj interface{}) {
	kd, ok := obj.(*v1alpha1.KusciaDeployment)
	if !ok {
		return
	}

	if kd.Namespace != common.KusciaCrossDomain {
		c.deploymentQueue.Add(fmt.Sprintf("%v/%v", common.KusciaCrossDomain, kd.Name))
		return
	}

	isInitiator, err := ikcommon.SelfClusterIsInitiator(c.domainLister, kd)
	if err != nil {
		nlog.Errorf("Failed to handle kuscia deployment %v delete event, %v", ikcommon.GetObjectNamespaceName(kd), err)
		return
	}

	if isInitiator {
		masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, kd)
		if err != nil {
			nlog.Errorf("Failed to handle kuscia deployment %v delete event, %v", ikcommon.GetObjectNamespaceName(kd), err)
			return
		}

		for masterDomain := range masterDomains {
			c.deploymentQueue.Add(fmt.Sprintf("%v%v/%v", ikcommon.DeleteEventKeyPrefix, masterDomain, kd.Name))
		}
	} else {
		host := ikcommon.GetObjectAnnotation(kd, common.InitiatorAnnotationKey)
		member := ikcommon.GetObjectAnnotation(kd, common.KusciaPartyMasterDomainAnnotationKey)
		hostMasterDomainID, err := utilsres.GetMasterDomain(c.domainLister, host)
		if err != nil {
			nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", host, err)
			return
		}

		hra := c.hostResourceManager.GetHostResourceAccessor(hostMasterDomainID, member)
		if hra == nil {
			nlog.Warnf("Host resource accessor for deployment %v is empty, skip processing it", ikcommon.GetObjectNamespaceName(kd))
			return
		}
		hra.EnqueueDeployment(fmt.Sprintf("%s/%s", member, kd.Name))
	}
}

// syncDeploymentHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncDeploymentHandler(ctx context.Context, key string) (err error) {
	key, deleteEvent := ikcommon.IsOriginalResourceDeleteEvent(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split deployment key %v, %v, skip processing it", key, err)
		return nil
	}

	if deleteEvent {
		nlog.Infof("Delete deployment %v cascaded resources", name)
		return c.deleteDeploymentCascadedResources(ctx, namespace, name)
	}

	originalKd, err := c.deploymentLister.KusciaDeployments(namespace).Get(name)
	if err != nil {
		// kuscia deployment was deleted in cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get deployment %v, maybe it's deleted, skip processing it", key)
			return nil
		}
		return err
	}

	selfIsInitiator, err := ikcommon.SelfClusterIsInitiator(c.domainLister, originalKd)
	if err != nil {
		return err
	}

	kd := originalKd.DeepCopy()
	if selfIsInitiator {
		return c.processDeploymentAsInitiator(ctx, kd)
	}
	return c.processDeploymentAsPartner(ctx, kd)
}

func (c *Controller) deleteDeploymentCascadedResources(ctx context.Context, namespace, name string) error {
	mirrorKd, err := c.deploymentLister.KusciaDeployments(namespace).Get(name)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if mirrorKd != nil {
		err = c.kusciaClient.KusciaV1alpha1().KusciaDeployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	kds, err := c.deploymentSummaryLister.KusciaDeploymentSummaries(namespace).Get(name)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if kds != nil {
		err = c.kusciaClient.KusciaV1alpha1().KusciaDeploymentSummaries(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *Controller) processDeploymentAsInitiator(ctx context.Context, deployment *v1alpha1.KusciaDeployment) error {
	if err := c.createOrUpdateMirrorDeployments(ctx, deployment); err != nil {
		return err
	}

	return c.createOrUpdateDeploymentSummary(ctx, deployment)
}

func (c *Controller) createOrUpdateMirrorDeployments(ctx context.Context, deployment *v1alpha1.KusciaDeployment) error {
	masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, deployment)
	if err != nil {
		nlog.Errorf("Failed to create or update mirror deployment %v, %v", ikcommon.GetObjectNamespaceName(deployment), err)
		return nil
	}

	for masterDomainID, partyDomainIDs := range masterDomains {
		mirrorDeployment, err := c.deploymentLister.KusciaDeployments(masterDomainID).Get(deployment.Name)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				err = c.createMirrorDeployment(ctx, deployment, masterDomainID, partyDomainIDs)
			}
			return err
		}

		if err = c.updateMirrorDeployment(ctx, deployment, mirrorDeployment, partyDomainIDs); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) createMirrorDeployment(ctx context.Context,
	deployment *v1alpha1.KusciaDeployment,
	masterDomainID string, partyDomainIDs []string) error {
	kd := &v1alpha1.KusciaDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: masterDomainID,
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:            ikcommon.GetObjectAnnotation(deployment, common.InitiatorAnnotationKey),
				common.InterConnKusciaPartyAnnotationKey: strings.Join(partyDomainIDs, "_"),
			},
		},
		Spec: *deployment.Spec.DeepCopy(),
	}

	if deployment.Labels != nil && deployment.Labels[common.LabelKusciaDeploymentAppType] != string(common.SCQLApp) {
		kd.Labels = map[string]string{
			common.LabelKusciaDeploymentAppType: deployment.Labels[common.LabelKusciaDeploymentAppType],
		}
	}

	_, err := c.kusciaClient.KusciaV1alpha1().KusciaDeployments(kd.Namespace).Create(ctx, kd, metav1.CreateOptions{})
	return err
}

func (c *Controller) updateMirrorDeployment(ctx context.Context, deployment,
	mirrorDeployment *v1alpha1.KusciaDeployment,
	partyDomainIDs []string) error {

	updated := false
	if mirrorDeployment.Annotations[common.InitiatorAnnotationKey] != deployment.Annotations[common.InitiatorAnnotationKey] {
		updated = true
		mirrorDeployment.Annotations[common.InitiatorAnnotationKey] = deployment.Annotations[common.InitiatorAnnotationKey]
	}

	kusciaParty := strings.Join(partyDomainIDs, "_")
	if mirrorDeployment.Annotations[common.InterConnKusciaPartyAnnotationKey] != kusciaParty {
		updated = true
		mirrorDeployment.Annotations[common.InterConnKusciaPartyAnnotationKey] = kusciaParty
	}

	if !reflect.DeepEqual(mirrorDeployment.Spec, deployment.Spec) {
		updated = true
		mirrorDeployment.Spec = *deployment.Spec.DeepCopy()
	}

	if updated {
		nlog.Infof("Update kuscia mirror deployment %v", ikcommon.GetObjectNamespaceName(mirrorDeployment))
		_, err := c.kusciaClient.KusciaV1alpha1().KusciaDeployments(mirrorDeployment.Namespace).Update(ctx, mirrorDeployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) createOrUpdateDeploymentSummary(ctx context.Context, deployment *v1alpha1.KusciaDeployment) error {
	masterDomains, err := ikcommon.GetPartyMasterDomains(c.domainLister, deployment)
	if err != nil {
		nlog.Errorf("Failed to create or update deploymentSummary for kuscia deployment %v, %v",
			ikcommon.GetObjectNamespaceName(deployment), err)
		return nil
	}

	for masterDomainID, partyDomainIDs := range masterDomains {
		deploymentSummary, err := c.deploymentSummaryLister.KusciaDeploymentSummaries(masterDomainID).Get(deployment.Name)
		if err != nil {
			// create deploymentSummary
			if k8serrors.IsNotFound(err) {
				if err = c.createDeploymentSummary(ctx, deployment, masterDomainID, partyDomainIDs); err != nil {
					return err
				}
				continue
			}
			return err
		}
		// update deploymentSummary
		if err = c.updateDeploymentSummary(ctx, deployment, deploymentSummary.DeepCopy()); err != nil {
			return err
		}

	}
	return nil
}

func (c *Controller) createDeploymentSummary(ctx context.Context,
	deployment *v1alpha1.KusciaDeployment,
	masterDomainID string,
	partyDomainIDs []string) error {
	deploymentSummary := &v1alpha1.KusciaDeploymentSummary{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: masterDomainID,
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:            ikcommon.GetObjectAnnotation(deployment, common.InitiatorAnnotationKey),
				common.InterConnKusciaPartyAnnotationKey: strings.Join(partyDomainIDs, "_"),
			},
		},
	}

	updateDeploymentSummaryPartyStatus(deployment, deploymentSummary)
	_, err := c.kusciaClient.KusciaV1alpha1().KusciaDeploymentSummaries(masterDomainID).Create(ctx, deploymentSummary, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *Controller) updateDeploymentSummary(ctx context.Context, deployment *v1alpha1.KusciaDeployment, deploymentSummary *v1alpha1.KusciaDeploymentSummary) error {
	if updateDeploymentSummaryPartyStatus(deployment, deploymentSummary) {
		deploymentSummary.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err := c.kusciaClient.KusciaV1alpha1().KusciaDeploymentSummaries(deploymentSummary.Namespace).Update(ctx, deploymentSummary, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) processDeploymentAsPartner(ctx context.Context, deployment *v1alpha1.KusciaDeployment) error {
	initiator := ikcommon.GetObjectAnnotation(deployment, common.InitiatorAnnotationKey)
	if initiator == "" {
		nlog.Errorf("Failed to get initiator from kuscia deployment %v, skip processing it", ikcommon.GetObjectNamespaceName(deployment))
		return nil
	}

	masterDomainID := ikcommon.GetObjectAnnotation(deployment, common.KusciaPartyMasterDomainAnnotationKey)
	if masterDomainID == "" {
		nlog.Errorf("Failed to get master domain id from kuscia deployment %v, skip processing it", ikcommon.GetObjectNamespaceName(deployment))
		return nil
	}

	hostMasterDomainID, err := utilsres.GetMasterDomain(c.domainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}
	hra := c.hostResourceManager.GetHostResourceAccessor(hostMasterDomainID, masterDomainID)
	if hra == nil {
		return fmt.Errorf("host resource accessor for kuscia deployment %v is empty of host/member %v/%v, retry",
			ikcommon.GetObjectNamespaceName(deployment), initiator, masterDomainID)
	}

	if !hra.HasSynced() {
		return fmt.Errorf("host %v resource accessor has not synced, retry", initiator)
	}

	hDeploymentSummary, err := hra.HostDeploymentSummaryLister().KusciaDeploymentSummaries(masterDomainID).Get(deployment.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			hDeploymentSummary, err = hra.HostKusciaClient().KusciaV1alpha1().KusciaDeploymentSummaries(masterDomainID).Get(ctx, deployment.Name, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Get deploymentSummary %v from host %v cluster failed, %v", deployment.Name, initiator, err)
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}

	kds := hDeploymentSummary.DeepCopy()
	if updateDeploymentSummaryPartyStatus(deployment, kds) {
		kds.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = hra.HostKusciaClient().KusciaV1alpha1().KusciaDeploymentSummaries(kds.Namespace).Update(ctx, kds, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func updateDeploymentSummaryPartyStatus(deployment *v1alpha1.KusciaDeployment, deploymentSummary *v1alpha1.KusciaDeploymentSummary) bool {
	updated := false
	if deployment.Status.Phase == v1alpha1.KusciaDeploymentPhaseFailed &&
		deployment.Status.Phase != deploymentSummary.Status.Phase {
		updated = true
		deploymentSummary.Status.Phase = deployment.Status.Phase
		deploymentSummary.Status.Reason = deployment.Status.Reason
		deploymentSummary.Status.Message = deployment.Status.Message
	}

	if deployment.Status.PartyDeploymentStatuses == nil {
		return updated
	}

	domainIDs := ikcommon.GetSelfClusterPartyDomainIDs(deployment)
	if domainIDs == nil {
		nlog.Errorf("Failed to get self cluster party domain ids from kuscia deployment %v, skip processing it", ikcommon.GetObjectNamespaceName(deployment))
		return updated
	}

	for _, domainID := range domainIDs {
		if statusInDeployment, ok := deployment.Status.PartyDeploymentStatuses[domainID]; ok {
			if deploymentSummary.Status.PartyDeploymentStatuses == nil {
				updated = true
				deploymentSummary.Status.PartyDeploymentStatuses = map[string]map[string]*v1alpha1.KusciaDeploymentPartyStatus{}
				deploymentSummary.Status.PartyDeploymentStatuses[domainID] = deployment.Status.PartyDeploymentStatuses[domainID]
				continue
			}

			if !reflect.DeepEqual(statusInDeployment, deploymentSummary.Status.PartyDeploymentStatuses[domainID]) {
				updated = true
				deploymentSummary.Status.PartyDeploymentStatuses[domainID] = deployment.Status.PartyDeploymentStatuses[domainID]
			}
		}
	}

	return updated
}
