package domain

import (
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	constants "github.com/secretflow/kuscia/pkg/common"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestAuth(t *testing.T) {
	testCases := []struct {
		name         string
		domain       *kusciaapisv1alpha1.Domain
		shouldUpdate bool
	}{
		{
			name:         "domain labels is nil",
			domain:       &kusciaapisv1alpha1.Domain{},
			shouldUpdate: true,
		},
		{
			name: "domain labels is empty array",
			domain: &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Labels: make(map[string]string, 0),
				},
			},
			shouldUpdate: true,
		},
		{
			name: "domain auth labels not exists",
			domain: &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"a": "b",
					},
				},
			},
			shouldUpdate: true,
		},
		{
			name: "domain auth labels exists,but value not expected",
			domain: &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constants.LabelDomainAuth: "",
					},
				},
			},
			shouldUpdate: true,
		},
		{
			name: "domain expected auth labels exists",
			domain: &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constants.LabelDomainAuth: authCompleted,
					},
				},
			},
			shouldUpdate: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, shouldCreateOrUpdate(testCase.domain), testCase.shouldUpdate)
		})
	}
}
