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
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	v1 "k8s.io/api/core/v1"

	"github.com/secretflow/kuscia/pkg/agent/config"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/middleware/plugin"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/common"
	uc "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
)

const (
	KubeStorageConfigDataAnnotation = "config-data.kuscia.secretflow/kube-storage"

	defaultTemplateRenderOption = "missingkey=zero"
)

func Register() {
	plugin.Register(common.PluginNameConfigRender, &configRender{})
}

type configRenderConfig struct {
	kusciaAPIProtocol common.Protocol
	// Todo: temporary solution for scql
	kusciaAPIToken string
	domainKeyData  string
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

	if dependencies != nil {
		cr.config.kusciaAPIProtocol = dependencies.AgentConfig.KusciaAPIProtocol
		cr.config.kusciaAPIToken = dependencies.AgentConfig.KusciaAPIToken
		cr.config.domainKeyData = dependencies.AgentConfig.DomainKeyData
	}

	hook.Register(common.PluginNameConfigRender, cr)
	return nil
}

// CanExec implements the hook.Handler interface.
// It returns true if point is equal to PointMakeMounts and obj.Mount.Name is equal to
// configTemplateVolumesAnnotation value.
func (cr *configRender) CanExec(ctx hook.Context) bool {
	switch ctx.Point() {
	case hook.PointMakeMounts:
		mCtx, ok := ctx.(*hook.MakeMountsContext)
		if !ok {
			return false
		}

		if mCtx.Mount.Name != mCtx.Pod.Annotations[common.ConfigTemplateVolumesAnnotationKey] {
			return false
		}

		return true
	case hook.PointK8sProviderSyncPod:
		syncPodCtx, ok := ctx.(*hook.K8sProviderSyncPodContext)
		if !ok {
			return false
		}

		if syncPodCtx.BkPod.Annotations[common.ConfigTemplateVolumesAnnotationKey] == "" {
			return false
		}

		return true
	default:
		return false
	}
}

// ExecHook implements the hook.Handler interface.
// It renders the configuration template and writes the generated real configuration content to a new file/directory.
// The value of hostPath will be replaced by the new file/directory path.
func (cr *configRender) ExecHook(ctx hook.Context) (*hook.Result, error) {
	result := &hook.Result{}

	switch ctx.Point() {
	case hook.PointMakeMounts:
		mCtx, ok := ctx.(*hook.MakeMountsContext)
		if !ok {
			return nil, fmt.Errorf("invalid context type %T", ctx)
		}

		if err := cr.handleMakeMountsContext(mCtx); err != nil {
			return nil, fmt.Errorf("failed to handle make mounts context: %v", err)
		}

		return result, nil
	case hook.PointK8sProviderSyncPod:
		syncPodCtx, ok := ctx.(*hook.K8sProviderSyncPodContext)
		if !ok {
			return nil, fmt.Errorf("invalid context type %T", ctx)
		}

		if err := cr.handleSyncPodContext(syncPodCtx); err != nil {
			return nil, fmt.Errorf("failed to handle sync pod context: %v", err)
		}

		return result, nil
	default:
		return nil, fmt.Errorf("invalid point %v", ctx.Point())
	}
}

func (cr *configRender) handleSyncPodContext(ctx *hook.K8sProviderSyncPodContext) error {
	pod := ctx.BkPod
	var configVolume *v1.Volume
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == pod.Annotations[common.ConfigTemplateVolumesAnnotationKey] {
			configVolume = &volume
			break
		}
	}

	if configVolume == nil || configVolume.ConfigMap == nil {
		nlog.Warnf("Config template volume not found in pod %q", format.Pod(pod))
		return nil
	}

	srcConfigMap, err := ctx.ResourceManager.GetConfigMap(configVolume.ConfigMap.Name)
	if err != nil {
		return fmt.Errorf("failed to get config map %q, detail-> %v", configVolume.ConfigMap.Name, err)
	}

	// TODO Let's assume that the environment variables of each container are not conflicting
	envs := map[string]string{}
	for _, c := range pod.Spec.Containers {
		for _, env := range c.Env {
			if _, ok := envs[env.Name]; !ok {
				envs[env.Name] = env.Value
			}
		}
	}

	data, err := cr.makeDataMap(ctx.Pod.Annotations, envs)
	if err != nil {
		return err
	}

	dstConfigMap, err := cr.renderConfigMap(srcConfigMap, data)
	if err != nil {
		return fmt.Errorf("failed to render config map %q, detail-> %v", srcConfigMap.Name, err)
	}

	ctx.Configmaps = append(ctx.Configmaps, dstConfigMap)

	nlog.Infof("Render config template k8s pod %q succeed, configMap=%v", format.Pod(ctx.Pod), dstConfigMap.Name)

	return nil
}

