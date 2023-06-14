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

package commands

import (
	"os"
	"time"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// Defaults for root command options
const (
	DefaultInformerResyncPeriod = 1 * time.Minute
	DefaultKubeNamespace        = corev1.NamespaceDefault
)

// Opts stores all the options for configuring the root command.
// It is used for setting flag values.
//
// You can set the default options by creating a new `Opts` struct and passing
// it into `SetDefaultOpts`
type Opts struct {
	// Namespace is used to watch for pods and other resources.
	Namespace string
	// NodeName is used when creating a node in Kubernetes.
	NodeName string
	// KeepNodeOnExit is used to identify whether information needs to be retained in k8s
	KeepNodeOnExit bool
	// AgentConfigFile is path of agent configuration file.
	AgentConfigFile string

	APIVersion   string
	AgentVersion string
}

// SetDefaultOpts sets default options for unset values on the passed in option struct.
// Fields tht are already set will not be modified.
func SetDefaultOpts(c *Opts) {
	c.Namespace = DefaultKubeNamespace

	// node name
	var err error
	if c.NodeName, err = os.Hostname(); err != nil {
		nlog.Fatalf("Cannot get hostname: %v", err)
		os.Exit(-1)
	}
}

func installFlags(flags *pflag.FlagSet, c *Opts) {
	flags.StringVarP(&c.Namespace, "namespace", "n", c.Namespace, "kubernetes namespace")
	flags.StringVar(&c.NodeName, "name", c.NodeName, "kubernetes node name")
	flags.StringVarP(&c.AgentConfigFile, "agent-config", "c", c.AgentConfigFile, "Additional agent config file")
	flags.BoolVar(&c.KeepNodeOnExit, "keep-node", true, "keep node info in k8s master after agent exit")
}
