// Copyright 2024 Ant Group Co., Ltd.
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

package dmflight

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/datamesh/config"
)

func TestNewBuiltinDirectIO(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinDirectIO(nil)
	assert.NotNil(t, channel)
	assert.Nil(t, channel.(*builtinDirectIO).config)

	assert.Error(t, channel.Read(context.Background(), nil, nil))
	assert.Error(t, channel.Write(context.Background(), nil, nil))
}

func TestDirectChannel_Endpoint(t *testing.T) {
	t.Parallel()
	channel := NewBuiltinDirectIO(&config.ExternalDataProxyConfig{
		Endpoint: "xyz",
	})
	assert.Equal(t, "xyz", channel.GetEndpointURI())
}
