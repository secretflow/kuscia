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

package resources

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rbaclisters "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/utils/pointer"

	"github.com/secretflow/kuscia/pkg/utils/common"
)

const (
	serviceAccountKind  = "ServiceAccount"
	tokenExpiredSeconds = 3650 * 24 * 3600
)

func CreateServiceToken(ctx context.Context, client kubernetes.Interface, domainID string) (*authenticationv1.TokenRequest, error) {
	tokenRes, err := client.CoreV1().ServiceAccounts(domainID).CreateToken(ctx, domainID, &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: pointer.Int64(tokenExpiredSeconds),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create service account %q token, detail-> %v", domainID, err)
	}
	return tokenRes, nil

}

func CreateRoleBinding(ctx context.Context, client kubernetes.Interface, domainID string, ownerRef *metav1.OwnerReference) error {
	// create service account if not exists
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainID,
			OwnerReferences: []metav1.OwnerReference{
				*ownerRef,
			},
		},
	}

	if _, err := client.CoreV1().ServiceAccounts(domainID).Create(ctx, sa, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create service account %q, detail-> %v", sa, err)
	}

	// create domain roleBinding if not exists
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            domainID,
			OwnerReferences: []metav1.OwnerReference{*ownerRef},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      serviceAccountKind,
				Name:      domainID,
				Namespace: domainID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     domainID,
		},
	}
	if _, err := client.RbacV1().RoleBindings(domainID).Create(ctx, roleBinding, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create role binding %q, detail-> %v", roleBinding.Name, err)
	}
	return nil
}

func CreateOrUpdateRole(ctx context.Context,
	client kubernetes.Interface,
	roleLister rbaclisters.RoleLister,
	rootDir, domainID string,
	ownerRef *metav1.OwnerReference) error {
	roleFilePath := filepath.Join(rootDir, "etc/conf", "domain-namespace-res.yaml")
	role := &rbacv1.Role{}
	input := struct {
		DomainID string
	}{
		DomainID: domainID,
	}
	if err := common.RenderRuntimeObject(roleFilePath, role, input); err != nil {
		return err
	}
	role.OwnerReferences = append(role.OwnerReferences, *ownerRef)

	curRole, err := roleLister.Roles(domainID).Get(role.Name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}

		_, err = client.RbacV1().Roles(domainID).Create(ctx, role, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create role %q, detail-> %v", role.Name, err)
		}
		return nil
	}

	if reflect.DeepEqual(curRole.Rules, role.Rules) {
		return nil
	}

	_, err = client.RbacV1().Roles(domainID).Update(ctx, role, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update role %q, detail-> %v", role.Name, err)
	}
	return nil
}
