// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v3"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/local/mounter"
	"github.com/secretflow/kuscia/pkg/agent/local/runtime/process/container"
	"github.com/secretflow/kuscia/pkg/agent/local/store"
	"github.com/secretflow/kuscia/pkg/agent/local/store/kii"
	"github.com/secretflow/kuscia/pkg/agent/local/store/layout"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

type ImageContext struct {
	ImageService ImageService
}

type ImageService interface {
	RemoveImage() error

	LoadImage(tarFile string) error

	PullImage(creds string) error

	ListImage() error

	TagImage() error

	MountImage() error

	BuiltinImage(manifestFile string) error

	ImageRun(name string) error
}

type runtimeConfig struct {
	Runtime  string `yaml:"runtime"`
	LogLevel string `yaml:"logLevel"`
}

func initRuntimeAndLogLevel(runtimeType string) (string, error) {

	confFile := path.Join(common.DefaultKusciaHomePath(), "etc/conf/kuscia.yaml")
	data, err := os.ReadFile(confFile)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %v", err)
	}

	localConfig := &runtimeConfig{}
	if err = yaml.Unmarshal(data, &localConfig); err != nil {
		return "", fmt.Errorf("parse config file failed %s", err.Error())
	}

	if localConfig.Runtime = strings.ToLower(strings.Trim(localConfig.Runtime, " ")); localConfig.Runtime == "" {
		return "", fmt.Errorf("runtime in config file(%s) is empty", confFile)
	}
	if localConfig.LogLevel != "" {
		err = nlog.ChangeLogLevel(localConfig.LogLevel)
		if err != nil {
			return "", fmt.Errorf("change log level failed: %s", err.Error())
		}
	}
	if runtimeType == config.ProcessRuntime || runtimeType == config.ContainerRuntime {
		return runtimeType, nil
	}

	if runtimeType == "" { // get from config file
		runtimeType = localConfig.Runtime
		return runtimeType, nil
	}
	return "", fmt.Errorf("invalidate runtime type: %s", runtimeType)

}

func NewImageService(runtimeType, storageDir string, args []string, cmd *cobra.Command) ImageService {
	var err error
	runtimeType, err = initRuntimeAndLogLevel(runtimeType)
	if err != nil {
		nlog.Fatal(err)
	}
	if runtimeType == config.ContainerRuntime {
		return &ContainerImage{
			args: args,
			cmd:  cmd,
		}
	} else {
		ociStore, err := store.NewOCIStore(storageDir, mounter.Plain)
		if err != nil {
			nlog.Fatal(err)
		}
		return &OciImage{
			args:       args,
			Store:      ociStore,
			StorageDir: storageDir,
		}
	}
}

func runContainerdCmd(ctx context.Context, name string, arg ...string) error {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

type OciImage struct {
	args       []string
	StorageDir string
	Store      store.Store
}

func (o *OciImage) RemoveImage() error {
	return o.Store.RemoveImage(o.args)
}

func (o *OciImage) LoadImage(tarFile string) error {
	return o.Store.LoadImage(tarFile)
}

func (o *OciImage) PullImage(creds string) error {
	var auth *runtimeapi.AuthConfig
	if creds != "" {
		up := strings.SplitN(creds, ":", 2)
		if len(up) != 2 {
			return fmt.Errorf("credentials must be username:password format")
		}
		auth = &runtimeapi.AuthConfig{
			Username: up[0],
			Password: up[1],
		}
	}
	return o.Store.PullImage(o.args[0], auth)
}

func (o *OciImage) ListImage() error {
	images, err := o.Store.ListImage()
	if err != nil {
		return fmt.Errorf("error: %s", err.Error())
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"REPOSITORY", "TAG", "IMAGE ID", "SIZE"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)

	for _, img := range images {
		table.Append([]string{
			img.Repository,
			img.Tag,
			img.ImageID,
			img.Size,
		})
	}

	table.Render()

	return nil
}

func (o *OciImage) TagImage() error {
	org, err := kii.NewImageName(o.args[0])
	if err != nil {
		return fmt.Errorf("SOURCE_IMAGE(%s) is invalidate", o.args[0])
	}
	targetImage := store.CheckTagCompliance(o.args[1])
	newImageName, err := kii.NewImageName(targetImage)
	if err != nil {
		return fmt.Errorf("TARGET_IMAGE(%s) is invalidate", o.args[1])
	}
	if err = o.Store.TagImage(org, newImageName); err != nil {
		return fmt.Errorf("tag image error: %s", err.Error())
	}
	return nil
}

func (o *OciImage) MountImage() error {

	return MountImage(o.Store, o.StorageDir, o.args[0], o.args[1], o.args[2])
}

func (o *OciImage) BuiltinImage(manifestFile string) error {
	return o.Store.RegisterImage(o.args[0], manifestFile)
}

func (o *OciImage) ImageRun(name string) error {
	if name == "" {
		name = uuid.NewString()
	}
	return runpStartContainer(name, o.args)

}

func runpStartContainer(cname string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no image specified in args")
	}

	sandboxBundle, imageStore, logDirectory, err := initContainerEnv(".")
	if err != nil {
		return err
	}

	c := &runtimeapi.ContainerConfig{
		Metadata: &runtimeapi.ContainerMetadata{
			Name: cname,
		},
		Image: &runtimeapi.ImageSpec{
			Image: args[0],
		},
		LogPath: "0.log",
	}
	if len(args) > 1 {
		c.Command = args[1:]
	}

	ctr, err := container.NewContainer(c, logDirectory, cname, sandboxBundle, imageStore)
	if err != nil {
		return fmt.Errorf("create container failed: %s", err.Error())
	}

	if err = ctr.Create(); err != nil {
		return fmt.Errorf("container init failed: %s", err.Error())
	}

	if err = ctr.Start(); err != nil {
		return fmt.Errorf("container start failed: %s", err.Error())
	}

	for {
		if status := ctr.GetStatus(); status.State() != runtimeapi.ContainerState_CONTAINER_RUNNING {
			break
		}
		time.Sleep(time.Second)
		print(".")
	}
	return nil
}

