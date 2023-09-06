package domain

import (
	"reflect"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	usedLimit   = 1
	unusedLimit = 1
	usedState   = "used"
	unusedState = "unused"
	tokenSize   = 32
)

func (c *Controller) newDomainTokenStatus(domain *kusciaapisv1alpha1.Domain) []kusciaapisv1alpha1.DeployTokenStatus {
	domainStatus := domain.Status
	if domainStatus == nil {
		domainStatus = &kusciaapisv1alpha1.DomainStatus{}
	}
	oldTokenStatuses := domainStatus.DeployTokenStatuses
	newTokenStatuses := make([]kusciaapisv1alpha1.DeployTokenStatus, 0)
	usedTokens := make([]kusciaapisv1alpha1.DeployTokenStatus, 0)
	unusedTokens := make([]kusciaapisv1alpha1.DeployTokenStatus, 0)
	c.sortTokenStatus(oldTokenStatuses)
	for _, tokenStatus := range oldTokenStatuses {
		if tokenStatus.Token == "" {
			nlog.Warn("token is empty")
			continue
		}
		state := tokenStatus.State
		if state == usedState {
			usedTokens = append(usedTokens, tokenStatus)
		} else if state == unusedState {
			unusedTokens = append(unusedTokens, tokenStatus)
		} else {
			nlog.Warnf("unsupported token state [%s] with token [%s]", state, tokenStatus.Token)
		}
	}
	used := len(usedTokens)
	if used > usedLimit {
		newTokenStatuses = append(newTokenStatuses, usedTokens[used-usedLimit:]...)
	} else {
		newTokenStatuses = append(newTokenStatuses, usedTokens...)
	}
	unused := len(unusedTokens)
	if unused > unusedLimit {
		newTokenStatuses = append(newTokenStatuses, unusedTokens[unused-unusedLimit:]...)
	} else {
		newTokenStatuses = append(newTokenStatuses, unusedTokens...)
		if unusedLimit-unused > 0 {
			newTokenStatuses = append(newTokenStatuses, c.generateTokenStatus(unusedLimit-unused)...)
		}
	}
	c.sortTokenStatus(newTokenStatuses)
	return newTokenStatuses
}

func (c *Controller) generateTokenStatus(size int) []kusciaapisv1alpha1.DeployTokenStatus {
	tokens := make([]kusciaapisv1alpha1.DeployTokenStatus, 0)
	for i := 0; i < size; i++ {
		tokens = append(tokens, kusciaapisv1alpha1.DeployTokenStatus{
			Token:              generateToken(),
			State:              unusedState,
			LastTransitionTime: metav1.Now(),
		})
	}
	return tokens
}

func (c *Controller) isTokenStatusEqual(oldStatus, newStatus []kusciaapisv1alpha1.DeployTokenStatus) bool {
	c.sortTokenStatus(oldStatus)
	c.sortTokenStatus(newStatus)
	return reflect.DeepEqual(oldStatus, newStatus)
}

func (c *Controller) sortTokenStatus(status []kusciaapisv1alpha1.DeployTokenStatus) {
	sort.SliceStable(status, func(i, j int) bool {
		return status[i].LastTransitionTime.Before(&status[j].LastTransitionTime)
	})
}

func generateToken() string {
	return string(common.GenerateRandomBytes(tokenSize))
}
