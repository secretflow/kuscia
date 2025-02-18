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
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/confmanager/service"
	"github.com/secretflow/kuscia/pkg/datamesh/config"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type IOperatorService interface {
	Start(ctx context.Context)
}

type operatorService struct {
	conf          *config.DataMeshConfig
	datasourceSvc IDomainDataSourceService
}

func NewOperatorService(config *config.DataMeshConfig, configService service.IConfigService) IOperatorService {
	return &operatorService{
		conf:          config,
		datasourceSvc: NewDomainDataSourceService(config, configService),
	}
}

func (o *operatorService) Start(ctx context.Context) {
	nlog.Infof("DataMesh operator service start")
	// register datasource with best efforts
	o.registerDatasourceBestEfforts()
	// do other logic
	go func() {
		o.doLogic(ctx)
	}()
}

func (o *operatorService) doLogic(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			nlog.Infof("DataMesh operator service stop")
			return
			//default:
			// TODO： check the status of domain data periodically
		}
	}
}

func (o *operatorService) registerDatasourceBestEfforts() {
	go func() {
		for !o.registerDefaultDatasource() {
			time.Sleep(5 * time.Second)
		}
	}()
}

func (o *operatorService) registerDefaultDatasource() bool {
	_, err := o.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(o.conf.KubeNamespace).Get(context.Background(),
		common.DefaultDataSourceID, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			if err = o.datasourceSvc.CreateDefaultDomainDataSource(context.Background()); err != nil {
				nlog.Errorf("Create default datasource %s failed, error: %v.", common.DefaultDataSourceID, err)
				return false
			}
		} else {
			nlog.Errorf("Get default datasource %s failed, error: %v.", common.DefaultDataSourceID, err)
			return false
		}
	}

	nlog.Infof("Datasource %s has been created successful.", common.DefaultDataSourceID)

	_, err = o.conf.KusciaClient.KusciaV1alpha1().DomainDataSources(o.conf.KubeNamespace).Get(context.
		Background(), common.DefaultDataProxyDataSourceID, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			if err = o.datasourceSvc.CreateDefaultDataProxyDomainDataSource(context.Background()); err != nil {
				nlog.Errorf("Create default datasource %s failed, error: %v.", common.DefaultDataProxyDataSourceID, err)
				return false
			}
		} else {
			nlog.Errorf("Get default datasource %s failed, error: %v.", common.DefaultDataProxyDataSourceID, err)
			return false
		}
	}
	nlog.Infof("Datasource %s has been created successful.", common.DefaultDataProxyDataSourceID)

	return true
}
