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

package interop

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/queue"
)

// runInteropConfigWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (c *Controller) runInteropConfigWorker(ctx context.Context) {
	for queue.HandleQueueItem(ctx, interopConfigQueueName, c.interopConfigQueue, c.interopConfigHandler, maxRetries) {
	}
}

// registerInteropConfigs is used to register all interop configs.
func (c *Controller) registerInteropConfigs() {
	interopConfigs, _ := c.interopConfigLister.List(labels.Everything())
	for _, ic := range interopConfigs {
		if err := c.registerInteropConfig(ic); err != nil {
			nlog.Warnf("Register interop config %v failed during startup， %v", ic.Name, err.Error())
		}
	}
}

// handleAddedorDeletedInteropConfig is used to handle added or deleted interop config.
func (c *Controller) handleAddedorDeletedInteropConfig(obj interface{}) {
	queue.EnqueueObjectWithKeyName(obj, c.interopConfigQueue)
}

// handleUpdatedInteropConfig is used to handle updated interop config.
func (c *Controller) handleUpdatedInteropConfig(oldObj, newObj interface{}) {
	oldIC, ok := oldObj.(*kusciaapisv1alpha1.InteropConfig)
	if !ok {
		nlog.Errorf("Object %#v is not a InteropConfig", oldObj)
		return
	}

	newIC, ok := newObj.(*kusciaapisv1alpha1.InteropConfig)
	if !ok {
		nlog.Errorf("Object %#v is not a InteropConfig", newObj)
		return
	}

	if oldIC.ResourceVersion == newIC.ResourceVersion {
		return
	}
	queue.EnqueueObjectWithKeyName(newObj, c.interopConfigQueue)
}

// interopConfigHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the resource
// with the current status of the resource.
func (c *Controller) interopConfigHandler(ctx context.Context, key string) error {
	ic, err := c.interopConfigLister.Get(key)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			c.deregisterInteropConfig(key)
			return nil
		}
		return err
	}

	if err = c.registerInteropConfig(ic); err != nil {
		return err
	}

	return nil
}

// registerInteropConfig is used to register interop config.
func (c *Controller) registerInteropConfig(ic *kusciaapisv1alpha1.InteropConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	icInfo := c.getInteropConfigInfo(ic.Name)
	if icInfo == nil {
		icInfo = &interopConfigInfo{
			host:    ic.Spec.Host,
			members: ic.Spec.Members,
		}
		c.setInteropConfigInfo(ic.Name, icInfo)

		for _, m := range icInfo.members {
			c.hostResourceManager.Register(ic.Spec.Host, m)
		}
		nlog.Infof("Interop config %v host %v added members list: %v", ic.Name, ic.Spec.Host, ic.Spec.Members)
		return nil
	}

	var deletedMembers, addedMembers []string
	for _, om := range icInfo.members {
		found := false
		for _, nm := range ic.Spec.Members {
			if om == nm {
				found = true
				break
			}
		}

		if !found {
			deletedMembers = append(deletedMembers, om)
		}
	}

	for _, nm := range ic.Spec.Members {
		found := false
		for _, om := range icInfo.members {
			if om == nm {
				found = true
				break
			}
		}

		if !found {
			addedMembers = append(addedMembers, nm)
		}
	}

	for _, m := range deletedMembers {
		c.hostResourceManager.Deregister(ic.Spec.Host, m)
	}

	for _, m := range addedMembers {
		go c.hostResourceManager.Register(ic.Spec.Host, m)
	}

	icInfo.host = ic.Spec.Host
	icInfo.members = ic.Spec.Members

	nlog.Infof("Interop config %v host %v added members list: %v, deleted members list: %v", ic.Name, ic.Spec.Host, addedMembers, deletedMembers)
	return nil
}

// deregisterInteropConfig is used to deregister interop config.
func (c *Controller) deregisterInteropConfig(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	icInfo := c.getInteropConfigInfo(key)
	if icInfo == nil {
		return
	}

	for _, member := range icInfo.members {
		c.hostResourceManager.Deregister(icInfo.host, member)
	}

	c.deleteInteropConfigInfo(key)
}

// getInteropConfigInfo is used to get interop config info.
func (c *Controller) getInteropConfigInfo(key string) *interopConfigInfo {
	info, exist := c.interopConfigInfos[key]
	if !exist {
		return nil
	}
	return info
}

// setInteropConfigInfo is used to set interop config info.
func (c *Controller) setInteropConfigInfo(key string, info *interopConfigInfo) {
	c.interopConfigInfos[key] = info
}

// deleteInteropConfigInfo is used to delete interop config info.
func (c *Controller) deleteInteropConfigInfo(key string) {
	delete(c.interopConfigInfos, key)
}
