// Copyright 2025 Ant Group Co., Ltd.
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
	"fmt"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/web/errorcode"
)

type TestConfig struct {
	Name    string `name:"name" default:"default" env:"TEST_NAME"`
	Port    int    `name:"port" required:"true" env:"TEST_PORT"`
	Enabled bool   `name:"enabled" env:"TEST_ENABLED"`
}

func (c *TestConfig) Validate(errs *errorcode.Errs) {
	if c.Port <= 0 {
		errs.AppendErr(fmt.Errorf("port must be positive"))
	}
}

func TestFlagEnvConfigLoader(t *testing.T) {
	t.Run("env loading", func(t *testing.T) {
		os.Setenv("TEST_CONFIG_NAME", "env-test")
		os.Setenv("TEST_CONFIG_PORT", "8080")
		defer os.Clearenv()

		config := &TestConfig{}
		loader := &FlagEnvConfigLoader{Source: SourceEnv}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		errs := &errorcode.Errs{}

		loader.RegisterFlags(config, "test-config", fs)
		loader.Process(config, "test-config", errs)

		assert.True(t, errorcode.NoError(errs))
		assert.Equal(t, "env-test", config.Name)
		assert.Equal(t, 8080, config.Port)
	})

	t.Run("required_field_validation", func(t *testing.T) {
		config := &TestConfig{}
		loader := &FlagEnvConfigLoader{Source: SourceFlag}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		errs := &errorcode.Errs{}

		loader.RegisterFlags(config, "test-config", fs)

		loader.Process(config, "test-config", errs)

		config.Validate(errs)

		assert.False(t, errorcode.NoError(errs))
		assert.Contains(t, errs.String(), "port must be positive")
	})

	t.Run("type conversion error", func(t *testing.T) {
		os.Setenv("TEST_CONFIG_PORT", "invalid")
		defer os.Unsetenv("TEST_CONFIG_PORT")

		config := &TestConfig{}
		loader := &FlagEnvConfigLoader{Source: SourceEnv}
		errs := &errorcode.Errs{}

		loader.Process(config, "test-config", errs)
		assert.Contains(t, errs.String(), "invalid syntax")
	})
}
