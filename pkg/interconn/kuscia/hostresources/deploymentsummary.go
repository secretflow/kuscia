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
	"reflect"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	ikcommon "github.com/secretflow/kuscia/pkg/interconn/kuscia/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

// runDeploymentSummaryWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runDeploymentSummaryWorker() {
	for queue.HandleQueueItem(context.Background(), c.deploymentSummaryQueueName,
		c.deploymentSummaryQueue, c.syncDeploymentSummaryHandler, maxRetries) {
	}
}

// handleAddedorDeletedDeploymentSummary is used to handle added or deleted deployment summary.
func (c *hostResourcesController) handleAddedorDeletedDeploymentSummary(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.deploymentSummaryQueue)
}

// handleUpdatedDeploymentSummary is used to handle updated deployment summary.
func (c *hostResourcesController) handleUpdatedDeploymentSummary(oldObj, newObj interface{}) {
	oldKds, ok := oldObj.(*kusciaapisv1alpha1.KusciaDeploymentSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeploymentSummary", oldObj)
		return
	}

	newKds, ok := newObj.(*kusciaapisv1alpha1.KusciaDeploymentSummary)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeploymentSummary", newObj)
		return
	}

	if oldKds.ResourceVersion == newKds.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newKds, c.deploymentSummaryQueue)
}

// TODO: Abstract into common interface
// syncDeploymentSummaryHandler is used to sync kuscia deployment summary between host and member cluster.
func (c *hostResourcesController) syncDeploymentSummaryHandler(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split deploymentSummary key %v, %v, skip processing it", key, err)
		return nil
	}

	hKds, err := c.hostDeploymentSummaryLister.KusciaDeploymentSummaries(namespace).Get(name)
	if err != nil {
		// DeploymentSummary is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Kuscia deploymentSummary %v may be deleted under host %v cluster, skip processing it", key, c.host)
			return nil
		}
		return err
	}

	return c.updateMemberDeployment(ctx, hKds.DeepCopy())
}

func (c *hostResourcesController) updateMemberDeployment(ctx context.Context, deploymentSummary *kusciaapisv1alpha1.KusciaDeploymentSummary) error {
	originalKd, err := c.memberDeploymentLister.KusciaDeployments(common.KusciaCrossDomain).Get(deploymentSummary.Name)
	if err != nil {
		return err
	}

	selfDomainIDs := ikcommon.GetSelfClusterPartyDomainIDs(originalKd)
	if selfDomainIDs == nil {
		nlog.Warnf("Failed to get self party domain ids from kuscia deployment %v, skip processing it",
			ikcommon.GetObjectNamespaceName(originalKd))
		return nil
	}

	domainIDMap := make(map[string]struct{})
	for _, domainID := range selfDomainIDs {
		domainIDMap[domainID] = struct{}{}
	}

	kd := originalKd.DeepCopy()
	if updateDeploymentPartyStatus(kd, deploymentSummary, domainIDMap) {
		kd.Status.LastReconcileTime = ikcommon.GetCurrentTime()
		if _, err = c.memberKusciaClient.KusciaV1alpha1().KusciaDeployments(kd.Namespace).UpdateStatus(ctx, kd, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func updateDeploymentPartyStatus(deployment *kusciaapisv1alpha1.KusciaDeployment,
	deploymentSummary *kusciaapisv1alpha1.KusciaDeploymentSummary,
	domainIDMap map[string]struct{}) bool {
	if deploymentSummary.Status.PartyDeploymentStatuses == nil {
		return false
	}

	if deployment.Status.PartyDeploymentStatuses == nil {
		deployment.Status.PartyDeploymentStatuses = deploymentSummary.Status.PartyDeploymentStatuses
		return true
	}

	updated := false
	for domainID, statusInSummary := range deploymentSummary.Status.PartyDeploymentStatuses {
		if _, exist := domainIDMap[domainID]; exist {
			continue
		}

		if !reflect.DeepEqual(statusInSummary, deployment.Status.PartyDeploymentStatuses[domainID]) {
			deployment.Status.PartyDeploymentStatuses[domainID] = deploymentSummary.Status.PartyDeploymentStatuses[domainID]
			updated = true
		}
	}

	return updated
}
