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

package mysql

import (
	"context"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/model"
	"github.com/secretflow/kuscia/pkg/kusciastorage/repository"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/errorcode"
	"github.com/secretflow/kuscia/pkg/web/framework"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
)

var (
	resourceTableScheme = `CREATE TABLE IF NOT EXISTS %s
			(
				id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
				name VARCHAR(256) NOT NULL,
				kind VARCHAR(256) NOT NULL,
				domain VARCHAR(256),
				data_id BIGINT(20) NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				PRIMARY KEY (id),
				INDEX idx_data_id (data_id),
				UNIQUE INDEX uk_name_kind_domain (name, kind, domain)
			);`
	dataTableScheme = `CREATE TABLE IF NOT EXISTS %s
			(
				id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
				content longtext NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				PRIMARY KEY (id)
			);`
)

var _ repository.IRepository = (*mysqlRepo)(nil)

// mysqlRepo defines a client which used to access to mysql database.
type mysqlRepo struct {
	config.FlagEnvConfigLoader

	Name              string `name:"db-name" usage:"db name which used to access"`
	Host              string `name:"db-host" usage:"db host which used to access"`
	Port              int    `name:"db-port" usage:"db port which used to access" default:"3306"`
	User              string `name:"db-user" usage:"db user which used to access"`
	Password          string `name:"db-password" usage:"db password which used to access"`
	ResourceTableName string `name:"db-resource-table-name" usage:"resource table name" default:"resource"`
	DataTableName     string `name:"db-data-table-name" usage:"data table name" default:"data"`

	Client *gorm.DB
}

// NewMysqlRepo returns a mysql repo instance.
func NewMysqlRepo() repository.IRepository {
	return &mysqlRepo{}
}

// Validate is used to validate mysql repo config.
func (r *mysqlRepo) Validate(errs *errorcode.Errs) {
	if r.Port <= 0 {
		errs.AppendErr(fmt.Errorf("server public port %v is illegal", r.Port))
	}
}

// Init is used to initialize mysql repo.
func (r *mysqlRepo) Init(registry framework.ConfBeanRegistry) error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		r.User, r.Password, r.Host, r.Port, r.Name)
	r.Client, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	// set table name
	model.SetResourceTableName(r.ResourceTableName)
	model.SetDataTableName(r.DataTableName)

	// create table
	err = r.Client.Exec(fmt.Sprintf(resourceTableScheme, r.ResourceTableName)).Error
	if err != nil {
		nlog.Warnf("Create table %s failed: %v", r.ResourceTableName, err.Error())
	}

	err = r.Client.Exec(fmt.Sprintf(dataTableScheme, r.DataTableName)).Error
	if err != nil {
		nlog.Warnf("Create table %s failed: %v", r.DataTableName, err.Error())
	}

	return nil
}

// Start is used to implement the Start method of framework.Bean interface.
func (r *mysqlRepo) Start(ctx context.Context, e framework.ConfBeanRegistry) error { return nil }

// Create is used to create resource and data.
func (r *mysqlRepo) Create(resource *model.Resource, data *model.Data) error {
	return r.Client.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return err
		}

		resource.DataID = &data.ID
		if err := tx.Create(resource).Error; err != nil {
			return err
		}

		return nil
	})
}

// CreateForRefResource is used to create reference resource for exist resource.
func (r *mysqlRepo) CreateForRefResource(domain, kind, name string, resource *model.Resource) error {
	return r.Client.Transaction(func(tx *gorm.DB) error {
		rs := &model.Resource{}
		if err := tx.Where("name = ? AND kind = ? AND domain = ?", name, kind, domain).First(rs).Error; err != nil {
			return err
		}

		resource.DataID = rs.DataID
		return tx.Create(resource).Error
	})
}

// Find is used to find resource data.
func (r *mysqlRepo) Find(domain, kind, name string) (*model.Data, error) {
	data := &model.Data{}
	resource := &model.Resource{}
	sql := fmt.Sprintf("SELECT * FROM %s WHERE id = (SELECT data_id FROM %s WHERE name = ? AND kind = ? AND domain = ?)",
		data.TableName(), resource.TableName())
	err := r.Client.Raw(sql, name, kind, domain).First(data).Error
	return data, err
}

// Delete is used to delete resource and data.
func (r *mysqlRepo) Delete(domain, kind, name string) error {
	return r.Client.Transaction(func(tx *gorm.DB) error {
		rs := &model.Resource{}
		dataTblName := (&model.Data{}).TableName()

		queryIDSQL := fmt.Sprintf("SELECT data_id FROM %s WHERE name = ? AND kind = ? AND domain = ?", rs.TableName())
		if err := r.Client.Raw(queryIDSQL, name, kind, domain).First(rs).Error; err != nil {
			return err
		}

		dataID := rs.DataID
		delResourceSQL := fmt.Sprintf("DELETE FROM %s WHERE name = ? AND kind = ? AND domain = ?", rs.TableName())
		if err := tx.Exec(delResourceSQL, name, kind, domain).Error; err != nil {
			return err
		}

		delDataSQL := "DELETE FROM %s WHERE id = ? and NOT EXISTS (SELECT 1 FROM %s WHERE data_id = ?)"
		delDataSQL = fmt.Sprintf(delDataSQL, dataTblName, rs.TableName())
		if err := tx.Exec(delDataSQL, dataID, dataID).Error; err != nil {
			return err
		}
		return nil
	})
}
