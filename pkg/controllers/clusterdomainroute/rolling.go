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
	crand "crypto/rand"
	"encoding/base64"
	"fmt"
	mrand "math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	doValidateReason             = "DoValidate"
	syncDomainPubKeyReason       = "SyncDomainPubKey"
	specificationUpdateReason    = "SpecificationUpdate"
	preRollingUpdateReason       = "PreRollingUpdate"
	intraRollingUpdateReason     = "IntraRollingUpdate"
	postRollingUpdateReason      = "PostRollingUpdate"
	sourceTokenUpdateReason      = "SoureceTokenUpdate"
	destinationTokenUpdateReason = "DestinationTokenUpdate"
)

func (c *Controller) preRollingClusterDomainRoute(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, force bool) error {
	cdr = cdr.DeepCopy()
	cdr.Status.TokenStatus.Revision = cdr.Status.TokenStatus.Revision + 1
	cdr.Status.TokenStatus.RevisionTime = metav1.Time{Time: time.Now()}

	msg := fmt.Sprintf("Clusterdomainroute rolling to next revision %d", cdr.Status.TokenStatus.Revision)
	setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteRunning, corev1.ConditionTrue, preRollingUpdateReason, msg))

	// Reset ready condition to false if specification has changed.
	if force {
		msg = "ClusterDomainRoute Specification has changed"
		setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionFalse, specificationUpdateReason, msg))
	}

	nlog.Infof("PreRollingClusterDomainRoute %s, new revision %d", cdr.Name, cdr.Status.TokenStatus.Revision)
	if _, err := c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().UpdateStatus(ctx, cdr, metav1.UpdateOptions{}); err != nil {
		return err
	}

	c.recorder.Event(cdr, corev1.EventTypeNormal, preRollingUpdateReason, msg)
	return nil
}

