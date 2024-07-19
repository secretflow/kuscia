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

package controller

import (
	"context"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	drExpireDuration       = 45 * time.Second
	updateStatusRetryCount = 3
)

func (c *DomainRouteController) isHeartbeatTimeout(dr *kusciaapisv1alpha1.DomainRoute) bool {
	tm, ok := c.drHeartbeat[dr.Name]
	if ok && time.Since(tm) > drExpireDuration {
		nlog.Warnf("dr(%s) heartbeat expired, last hb time:%s", dr.Name, tm.String())
		return true
	}

	if !ok {
		c.drHeartbeat[dr.Name] = time.Now()
	}

	return false
}

func (c *DomainRouteController) refreshHeartbeatTime(dr *kusciaapisv1alpha1.DomainRoute) {
	c.drHeartbeat[dr.Name] = time.Now()
}

func (c *DomainRouteController) markDestUnreachable(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	if !dr.Status.IsDestinationUnreachable && c.isHeartbeatTimeout(dr) {
		dr = dr.DeepCopy()
		dr.Status.IsDestinationUnreachable = true
		err := updateDestUnreachableStatus(ctx, c.kusciaClient, dr, updateStatusRetryCount)
		if err != nil {
			nlog.Warnf("markDestUnreachable of dr(%s) fail, err: %v", dr.Name, err)
			return err
		}
		nlog.Warnf("markDestUnreachable of dr(%s)", dr.Name)
	}

	return nil
}

func (c *DomainRouteController) markDestReachable(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) error {
	if dr.Status.IsDestinationUnreachable {
		dr = dr.DeepCopy()
		dr.Status.IsDestinationUnreachable = false
		err := updateDestUnreachableStatus(ctx, c.kusciaClient, dr, updateStatusRetryCount)
		if err != nil {
			nlog.Warnf("markDestReachable of dr(%s) fail, err: %v", dr.Name, err)
			return err
		}
		nlog.Infof("markDestReachable of dr(%s)", dr.Name)
	}
	return nil
}

func updateDestUnreachableStatus(ctx context.Context, kusciaClient kusciaclientset.Interface,
	dr *kusciaapisv1alpha1.DomainRoute,
	retryCount int) error {
	var err error
	hasConflict := false

	for i := 0; i < retryCount; retryCount++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Millisecond * 50):
			}
		}

		if !hasConflict {
			_, err = kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).UpdateStatus(ctx, dr,
				metav1.UpdateOptions{})
			if err == nil {
				return nil
			}
			if !k8serrors.IsConflict(err) {
				continue
			}
			hasConflict = true
		}

		newDr, err := kusciaClient.KusciaV1alpha1().DomainRoutes(dr.Namespace).Get(ctx, dr.Name,
			metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			nlog.Warnf("Dr(%s) is not exist", dr.Name)
			return err
		}
		if err != nil {
			nlog.Warnf("Get Dr(%s) fail: %v", dr.Name, err)
			continue
		}

		if newDr.Status.IsDestinationUnreachable == dr.Status.IsDestinationUnreachable {
			return nil
		}

		if newDr.Status.TokenStatus.RevisionInitializer != dr.Status.TokenStatus.RevisionInitializer ||
			newDr.Status.TokenStatus.RevisionToken.Revision != dr.Status.TokenStatus.RevisionToken.Revision {
			nlog.Warnf("Dr(%s)'s <Initializer,Revision> has changed from <%s,%d> to <%s,%d>, so skip updateStatus",
				dr.Name, dr.Status.TokenStatus.RevisionInitializer, dr.Status.TokenStatus.RevisionToken.Revision,
				newDr.Status.TokenStatus.RevisionInitializer, newDr.Status.TokenStatus.RevisionToken.Revision)
			return nil
		}

		newDr.Status.IsDestinationUnreachable = dr.Status.IsDestinationUnreachable
		dr = newDr

		hasConflict = false
	}
	return err
}
