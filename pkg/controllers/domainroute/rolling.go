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

package domainroute

import (
	"context"
	"fmt"
	mrand "math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func (c *controller) preRollingSourceDomainRoute(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	initializer, err := c.selectInitializer(ctx, dr)
	if err != nil {
		nlog.Warnf("Choose initializer for preRollingSourceDomainRoute %s fail: %v", dr.Name, err)
		return err
	}

	dr = dr.DeepCopy()
	dr.Status.TokenStatus.RevisionInitializer = initializer
	dr.Status.TokenStatus.RevisionToken = kusciaapisv1alpha1.DomainRouteToken{
		RevisionTime: metav1.Time{Time: time.Now()},
		Revision:     dr.Status.TokenStatus.RevisionToken.Revision,
		IsReady:      false,
	}
	_, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{})
	if err == nil {
		nlog.Infof("PreRollingDomainRoute %s/%s, new revision %d", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision)
	}
	return err
}

func (c *controller) selectInitializer(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) (string, error) {
	// randomly choose a gateway from source namespace as revision intializer
	gateways, err := c.gatewayLister.Gateways(dr.Namespace).List(labels.Everything())
	if err != nil {
		return "", err
	}
	var liveGateways []string
	for _, g := range gateways {
		if time.Since(g.Status.HeartbeatTime.Time) < common.GatewayLiveTimeout || g.Status.HeartbeatTime.Time.IsZero() {
			if g.Name == dr.Status.TokenStatus.RevisionInitializer {
				return g.Name, nil
			}
			liveGateways = append(liveGateways, g.Name)
		}
	}

	if len(liveGateways) == 0 {
		return "", fmt.Errorf("there is no live gateway instance of %s", dr.Namespace)
	}

	initializer := liveGateways[mrand.Intn(len(liveGateways))]
	return initializer, nil
}

func (c *controller) ensureInitializer(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) (bool, error) {
	// randomly choose a gateway from source namespace as revision intializer
	initializer, err := c.selectInitializer(ctx, dr)
	if err != nil || initializer == dr.Status.TokenStatus.RevisionInitializer {
		return false, err
	}

	dr = dr.DeepCopy()
	dr.Status.TokenStatus.RevisionInitializer = initializer
	_, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{})
	if err != nil {
		return false, err
	}
	if err == nil {
		nlog.Infof("domainroute %s/%s select initializer %s", dr.Namespace, dr.Name, initializer)
	}
	return true, nil
}

func (c *controller) postRollingSourceDomainRoute(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	n := len(dr.Status.TokenStatus.Tokens)
	if n == 0 || dr.Status.TokenStatus.Tokens[n-1].Revision != dr.Status.TokenStatus.RevisionToken.Revision {
		dr = dr.DeepCopy()
		dr.Status.TokenStatus.Tokens = append(dr.Status.TokenStatus.Tokens, dr.Status.TokenStatus.RevisionToken)
		if n > 1 {
			dr.Status.TokenStatus.Tokens = dr.Status.TokenStatus.Tokens[n-1:]
		}
		_, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{})
		if err == nil {
			nlog.Infof("Rolling update source domainroute %s/%s finish, revision %d", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision)
		}
		return err
	}
	return nil
}

func (c *controller) postRollingDestinationDomainRoute(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	n := len(dr.Status.TokenStatus.Tokens)

	if time.Since(dr.Status.TokenStatus.RevisionToken.ExpirationTime.Time) < 0 &&
		dr.Status.TokenStatus.RevisionToken.Token != "" &&
		(n == 0 || dr.Status.TokenStatus.Tokens[n-1].Revision != dr.Status.TokenStatus.RevisionToken.Revision) {
		dr = dr.DeepCopy()
		dr.Status.TokenStatus.Tokens = append(dr.Status.TokenStatus.Tokens, dr.Status.TokenStatus.RevisionToken)
		_, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{})
		if err == nil {
			nlog.Infof("Post rolling update destination domainroute %s/%s, revision %d, waiting for all instance sync token", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision)
		}
		return err
	}

	if !dr.Status.TokenStatus.RevisionToken.IsReady && c.checkEffectiveInstances(dr) {
		dr = dr.DeepCopy()
		n = len(dr.Status.TokenStatus.Tokens)
		dr.Status.TokenStatus.Tokens[n-1].IsReady = true
		dr.Status.TokenStatus.RevisionToken.IsReady = true
		_, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{})
		if err == nil {
			// update source after all instances in destination have taken effect
			nlog.Infof("Rolling update destination domainroute %s/%s finish, revision %d", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision)
		}
		return err

	}
	return nil
}
