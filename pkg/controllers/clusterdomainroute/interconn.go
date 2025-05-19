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

package clusterdomainroute

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func (c *controller) checkInteropConfig(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute,
	sourceRole, destRole kusciaapisv1alpha1.DomainRole) error {
	configName := getInteropConfigName(cdr.Spec.Source, cdr.Spec.Destination)
	hasCreatedInteropConfig, err := c.hasCreatedInteropConfig(configName)
	if err != nil {
		return err
	}

	needCreate, err := c.needCreateInteropConfig(cdr, sourceRole, destRole)
	if err != nil {
		return err
	}

	if !needCreate {
		if hasCreatedInteropConfig {
			return c.deleteInteropConfig(ctx, cdr)
		}
		return nil
	}

	// needCreateInteropConfig
	if hasCreatedInteropConfig || !isTimeToCreateInteropConfig(cdr) {
		return nil
	}

	interopConfig := &kusciaapisv1alpha1.InteropConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cdr, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("ClusterDomainRoute")),
			},
		},
		Spec: kusciaapisv1alpha1.InteropConfigSpec{
			Host: cdr.Spec.Destination,
			Members: []string{
				cdr.Spec.Source,
			},
		},
	}

	if _, err := c.kusciaClient.KusciaV1alpha1().InteropConfigs().Create(ctx, interopConfig,
		metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		nlog.Warnf("Create InteropConfig(%s) fail: %v", configName, err)
		return err
	}

	nlog.Infof("Create InteropConfig(%s) success", configName)
	return nil
}

func (c *controller) deleteInteropConfig(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute) error {
	configName := getInteropConfigName(cdr.Spec.Source, cdr.Spec.Destination)
	if err := c.kusciaClient.KusciaV1alpha1().InteropConfigs().Delete(ctx, configName,
		metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		nlog.Warnf("Delete InteropConfig(%s) fail: %v", configName, err)
		return err
	}
	nlog.Infof("Delete InteropConfig(%s) success", configName)
	return nil
}

func (c *controller) hasCreatedInteropConfig(configName string) (bool, error) {
	_, err := c.interopLister.Get(configName)

	if err == nil {
		return true, nil
	}

	if k8serrors.IsNotFound(err) {
		return false, nil
	}

	return false, err
}

func (c *controller) needCreateInteropConfig(cdr *kusciaapisv1alpha1.ClusterDomainRoute,
	sourceRole, destRole kusciaapisv1alpha1.DomainRole) (bool, error) {
	if sourceRole == kusciaapisv1alpha1.Partner || destRole != kusciaapisv1alpha1.Partner {
		return false, nil
	}
	domain, err := c.kusciaClient.KusciaV1alpha1().Domains().Get(c.ctx, cdr.Spec.Destination, metav1.GetOptions{})
	if err != nil {
		nlog.Warnf("get Domain %s fail: %v", cdr.Spec.Destination, err)
		return false, err
	}
	// BFIA doesn't need interop
	if domain.Spec.Role == kusciaapisv1alpha1.Partner && domain.Spec.MasterDomain == domain.Name && cdr.Spec.InterConnProtocol != kusciaapisv1alpha1.InterConnBFIA {
		return true, nil
	}
	return false, nil
}

func getInteropConfigName(source, dest string) string {
	return fmt.Sprintf("%s-2-%s", source, dest)
}

func isTimeToCreateInteropConfig(cdr *kusciaapisv1alpha1.ClusterDomainRoute) bool {
	return !needGenerateToken(cdr) || len(cdr.Status.TokenStatus.SourceTokens) > 0
}

func needGenerateToken(cdr *kusciaapisv1alpha1.ClusterDomainRoute) bool {
	return cdr.Spec.AuthenticationType == kusciaapisv1alpha1.DomainAuthenticationToken || cdr.Spec.BodyEncryption != nil
}
