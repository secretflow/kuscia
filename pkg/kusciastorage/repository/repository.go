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

package repository

import (
	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/model"
	"github.com/secretflow/kuscia/pkg/web/framework"
)

// IRepository defines an interface which used to handle repository resource.
type IRepository interface {
	framework.Config
	framework.Bean

	Create(resource *model.Resource, data *model.Data) error
	CreateForRefResource(domain, kind, name string, resource *model.Resource) error
	Find(domain, kind, name string) (*model.Data, error)
	Delete(domain, kind, name string) error
}
