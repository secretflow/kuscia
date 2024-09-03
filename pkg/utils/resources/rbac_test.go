// Copyright 2024 Ant Group Co., Ltd.
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

package resources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestCreateRoleBinding(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "alice",
			Name:      "test-pod",
		},
	}
	kubeClient := kubefake.NewSimpleClientset(pod)

	ownerRef := metav1.NewControllerRef(pod, corev1.SchemeGroupVersion.WithKind("Pod"))

	err := CreateRoleBinding(context.Background(), kubeClient, "alice", ownerRef)
	assert.NoError(t, err)

	_, err = kubeClient.CoreV1().ServiceAccounts("alice").Get(context.Background(), "alice", metav1.GetOptions{})
	assert.NoError(t, err)

	err = CreateRoleBinding(context.Background(), kubeClient, "alice", ownerRef)
	assert.NoError(t, err)
}
