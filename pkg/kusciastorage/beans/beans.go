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

package beans

import (
	"fmt"

	"github.com/secretflow/kuscia/pkg/kusciastorage/common"
	"github.com/secretflow/kuscia/pkg/kusciastorage/domain/service"
	"github.com/secretflow/kuscia/pkg/kusciastorage/repository/mysql"
	"github.com/secretflow/kuscia/pkg/kusciastorage/server"
	"github.com/secretflow/kuscia/pkg/web/framework/engine"
)

// InjectBeans inject beans which used to init dependent function.
func InjectBeans(engine *engine.Engine) (err error) {
	err = engine.UseBeanWithConfig(common.BeanNameForServer, server.NewServer())
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", common.BeanNameForServer, err.Error())
	}

	err = engine.UseBeanWithConfig(common.BeanNameForMysqlRepository, mysql.NewMysqlRepo())
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", common.BeanNameForMysqlRepository, err.Error())
	}

	err = engine.UseBean(common.BeanNameForResourceService, nil, service.NewResourceService())
	if err != nil {
		return fmt.Errorf("inject bean %s failed: %v", common.BeanNameForResourceService, err.Error())
	}

	return nil
}
