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
package domainroute

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/secretflow/kuscia/pkg/common"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func compareTokens(tokens1, tokens2 []kusciaapisv1alpha1.DomainRouteToken) bool {
	if len(tokens1) != len(tokens2) {
		return false
	}

	for i := 0; i < len(tokens1); i++ {
		if tokens1[i].Revision != tokens2[i].Revision {
			return false
		}
	}
	return true
}

func (c *controller) checkEffectiveInstances(dr *kusciaapisv1alpha1.DomainRoute) bool {
	if len(dr.Status.TokenStatus.Tokens) == 0 {
		nlog.Warnf("Domainroute %s/%s checkEffectiveInstances failed: tokens is nil, please check the result of handshake in instance's log ", dr.Namespace, dr.Name)
		return false
	}

	gateways, err := c.gatewayLister.Gateways(dr.Namespace).List(labels.Everything())
	if err != nil {
		nlog.Errorf("Domainroute %s/%s checkEffectiveInstances error: List gateways  failed with %v", dr.Namespace, dr.Name, err)
		return false
	}
	if gateways == nil {
		nlog.Warnf("Domainroute %s/%s checkEffectiveInstances error: not found effective gateway in %s, please deploy first", dr.Namespace, dr.Name, dr.Namespace)
		return false
	}
	liveGateways := map[string]bool{}
	for i, gw := range gateways {
		if time.Since(gw.Status.HeartbeatTime.Time) < common.GatewayLiveTimeout {
			liveGateways[gateways[i].Name] = true
		}
	}

	found := 0
	n := len(dr.Status.TokenStatus.Tokens)
	for _, ins := range dr.Status.TokenStatus.Tokens[n-1].EffectiveInstances {
		if liveGateways[ins] {
			found++
		}
	}

	return found == len(liveGateways)
}

func DoValidate(spec *kusciaapisv1alpha1.DomainRouteSpec) error {
	if spec.Source == "" || spec.Destination == "" {
		return fmt.Errorf("source or destination is null")
	}
	// body encryption will use tokenConfig to generate encrypt key
	if spec.BodyEncryption != nil {
		if spec.TokenConfig == nil {
			return fmt.Errorf("field TokenConfig is null")
		}
	}

	switch spec.AuthenticationType {
	case kusciaapisv1alpha1.DomainAuthenticationToken:
		if spec.TokenConfig == nil {
			return fmt.Errorf("field TokenConfig is null")
		}
	case kusciaapisv1alpha1.DomainAuthenticationMTLS:
		if spec.MTLSConfig == nil {
			return fmt.Errorf("field MTLSConfig is null")
		}
		if spec.MTLSConfig.SourceClientCert == "" {
			return fmt.Errorf("field SourceClientCert is null")
		}
		if _, err := base64.StdEncoding.DecodeString(spec.MTLSConfig.SourceClientCert); err != nil {
			return fmt.Errorf("field SourceClientCert is format error, must be base64 encoded")
		}
		if spec.MTLSConfig.TLSCA != "" {
			if _, err := base64.StdEncoding.DecodeString(spec.MTLSConfig.TLSCA); err != nil {
				return fmt.Errorf("field TLSCA is format error, must be base64 encoded")
			}
		}
		if spec.MTLSConfig.SourceClientPrivateKey != "" {
			if _, err := base64.StdEncoding.DecodeString(spec.MTLSConfig.SourceClientPrivateKey); err != nil {
				return fmt.Errorf("field SourceClientPrivateKey is format error, must be base64 encoded")
			}
		}
	case kusciaapisv1alpha1.DomainAuthenticationNone:
	default:
		return fmt.Errorf("unsupport type %s", spec.AuthenticationType)
	}
	if spec.TokenConfig != nil {
		if spec.TokenConfig.SourcePublicKey != "" {
			// publickey must be base64 encoded
			if _, err := base64.StdEncoding.DecodeString(spec.TokenConfig.SourcePublicKey); err != nil {
				return fmt.Errorf("sourcePublicKey is format err, must be base64 encoded, err :%v ", err)
			}
		}
		if spec.TokenConfig.DestinationPublicKey != "" {
			if _, err := base64.StdEncoding.DecodeString(spec.TokenConfig.DestinationPublicKey); err != nil {
				return fmt.Errorf("destinationPublicKey is format err, must be base64 encoded, err :%v ", err)
			}
		}
	}

	return nil
}

func (c *controller) needRollingToNext(ctx context.Context, dr *kusciaapisv1alpha1.DomainRoute) bool {
	if !dr.Status.TokenStatus.RevisionToken.IsReady {
		rollElapsedTime := time.Since(dr.Status.TokenStatus.RevisionToken.RevisionTime.Time)
		if rollElapsedTime > domainRouteSyncPeriod {
			nlog.Warnf("Domainroute %s/%s token is waiting more than %d minutes for ready, so need to re-handshake", dr.Namespace, dr.Name, domainRouteSyncPeriod/time.Minute)
			return true
		}
	} else {
		rollingUpdatePeriod := time.Duration(dr.Spec.TokenConfig.RollingUpdatePeriod) * time.Second
		tokenUsedTime := time.Since(dr.Status.TokenStatus.RevisionToken.RevisionTime.Time)
		if rollingUpdatePeriod > 0 && (tokenUsedTime > rollingUpdatePeriod || time.Now().After(dr.Status.TokenStatus.RevisionToken.ExpirationTime.Time)) {
			nlog.Warnf("Domainroute %s/%s token is out of time, need to rolling", dr.Namespace, dr.Name)
			return true
		}
	}
	return false
}
