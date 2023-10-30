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
	"fmt"
	"reflect"

	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// createNamespace is used to create domain namespace.
func (c *Controller) createNamespace(domain *kusciaapisv1alpha1.Domain) error {
	nlog.Infof("Create domain namespace %v", domain.Name)
	ns, err := c.namespaceLister.Get(domain.Name)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("get namespace %v for domain failed, %v", domain.Name, err.Error())
	}

	if ns != nil {
		nlog.Infof("Namespace %v already exist, update it by domain definition", domain.Name)
		return c.updateNamespace(domain)
	}

	ns = &apicorev1.Namespace{
		ObjectMeta: apismetav1.ObjectMeta{
			Name: domain.Name,
			Labels: map[string]string{
				common.LabelDomainName: domain.Name,
			},
		},
	}

	if domain.Spec.Role != "" {
		ns.Labels[common.LabelDomainRole] = string(domain.Spec.Role)
	}

	// Todo: Currently, only one protocol is supported, and in the future, multiple protocols will be supported
	if len(domain.Spec.InterConnProtocols) > 0 && domain.Spec.InterConnProtocols[0] != "" {
		ns.Labels[common.LabelInterConnProtocols] = string(domain.Spec.InterConnProtocols[0])
	}

	_, err = c.kubeClient.CoreV1().Namespaces().Create(context.Background(), ns, apismetav1.CreateOptions{})
	if err != nil && k8serrors.IsAlreadyExists(err) {
		nlog.Warnf("Namespace %v already exist for domain %v, skip to create it", ns.Name, domain.Name)
		return nil
	}
	return err
}

// updateNamespace is used to update domain namespace.
func (c *Controller) updateNamespace(domain *kusciaapisv1alpha1.Domain) error {
	nlog.Infof("Update domain namespace %v", domain.Name)

	ns, err := c.namespaceLister.Get(domain.Name)
	if err != nil {
		return err
	}

	if ns.DeletionTimestamp != nil {
		return nil
	}

	nsCopy := ns.DeepCopy()
	if nsCopy.Labels == nil {
		nsCopy.Labels = make(map[string]string)
	}

	delete(nsCopy.Labels, common.LabelDomainDeleted)

	if _, exist := nsCopy.Labels[common.LabelDomainName]; !exist {
		nsCopy.Labels[common.LabelDomainName] = domain.Name
	}

	updateNamespaceLabel(nsCopy, string(domain.Spec.Role), common.LabelDomainRole)

	newInterConnProtocols := ""
	if len(domain.Spec.InterConnProtocols) > 0 && domain.Spec.InterConnProtocols[0] != "" {
		newInterConnProtocols = string(domain.Spec.InterConnProtocols[0])
	}
	updateNamespaceLabel(nsCopy, newInterConnProtocols, common.LabelInterConnProtocols)

	if !reflect.DeepEqual(ns.Labels, nsCopy.Labels) {
		_, err = c.kubeClient.CoreV1().Namespaces().Update(context.Background(), nsCopy, apismetav1.UpdateOptions{})
		return err
	}

	return nil
}

func updateNamespaceLabel(ns *apicorev1.Namespace, newLabelValue string, labelName string) {
	oldLabelValue := ns.Labels[labelName]
	if newLabelValue != "" && newLabelValue != oldLabelValue {
		ns.Labels[labelName] = newLabelValue
	} else if newLabelValue == "" {
		delete(ns.Labels, labelName)
	}
}

// deleteNamespace is used to delete domain namespace.
// Currently, only a special label is marked on the namespace, rather than deleting the namespace.
func (c *Controller) deleteNamespace(name string) error {
	nlog.Infof("Delete domain namespace %v", name)

	ns, err := c.namespaceLister.Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if ns.Labels != nil {
		if _, exist := ns.Labels[common.LabelDomainDeleted]; exist {
			return nil
		}
	}

	ns = ns.DeepCopy()
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}

	ns.Labels[common.LabelDomainDeleted] = "true"

	_, err = c.kubeClient.CoreV1().Namespaces().Update(context.Background(), ns, apismetav1.UpdateOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}
