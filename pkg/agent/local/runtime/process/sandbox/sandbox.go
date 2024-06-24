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

package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	metaFile = "meta.json"
)

type MetaData struct {
	// ID is the sandbox id.
	ID string `json:"id"`
	// Name is the sandbox name.
	Name string `json:"name"`
	// Config is the CRI sandbox config.
	Config *runtime.PodSandboxConfig `json:"config"`
	// IP of Pod if it is attached to non host network
	IP string `json:"ip"`
	// Creation time
	CreatedAt time.Time `json:"createdAt"`
}

type State uint32

// Sandbox contains all resources associated with the sandbox. All methods to
// mutate the internal state are thread safe.
type Sandbox struct {
	MetaData

	Bundle *layout.Bundle

	sync.RWMutex

	state runtime.PodSandboxState
}

// NewSandbox creates an internally used sandbox type.
func NewSandbox(config *runtime.PodSandboxConfig, ip, rootDir string) (s *Sandbox, retErr error) {
	s = &Sandbox{
		MetaData: MetaData{
			ID:        common.GenerateID(16),
			Config:    config,
			IP:        ip,
			CreatedAt: time.Now(),
		},
		state: runtime.PodSandboxState_SANDBOX_READY,
	}

	s.Name = makeSandboxName(config.Metadata, s.ID)

	sandboxRootDir := filepath.Join(rootDir, s.Name)
	bundle, err := layout.NewBundle(sandboxRootDir)
	if err != nil {
		return nil, err
	}
	s.Bundle = bundle
	if err := paths.EnsureDirectory(s.Bundle.GetRootDirectory(), true); err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			if err := os.RemoveAll(sandboxRootDir); err != nil {
				nlog.Warnf("Failed to remove sandbox root directory %q: %v", sandboxRootDir, err)
			}
		}
	}()

	metaPath := filepath.Join(s.Bundle.GetRootDirectory(), metaFile)
	if err := paths.WriteJSON(metaPath, &s.MetaData); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Sandbox) Stop() {
	s.Lock()
	defer s.Unlock()

	if s.state != runtime.PodSandboxState_SANDBOX_NOTREADY {
		s.state = runtime.PodSandboxState_SANDBOX_NOTREADY
	}
}

func (s *Sandbox) State() runtime.PodSandboxState {
	s.RLock()
	defer s.RUnlock()
	return s.state
}

func makeSandboxName(meta *runtime.PodSandboxMetadata, sandboxID string) string {
	return strings.Join([]string{
		meta.Namespace,                  // 0
		meta.Name,                       // 1
		sandboxID,                       // 2
		fmt.Sprintf("%d", meta.Attempt), // 3
	}, "_")
}
