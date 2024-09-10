// Copyright 2024 Ant Group Co., Ltd.
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

	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// createDomainConfig is used to create domain config.
func (c *Controller) createDomainConfig(domain *kusciaapisv1alpha1.Domain) error {
	_, err := c.configmapLister.ConfigMaps(domain.Name).Get(domainConfigName)
	if err != nil && k8serrors.IsNotFound(err) {
		cm := c.makeDomainConfig(domain)
		nlog.Infof("Create domain config %v under domain %v", cm.Name, cm.Namespace)
		_, err = c.kubeClient.CoreV1().ConfigMaps(cm.Namespace).Create(context.Background(), cm, apismetav1.CreateOptions{})
		if k8serrors.IsAlreadyExists(err) {
			return nil
		}
	}
	return err
}

// makeDomainConfig is used to make domain config.
func (c *Controller) makeDomainConfig(domain *kusciaapisv1alpha1.Domain) *apicorev1.ConfigMap {
	return &apicorev1.ConfigMap{
		ObjectMeta: apismetav1.ObjectMeta{
			Name:      domainConfigName,
			Namespace: domain.Name,
			Labels: map[string]string{
				common.LabelDomainName: domain.Name,
			},
		},
		Data: map[string]string{},
	}
}
