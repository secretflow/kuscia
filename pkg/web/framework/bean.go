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

package framework

import (
	"context"
)

// Default Bean Names
const (
	ConfName      = "kusciaapp"
	GinServerName = "server"
	MetricsName   = "metrics"
)

// ConfBeanRegistry is a subset of the Framework Interface. Objects that implement the Framework interface can be used
// as ConfBeanRegistry, which is used to obtain other required config and bean in the Bean Init and Start stages。
type ConfBeanRegistry interface {
	GetBeanByName(name string) (Bean, bool)
	GetConfigByName(name string) (Config, bool)
}

type Bean interface {
	Init(e ConfBeanRegistry) error
	Start(ctx context.Context, e ConfBeanRegistry) error
}
