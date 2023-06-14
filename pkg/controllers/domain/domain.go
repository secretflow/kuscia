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

package domain

import (
	"context"
	"reflect"
	"sort"

	apicorev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// syncDomainStatus is used to sync domain status according to domain nodes status.
func (c *Controller) syncDomainStatus() {
	domains, err := c.domainLister.List(labels.Everything())
	if err != nil {
		nlog.Warnf("List domain failed, %s", err.Error())
		return
	}

	for _, dm := range domains {
		dm = dm.DeepCopy()
		if _, err = c.namespaceLister.Get(dm.Name); err != nil {
			nlog.Warnf("Get namespace %v failed, %v, skip to update the domain %v status", dm.Name, err, dm.Name)
			continue
		}

		nodeReq, _ := labels.NewRequirement(common.LabelNodeNamespace, selection.Equals, []string{dm.Name})
		nodes, err := c.nodeLister.List(labels.NewSelector().Add(*nodeReq))
		if err != nil {
			nlog.Warnf("List nodes for domain %v failed, %v", dm.Name, err.Error())
			continue
		}

		oldStatus := dm.Status
		newStatus := c.getDomainStatus(nodes)
		if !c.isDomainStatusEqual(oldStatus, newStatus) {
			nlog.Infof("Updating domain %v status", dm.Name)
			dm.Status = newStatus
			if err = c.updateDomainStatus(dm); err != nil {
				nlog.Warnf("Update domain node status failed, %v", err.Error())
			}
		}
	}
}

// getDomainStatus is used to get domain status.
func (c *Controller) getDomainStatus(nodes []*apicorev1.Node) *kusciaapisv1alpha1.DomainStatus {
	var domainStatus kusciaapisv1alpha1.DomainStatus
	for _, node := range nodes {
		var (
			status             string
			lastHeartbeatTime  apismetav1.Time
			lastTransitionTime apismetav1.Time
		)

		for _, cond := range node.Status.Conditions {
			if cond.Type == apicorev1.NodeReady {
				switch cond.Status {
				case apicorev1.ConditionTrue:
					status = nodeStatusReady
				default:
					status = nodeStatusNotReady
				}
				lastHeartbeatTime = cond.LastHeartbeatTime
				lastTransitionTime = cond.LastTransitionTime
				break
			}
		}

		nodeStatus := kusciaapisv1alpha1.NodeStatus{
			Name:               node.Name,
			Version:            node.Status.NodeInfo.KubeletVersion,
			Status:             status,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
		}

		domainStatus.NodeStatuses = append(domainStatus.NodeStatuses, nodeStatus)
	}

	if len(domainStatus.NodeStatuses) == 0 {
		return nil
	}

	return &domainStatus
}

// isDomainStatusEqual is used to check whether the new domain status is equal to the old domain status.
func (c *Controller) isDomainStatusEqual(oldStatus, newStatus *kusciaapisv1alpha1.DomainStatus) bool {
	if oldStatus == nil || newStatus == nil {
		return oldStatus == newStatus
	}

	if len(oldStatus.NodeStatuses) != len(newStatus.NodeStatuses) {
		return false
	}

	c.sortNodeStatus(oldStatus.NodeStatuses)
	c.sortNodeStatus(newStatus.NodeStatuses)
	return reflect.DeepEqual(oldStatus, newStatus)
}

// sortNodeStatus is used to sort node status.
func (c *Controller) sortNodeStatus(status []kusciaapisv1alpha1.NodeStatus) {
	sort.SliceStable(status, func(i, j int) bool {
		return status[i].Name < status[j].Name
	})
}

// updateDomainStatus is used to update domain status with retrying.
func (c *Controller) updateDomainStatus(domain *kusciaapisv1alpha1.Domain) error {
	nlog.Infof("Update domain %v status", domain.Name)
	_, err := c.kusciaClient.KusciaV1alpha1().Domains().UpdateStatus(context.Background(), domain, apismetav1.UpdateOptions{})
	return err
}
