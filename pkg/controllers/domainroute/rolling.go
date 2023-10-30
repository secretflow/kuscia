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

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func (c *controller) preRollingSourceDomainRoute(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute, force bool) error {
	// randomly choose a gateway from source namespace as revision intializer
	gateways, err := c.gatewayLister.Gateways(dr.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	var liveGateways []string
	for i := range gateways {
		if time.Since(gateways[i].Status.HeartbeatTime.Time) < gatewayLiveTimeout {
			liveGateways = append(liveGateways, gateways[i].Name)
		}
	}
	if len(liveGateways) == 0 {
		nlog.Warnf("No available gateway in source namespace %s", dr.Namespace)
		return nil
	}
	initializer := liveGateways[mrand.Intn(len(liveGateways))]

	dr = dr.DeepCopy()
	dr.Status.TokenStatus.RevisionToken = kusciaapisv1alpha1.DomainRouteToken{
		RevisionTime: metav1.Time{Time: time.Now()},
		Revision:     dr.Status.TokenStatus.RevisionToken.Revision + 1,
	}
	dr.Status.TokenStatus.RevisionInitializer = initializer

	msg := fmt.Sprintf("PreRollingDomainRoute %s/%s, new revision %d, initializer: %s", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision, initializer)
	nlog.Infof(msg)
	if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *controller) postRollingSourceDomainRoute(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	n := len(dr.Status.TokenStatus.Tokens)
	if n == 0 || dr.Status.TokenStatus.Tokens[n-1].Revision != dr.Status.TokenStatus.RevisionToken.Revision {
		dr = dr.DeepCopy()
		dr.Status.TokenStatus.Tokens = append(dr.Status.TokenStatus.Tokens, dr.Status.TokenStatus.RevisionToken)
		if n > 1 {
			dr.Status.TokenStatus.Tokens = dr.Status.TokenStatus.Tokens[n-1:]
		}
		msg := fmt.Sprintf("Rolling update source domainroute %s/%s finish, revision %d", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision)
		nlog.Info(msg)
		_, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{})
		if err != nil {
			nlog.Error(err)
			return err
		}
	}
	return nil
}

func (c *controller) postRollingDestinationDomainRoute(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	n := len(dr.Status.TokenStatus.Tokens)
	if dr.Status.TokenStatus.RevisionToken.Token != "" && (n == 0 || dr.Status.TokenStatus.Tokens[n-1].Revision != dr.Status.TokenStatus.RevisionToken.Revision) {
		dr = dr.DeepCopy()
		dr.Status.TokenStatus.Tokens = append(dr.Status.TokenStatus.Tokens, dr.Status.TokenStatus.RevisionToken)

		msg := fmt.Sprintf("Post rolling update destination domainroute %s/%s, revision %d, waiting for all instance sync token", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision)
		nlog.Info(msg)
		_, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{})
		if err != nil {
			nlog.Error(err)
			return err
		}
	}

	if !dr.Status.TokenStatus.RevisionToken.IsReady && c.checkEffectiveInstances(dr) {
		n = len(dr.Status.TokenStatus.Tokens)
		dr.Status.TokenStatus.Tokens[n-1].IsReady = true
		dr.Status.TokenStatus.RevisionToken.IsReady = true
		// update source after all instances in destination have taken effect
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr, metav1.UpdateOptions{}); err != nil {
			nlog.Error(err)
			return err
		}
		msg := fmt.Sprintf("Rolling update destination domainroute %s/%s finish, revision %d", dr.Namespace, dr.Name, dr.Status.TokenStatus.RevisionToken.Revision)
		nlog.Debug(msg)
	}
	return nil
}
