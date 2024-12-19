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

package commands

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"testing"
	"time"

	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/reporter/config"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func isTCPPortOpened(address string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func Test_Run(t *testing.T) {
	config := config.NewDefaultReporterConfig("")
	config.KubeClient = kubefake.NewSimpleClientset()
	config.KusciaClient = kusciafake.NewSimpleClientset()
	// cfg := mockModuleRuntimeConfig(t)
	config.DomainID = "test-domain"
	config.DomainKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		assert.NoError(t, Run(ctx, config))
	}()
	// check if port is open
	grpcAddress := fmt.Sprintf("127.0.0.1:%d", config.HTTPPort)

	assert.NoError(t, wait.Poll(time.Millisecond*100, time.Second, func() (bool, error) {
		return isTCPPortOpened(grpcAddress, time.Millisecond*20), nil
	}))

	defer cancel()
}
