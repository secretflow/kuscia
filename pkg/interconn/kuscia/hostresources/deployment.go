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
	"reflect"

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

// runDeploymentWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runDeploymentWorker() {
	for queue.HandleQueueItem(context.Background(), c.deploymentQueueName, c.deploymentQueue, c.syncDeploymentHandler, maxRetries) {
	}
}

// handleAddedorDeletedDeployment is used to handle added deployment.
func (c *hostResourcesController) handleAddedDeployment(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.deploymentQueue)
}

// handleUpdatedDeployment is used to handle updated deployment.
func (c *hostResourcesController) handleUpdatedDeployment(oldObj, newObj interface{}) {
	oldKd, ok := oldObj.(*kusciaapisv1alpha1.KusciaDeployment)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeployment", oldObj)
		return
	}

	newKd, ok := newObj.(*kusciaapisv1alpha1.KusciaDeployment)
	if !ok {
		nlog.Errorf("Object %#v is not a KusciaDeployment", newObj)
		return
	}

	if oldKd.ResourceVersion == newKd.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newKd, c.deploymentQueue)
}

// handleDeletedDeployment is used to handle deleted deployment.
func (c *hostResourcesController) handleDeletedDeployment(obj interface{}) {
	kd, ok := obj.(*kusciaapisv1alpha1.KusciaDeployment)
	if !ok {
		return
	}

	c.deploymentQueue.Add(fmt.Sprintf("%v%v/%v",
		ikcommon.DeleteEventKeyPrefix, common.KusciaCrossDomain, kd.Name))
}

// TODO: Abstract into common interface
// syncDeploymentHandler is used to sync kuscia deployment between host and member cluster.
func (c *hostResourcesController) syncDeploymentHandler(ctx context.Context, key string) error {
	key, deleteEvent := ikcommon.IsOriginalResourceDeleteEvent(key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Failed to split deployment key %v, %v, skip processing it", key, err)
		return nil
	}

	if deleteEvent {
		return c.deleteDeployment(ctx, namespace, name)
	}

	hKd, err := c.hostDeploymentLister.KusciaDeployments(namespace).Get(name)
	if err != nil {
		// Deployment is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Can't get host %v deployment %v, maybe it's deleted, skip processing it", c.host, key)
			return nil
		}
		return err
	}

	return c.processDeployment(ctx, hKd)
}

func (c *hostResourcesController) deleteDeployment(ctx context.Context, namespace, name string) error {
	kd, err := c.memberDeploymentLister.KusciaDeployments(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if kd.Annotations == nil || kd.Annotations[common.InitiatorMasterDomainAnnotationKey] != c.host {
		return nil
	}

	nlog.Infof("Host %v deployment %v is deleted, so clean up member deployment %v", c.host, name, fmt.Sprintf("%v/%v", namespace, name))
	err = c.memberKusciaClient.KusciaV1alpha1().KusciaDeployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *hostResourcesController) processDeployment(ctx context.Context, deployment *kusciaapisv1alpha1.KusciaDeployment) error {
	memberDeployment, err := c.memberDeploymentLister.KusciaDeployments(common.KusciaCrossDomain).Get(deployment.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			err = c.createDeployment(ctx, deployment)
		}
		return err
	}
	return c.updateDeployment(ctx, deployment, memberDeployment)
}

func (c *hostResourcesController) createDeployment(ctx context.Context, hostDeployment *kusciaapisv1alpha1.KusciaDeployment) error {
	initiator := ikcommon.GetObjectAnnotation(hostDeployment, common.InitiatorAnnotationKey)
	initiatorMasterDomainID, err := utilsres.GetMasterDomain(c.memberDomainLister, initiator)
	if err != nil {
		nlog.Errorf("Failed to get initiator %v master domain id, %v, skip processing it", initiator, err)
		return nil
	}

	kd := &kusciaapisv1alpha1.KusciaDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostDeployment.Name,
			Namespace: common.KusciaCrossDomain,
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:               initiator,
				common.SelfClusterAsInitiatorAnnotationKey:  common.False,
				common.InitiatorMasterDomainAnnotationKey:   initiatorMasterDomainID,
				common.InterConnKusciaPartyAnnotationKey:    ikcommon.GetObjectAnnotation(hostDeployment, common.InterConnKusciaPartyAnnotationKey),
				common.KusciaPartyMasterDomainAnnotationKey: hostDeployment.Namespace,
			},
			Labels: hostDeployment.Labels,
		},
		Spec: *hostDeployment.Spec.DeepCopy(),
	}

	_, err = c.memberKusciaClient.KusciaV1alpha1().KusciaDeployments(kd.Namespace).Create(ctx, kd, metav1.CreateOptions{})
	return err
}

func (c *hostResourcesController) updateDeployment(ctx context.Context, hostDeployment, memberDeployment *kusciaapisv1alpha1.KusciaDeployment) error {
	updated := false
	if !reflect.DeepEqual(memberDeployment.Spec, hostDeployment.Spec) {
		updated = true
		memberDeployment.Spec = *hostDeployment.Spec.DeepCopy()
	}

	if updated {
		_, err := c.memberKusciaClient.KusciaV1alpha1().KusciaDeployments(memberDeployment.Namespace).Update(ctx, memberDeployment, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
