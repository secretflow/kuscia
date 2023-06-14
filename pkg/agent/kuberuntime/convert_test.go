/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kuberuntime

import (
	"testing"

	"github.com/stretchr/testify/assert"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
)

func TestConvertTopkgcontainerImageSpec(t *testing.T) {
	testCases := []struct {
		input    *runtimeapi.Image
		expected pkgcontainer.ImageSpec
	}{
		{
			input: &runtimeapi.Image{
				Id:   "test",
				Spec: nil,
			},
			expected: pkgcontainer.ImageSpec{
				Image:       "test",
				Annotations: []pkgcontainer.Annotation(nil),
			},
		},
		{
			input: &runtimeapi.Image{
				Id: "test",
				Spec: &runtimeapi.ImageSpec{
					Annotations: nil,
				},
			},
			expected: pkgcontainer.ImageSpec{
				Image:       "test",
				Annotations: []pkgcontainer.Annotation(nil),
			},
		},
		{
			input: &runtimeapi.Image{
				Id: "test",
				Spec: &runtimeapi.ImageSpec{
					Annotations: map[string]string{},
				},
			},
			expected: pkgcontainer.ImageSpec{
				Image:       "test",
				Annotations: []pkgcontainer.Annotation(nil),
			},
		},
		{
			input: &runtimeapi.Image{
				Id: "test",
				Spec: &runtimeapi.ImageSpec{
					Annotations: map[string]string{
						"kubernetes.io/os":             "linux",
						"kubernetes.io/runtimehandler": "handler",
					},
				},
			},
			expected: pkgcontainer.ImageSpec{
				Image: "test",
				Annotations: []pkgcontainer.Annotation{
					{
						Name:  "kubernetes.io/os",
						Value: "linux",
					},
					{
						Name:  "kubernetes.io/runtimehandler",
						Value: "handler",
					},
				},
			},
		},
	}

	for _, test := range testCases {
		actual := topkgcontainerImageSpec(test.input)
		assert.Equal(t, test.expected, actual)
	}
}

func TestConvertToRuntimeAPIImageSpec(t *testing.T) {
	testCases := []struct {
		input    pkgcontainer.ImageSpec
		expected *runtimeapi.ImageSpec
	}{
		{
			input: pkgcontainer.ImageSpec{
				Image:       "test",
				Annotations: nil,
			},
			expected: &runtimeapi.ImageSpec{
				Image:       "test",
				Annotations: map[string]string{},
			},
		},
		{
			input: pkgcontainer.ImageSpec{
				Image:       "test",
				Annotations: []pkgcontainer.Annotation{},
			},
			expected: &runtimeapi.ImageSpec{
				Image:       "test",
				Annotations: map[string]string{},
			},
		},
		{
			input: pkgcontainer.ImageSpec{
				Image: "test",
				Annotations: []pkgcontainer.Annotation{
					{
						Name:  "kubernetes.io/os",
						Value: "linux",
					},
					{
						Name:  "kubernetes.io/runtimehandler",
						Value: "handler",
					},
				},
			},
			expected: &runtimeapi.ImageSpec{
				Image: "test",
				Annotations: map[string]string{
					"kubernetes.io/os":             "linux",
					"kubernetes.io/runtimehandler": "handler",
				},
			},
		},
	}

	for _, test := range testCases {
		actual := toRuntimeAPIImageSpec(test.input)
		assert.Equal(t, test.expected, actual)
	}
}
