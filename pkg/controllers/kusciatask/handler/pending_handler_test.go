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

package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func TestInitHandler_Handle(t *testing.T) {
	kusciatask := &kusciaapisv1alpha1.KusciaTask{}

	handler := NewPendingHandler()
	_, err := handler.Handle(kusciatask)
	assert.NoError(t, err)
	assert.Equal(t, kusciaapisv1alpha1.TaskCreating, kusciatask.Status.Phase)
}
