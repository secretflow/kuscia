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

package container

import (
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/cmd/kuscia/utils"
	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/container"
	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

// runCommand represents the pull command
func runCommand(cmdCtx *utils.Context) *cobra.Command {
	cname := uuid.NewString()
	runCommand := &cobra.Command{
		Use:                   "run [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short:                 "Create and run a new container from an image",
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		Example: `
# run image
kuscia container run secretflow/secretflow:latest bash

`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			if cmdCtx.RuntimeType == config.ContainerRuntime {
				cmd := []string{"-a=/home/kuscia/containerd/run/containerd.sock", "run"}
				cmd = append(cmd, args...)
				if err := utils.RunContainerdCmd(ctx, "ctr", cmd...); err != nil {
					nlog.Fatal(err)
				}
			} else {
				if cname == "" {
					cname = uuid.NewString()
				}
				runpStartContainer(cname, args)
			}
		},
	}

	runCommand.Flags().StringVarP(&cname, "name", "", "test", "container name")

	return runCommand
}

func runpStartContainer(cname string, args []string) {
	sandboxBundle, imageStore, logDirectory := initContainerEnv(".")

	c := &runtime.ContainerConfig{
		Metadata: &runtime.ContainerMetadata{
			Name: cname,
		},
		Image: &runtime.ImageSpec{
			Image: args[0],
		},
		LogPath: "0.log",
	}
	if len(args) > 1 {
		c.Command = args[1:]
	}

	container, err := container.NewContainer(c, logDirectory, cname, sandboxBundle, imageStore)
	if err != nil {
		nlog.Fatalf("Create container failed: %s", err.Error())
	}

	if err := container.Create(); err != nil {
		nlog.Fatalf("Container init failed: %s", err.Error())
	}

	if err := container.Start(); err != nil {
		nlog.Fatalf("Container start failed: %s", err.Error())
	}

	for {
		if status := container.GetStatus(); status.State() != runtime.ContainerState_CONTAINER_RUNNING {
			break
		}
		time.Sleep(time.Second)
		print(".")
	}
}

func initContainerEnv(rootDir string) (*layout.Bundle, store.Store, string) {
	imageStoreDir := filepath.Join(rootDir, config.DefaultImageStoreDir())
	sandboxDir := filepath.Join(rootDir, "sandbox")
	logDirectory := filepath.Join(rootDir, "logs")

	imageStore, err := store.NewOCIStore(imageStoreDir, mounter.Plain)
	if err != nil {
		nlog.Fatalf("Create image store(%s) failed: %s", imageStoreDir, err.Error())
	}
	sandboxBundle, err := layout.NewBundle(sandboxDir)
	if err != nil {
		nlog.Fatalf("Create image sandbox bounle(%s) failed: %s", sandboxDir, err.Error())
	}

	if err := paths.EnsureDirectory(logDirectory, true); err != nil {
		nlog.Fatalf("Create logs dir(%s) failed: %s", logDirectory, err.Error())
	}

	return sandboxBundle, imageStore, logDirectory
}
