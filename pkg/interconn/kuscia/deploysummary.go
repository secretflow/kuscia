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
package kuscia

import (
	"context"
	"reflect"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

// runDeploymentSummaryWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runDeploymentSummaryWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, deploymentSummaryQueueName, c.deploymentSummaryQueue, c.syncDeploymentSummaryHandler, maxRetries) {
	}
}

// handleAddedDeploymentSummary is used to handle added deploymentSummary.
func (c *Controller) handleAddedDeploymentSummary(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.deploymentSummaryQueue)
}

// handleUpdatedDeploymentSummary is used to handle updated deploymentSummary.
func (c *Controller) handleUpdatedDeploymentSummary(oldObj, newObj interface{}) {
	oldKds, ok := oldObj.(*v1alpha1.KusciaDeploymentSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeploymentSummary", oldObj)
		return
	}

	newKds, ok := newObj.(*v1alpha1.KusciaDeploymentSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeploymentSummary", newObj)
		return
	}

	if oldKds.ResourceVersion == newKds.ResourceVersion {
		return
	}

	queue.EnqueueObjectWithKey(newKds, c.deploymentSummaryQueue)
}

// syncDeploymentSummaryHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) syncDeploymentSummaryHandler(ctx context.Context, key string) (err error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split deploymentSummary key %v, %v, skip processing it", key, err)
		return nil
	}

	originalKds, err := c.deploymentSummaryLister.KusciaDeploymentSummaries(namespace).Get(name)
	if err != nil {
		// deploymentSummary was deleted in cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get deploymentSummary %v, maybe it was deleted, skip processing it", key)
			return nil
		}
		return err
	}

	return c.updateDeployment(ctx, originalKds.DeepCopy())
}

func (c *Controller) updateDeployment(ctx context.Context, deploymentSummary *v1alpha1.KusciaDeploymentSummary) error {
	originalKd, err := c.deploymentLister.KusciaDeployments(common.KusciaCrossDomain).Get(deploymentSummary.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			originalKd, err = c.kusciaClient.KusciaV1alpha1().KusciaDeployments(common.KusciaCrossDomain).Get(ctx,
				deploymentSummary.Name, metav1.GetOptions{})
		}
		if err != nil {
			nlog.Errorf("Failed to get deployment %v, %v", deploymentSummary.Name, err)
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}

	partyDomainIDs := ikcommon.GetInterConnKusciaPartyDomainIDs(deploymentSummary)
	if partyDomainIDs == nil {
		nlog.Warnf("Failed to get interconn kuscia party domain ids from deploymentSummary %v, skip processing it",
			ikcommon.GetObjectNamespaceName(deploymentSummary))
		return nil
	}

	kd := originalKd.DeepCopy()
	if updateDeploymentPartyStatus(kd, deploymentSummary, partyDomainIDs) {
		kd.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = c.kusciaClient.KusciaV1alpha1().KusciaDeployments(kd.Namespace).UpdateStatus(ctx, kd, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func updateDeploymentPartyStatus(deployment *v1alpha1.KusciaDeployment,
	deploymentSummary *v1alpha1.KusciaDeploymentSummary,
	domainIDs []string) bool {
	if deploymentSummary.Status.PartyDeploymentStatuses == nil {
		return false
	}

	if deployment.Status.PartyDeploymentStatuses == nil {
		deployment.Status.PartyDeploymentStatuses = deploymentSummary.Status.PartyDeploymentStatuses
		return true
	}

	updated := false
	for _, domainID := range domainIDs {
		if statusInDeploymentSummary, ok := deploymentSummary.Status.PartyDeploymentStatuses[domainID]; ok {
			if !reflect.DeepEqual(statusInDeploymentSummary, deployment.Status.PartyDeploymentStatuses[domainID]) {
				updated = true
				deployment.Status.PartyDeploymentStatuses[domainID] = deploymentSummary.Status.PartyDeploymentStatuses[domainID]
			}
		}
	}
	return updated
}
