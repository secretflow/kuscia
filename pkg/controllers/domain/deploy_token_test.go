package domain

import (
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestUpdateDomainToken(t *testing.T) {
	c := Controller{}
	testCases := []struct {
		name         string
		domain       *kusciaapisv1alpha1.Domain
		shouldUpdate bool
		size         int
	}{
		{
			name:         "nilDomainStatus",
			domain:       &kusciaapisv1alpha1.Domain{},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "emptyDomainStatus",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{},
			},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "oneUnused",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              unusedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: false,
			size:         unusedLimit,
		},
		{
			name: "emptyToken",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              "",
							State:              unusedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "unsupportedState",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              "bad",
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         unusedLimit,
		},
		{
			name: "oneUsed",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              usedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         usedLimit + unusedLimit,
		},
		{
			name: "oneUsedAndOneUnused",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              unusedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              usedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			size:         usedLimit + unusedLimit,
			shouldUpdate: false,
		},
		{
			name: "twoUsedAndTwoUnused",
			domain: &kusciaapisv1alpha1.Domain{
				Status: &kusciaapisv1alpha1.DomainStatus{
					DeployTokenStatuses: []kusciaapisv1alpha1.DeployTokenStatus{
						{
							Token:              generateToken(),
							State:              unusedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              unusedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              usedState,
							LastTransitionTime: metav1.Now(),
						},
						{
							Token:              generateToken(),
							State:              usedState,
							LastTransitionTime: metav1.Now(),
						},
					},
				},
			},
			shouldUpdate: true,
			size:         usedLimit + unusedLimit,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			domain := testCase.domain
			var oldTokenStatuses []kusciaapisv1alpha1.DeployTokenStatus
			if domain.Status != nil {
				oldTokenStatuses = domain.Status.DeployTokenStatuses
			}
			newTokenStatuses := c.newDomainTokenStatus(domain)
			assert.Equal(t, !c.isTokenStatusEqual(oldTokenStatuses, newTokenStatuses), testCase.shouldUpdate)
			assert.Equal(t, len(newTokenStatuses), testCase.size, testCase.name)
		})
	}
}
