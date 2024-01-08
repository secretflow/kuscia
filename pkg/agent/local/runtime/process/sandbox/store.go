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
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/errdefs"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	defaultCleanupInterval       = 60 * time.Second
	defaultSandboxProtectionTime = 10 * time.Second
)

// Store stores all sandboxes.
type Store struct {
	lock                  sync.RWMutex
	sandboxes             map[string]*Sandbox
	sandboxRootDir        string
	stopCh                chan struct{}
	cleanupInterval       time.Duration
	sandboxProtectionTime time.Duration
}

// NewStore creates a sandbox store.
func NewStore(sandboxRootDir string) *Store {
	return &Store{
		sandboxes:             make(map[string]*Sandbox),
		sandboxRootDir:        sandboxRootDir,
		stopCh:                make(chan struct{}),
		cleanupInterval:       defaultCleanupInterval,
		sandboxProtectionTime: defaultSandboxProtectionTime,
	}
}

// Add a sandbox into the store.
func (s *Store) Add(sb *Sandbox) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.sandboxes[sb.ID]; ok {
		return errdefs.ErrAlreadyExists
	}
	s.sandboxes[sb.ID] = sb
	return nil
}

// Get returns the sandbox with specified id.
func (s *Store) Get(id string) (*Sandbox, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if sb, ok := s.sandboxes[id]; ok {
		return sb, true
	}
	return nil, false
}

// List lists all sandboxes.
func (s *Store) List() []*Sandbox {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var sandboxes []*Sandbox
	for _, sb := range s.sandboxes {
		sandboxes = append(sandboxes, sb)
	}
	return sandboxes
}

// Delete deletes the sandbox with specified id.
func (s *Store) Delete(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.sandboxes, id)
}

func (s *Store) Stop() {
	close(s.stopCh)
}

func (s *Store) Monitor() {
	ticker := time.NewTicker(s.cleanupInterval)
	go func() {
		for {
			select {
			case <-s.stopCh:
				break
			case <-ticker.C:
				if err := s.cleanup(s.sandboxRootDir); err != nil {
					nlog.Errorf("Failed to cleanup sandbox root directory: %v", err)
				}
			}
		}
	}()
}

func (s *Store) cleanup(sandboxRootDir string) error {
	entries, err := os.ReadDir(sandboxRootDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sandboxDir := filepath.Join(sandboxRootDir, entry.Name())
		sandboxMeta := &MetaData{}
		metaFile := filepath.Join(sandboxDir, metaFile)
		if err := paths.ReadJSON(metaFile, sandboxMeta); err != nil {
			nlog.Warnf("Failed to read json file %s: %v", metaFile, err)
			continue
		}

		if time.Since(sandboxMeta.CreatedAt) < s.sandboxProtectionTime {
			continue
		}

		_, ok := s.Get(sandboxMeta.ID)
		if ok {
			continue
		}

		nlog.Infof("Remove nonexistent sandbox %q directory %q", sandboxMeta.ID, sandboxDir)
		if err := os.RemoveAll(sandboxDir); err != nil {
			nlog.Errorf("Failed to remove nonexistent sandbox directory %q: %v", sandboxDir, err)
		}
	}

	return nil
}
