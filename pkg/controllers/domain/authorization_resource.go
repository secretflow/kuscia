package domain

import (
	"fmt"
	"path/filepath"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	constants "github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	authorizationHeader = "Authorization"
	serviceAccountKind  = "ServiceAccount"
	clusterRoleKind     = "ClusterRole"
	authCompleted       = "completed"
	tokenExpiredSeconds = 3650 * 24 * 3600
)

func (c *Controller) createOrUpdateAuth(domain *kusciaapisv1alpha1.Domain) error {
	ownerRef := metav1.NewControllerRef(domain, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("Domain"))
	domainID := domain.Name
	// create service account if not exists
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainID,
			OwnerReferences: []metav1.OwnerReference{
				*ownerRef,
			},
		},
	}
	if _, err := c.kubeClient.CoreV1().ServiceAccounts(domainID).Create(c.ctx, sa, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		nlog.Errorf("Create serviceAccount [%s] error: %v", sa.Name, err.Error())
		return err
	}

	// create domain role if not exists
	roleFilePath := filepath.Join(c.RootDir, "etc/conf", "domain-namespace-res.yaml")
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
	if _, err := c.kubeClient.RbacV1().Roles(domainID).Create(c.ctx, role, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		nlog.Errorf("Create role [%s] error: %v", role.Name, err.Error())
		return err
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
			Kind:     role.Kind,
			Name:     role.Name,
		},
	}
	if _, err := c.kubeClient.RbacV1().RoleBindings(domainID).Create(c.ctx, roleBinding, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		nlog.Errorf("Create roleBinding [%s] error: %v", roleBinding.Name, err.Error())
		return err
	}

	// create domain clusterRoleBinding if not exists
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
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
			Kind:     clusterRoleKind,
			Name:     "domain-cluster-res",
		},
	}
	if _, err := c.kubeClient.RbacV1().ClusterRoleBindings().Create(c.ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		nlog.Errorf("Create clusterRoleBinding [%s] error: %v", clusterRoleBinding.Name, err.Error())
		return err
	}

	// create domainRoute if necessary
	authCenter := domain.Spec.AuthCenter
	if authCenter != nil {
		dest := c.Namespace

		cdr := &kusciaapisv1alpha1.ClusterDomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:            fmt.Sprintf("%s-%s", domainID, dest),
				OwnerReferences: []metav1.OwnerReference{*ownerRef},
			},
			Spec: kusciaapisv1alpha1.ClusterDomainRouteSpec{
				DomainRouteSpec: kusciaapisv1alpha1.DomainRouteSpec{
					Source:             domainID,
					Destination:        dest,
					InterConnProtocol:  getInterConnProtocol(domain),
					AuthenticationType: authCenter.AuthenticationType,
					TokenConfig: &kusciaapisv1alpha1.TokenConfig{
						TokenGenMethod:      authCenter.TokenGenMethod,
						RollingUpdatePeriod: 600,
					},
				},
			},
		}
		// create apiServer auth token
		tokenRes, err := c.kubeClient.CoreV1().ServiceAccounts(domainID).CreateToken(c.ctx, sa.Name, &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: pointer.Int64(tokenExpiredSeconds),
			},
		}, metav1.CreateOptions{})
		if err != nil {
			nlog.Errorf("Create serviceAccount [%s] token error: %v", sa.Name, err.Error())
			return err
		}
		key, value := buildAuthorizationHeader(tokenRes.Status.Token)
		cdr.Spec.RequestHeadersToAdd = map[string]string{
			key: value,
		}
		// create clusterDomainRoute domain to master
		if _, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Create(c.ctx, cdr, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
			nlog.Errorf("Create clusterDomainRoute [%s] error: %v", cdr.Name, err.Error())
			return err
		}
		nlog.Infof("Create clusterDomainRoute [%s] success", cdr.Name)
	}

	// label domain auth completed
	nlog.Infof("Domain [%s] auth init completed", domainID)
	newDomain := domain.DeepCopy()
	if newDomain.Labels == nil {
		newDomain.Labels = make(map[string]string, 0)
	}
	newDomain.Labels[constants.LabelDomainAuth] = authCompleted
	if _, err := c.kusciaClient.KusciaV1alpha1().Domains().Update(c.ctx, newDomain, metav1.UpdateOptions{}); err != nil && !k8serrors.IsConflict(err) {
		nlog.Warnf("Update domain [%s] auth label error: %s", domainID, err.Error())
		return err
	}
	return nil
}

func buildAuthorizationHeader(token string) (string, string) {
	return authorizationHeader, fmt.Sprintf("Bearer %s", token)
}

func shouldCreateOrUpdate(domain *kusciaapisv1alpha1.Domain) bool {
	labels := domain.Labels
	if labels == nil {
		return true
	}
	val, ok := labels[constants.LabelDomainAuth]
	if ok {
		return val != authCompleted
	}
	return true
}

func getInterConnProtocol(domain *kusciaapisv1alpha1.Domain) kusciaapisv1alpha1.InterConnProtocolType {
	if domain.Spec.Role == kusciaapisv1alpha1.Partner && len(domain.Spec.InterConnProtocols) > 0 &&
		domain.Spec.InterConnProtocols[0] != kusciaapisv1alpha1.InterConnKuscia {
		return domain.Spec.InterConnProtocols[0]
	}
	return kusciaapisv1alpha1.InterConnKuscia
}

// send-content/ds
