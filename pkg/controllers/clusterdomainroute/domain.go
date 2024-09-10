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
package clusterdomainroute

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
)

func (c *controller) syncDomainPubKey(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute) (bool, error) {
	if cdr.Spec.TokenConfig != nil && (cdr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenMethodRSA || cdr.Spec.TokenConfig.TokenGenMethod == kusciaapisv1alpha1.TokenGenUIDRSA) {
		cdrCopy := cdr.DeepCopy()
		needUpdate := false
		srcRsaPub := c.getPublicKeyFromDomain(cdr.Spec.Source)
		if srcRsaPub != "" && cdr.Spec.TokenConfig.SourcePublicKey != srcRsaPub {
			cdrCopy.Spec.TokenConfig.SourcePublicKey = srcRsaPub
			needUpdate = true
		}

		destRsaPub := c.getPublicKeyFromDomain(cdr.Spec.Destination)
		if destRsaPub != "" && cdrCopy.Spec.TokenConfig.DestinationPublicKey != destRsaPub {
			cdrCopy.Spec.TokenConfig.DestinationPublicKey = destRsaPub
			needUpdate = true
		}

		if needUpdate {
			_, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{})
			if err != nil && !k8serrors.IsConflict(err) {
				return true, err
			}
			if err == nil {
				nlog.Infof("ClusterDomainRoute %s update publickey", cdr.Name)
			}
			return true, nil
		}
	}
	return false, nil
}

func (c *controller) getPublicKeyFromDomain(namespace string) string {
	domain, err := c.domainLister.Get(namespace)
	if err != nil {
		nlog.Errorf("Get domain %s error, %s ", namespace, err.Error())
		return ""
	}
	if domain.Spec.Cert != "" {
		rsaPubData, err := getPublickeyFromCert(domain.Spec.Cert)
		if err != nil {
			nlog.Errorf("Domain %s cert format error", namespace)
			return ""
		}
		return base64.StdEncoding.EncodeToString(rsaPubData)
	}
	nlog.Warnf("Domain %s cert is nil", namespace)
	return ""
}

func getPublickeyFromCert(certString string) ([]byte, error) {
	certPem, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		return nil, err
	}
	certData, _ := pem.Decode(certPem)
	if certData == nil {
		return nil, fmt.Errorf("%s", "pem Decode fail")
	}
	cert, err := x509.ParseCertificate(certData.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s", "Cant get publickey from src domain")
	}
	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(rsaPub),
	}
	return pem.EncodeToMemory(block), nil
}

func (c *controller) syncServiceToken(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute) (bool, error) {
	ns := cdr.Spec.Source
	domain, err := c.domainLister.Get(ns)
	if err != nil {
		return false, fmt.Errorf("get domain %s fail: %v", ns, err.Error())
	}

	syncFun := func() (bool, error) {
		key := common.AuthorizationHeaderName

		if cdr.Spec.RequestHeadersToAdd != nil && cdr.Spec.RequestHeadersToAdd[key] != "" {
			return false, nil
		}

		ownerRef := metav1.NewControllerRef(domain, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("Domain"))

		if err = resources.CreateOrUpdateRole(context.Background(), c.kubeClient, c.roleLister, c.rootDir, domain.Name, ownerRef); err != nil {
			return false, err
		}

		if err = resources.CreateRoleBinding(context.Background(), c.kubeClient, domain.Name, ownerRef); err != nil {
			return false, err
		}

		serviceToken, err := resources.CreateServiceToken(context.Background(), c.kubeClient, domain.Name)
		if err != nil {
			return false, err
		}

		value := fmt.Sprintf("Bearer %s", serviceToken.Status.Token)
		cdrCopy := cdr.DeepCopy()

		if cdrCopy.Spec.RequestHeadersToAdd == nil {
			cdrCopy.Spec.RequestHeadersToAdd = map[string]string{}
		}

		cdrCopy.Spec.RequestHeadersToAdd[key] = value

		_, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().Update(ctx, cdrCopy, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}

		nlog.Infof("ClusterDomainRoute %s update k3s token", cdr.Name)

		return true, nil
	}

	if domain.Spec.Role == kusciaapisv1alpha1.Partner &&
		len(domain.Spec.InterConnProtocols) > 0 &&
		domain.Spec.InterConnProtocols[0] == kusciaapisv1alpha1.InterConnKuscia {
		if domain.Spec.MasterDomain == domain.Name || domain.Spec.MasterDomain == "" {
			return syncFun()
		}
	}

	return false, nil
}

func (c *controller) getDomainRole(cdr *kusciaapisv1alpha1.ClusterDomainRoute) (kusciaapisv1alpha1.DomainRole,
	kusciaapisv1alpha1.DomainRole, error) {
	s, err := c.domainLister.Get(cdr.Spec.Source)
	if err != nil {
		nlog.Warnf("get Domain %s fail: %v", cdr.Spec.Source, err)
		return "", "", err
	}

	d, err := c.domainLister.Get(cdr.Spec.Destination)
	if err != nil {
		nlog.Warnf("get Domain %s fail: %v", cdr.Spec.Destination, err)
		return "", "", err
	}

	return s.Spec.Role, d.Spec.Role, nil
}
