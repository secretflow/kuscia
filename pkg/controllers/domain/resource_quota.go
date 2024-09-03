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
	"strconv"

	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// createResourceQuota is used to create resource quota for domain.
func (c *Controller) createResourceQuota(domain *kusciaapisv1alpha1.Domain) error {
	resourceQuota := c.makeResourceQuota(domain)
	if resourceQuota == nil {
		return nil
	}

	return c.createOrUpdateResourceQuota(resourceQuota, true)
}

// updateResourceQuota is used to update resource quota for domain.
func (c *Controller) updateResourceQuota(domain *kusciaapisv1alpha1.Domain) error {
	ns, err := c.namespaceLister.Get(domain.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if ns.DeletionTimestamp != nil {
		return nil
	}

	oldRQ, err := c.resourceQuotaLister.ResourceQuotas(domain.Name).Get(resourceQuotaName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			resourceQuota := c.makeResourceQuota(domain)
			if resourceQuota == nil {
				return nil
			}
			return c.createOrUpdateResourceQuota(resourceQuota, true)
		}
		return err
	}

	rq := domain.Spec.ResourceQuota
	if rq == nil {
		return c.deleteResourceQuota(resourceQuotaName, domain.Name)
	}

	newRL := c.buildResourceList(rq)
	if c.needUpdateResourceList(oldRQ.Spec.Hard, newRL) {
		resourceQuota := c.makeResourceQuota(domain)
		return c.createOrUpdateResourceQuota(resourceQuota, false)
	}
	return nil
}

// makeResourceQuota is used to make resource quota.
func (c *Controller) makeResourceQuota(domain *kusciaapisv1alpha1.Domain) *apicorev1.ResourceQuota {
	if domain == nil ||
		domain.Spec.ResourceQuota == nil ||
		domain.Spec.ResourceQuota.PodMaxCount == nil {
		return nil
	}

	return &apicorev1.ResourceQuota{
		ObjectMeta: apismetav1.ObjectMeta{
			Name:      resourceQuotaName,
			Namespace: domain.Name,
			Labels: map[string]string{
				common.LabelDomainName: domain.Name,
			},
		},
		Spec: apicorev1.ResourceQuotaSpec{
			Hard: c.buildResourceList(domain.Spec.ResourceQuota),
		},
	}
}

// buildResourceList is used to build resource list for resource quota.
func (c *Controller) buildResourceList(rq *kusciaapisv1alpha1.DomainResourceQuota) apicorev1.ResourceList {
	rl := apicorev1.ResourceList{}
	if rq != nil && rq.PodMaxCount != nil {
		rl[apicorev1.ResourcePods] = resource.MustParse(strconv.Itoa(*rq.PodMaxCount))
	}

	return rl
}

// needUpdateResourceList is used to check if need to update resource list.
func (c *Controller) needUpdateResourceList(oldRL, newRL apicorev1.ResourceList) bool {
	if len(oldRL) != len(newRL) {
		return true
	}

	for k, v := range newRL {
		if !oldRL[k].Equal(v) {
			return true
		}
	}
	return false
}

// createOrUpdateResourceQuota is used to creat or update resource quota.
func (c *Controller) createOrUpdateResourceQuota(rq *apicorev1.ResourceQuota, isCreated bool) error {
	if rq == nil {
		return nil
	}

	var err error
	switch isCreated {
	case true:
		nlog.Infof("Create resource quota %v under domain %v", rq.Name, rq.Namespace)
		_, err = c.kubeClient.CoreV1().ResourceQuotas(rq.Namespace).Create(context.Background(), rq, apismetav1.CreateOptions{})
		if k8serrors.IsAlreadyExists(err) {
			return nil
		}
	default:
		nlog.Infof("Update resource quota %v under domain %v", rq.Name, rq.Namespace)
		_, err = c.kubeClient.CoreV1().ResourceQuotas(rq.Namespace).Update(context.Background(), rq, apismetav1.UpdateOptions{})
	}
	return err
}

// deleteResourceQuota is used to delete resource quota with retrying.
func (c *Controller) deleteResourceQuota(name, namespace string) error {
	nlog.Infof("Delete resource quota %v under %v domain", resourceQuotaName, namespace)

	err := c.kubeClient.CoreV1().ResourceQuotas(namespace).Delete(context.Background(), name, apismetav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}