func (cr *configRender) renderConfigMap(srcConfigMap *v1.ConfigMap, data map[string]string) (*v1.ConfigMap, error) {
	dstConfigMap := srcConfigMap.DeepCopy()
	newData := map[string]string{}

	for key, value := range srcConfigMap.Data {
		config, err := cr.renderConfig(value, data)
		if err != nil {
			return nil, err
		}

		newData[key] = config
	}

	dstConfigMap.Data = newData
	return dstConfigMap, nil
}

func (cr *configRender) handleMakeMountsContext(ctx *hook.MakeMountsContext) error {
	envs := map[string]string{}
	for _, env := range ctx.Envs {
		envs[env.Name] = env.Value
	}

	data, err := cr.makeDataMap(ctx.Pod.Annotations, envs)
	if err != nil {
		return err
	}

	configPath := filepath.Join(ctx.PodVolumesDir, "config-render", ctx.Container.Name, ctx.Mount.Name, ctx.Mount.SubPath)

	hostPath := *ctx.HostPath
	info, err := os.Stat(hostPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := cr.renderConfigDirectory(hostPath, configPath, data); err != nil {
			return fmt.Errorf("failed to render config templates in %q, detail-> %v", hostPath, err)
		}
	} else {
		if err := cr.renderConfigFile(hostPath, configPath, data); err != nil {
			return fmt.Errorf("failed to render config template file %q, detail-> %v", hostPath, err)
		}
	}

	*ctx.HostPath = configPath

	nlog.Infof("Render config template for container %q in pod %q succeed, templatePath=%v, configPath=%v",
		ctx.Container.Name, format.Pod(ctx.Pod), hostPath, configPath)

	return nil
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

	configContent, err := cr.renderConfig(string(templateContent), data)
	if err != nil {
		return err
	}

	info, err := os.Stat(templateFile)
	if err != nil {
		return err
	}
	if err = os.WriteFile(configFile, []byte(configContent), info.Mode()); err != nil {
		return fmt.Errorf("failed to write config file %q, detail-> %v", configFile, err)
	}

	nlog.Debugf("Render config template file succeed, templateFile=%v, configFile=%v", templateFile, configFile)

	return nil
}

func (cr *configRender) renderConfig(templateContent string, data map[string]string) (string, error) {
	// replace {{{pattern}}} --> {{kuscia "pattern"}}
	re := regexp.MustCompile(`\{\{\{.+\}\}\}`)
	result := re.ReplaceAllStringFunc(templateContent, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) > 0 {
			return "{{kuscia " + "\"" + submatch[0][3:len(submatch[0])-3] + "\"" + "}}"
		}
		return match
	})

	// use to replace values
	kusciaQueryValue := func(query string) string {
		value := uc.QueryByFields(buildStructMap(data), query)
		if value == nil {
			return ""
		}

		if _, ok := value.(string); ok {
			return value.(string)
		}
		output, _ := json.Marshal(value)
		return string(output)
	}

	tmpl, err := template.New("config-template").Option(defaultTemplateRenderOption).Funcs(template.FuncMap{"kuscia": kusciaQueryValue}).Parse(string(result))
	if err != nil {
		return "", fmt.Errorf("failed to parse config template, detail-> %v", err)
	}

	quoteData := make(map[string]string)
	for k, v := range data {
		quoteData[k] = strings.Trim(strconv.Quote(v), "\"")
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, quoteData); err != nil {
		return "", fmt.Errorf("failed to execute config template, detail-> %v", err)
	}

	return buf.String(), nil
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
	return map[string]string{
		common.EnvKusciaAPIProtocol:   string(cr.config.kusciaAPIProtocol),
		common.EnvKusciaAPIToken:      cr.config.kusciaAPIToken,
		common.EnvKusciaDomainKeyData: cr.config.domainKeyData,
	}
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
		dst[strings.ToUpper(k)] = v
	}
}

func buildStructMap(value map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range value {
		result[k] = v

		// try json
		var payload interface{}
		if err := json.Unmarshal([]byte(v), &payload); err == nil {
			switch reflect.TypeOf(payload).Kind() {
			case reflect.Array, reflect.Map, reflect.Slice, reflect.Struct:
				result[k] = payload
			}

		}

		// TODO: try yaml
		//yamlPayload := make(map[string]interface{})
		//if err := yaml.Unmarshal([]byte(v), &yamlPayload); err == nil {
		//	switch reflect.TypeOf(yamlPayload).Kind() {
		//	case reflect.Array, reflect.Map, reflect.Slice:
		//		result[k] = yamlPayload
		//	default:
		//	}
		//}
	}

	return result
}
