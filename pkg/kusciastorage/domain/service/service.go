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

package service

import (
	"context"

	"github.com/secretflow/kuscia/pkg/kusciastorage/common"
	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/model"
	"github.com/secretflow/kuscia/pkg/kusciastorage/repository"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

var _ IService = (*resourceService)(nil)

// IService defines an interface which used to handle resource.
type IService interface {
	framework.Bean

	Create(resource *model.Resource, data *model.Data) error
	CreateForRefResource(domain, kind, name string, resource *model.Resource) error
	Find(domain, kind, name string) (*model.Data, error)
	Delete(domain, kind, name string) error
}

// resourceService defines resource service info which used to handle resource.
type resourceService struct {
	mysqlRepo repository.IRepository
}

// NewResourceService returns a resource service instance.
func NewResourceService() IService {
	return &resourceService{}
}

// Init is used to initialize resource service.
func (s *resourceService) Init(registry framework.ConfBeanRegistry) error {
	mysqlRepo, _ := registry.GetBeanByName(common.BeanNameForMysqlRepository)
	s.mysqlRepo, _ = mysqlRepo.(repository.IRepository)
	return nil
}

// Start is used to implement the Start method of framework.Bean interface.
func (s *resourceService) Start(ctx context.Context, e framework.ConfBeanRegistry) error {
	return nil
}

// Create is used to create resource and data.
func (s *resourceService) Create(resource *model.Resource, data *model.Data) error {
	if err := s.mysqlRepo.Create(resource, data); err != nil {
		return err
	}
	return nil
}

// CreateForRefResource is used to create reference resource.
func (s *resourceService) CreateForRefResource(domain, kind, name string, resource *model.Resource) error {
	if err := s.mysqlRepo.CreateForRefResource(domain, kind, name, resource); err != nil {
		return err
	}
	return nil
}

// Find is used to find data.
func (s *resourceService) Find(domain, kind, name string) (*model.Data, error) {
	return s.mysqlRepo.Find(domain, kind, name)
}

// Delete is used to delete resource and data.
func (s *resourceService) Delete(domain, kind, name string) error {
	if err := s.mysqlRepo.Delete(domain, kind, name); err != nil {
		return err
	}
	return nil
}
