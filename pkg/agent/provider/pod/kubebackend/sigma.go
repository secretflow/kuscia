// OPENSOURCE-CLEANUP DELETE_FILE

// Copyright 2023 Ant Group Co., Ltd.
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

package kubebackend

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	labelSigmaBizName        = "meta.k8s.alipay.com/biz-name"
	labelSigmaMigrationLevel = "meta.k8s.alipay.com/migration-level"

	labelSigmaAppName       = "sigma.ali/app-name"
	labelSigmaInstanceGroup = "sigma.ali/instance-group"
	labelSigmaSite          = "sigma.ali/site"
	labelSigmaDeployUnit    = "sigma.ali/deploy-unit"
	labelSigmaZone          = "meta.k8s.alipay.com/zone"
	labelEnableDefaultRoute = "ali.EnableDefaultRoute"

	envSigmaAppName = "ALIPAY_APP_APPNAME"
	envSigmaZone    = "ALIPAY_APP_ZONE"

	annotationSigmaAutoEviction = "pod.k8s.alipay.com/auto-eviction"
)

type sigmaConfig struct {
	AllowOverQuota bool `yaml:"allowOverQuota,omitempty"`
}

type sigma struct {
	config sigmaConfig
}

func (s *sigma) Init(cfg *yaml.Node) error {
	if err := cfg.Decode(&s.config); err != nil {
		return err
	}

	nlog.Infof("Sigma config: %+v", s.config)

	return nil
}

func (s *sigma) PreSyncPod(bkPod *v1.Pod) {
	if bkPod.Labels == nil {
		bkPod.Labels = map[string]string{}
	}

	fillLabelIfNotExist(bkPod, labelSigmaAppName, os.Getenv(envSigmaAppName))
	fillLabelIfNotExist(bkPod, labelSigmaZone, os.Getenv(envSigmaZone))
	fillLabelIfNotExist(bkPod, labelSigmaInstanceGroup, fmt.Sprintf("%shost", bkPod.Labels[labelSigmaAppName]))
	fillLabelIfNotExist(bkPod, labelSigmaMigrationLevel, "L3")
	fillLabelIfNotExist(bkPod, labelEnableDefaultRoute, "true")

	if bkPod.Annotations == nil {
		bkPod.Annotations = map[string]string{}
	}

	fillAnnotationIfNotExist(bkPod, annotationSigmaAutoEviction, "true")

	bkPod.Spec.Volumes = append(bkPod.Spec.Volumes, v1.Volume{
		Name: "logs",
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	})

	for i := range bkPod.Spec.Containers {
		c := &bkPod.Spec.Containers[i]
		c.VolumeMounts = append(c.VolumeMounts, v1.VolumeMount{
			Name:      "logs",
			MountPath: "/home/admin/logs",
		})

		fillResource(c)
	}

	if s.config.AllowOverQuota {
		if bkPod.Spec.Affinity == nil {
			bkPod.Spec.Affinity = &v1.Affinity{}
		}
		if bkPod.Spec.Affinity.NodeAffinity == nil {
			bkPod.Spec.Affinity.NodeAffinity = &v1.NodeAffinity{}
		}
		if bkPod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			bkPod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &v1.NodeSelector{}
		}

		bkPod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(
			bkPod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
			v1.NodeSelectorTerm{
				MatchExpressions: []v1.NodeSelectorRequirement{
					{
						Key:      "sigma.ali/is-over-quota",
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"true"},
					},
				},
			},
		)

		bkPod.Spec.Tolerations = append(bkPod.Spec.Tolerations, v1.Toleration{
			Key:      "sigma.ali/is-over-quota",
			Operator: v1.TolerationOpEqual,
			Value:    "true",
			Effect:   v1.TaintEffectNoSchedule,
		})
	}

	return
}

func fillResource(c *v1.Container) {
	if c.Resources.Limits == nil {
		c.Resources.Limits = map[v1.ResourceName]resource.Quantity{}
	}

	if _, ok := c.Resources.Limits[v1.ResourceCPU]; !ok {
		c.Resources.Limits[v1.ResourceCPU] = *resource.NewQuantity(2, resource.BinarySI)
	}
	if _, ok := c.Resources.Limits[v1.ResourceMemory]; !ok {
		c.Resources.Limits[v1.ResourceMemory] = *resource.NewQuantity(2*1000*1000*1000, resource.BinarySI)
	}
	if _, ok := c.Resources.Limits[v1.ResourceEphemeralStorage]; !ok {
		c.Resources.Limits[v1.ResourceEphemeralStorage] = *resource.NewQuantity(10*1000*1000*1000, resource.BinarySI)
	}

	if c.Resources.Requests == nil {
		c.Resources.Requests = map[v1.ResourceName]resource.Quantity{}
	}

	if _, ok := c.Resources.Requests[v1.ResourceCPU]; !ok {
		c.Resources.Requests[v1.ResourceCPU] = *resource.NewQuantity(2, resource.BinarySI)
	}
	if _, ok := c.Resources.Requests[v1.ResourceMemory]; !ok {
		c.Resources.Requests[v1.ResourceMemory] = *resource.NewQuantity(2*1000*1000*1000, resource.BinarySI)
	}
	if _, ok := c.Resources.Requests[v1.ResourceEphemeralStorage]; !ok {
		c.Resources.Requests[v1.ResourceEphemeralStorage] = *resource.NewQuantity(10*1000*1000*1000, resource.BinarySI)
	}
}

func fillLabelIfNotExist(bkPod *v1.Pod, key, value string) {
	if _, ok := bkPod.Labels[key]; !ok {
		bkPod.Labels[key] = value
	}
}

func fillAnnotationIfNotExist(bkPod *v1.Pod, key, value string) {
	if _, ok := bkPod.Annotations[key]; !ok {
		bkPod.Annotations[key] = value
	}
}

func init() {
	RegisterBackendPlugin("sigma", &sigma{})
}
