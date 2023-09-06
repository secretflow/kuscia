package domain

import (
	"fmt"
	"path/filepath"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	if !shouldCreateOrUpdate(domain) {
		return nil
	}
	domainID := domain.Name
	// create service account if not exists
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainID,
		},
	}
	if _, err := c.kubeClient.CoreV1().ServiceAccounts(domainID).Create(c.ctx, sa, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
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
	if _, err := c.kubeClient.RbacV1().Roles(domainID).Create(c.ctx, role, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		nlog.Errorf("Create role [%s] error: %v", role.Name, err.Error())
		return err
	}
	// create domain roleBinding if not exists
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainID,
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
	if _, err := c.kubeClient.RbacV1().RoleBindings(domainID).Create(c.ctx, roleBinding, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		nlog.Errorf("Create roleBinding [%s] error: %v", roleBinding.Name, err.Error())
		return err
	}

	// create domain clusterRoleBinding if not exists
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainID,
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
	if _, err := c.kubeClient.RbacV1().ClusterRoleBindings().Create(c.ctx, clusterRoleBinding, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		nlog.Errorf("Create clusterRoleBinding [%s] error: %v", clusterRoleBinding.Name, err.Error())
		return err
	}

	// create domainRoute if necessary
	authCenter := domain.Spec.AuthCenter
	if c.IsMaster && authCenter != nil {
		dest := c.Namespace
		domainRoute := &kusciaapisv1alpha1.DomainRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", domainID, dest),
			},
			Spec: kusciaapisv1alpha1.DomainRouteSpec{
				Source:      domainID,
				Destination: dest,
				Endpoint: kusciaapisv1alpha1.DomainEndpoint{
					Host: dest,
					Ports: []kusciaapisv1alpha1.DomainPort{
						{
							Name:     "http",
							IsTLS:    true,
							Port:     1080,
							Protocol: kusciaapisv1alpha1.DomainRouteProtocolHTTP,
						},
					},
				},
				InterConnProtocol:  kusciaapisv1alpha1.InterConnKuscia,
				AuthenticationType: authCenter.AuthenticationType,
				TokenConfig: &kusciaapisv1alpha1.TokenConfig{
					TokenGenMethod: authCenter.TokenGenMethod,
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
		domainRoute.Spec.RequestHeadersToAdd = map[string]string{
			key: value,
		}
		// create domainRoute domain to master
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dest).Create(c.ctx, domainRoute, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
			nlog.Errorf("Create domainRoute [%s] error: %v", domainRoute.Name, err.Error())
			return err
		}
	}

	// label domain auth completed
	nlog.Infof("Domain [%s] auth init completed", domainID)
	newDomain := domain.DeepCopy()
	if newDomain.Labels == nil {
		newDomain.Labels = make(map[string]string, 0)
	}
	newDomain.Labels[constants.LabelDomainAuth] = authCompleted
	if _, err := c.kusciaClient.KusciaV1alpha1().Domains().Update(c.ctx, newDomain, metav1.UpdateOptions{}); err != nil {
		nlog.Errorf("Update domain [%s] auth label error: %s", domainID, err.Error())
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
