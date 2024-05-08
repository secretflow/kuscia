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

package model

import "time"

var (
	resourceTableName string
	dataTableName     string
)

// Resource defines the resource table schema.
type Resource struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Domain string `gorm:"column:domain"`
	Kind   string `gorm:"column:kind"`
	Name   string `gorm:"column:name"`
	DataID *uint  `gorm:"column:data_id"`
}

// TableName return the resource table name.
func (Resource) TableName() string {
	return resourceTableName
}

// SetResourceTableName is used to set resource table name.
func SetResourceTableName(name string) {
	resourceTableName = name
}

// Data defines the data table schema.
type Data struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Content string `gorm:"column:content"`
}

// TableName return the data table name.ddd
func (Data) TableName() string {
	return dataTableName
}

// SetDataTableName is used to set data table name.
func SetDataTableName(name string) {
	dataTableName = name
}
