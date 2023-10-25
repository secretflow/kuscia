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
package resources

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/util/retry"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
)

// PatchDomainDataGrant is used to patch domaindatagrant.
func PatchDomainDataGrant(ctx context.Context, kusciaClient kusciaclientset.Interface, oldDdg, newDdg *kusciaapisv1alpha1.DomainDataGrant) error {
	oldData, err := json.Marshal(oldDdg)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newDdg)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &kusciaapisv1alpha1.DomainDataGrant{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for domaindatagrant %v/%v, %v", newDdg.Namespace, newDdg.Name, err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	patchFn := func() error {
		_, err = kusciaClient.KusciaV1alpha1().DomainDataGrants(newDdg.Namespace).Patch(ctx, newDdg.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// PatchDomainDataGrantStatus is used to patch domaindatagrant status.
func PatchDomainDataGrantStatus(ctx context.Context, kusciaClient kusciaclientset.Interface, oldDbg, newDbg *kusciaapisv1alpha1.DomainDataGrant) error {
	oldData, err := json.Marshal(oldDbg)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newDbg)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &kusciaapisv1alpha1.DomainDataGrant{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for domaindatagrant %v/%v, %v", newDbg.Namespace, newDbg.Name, err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	patchFn := func() error {
		_, err = kusciaClient.KusciaV1alpha1().DomainDataGrants(newDbg.Namespace).Patch(ctx, newDbg.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// ExtractDomainDataGrantAnnotations is used to extract domaindatagrant annotations.
func ExtractDomainDataGrantAnnotations(p *kusciaapisv1alpha1.DomainDataGrant) *kusciaapisv1alpha1.DomainDataGrant {
	pp := &kusciaapisv1alpha1.DomainDataGrant{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Annotations = p.Annotations
	pp.Annotations = p.Annotations
	if pp.Annotations == nil {
		pp.Annotations = map[string]string{}
	}
	return pp
}

// ExtractDomainDataGrantLabels is used to extract domainadatagrant labels.
func ExtractDomainDataGrantLabels(p *kusciaapisv1alpha1.DomainDataGrant) *kusciaapisv1alpha1.DomainDataGrant {
	pp := &kusciaapisv1alpha1.DomainDataGrant{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	if pp.Labels == nil {
		pp.Labels = map[string]string{}
	}
	return pp
}

// ExtractDomainDataGrantSpec is used to extract domaindatagrant spec.
func ExtractDomainDataGrantSpec(p *kusciaapisv1alpha1.DomainDataGrant) *kusciaapisv1alpha1.DomainDataGrant {
	pp := &kusciaapisv1alpha1.DomainDataGrant{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	if pp.Labels == nil {
		pp.Labels = map[string]string{}
	}
	pp.Annotations = p.Annotations
	if pp.Annotations == nil {
		pp.Annotations = map[string]string{}
	}
	pp.Spec = p.Spec
	return pp
}

// ExtractDomainDataGrantStatus is used to extract domaindatagrant status.
func ExtractDomainDataGrantStatus(p *kusciaapisv1alpha1.DomainDataGrant) *kusciaapisv1alpha1.DomainDataGrant {
	pp := &kusciaapisv1alpha1.DomainDataGrant{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Status = p.Status
	return pp
}
