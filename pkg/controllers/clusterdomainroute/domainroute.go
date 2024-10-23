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
package clusterdomainroute

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func (c *controller) checkDomainRoute(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, namespace, drName string) (bool, error) {
	dr, err := c.domainRouteLister.DomainRoutes(namespace).Get(drName)
	if k8serrors.IsNotFound(err) {
		nlog.Infof("Not found domainroute %s/%s, so create it", namespace, drName)
		if _, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Create(ctx, newDomainRoute(cdr, drName, namespace), metav1.CreateOptions{}); err != nil {
			return false, err
		}
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if !metav1.IsControlledBy(dr, cdr) {
		return false, fmt.Errorf("DomainRoute %s already exists in namespace %s and is not managed by ClusterDomainRoute", drName, namespace)
	}
	if needDeleteDr(cdr, dr) {
		nlog.Infof("Delete domainroute %s/%s", namespace, drName)
		return true, c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Delete(ctx, dr.Name, metav1.DeleteOptions{})
	}

	if !compareSpec(cdr, dr) {
		drCopy := dr.DeepCopy()
		drCopy.Labels = cdr.Labels
		drCopy.Spec = cdr.Spec.DomainRouteSpec
		nlog.Infof("Update domainroute %s/%s", namespace, drName)
		if _, err = c.kusciaClient.KusciaV1alpha1().DomainRoutes(namespace).Update(ctx, drCopy, metav1.UpdateOptions{}); err != nil && !k8serrors.IsConflict(err) {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func newDomainRoute(cdr *kusciaapisv1alpha1.ClusterDomainRoute, name, namespace string) *kusciaapisv1alpha1.DomainRoute {
	return &kusciaapisv1alpha1.DomainRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    cdr.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cdr, kusciaapisv1alpha1.SchemeGroupVersion.WithKind("ClusterDomainRoute")),
			},
		},
		Spec:   cdr.Spec.DomainRouteSpec,
		Status: kusciaapisv1alpha1.DomainRouteStatus{},
	}
}

func compareSpec(cdr *kusciaapisv1alpha1.ClusterDomainRoute, dr *kusciaapisv1alpha1.DomainRoute) bool {
	if !reflect.DeepEqual(cdr.Labels, dr.Labels) {
		return false
	}

	if !reflect.DeepEqual(cdr.Spec.DomainRouteSpec, dr.Spec) {
		return false
	}

	return true
}

func (c *controller) syncStatusFromDomainroute(cdr *kusciaapisv1alpha1.ClusterDomainRoute,
	srcdr *kusciaapisv1alpha1.DomainRoute, destdr *kusciaapisv1alpha1.DomainRoute) (bool, error) {
	needUpdate := false
	cdr = cdr.DeepCopy()

	isSrcTokenChanged := srcdr != nil && !reflect.DeepEqual(cdr.Status.TokenStatus.SourceTokens, srcdr.Status.TokenStatus.Tokens)
	isSrcStatusChanged := srcdr != nil && !srcdr.Status.IsDestinationUnreachable != IsReady(&cdr.Status)

	// init new condition
	setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionTrue, "", "Success"))

	if isSrcTokenChanged || isSrcStatusChanged {
		cdr.Status.TokenStatus.SourceTokens = srcdr.Status.TokenStatus.Tokens
		needUpdate = true

		if len(cdr.Status.TokenStatus.SourceTokens) == 0 {
			if !srcdr.Status.IsDestinationAuthorized {
				setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "DestinationIsNotAuthrized", "TokenNotGenerate"))
			} else {
				setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "TokenNotGenerate", "TokenNotGenerate"))
			}
		} else if srcdr.Status.IsDestinationUnreachable {
			nlog.Infof("set cdr(%s) ready condition.reason=DestinationUnreachable", cdr.Name)
			setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse,
				"DestinationUnreachable", "DestinationUnreachable"))
		}
	}
	if destdr != nil && !reflect.DeepEqual(cdr.Status.TokenStatus.DestinationTokens, destdr.Status.TokenStatus.Tokens) {
		cdr.Status.TokenStatus.DestinationTokens = destdr.Status.TokenStatus.Tokens
		needUpdate = true
		if len(cdr.Status.TokenStatus.DestinationTokens) == 0 {
			setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "TokenNotGenerate", "TokenNotGenerate"))
		}
	}

	if IsReady(&cdr.Status) && IsTokenHeartBeatTimeout(cdr.Status.TokenStatus.DestinationTokens) {
		setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "HeartBeatTimeout", "HeartBeatTimeout"))
		needUpdate = true
	}

	if needUpdate {
		sn := len(cdr.Status.TokenStatus.SourceTokens)
		dn := len(cdr.Status.TokenStatus.DestinationTokens)
		if sn > 0 && dn > 0 && cdr.Status.TokenStatus.SourceTokens[sn-1].Revision != cdr.Status.TokenStatus.DestinationTokens[dn-1].Revision {
			setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, "TokenRevisionNotMatch", "TokenRevisionNotMatch"))
		}

		_, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().UpdateStatus(c.ctx, cdr, metav1.UpdateOptions{})
		if err != nil && !k8serrors.IsConflict(err) {
			return true, err
		}
		if err == nil {
			nlog.Infof("ClusterDomainRoute %s update status", cdr.Name)
		}
		return true, nil
	}

	return false, nil
}

func IsTokenHeartBeatTimeout(tokens []kusciaapisv1alpha1.DomainRouteToken) bool {
	readyTokens := make([]kusciaapisv1alpha1.DomainRouteToken, 0)
	for _, token := range tokens {
		if token.IsReady {
			readyTokens = append(readyTokens, token)
		}
	}
	// no ready tokens
	if len(readyTokens) <= 0 {
		return false
	}

	// heartbeatTime not exists compatible with lower versions
	noHeartbeat := true
	for _, token := range readyTokens {
		noHeartbeat = noHeartbeat && token.HeartBeatTime.Time.IsZero()
	}
	if noHeartbeat {
		return false
	}

	for _, token := range readyTokens {
		// heartbeatTime exists and not timeout
		if !token.HeartBeatTime.Time.IsZero() && time.Since(token.HeartBeatTime.Time) <= 2*common.GatewayHealthCheckDuration {
			return false
		}
	}
	// heartbeatTime exists and all timeout
	return true
}

func newCondition(condType kusciaapisv1alpha1.ClusterDomainRouteConditionType, status corev1.ConditionStatus, reason, message string) *kusciaapisv1alpha1.ClusterDomainRouteCondition {
	now := metav1.Now()
	return &kusciaapisv1alpha1.ClusterDomainRouteCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
}

func setCondition(status *kusciaapisv1alpha1.ClusterDomainRouteStatus, condition *kusciaapisv1alpha1.ClusterDomainRouteCondition) bool {
	for i, v := range status.Conditions {
		if v.Type == condition.Type {
			if v.Status == condition.Status {
				return false
			}
			status.Conditions[i] = *condition
			return true
		}
	}

	status.Conditions = append(status.Conditions, *condition)
	return true
}

func (c *controller) checkAliveInstance(ns string) error {
	gateways, err := c.gatewayLister.Gateways(ns).List(labels.Everything())
	if err != nil {
		return err
	}

	for _, g := range gateways {
		if time.Since(g.Status.HeartbeatTime.Time) < common.GatewayLiveTimeout || g.Status.HeartbeatTime.Time.IsZero() {
			return nil
		}
	}

	return fmt.Errorf("there is no live gateway instance of %s", ns)
}
