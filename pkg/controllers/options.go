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

package controllers

import (
	"fmt"

	"github.com/spf13/pflag"
)

// Options is the main context object for the domain controller.
type Options struct {
	Kubeconfig string
	MasterURL  string
	Workers    int
	// QPS indicates the maximum QPS to the master from this client.
	// If it's zero, the created RESTClient will use DefaultQPS: 5
	QPS float64
	// Maximum burst for throttle.
	// If it's zero, the created RESTClient will use DefaultBurst: 10.
	Burst int
	// HealthCheckPort is the port that the health check server listens on.
	HealthCheckPort int

	ControllerName string
}

// NewOptions creates a new options with a default config.
func NewOptions() *Options {
	return &Options{}
}

// Validate whether the parameter is legal.
func (o *Options) Validate() error {
	if o.QPS < 0 {
		return fmt.Errorf("invalid config qps: %v", o.QPS)
	}

	if o.Burst < 0 {
		return fmt.Errorf("invalid config burst: %v", o.Burst)
	}

	if o.HealthCheckPort < 0 {
		return fmt.Errorf("invalid config health-check-port: %v", o.HealthCheckPort)
	}

	return nil
}

// AddFlags adds flags for a specific server to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.MasterURL, "master", "",
		`The url of the Kubernetes API server,
		 will overrides any value in kubeconfig, only required if out-of-cluster.`)
	fs.StringVar(&o.Kubeconfig, "kubeconfig", "",
		"Path to a kubeconfig. Only required if out-of-cluster.")
	fs.IntVar(&o.Workers, "threads", 4, `How many workers to process the main logic`)
	fs.Float64Var(&o.QPS, "qps", 500, "KubeClient: QPS indicates the maximum QPS to the master from this client.")
	fs.IntVar(&o.Burst, "burst", 1000, "KubeClient: Maximum burst for throttle.")
	fs.IntVar(&o.HealthCheckPort, "health-check-port", 8080, "Port that the health check server listens on.")
}
