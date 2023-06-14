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

package source

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type Merger interface {
	// Merge invoked when a change from a source is received.  May also function as an incremental
	// merger if you wish to consume changes incrementally.  Must be reentrant when more than
	// one source is defined.
	Merge(source string, update interface{}) error
}

type InitConfig struct {
	Namespace  string
	NodeName   types.NodeName
	SourceCfg  *config.SourceCfg
	KubeClient kubernetes.Interface
	Updates    chan<- kubetypes.PodUpdate
	Recorder   record.EventRecorder
}

type Manager struct {
	file      *sourceFile
	apiserver *sourceApiserver

	// Invoked when an update is sent to a source.
	merger Merger

	sourceCh chan kubetypes.PodUpdate
}

func NewManager(cfg *InitConfig) *Manager {
	m := &Manager{
		sourceCh: make(chan kubetypes.PodUpdate, 20),
		merger:   newPodStorage(cfg.Updates, PodConfigNotificationIncremental, cfg.Recorder),
	}

	if cfg.KubeClient != nil {
		m.apiserver = newApiserverSource(cfg, m.sourceCh)
	}

	if cfg.SourceCfg.File.Enable && cfg.SourceCfg.File.Path != "" {
		m.file = newSourceFile(cfg, m.sourceCh)
	}

	return m
}

// Run starts all sources
func (m *Manager) Run(stopCh <-chan struct{}) error {
	go m.listen()

	if m.apiserver != nil {
		if err := m.apiserver.run(stopCh); err != nil {
			return fmt.Errorf("failed to run apiserver source, detail-> %v", err)
		}
	}

	if m.file != nil {
		m.file.run()
	}

	return nil
}

func (m *Manager) listen() {
	for change := range m.sourceCh {
		nlog.Debugf("Receive pod event from source %q", change.Source)

		if err := m.merger.Merge(change.Source, change); err != nil {
			nlog.Errorf("failed to merge pods from %v: %v", change.Source, err)
		}
	}
}

func (m *Manager) AllReady() bool {
	return true
}
