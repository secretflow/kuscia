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

package config

import (
	"crypto/rsa"

	"github.com/spf13/pflag"

	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/utils/kusciaconfig"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/web/framework/config"
)

type DataProxyMode string

const (
	ModeDirect DataProxyMode = "direct"
	ModeProxy  DataProxyMode = "proxy"
)

type DataMeshConfig struct {
	RootDir        string
	ListenAddr     string // default is empyt
	HTTPPort       int32
	GRPCPort       int32
	Debug          bool
	ConnectTimeOut int
	ReadTimeout    int
	WriteTimeout   int
	IdleTimeout    int
	Initiator      string
	FlagSet        *pflag.FlagSet
	DomainKey      *rsa.PrivateKey
	TLS            config.TLSServerConfig
	KusciaClient   kusciaclientset.Interface
	KubeNamespace  string
	DisableTLS     bool              `yaml:"disableTLS,omitempty"`
	DataProxyList  []DataProxyConfig `yaml:"dataProxyList,omitempty"`
	InterceptorLog *nlog.NLog        `yaml:"-"`
}

type DataProxyConfig struct {
	Endpoint        string                  `yaml:"endpoint,omitempty"`
	ClientTLSConfig *kusciaconfig.TLSConfig `yaml:"clientTLSConfig,omitempty"`
	// DatasourceTypes claims which dataSources proxy by this dataProxy, empty means all types that builtin dataProxy unsupported
	DataSourceTypes []string `yaml:"dataSourceTypes,omitempty"`
	// io mode type: proxy(app --> datamesh --> datasource) or direct(app --> datasource)
	Mode string `yaml:"mode,omitempty"`
}

type DbConfig struct {
	Type       string            `mapstructure:"type"`
	TableAlias DbTableAlias      `mapstructure:"table_alias"`
	Sqlite     SqliteStoreConfig `mapstructure:"sqlite"`
	Mysql      MysqlStoreConfig  `mapstructure:"mysql"`
}

type SqliteStoreConfig struct {
	Dsn                string `mapstructure:"dsn"`
	AutoCreateDisable  bool   `mapstructure:"auto_create_disable"`
	AutoMigrateDisable bool   `mapstructure:"auto_migrate_disable"`
}

type MysqlStoreConfig struct {
	//user:password@tcp(127.0.0.1:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	Dsn string `mapstructure:"dsn"`
}

type DbTableAlias struct {
	DataTable  string `mapstructure:"data_table"`
	DataSource string `mapstructure:"data_source"`
	DataObject string `mapstructure:"data_object"`
}

func NewDefaultDataMeshConfig() *DataMeshConfig {
	return &DataMeshConfig{
		HTTPPort:       8070,
		GRPCPort:       8071,
		ConnectTimeOut: 5,
		ReadTimeout:    20,
		WriteTimeout:   20,
		IdleTimeout:    300,
		DisableTLS:     false,
	}
}