func initContainerEnv(rootDir string) (*layout.Bundle, store.Store, string, error) {
	imageStoreDir := filepath.Join(rootDir, config.DefaultImageStoreDir())
	sandboxDir := filepath.Join(rootDir, "sandbox")
	logDirectory := filepath.Join(rootDir, "logs")

	imageStore, err := store.NewOCIStore(imageStoreDir, mounter.Plain)
	if err != nil {
		return nil, nil, "", fmt.Errorf("create image store(%s) failed: %s", imageStoreDir, err.Error())
	}
	sandboxBundle, err := layout.NewBundle(sandboxDir)
	if err != nil {
		return nil, nil, "", fmt.Errorf("create image sandbox bounle(%s) failed: %s", sandboxDir, err.Error())
	}

	if err := paths.EnsureDirectory(logDirectory, true); err != nil {
		return nil, nil, "", fmt.Errorf("create logs dir(%s) failed: %s", logDirectory, err.Error())
	}

	return sandboxBundle, imageStore, logDirectory, nil
}

type ContainerImage struct {
	args []string
	cmd  *cobra.Command
}

func (c *ContainerImage) RemoveImage() error {
	cmdArgs := []string{"--runtime-endpoint", common.DefaultCRIRemoteEndpoint(), "rmi"}
	if len(c.args) > 0 {
		cmdArgs = append(cmdArgs, c.args...)
	}
	return runContainerdCmd(c.cmd.Context(), "crictl", cmdArgs...)

}

func (c *ContainerImage) LoadImage(tarFile string) error {
	return runContainerdCmd(c.cmd.Context(), "ctr", "--address", common.ContainerdSocket(), "--namespace", common.KusciaDefaultNamespaceOfContainerd, "images", "import", "--no-unpack", tarFile)
}

func (c *ContainerImage) PullImage(creds string) error {
	return runContainerdCmd(c.cmd.Context(), "crictl", "--runtime-endpoint", common.DefaultCRIRemoteEndpoint(), "pull", "--creds", creds, c.args[0])
}

func (c *ContainerImage) ListImage() error {
	return runContainerdCmd(c.cmd.Context(), "crictl", "--runtime-endpoint", common.DefaultCRIRemoteEndpoint(), "images", "ls")
}

func (c *ContainerImage) TagImage() error {
	targetImage := store.CheckTagCompliance(c.args[1])
	return runContainerdCmd(c.cmd.Context(), "ctr", "--address", common.ContainerdSocket(), "--namespace", common.KusciaDefaultNamespaceOfContainerd, "image", "tag", c.args[0], targetImage)
}

func (c *ContainerImage) MountImage() error {

	return fmt.Errorf("not support container image mount")
}

func (c *ContainerImage) BuiltinImage(manifestFile string) error {
	return fmt.Errorf("not support container image builtin")

}

func (c *ContainerImage) ImageRun(name string) error {
	cmdArgs := []string{fmt.Sprintf("-a=%s", common.ContainerdSocket()), fmt.Sprintf("-n=%s", common.KusciaDefaultNamespaceOfContainerd), "run"}
	cmdArgs = append(cmdArgs, c.args...)
	return runContainerdCmd(c.cmd.Context(), "ctr", cmdArgs...)
}

func MountImage(store store.Store, storageDir string, mountID, image, targetPath string) error {
	nlog.Infof("mountID=%s, image=%s, targetPath=%s", mountID, image, targetPath)
	imageName, err := kii.NewImageName(image)
	if err != nil {
		return err
	}

	bundle, err := layout.NewBundle(storageDir)
	if err != nil {
		return err
	}

	rBundle := bundle.GetContainerBundle(mountID)

	return store.MountImage(imageName, rBundle.GetFsWorkingDirPath(), targetPath)
}
