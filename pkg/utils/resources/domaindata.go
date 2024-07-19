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

//nolint:dupl
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

// PatchDomainData is used to patch domaindata.
func PatchDomainData(ctx context.Context, kusciaClient kusciaclientset.Interface, oldDd, newDd *kusciaapisv1alpha1.DomainData) error {
	oldData, err := json.Marshal(oldDd)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newDd)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &kusciaapisv1alpha1.DomainData{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for domaindata %v/%v, %v", newDd.Namespace, newDd.Name, err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	patchFn := func() error {
		_, err = kusciaClient.KusciaV1alpha1().DomainDatas(newDd.Namespace).Patch(ctx, newDd.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// PatchDomainDataStatus is used to patch domaindata status.
func PatchDomainDataStatus(ctx context.Context, kusciaClient kusciaclientset.Interface, oldDb, newDb *kusciaapisv1alpha1.DomainData) error {
	oldData, err := json.Marshal(oldDb)
	if err != nil {
		return err
	}

	newData, err := json.Marshal(newDb)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &kusciaapisv1alpha1.DomainData{})
	if err != nil {
		return fmt.Errorf("failed to create merge patch for domaindata %v/%v, %v", newDb.Namespace, newDb.Name, err)
	}

	if string(patchBytes) == "{}" {
		return nil
	}

	patchFn := func() error {
		_, err = kusciaClient.KusciaV1alpha1().DomainDatas(newDb.Namespace).Patch(ctx, newDb.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
		return err
	}

	return retry.OnError(retry.DefaultBackoff, net.IsConnectionRefused, patchFn)
}

// ExtractDomainDataAnnotations is used to extract domaindata annotations.
func ExtractDomainDataAnnotations(p *kusciaapisv1alpha1.DomainData) *kusciaapisv1alpha1.DomainData {
	pp := &kusciaapisv1alpha1.DomainData{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Annotations = p.Annotations
	if pp.Annotations == nil {
		pp.Annotations = map[string]string{}
	}
	return pp
}

// ExtractDomainDataLabels is used to extract domainadata labels.
func ExtractDomainDataLabels(p *kusciaapisv1alpha1.DomainData) *kusciaapisv1alpha1.DomainData {
	pp := &kusciaapisv1alpha1.DomainData{}
	pp.Namespace = p.Namespace
	pp.Name = p.Name
	pp.Labels = p.Labels
	if pp.Labels == nil {
		pp.Labels = map[string]string{}
	}
	return pp
}

// ExtractDomainDataSpec is used to extract domainData spec.
func ExtractDomainDataSpec(p *kusciaapisv1alpha1.DomainData) *kusciaapisv1alpha1.DomainData {
	pp := &kusciaapisv1alpha1.DomainData{}
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
