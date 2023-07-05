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
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
)

// runConfigMapWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *hostResourcesController) runConfigMapWorker() {
	ctx := context.Background()
	queueName := generateQueueName(hostConfigMapsQueueName, c.host, c.member)
	for queue.HandleQueueItem(ctx, queueName, c.configMapsQueue, c.syncConfigMapHandler, maxRetries) {
	}
}

// handleAddedOrDeletedConfigMap is used to handle added or deleted configmap.
func (c *hostResourcesController) handleAddedOrDeletedConfigMap(obj interface{}) {
	queue.EnqueueObjectWithKey(obj, c.configMapsQueue)
}

// handleUpdatedConfigMap is used to handle updated configmap.
func (c *hostResourcesController) handleUpdatedConfigMap(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*corev1.ConfigMap)
	if !ok {
		nlog.Errorf("Object %#v is not a ConfigMap", oldObj)
		return
	}

	newPod, ok := newObj.(*corev1.ConfigMap)
	if !ok {
		nlog.Errorf("Object %#v is not a ConfigMap", newObj)
		return
	}

	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKey(newPod, c.configMapsQueue)
}

// Todo: Abstract into common interface
// syncConfigMapHandler is used to sync configmap between host and member cluster.
func (c *hostResourcesController) syncConfigMapHandler(ctx context.Context, key string) error {
	namespace, name, err := getNamespaceAndNameFromKey(key, c.member)
	if err != nil {
		nlog.Error(err)
		return nil
	}

	hConfigMap, err := c.hostConfigMapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		// configmap is deleted under host cluster
		if k8serrors.IsNotFound(err) {
			nlog.Infof("ConfigMap %v may be deleted under host %v cluster, so delete the configmap under member cluster", key, c.host)
			if err = c.memberKubeClient.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			return nil
		}
		return err
	}

	hConfigMapCopy := hConfigMap.DeepCopy()
	hrv := hConfigMapCopy.ResourceVersion

	hExtractedConfigMap := utilsres.ExtractConfigMap(hConfigMapCopy)
	hExtractedConfigMap.Labels[common.LabelResourceVersionUnderHostCluster] = hrv

	mConfigMap, err := c.memberConfigMapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			nlog.Infof("Failed to get configmap %v under member cluster, create it...", key)
			if _, err = c.memberKubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, hExtractedConfigMap, metav1.CreateOptions{}); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					return nil
				}
				return fmt.Errorf("create configmap %v from host cluster failed, %v", key, err.Error())
			}
			return nil
		}
		return err
	}

	mConfigMapCopy := mConfigMap.DeepCopy()
	mrv := mConfigMapCopy.Labels[common.LabelResourceVersionUnderHostCluster]
	if !utilsres.CompareResourceVersion(hrv, mrv) {
		nlog.Infof("Configmap %v resource version %v under host cluster is not greater than the value %v under member cluster, skip this event...", key, hrv, mrv)
		return nil
	}

	mExtractedConfigMap := utilsres.ExtractConfigMap(mConfigMapCopy)
	if err = utilsres.PatchConfigMap(ctx, c.memberKubeClient, mExtractedConfigMap, hExtractedConfigMap); err != nil {
		return fmt.Errorf("patch member configmap %v failed, %v", key, err.Error())
	}
	return nil
}
