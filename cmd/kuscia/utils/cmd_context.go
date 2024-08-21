// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type Context struct {
	StorageDir  string
	RuntimeType string
	Store       store.Store
}

type runtimeConfig struct {
	Runtime string `yaml:"runtime"`
}

func (c *Context) CheckRuntime() error {
	if c.RuntimeType == config.ProcessRuntime || c.RuntimeType == config.ContainerRuntime {
		return nil
	}

	if c.RuntimeType == "" { // get from config file
		file, err := os.Executable()
		if err != nil {
			return fmt.Errorf("get exe path failed")
		}
		confFile := path.Join(filepath.Dir(filepath.Dir(file)), "etc/conf/kuscia.yaml")
		data, err := os.ReadFile(confFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %v", err)
		}

		config := &runtimeConfig{}
		if err = yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parse config file failed %s", err.Error())
		}

		if config.Runtime = strings.ToLower(strings.Trim(config.Runtime, " ")); config.Runtime == "" {
			return fmt.Errorf("runtime in config file(%s) is empty", confFile)
		}

		c.RuntimeType = config.Runtime
		return c.CheckRuntime()
	}

	return fmt.Errorf("invalidate runtime type: %s", c.RuntimeType)
}

func (c *Context) InitBeforeRunCommand(cmd *cobra.Command, args []string) {
	var err error
	if err := c.CheckRuntime(); err != nil {
		nlog.Fatal(err)
	}

	if c.RuntimeType == config.ProcessRuntime {
		c.Store, err = store.NewOCIStore(c.StorageDir, mounter.Plain)
		if err != nil {
			nlog.Fatal(err)
		}
	}
}

func RunContainerdCmd(ctx context.Context, name string, arg ...string) error {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}
