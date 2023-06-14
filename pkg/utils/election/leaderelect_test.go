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

package election

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/utils/signals"
)

func Test_k8sElector_Elect(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()

	newLeaderCh := make(chan struct{})
	startedLeadingCh := make(chan struct{})
	stoppedLeadingCh := make(chan struct{})
	onNewLeader := func(identity string) {
		close(newLeaderCh)
	}
	onStartedLeading := func(ctx context.Context) {
		close(startedLeadingCh)
	}
	onStoppedLeading := func() {
		close(stoppedLeadingCh)
	}

	elector := NewElector(
		kubeClient,
		"test",
		WithOnNewLeader(onNewLeader),
		WithOnStartedLeading(onStartedLeading),
		WithOnStoppedLeading(onStoppedLeading))

	elector.Elect(context.Background())

	ticker := time.NewTicker(time.Second)
	go func() {
		<-newLeaderCh
		<-startedLeadingCh
		elector.Stop()
		<-stoppedLeadingCh
	}()

	select {
	case <-elector.Stopped():
		return
	case <-ticker.C:
		t.Fatal("Timeout")
	}
}

func Test_k8sElector_Run(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	stopCh := make(chan struct{})
	newLeaderCh := 0
	startedLeadingCh := 0
	stoppedLeadingCh := 0
	onNewLeader := func(identity string) {
		newLeaderCh++
	}
	onStartedLeading := func(ctx context.Context) {
		startedLeadingCh++
	}
	onStoppedLeading := func() {
		stoppedLeadingCh++
	}
	ctx := signals.NewKusciaContextWithStopCh(stopCh)
	elector := NewElector(
		kubeClient,
		"test",
		WithOnNewLeader(onNewLeader),
		WithOnStartedLeading(onStartedLeading),
		WithOnStoppedLeading(onStoppedLeading))

	go func() {
		time.Sleep(1 * time.Second)

		close(stopCh)
	}()
	elector.Run(ctx)
	assert.Equal(t, newLeaderCh, 1)
	assert.Equal(t, startedLeadingCh, 1)
	assert.True(t, stoppedLeadingCh > 1)
}