func (c *Controller) intraRollingClusterDomainRouteRand(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, sourcedr, destdr *kusciaapisv1alpha1.DomainRoute) error {
	revisionToken := kusciaapisv1alpha1.DomainRouteToken{
		Revision:     cdr.Status.TokenStatus.Revision,
		RevisionTime: cdr.Status.TokenStatus.RevisionTime,
	}

	// randomly generate token for this revision
	if cdr.Status.TokenStatus.Revision != sourcedr.Status.TokenStatus.RevisionToken.Revision && cdr.Status.TokenStatus.Revision != destdr.Status.TokenStatus.RevisionToken.Revision {
		begin := time.Now()
		b := make([]byte, 32)
		_, err := crand.Read(b)
		if err != nil {
			return err
		}
		revisionToken.Token = base64.StdEncoding.EncodeToString(b)
		nlog.Infof("ClusterDomain %s generate new token, use:%s", cdr.Name, time.Since(begin).String())
	} else if cdr.Status.TokenStatus.Revision == sourcedr.Status.TokenStatus.RevisionToken.Revision {
		revisionToken.Token = sourcedr.Status.TokenStatus.RevisionToken.Token
	} else {
		revisionToken.Token = destdr.Status.TokenStatus.RevisionToken.Token
	}

	if cdr.Status.TokenStatus.Revision != sourcedr.Status.TokenStatus.RevisionToken.Revision {
		sourcedr = sourcedr.DeepCopy()
		sourcedr.Status.TokenStatus.RevisionToken = revisionToken
		nlog.Debugf("Intra rolling update domainroute %s/%s", sourcedr.Namespace, sourcedr.Name)
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(sourcedr.Namespace).UpdateStatus(ctx, sourcedr, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	if cdr.Status.TokenStatus.Revision != destdr.Status.TokenStatus.RevisionToken.Revision {
		destdr = destdr.DeepCopy()
		destdr.Status.TokenStatus.RevisionToken = revisionToken
		nlog.Debugf("Intra rolling update domainroute %s/%s", destdr.Namespace, destdr.Name)
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(destdr.Namespace).UpdateStatus(ctx, destdr, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	msg := fmt.Sprintf("Source and destination domainroute rolling to next revision %d", cdr.Status.TokenStatus.Revision)
	c.recorder.Event(cdr, corev1.EventTypeNormal, intraRollingUpdateReason, msg)
	nlog.Info(msg)
	return nil
}

func (c *Controller) intraRollingClusterDomainRouteRSA(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, sourcedr, destdr *kusciaapisv1alpha1.DomainRoute) error {
	revisionToken := kusciaapisv1alpha1.DomainRouteToken{
		Revision:     cdr.Status.TokenStatus.Revision,
		RevisionTime: cdr.Status.TokenStatus.RevisionTime,
	}

	// randomly choose a gateway from source namespace as revision intializer
	gateways, err := c.gatewayLister.Gateways(cdr.Spec.Source).List(labels.Everything())
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
		nlog.Warnf("No available gateway in source namespace %s", cdr.Spec.Source)
		return nil
	}
	initializer := liveGateways[mrand.Intn(len(liveGateways))]

	if cdr.Status.TokenStatus.Revision != sourcedr.Status.TokenStatus.RevisionToken.Revision {
		sourcedr = sourcedr.DeepCopy()
		sourcedr.Status.TokenStatus.RevisionToken = revisionToken
		sourcedr.Status.TokenStatus.RevisionInitializer = initializer
		nlog.Infof("Intra rolling update domainroute %s/%s, revision: %d, initializer: %s", sourcedr.Namespace, sourcedr.Name, cdr.Status.TokenStatus.Revision, initializer)
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(cdr.Spec.Source).UpdateStatus(ctx, sourcedr, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	if cdr.Status.TokenStatus.Revision != destdr.Status.TokenStatus.RevisionToken.Revision {
		destdr = destdr.DeepCopy()
		destdr.Status.TokenStatus.RevisionToken = revisionToken
		nlog.Infof("Intra rolling update domainroute %s/%s, revision: %d", destdr.Namespace, destdr.Name, cdr.Status.TokenStatus.Revision)
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(cdr.Spec.Destination).UpdateStatus(ctx, destdr, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	msg := fmt.Sprintf("Source and destination domainroute rolling to next revision %d", cdr.Status.TokenStatus.Revision)
	c.recorder.Event(cdr, corev1.EventTypeNormal, intraRollingUpdateReason, msg)
	nlog.Info(msg)
	return nil
}

func (c *Controller) postRollingClusterDomainRoute(ctx context.Context, cdr *kusciaapisv1alpha1.ClusterDomainRoute, sourcedr, destdr *kusciaapisv1alpha1.DomainRoute) error {
	n := len(cdr.Status.TokenStatus.SourceTokens)
	if n == 0 || cdr.Status.TokenStatus.SourceTokens[n-1].Revision != cdr.Status.TokenStatus.Revision || !checkReadyCondition(cdr) {
		cdr = cdr.DeepCopy()
		sourceTokens := append(cdr.Status.TokenStatus.SourceTokens, sourcedr.Status.TokenStatus.RevisionToken)
		destinationTokens := append(cdr.Status.TokenStatus.DestinationTokens, destdr.Status.TokenStatus.RevisionToken)

		if n > 0 {
			sourceTokens = sourceTokens[n-1:]
			destinationTokens = destinationTokens[n-1:]
		}

		cdr.Status.TokenStatus.SourceTokens = sourceTokens
		cdr.Status.TokenStatus.DestinationTokens = destinationTokens

		msg := fmt.Sprintf("Clusterdomainroute finish rolling revision %d", cdr.Status.TokenStatus.Revision)
		setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRoutePending, corev1.ConditionTrue, postRollingUpdateReason, msg))
		setCondition(&cdr.Status, newCondition(kusciaapisv1alpha1.ClusterDomainRouteReady, corev1.ConditionTrue, postRollingUpdateReason, msg))

		nlog.Infof("Post rolling update clusterdomainroute %s, revision %d", cdr.Name, cdr.Status.TokenStatus.Revision)
		var err error
		if cdr, err = c.kusciaClient.KusciaV1alpha1().ClusterDomainRoutes().UpdateStatus(ctx, cdr, metav1.UpdateOptions{}); err != nil {
			nlog.Error(err)
			return err
		}

		c.recorder.Event(cdr, corev1.EventTypeNormal, postRollingUpdateReason, msg)
	}

	// update destination in advance
	if !compareTokens(cdr.Status.TokenStatus.DestinationTokens, destdr.Status.TokenStatus.Tokens) {
		destdr = destdr.DeepCopy()
		destdr.Status.TokenStatus.Tokens = cdr.Status.TokenStatus.DestinationTokens
		msg := fmt.Sprintf("Post rolling update destination domainroute %s/%s, revision %d", destdr.Namespace, destdr.Name, destdr.Status.TokenStatus.RevisionToken.Revision)
		nlog.Debug(msg)
		if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(cdr.Spec.Destination).UpdateStatus(ctx, destdr, metav1.UpdateOptions{}); err != nil {
			nlog.Error(err)
			return err
		}
		c.recorder.Event(cdr, corev1.EventTypeNormal, destinationTokenUpdateReason, msg)
	}

	if c.checkEffectiveInstances(destdr) {
		// update source after all instances in destination have taken effect
		if !compareTokens(cdr.Status.TokenStatus.SourceTokens, sourcedr.Status.TokenStatus.Tokens) {
			sourcedr = sourcedr.DeepCopy()
			sourcedr.Status.TokenStatus.Tokens = cdr.Status.TokenStatus.SourceTokens
			msg := fmt.Sprintf("Post rolling update source domainroute %s/%s, revision %d", sourcedr.Namespace, sourcedr.Name, sourcedr.Status.TokenStatus.RevisionToken.Revision)
			nlog.Debug(msg)
			if _, err := c.kusciaClient.KusciaV1alpha1().DomainRoutes(cdr.Spec.Source).UpdateStatus(ctx, sourcedr, metav1.UpdateOptions{}); err != nil {
				nlog.Error(err)
				return err
			}
			c.recorder.Event(cdr, corev1.EventTypeNormal, sourceTokenUpdateReason, msg)
		}
	}

	return nil
}

func newCondition(condType kusciaapisv1alpha1.ClusterDomainRouteConditionType, status corev1.ConditionStatus, reason, message string) *kusciaapisv1alpha1.ClusterDomainRouteCondition {
	return &kusciaapisv1alpha1.ClusterDomainRouteCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func setCondition(status *kusciaapisv1alpha1.ClusterDomainRouteStatus, condition *kusciaapisv1alpha1.ClusterDomainRouteCondition) {
	var currentCond *kusciaapisv1alpha1.ClusterDomainRouteCondition
	for i := range status.Conditions {
		cond := &status.Conditions[i]

		if cond.Type != condition.Type {
			// DO NOT TOUCH READY CONDITION
			if cond.Type == kusciaapisv1alpha1.ClusterDomainRouteReady || condition.Type == kusciaapisv1alpha1.ClusterDomainRouteReady {
				continue
			}

			if cond.Status == corev1.ConditionTrue {
				cond.Status = corev1.ConditionFalse
				cond.LastUpdateTime = condition.LastTransitionTime
				cond.LastTransitionTime = condition.LastTransitionTime
				cond.Reason = condition.Reason
				cond.Message = condition.Message
			}
			continue
		}

		currentCond = cond
		// Do not update lastTransitionTime if the status of the condition doesn't change.
		if cond.Status == condition.Status {
			condition.LastTransitionTime = cond.LastTransitionTime
		}
		status.Conditions[i] = *condition
	}

	if currentCond == nil {
		status.Conditions = append(status.Conditions, *condition)
	}
}
