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

package container

import (
	"sync"

	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/errdefs"
)

// Store stores all Containers.
type Store struct {
	lock       sync.RWMutex
	containers map[string]*Container
}

// NewStore creates a container store.
func NewStore() *Store {
	return &Store{
		containers: make(map[string]*Container),
	}
}

// Add a container into the store. Returns store.ErrAlreadyExist if the
// container already exists.
func (s *Store) Add(c *Container) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.containers[c.ID]; ok {
		return errdefs.ErrAlreadyExists
	}
	s.containers[c.ID] = c
	return nil
}

// Get returns the container with specified id. Returns errdefs.ErrNotExist
// if the container doesn't exist.
func (s *Store) Get(id string) (*Container, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if c, ok := s.containers[id]; ok {
		return c, nil
	}
	return nil, errdefs.ErrNotFound
}

// List lists all containers.
func (s *Store) List() []*Container {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var containers []*Container
	for _, c := range s.containers {
		containers = append(containers, c)
	}
	return containers
}

// Delete deletes the container from store with specified id.
func (s *Store) Delete(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.containers, id)
}
