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

package configrender

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	pluginNameConfigRender = "config-render"

	KubeStorageConfigDataAnnotation = "config-data.kuscia.secretflow/kube-storage"
)

func Register() {
	plugin.Register(pluginNameConfigRender, &configRender{})
}

type configRenderConfig struct {
}

type configRender struct {
	config configRenderConfig
}

// Type implements the plugin.Plugin interface.
func (cr *configRender) Type() string {
	return hook.PluginType
}

// Init implements the plugin.Plugin interface.
func (cr *configRender) Init(dependencies *plugin.Dependencies, cfg *config.PluginCfg) error {
	if err := cfg.Config.Decode(&cr.config); err != nil {
		return err
	}

	hook.Register(pluginNameConfigRender, cr)
	return nil
}

// CanExec implements the hook.Handler interface.
// It returns true if point is equal to PointMakeMounts and obj.Mount.Name is equal to
// configTemplateVolumesAnnotation value.
func (cr *configRender) CanExec(obj interface{}, point hook.Point) bool {
	if point != hook.PointMakeMounts {
		return false
	}

	mObj, ok := obj.(*hook.MakeMountsObj)
	if !ok {
		return false
	}

	if mObj.Mount.Name != mObj.Pod.Annotations[common.ConfigTemplateVolumesAnnotationKey] {
		return false
	}

	return true
}

// ExecHook implements the hook.Handler interface.
// It renders the configuration template and writes the generated real configuration content to a new file/directory.
// The value of hostPath will be replaced by the new file/directory path.
func (cr *configRender) ExecHook(obj interface{}, point hook.Point) (*hook.Result, error) {
	result := &hook.Result{}

	mObj, ok := obj.(*hook.MakeMountsObj)
	if !ok {
		return nil, fmt.Errorf("can't convert object to MakeMountsObj")
	}

	envs := map[string]string{}
	for _, env := range mObj.Envs {
		envs[env.Name] = env.Value
	}

	data, err := cr.makeDataMap(mObj.Pod.Annotations, envs)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(mObj.PodVolumesDir, "config-render", mObj.Container.Name, mObj.Mount.Name, mObj.Mount.SubPath)

	hostPath := *mObj.HostPath
	info, err := os.Stat(hostPath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		if err := cr.renderConfigDirectory(hostPath, configPath, data); err != nil {
			return nil, fmt.Errorf("failed to render config templates in %q, detail-> %v", hostPath, err)
		}
	} else {
		if err := cr.renderConfigFile(hostPath, configPath, data); err != nil {
			return nil, fmt.Errorf("failed to render config template file %q, detail-> %v", hostPath, err)
		}
	}

	*mObj.HostPath = configPath

	nlog.Infof("Render config template for container %q in pod %q succeed, templatePath=%v, configPath=%v",
		mObj.Container.Name, format.Pod(mObj.Pod), hostPath, configPath)

	return result, nil
}

func (cr *configRender) renderConfigDirectory(templateDir, configDir string, data map[string]string) error {
	info, err := os.Stat(templateDir)
	if err != nil {
		return err
	}

	if err := paths.EnsureDirectoryPerm(configDir, true, info.Mode()); err != nil {
		return fmt.Errorf("failed to ensure directory %q exists, detail-> %v", configDir, err)
	}

	err = filepath.Walk(templateDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}

		configFile := filepath.Join(configDir, relPath)

		if err := cr.renderConfigFile(path, configFile, data); err != nil {
			return fmt.Errorf("failed to render config template %q, detail-> %v", path, err)
		}

		return nil
	})

	return err
}

func (cr *configRender) renderConfigFile(templateFile, configFile string, data map[string]string) error {
	if err := paths.EnsureDirectory(filepath.Dir(configFile), true); err != nil {
		return fmt.Errorf("failed to ensure directory %q exists, detail-> %v", filepath.Dir(configFile), err)
	}

	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		return err
	}

	tmpl, err := template.New("config-template").Option("missingkey=zero").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse config template, detail-> %v", err)
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute config template, detail-> %v", err)
	}

	info, err := os.Stat(templateFile)
	if err != nil {
		return err
	}
	if err = os.WriteFile(configFile, buf.Bytes(), info.Mode()); err != nil {
		return fmt.Errorf("failed to write config file %q, detail-> %v", configFile, err)
	}

	nlog.Debugf("Render config template file succeed, templateFile=%v, configFile=%v", templateFile, configFile)

	return nil
}

func (cr *configRender) makeDataMap(annotations, envs map[string]string) (map[string]string, error) {
	mergedData := map[string]string{}
	var err error

	mergeDataMap(mergedData, envs)

	data := cr.makeDataMapFromLocal()
	mergeDataMap(mergedData, data)

	data = cr.makeDataMapFromCM()
	mergeDataMap(mergedData, data)

	if annotations[KubeStorageConfigDataAnnotation] == "true" {
		data, err = cr.makeDataMapFromKubeStorage()
		if err != nil {
			return nil, fmt.Errorf("failed to get config data from storage service, detail-> %v", err)
		}
		mergeDataMap(mergedData, data)
	}

	return mergedData, nil
}

func (cr *configRender) makeDataMapFromLocal() map[string]string {
	return map[string]string{}
}

func (cr *configRender) makeDataMapFromCM() map[string]string {
	res := map[string]string{}
	return res
}

func (cr *configRender) makeDataMapFromKubeStorage() (map[string]string, error) {
	res := map[string]string{}
	return res, nil
}

// mergeDataMap merges the contents of srcMap into dstMap. The variables in the configuration template
// are all uppercase letters, so the keys are converted to uppercase letters here.
func mergeDataMap(dst map[string]string, src map[string]string) {
	for k, v := range src {
		dst[strings.ToUpper(k)] = strings.Trim(strconv.Quote(v), "\"")
	}
}
