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

package service

import (
	"context"
	"fmt"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/utils"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
	v1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type IAppImageService interface {
	CreateAppImage(ctx context.Context, request *kusciaapi.CreateAppImageRequest) *kusciaapi.CreateAppImageResponse
	QueryAppImage(ctx context.Context, request *kusciaapi.QueryAppImageRequest) *kusciaapi.QueryAppImageResponse
	UpdateAppImage(ctx context.Context, request *kusciaapi.UpdateAppImageRequest) *kusciaapi.UpdateAppImageResponse
	DeleteAppImage(ctx context.Context, request *kusciaapi.DeleteAppImageRequest) *kusciaapi.DeleteAppImageResponse
	BatchQueryAppImage(ctx context.Context, request *kusciaapi.BatchQueryAppImageRequest) *kusciaapi.BatchQueryAppImageResponse
}

type appImageService struct {
	kusciaClient kusciaclientset.Interface
	conf         *config.KusciaAPIConfig
}

func NewAppImageService(config *config.KusciaAPIConfig) IAppImageService {
	switch config.RunMode {
	case common.RunModeLite:
		return &appImageServiceLite{
			kusciaAPIClient: proxy.NewKusciaAPIClient(""),
		}
	default:
		return &appImageService{
			kusciaClient: config.KusciaClient,
			conf:         config,
		}
	}
}

func (s appImageService) CreateAppImage(ctx context.Context, request *kusciaapi.CreateAppImageRequest) *kusciaapi.CreateAppImageResponse {
	// validate
	if err := validateCreateAppImageRequest(request); err != nil {
		return &kusciaapi.CreateAppImageResponse{Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error())}
	}
	// build resources
	k8sAppImage, err := buildK8sAppImage(request.Name, request.Image, request.ConfigTemplates, request.DeployTemplates)
	if err != nil {
		return &kusciaapi.CreateAppImageResponse{Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrCreateAppImage, err.Error())}
	}

	// create resources
	if _, err := s.kusciaClient.KusciaV1alpha1().AppImages().Create(ctx, k8sAppImage, metav1.CreateOptions{}); err != nil {
		return &kusciaapi.CreateAppImageResponse{Status: utils.BuildErrorResponseStatus(errorcode.GetAppImageErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrCreateAppImage), err.Error())}
	}

	return &kusciaapi.CreateAppImageResponse{Status: utils.BuildSuccessResponseStatus()}
}

func (s appImageService) QueryAppImage(ctx context.Context, request *kusciaapi.QueryAppImageRequest) *kusciaapi.QueryAppImageResponse {
	// validate
	appImageName := request.Name
	if appImageName == "" {
		return &kusciaapi.QueryAppImageResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "appimage name can not be empty"),
		}
	}
	// get resources
	k8sAppImage, err := s.kusciaClient.KusciaV1alpha1().AppImages().Get(ctx, appImageName, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.QueryAppImageResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetAppImageErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrQueryAppImage), err.Error()),
		}
	}

	return &kusciaapi.QueryAppImageResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   buildAppImage(k8sAppImage),
	}
}

func (s appImageService) BatchQueryAppImage(ctx context.Context, request *kusciaapi.BatchQueryAppImageRequest) *kusciaapi.BatchQueryAppImageResponse {
	// do validate
	names := request.Names
	if len(names) == 0 {
		return &kusciaapi.BatchQueryAppImageResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "names can not be empty"),
		}
	}
	for i, name := range names {
		if name == "" {
			return &kusciaapi.BatchQueryAppImageResponse{
				Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, fmt.Sprintf("name can not be empty on index %d", i)),
			}
		}
	}
	appImages := make([]*kusciaapi.QueryAppImageResponseData, len(names))
	for i, appImageName := range names {
		// get resources
		k8sAppImage, err := s.kusciaClient.KusciaV1alpha1().AppImages().Get(ctx, appImageName, metav1.GetOptions{})
		if err != nil {
			return &kusciaapi.BatchQueryAppImageResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetAppImageErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrBatchQueryAppImage), fmt.Sprintf("can't get appimage for %s, err: %v", appImageName, err)),
			}
		}
		appImages[i] = buildAppImage(k8sAppImage)
	}
	return &kusciaapi.BatchQueryAppImageResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   appImages,
	}
}

func (s appImageService) UpdateAppImage(ctx context.Context, request *kusciaapi.UpdateAppImageRequest) *kusciaapi.UpdateAppImageResponse {
	// validate
	if err := validateUpdateAppImageRequest(request); err != nil {
		return &kusciaapi.UpdateAppImageResponse{Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error())}
	}

	// get resource
	k8sAppImage, err := s.kusciaClient.KusciaV1alpha1().AppImages().Get(ctx, request.Name, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.UpdateAppImageResponse{Status: utils.BuildErrorResponseStatus(errorcode.GetAppImageErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrUpdateAppImage), err.Error())}
	}

	// override resources
	if err := overrideK8sAppImage(k8sAppImage, request.Image, request.ConfigTemplates, request.DeployTemplates); err != nil {
		return &kusciaapi.UpdateAppImageResponse{Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrUpdateAppImage, err.Error())}
	}

	// update resources
	if _, err := s.kusciaClient.KusciaV1alpha1().AppImages().Update(ctx, k8sAppImage, metav1.UpdateOptions{}); err != nil {
		return &kusciaapi.UpdateAppImageResponse{Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrUpdateAppImage, err.Error())}
	}

	return &kusciaapi.UpdateAppImageResponse{Status: utils.BuildSuccessResponseStatus()}
}

func (s appImageService) DeleteAppImage(ctx context.Context, request *kusciaapi.DeleteAppImageRequest) *kusciaapi.DeleteAppImageResponse {
	// validate
	appImageName := request.Name
	if appImageName == "" {
		return &kusciaapi.DeleteAppImageResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "appimage name can not be empty"),
		}
	}
	// delete resources
	err := s.kusciaClient.KusciaV1alpha1().AppImages().Delete(ctx, appImageName, metav1.DeleteOptions{})
	if err != nil {
		return &kusciaapi.DeleteAppImageResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrDeleteAppImage, err.Error()),
		}
	}
	return &kusciaapi.DeleteAppImageResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func validateCreateAppImageRequest(request *kusciaapi.CreateAppImageRequest) error {
	// do validate
	if request.Name == "" {
		return fmt.Errorf("appimage name can not be empty")
	}
	if request.Image == nil || request.Image.Name == "" {
		return fmt.Errorf("base image name can not be empty")
	}
	if err := validateDeployTemplates(request.DeployTemplates); err != nil {
		return err
	}
	// do k8s validate
	if err := resources.ValidateK8sName(request.Name, "appimage_id"); err != nil {
		return err
	}
	return nil
}

func validateUpdateAppImageRequest(request *kusciaapi.UpdateAppImageRequest) error {
	if request.Name == "" {
		return fmt.Errorf("appimage name can not be empty")
	}
	if request.Image == nil || request.Image.Name == "" {
		return fmt.Errorf("base image name can not be empty")
	}
	if request.DeployTemplates != nil {
		if err := validateDeployTemplates(request.DeployTemplates); err != nil {
			return err
		}
	}
	return nil
}

func validateDeployTemplates(deployTemplates []*kusciaapi.DeployTemplate) error {
	if len(deployTemplates) == 0 {
		return fmt.Errorf("deploy templates can not be empty")
	}
	for _, deployTemplate := range deployTemplates {
		if deployTemplate.Name == "" {
			return fmt.Errorf("deploy template name can not be empty")
		}
		if len(deployTemplate.Containers) == 0 {
			return fmt.Errorf("containers can not be empty")
		}
		if deployTemplate.Replicas < 0 {
			return fmt.Errorf("replicas can not be less than 0")
		}
		for _, container := range deployTemplate.Containers {
			if container.Name == "" {
				return fmt.Errorf("container name can not be empty")
			}
		}
	}
	return nil
}

func buildAppImage(k8sAppImage *v1alpha1.AppImage) *kusciaapi.QueryAppImageResponseData {
	return &kusciaapi.QueryAppImageResponseData{
		Name: k8sAppImage.GetName(),
		Image: &kusciaapi.AppImageInfo{
			Name: k8sAppImage.Spec.Image.Name,
			Tag:  k8sAppImage.Spec.Image.Tag,
			Id:   k8sAppImage.Spec.Image.ID,
			Sign: k8sAppImage.Spec.Image.Sign,
		},
		ConfigTemplates: k8sAppImage.Spec.ConfigTemplates,
		DeployTemplates: convertSlice(k8sAppImage.Spec.DeployTemplates, apiDeployTemplate),
	}
}

func buildK8sAppImage(name string, imageInfo *kusciaapi.AppImageInfo, configTemplates map[string]string, deployTemplates []*kusciaapi.DeployTemplate) (*v1alpha1.AppImage, error) {
	k8sDeployTemplates, err := convertSliceWithError(deployTemplates, k8sDeployTemplate)
	if err != nil {
		return nil, err
	}
	return &v1alpha1.AppImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.AppImageSpec{
			Image: v1alpha1.AppImageInfo{
				Name: imageInfo.Name,
				Tag:  imageInfo.Tag,
				ID:   imageInfo.Id,
				Sign: imageInfo.Sign,
			},
			ConfigTemplates: configTemplates,
			DeployTemplates: k8sDeployTemplates,
		},
	}, nil
}

func overrideK8sAppImage(k8sAppImage *v1alpha1.AppImage, imageInfo *kusciaapi.AppImageInfo, configTemplates map[string]string, deployTemplates []*kusciaapi.DeployTemplate) error {
	k8sAppImage.Spec.Image = v1alpha1.AppImageInfo{
		Name: imageInfo.Name,
		Tag:  imageInfo.Tag,
		ID:   imageInfo.Id,
		Sign: imageInfo.Sign,
	}
	if configTemplates != nil {
		k8sAppImage.Spec.ConfigTemplates = configTemplates
	}
	if deployTemplates != nil {
		k8sDeployTemplates, err := convertSliceWithError(deployTemplates, k8sDeployTemplate)
		if err != nil {
			return err
		}
		k8sAppImage.Spec.DeployTemplates = k8sDeployTemplates
	}
	return nil
}

func k8sDeployTemplate(template *kusciaapi.DeployTemplate) (ret v1alpha1.DeployTemplate, err error) {
	ret.Name = template.Name
	ret.Role = template.Role
	ret.Replicas = &template.Replicas
	// if template.NetworkPolicy != nil {
	// 	ret.NetworkPolicy = &v1alpha1.NetworkPolicy{Ingresses: convertSlice(template.NetworkPolicy.Ingresses, k8sIngress)}
	// }
	k8sContainers, err := convertSliceWithError(template.Containers, k8sContainer)
	if err != nil {
		return ret, err
	}
	ret.Spec = v1alpha1.PodSpec{
		Containers:    k8sContainers,
		RestartPolicy: v1.RestartPolicy(template.RestartPolicy),
	}
	return
}

func apiDeployTemplate(template v1alpha1.DeployTemplate) *kusciaapi.DeployTemplate {
	ret := &kusciaapi.DeployTemplate{}
	ret.Name = template.Name
	ret.Role = template.Role
	if template.Replicas != nil {
		ret.Replicas = *template.Replicas
	}
	// if template.NetworkPolicy != nil {
	// 	ret.NetworkPolicy = &kusciaapi.NetworkPolicy{Ingresses: convertSlice(template.NetworkPolicy.Ingresses, apiIngress)}
	// }
	ret.Containers = convertSlice(template.Spec.Containers, apiContainer)
	ret.RestartPolicy = string(template.Spec.RestartPolicy)
	return ret
}

// func k8sIngress(ingress *kusciaapi.Ingress) (ret v1alpha1.Ingress) {
// 	if ingress.From != nil {
// 		ret.From = v1alpha1.IngressFrom{Roles: ingress.From.Roles}
// 	}
// 	ret.Ports = convertSlice(ingress.Ports, k8sIngressPort)
// 	return
// }

// func apiIngress(ingress v1alpha1.Ingress) *kusciaapi.Ingress {
// 	return &kusciaapi.Ingress{
// 		From:  &kusciaapi.IngressFrom{Roles: ingress.From.Roles},
// 		Ports: convertSlice(ingress.Ports, apiIngressPort),
// 	}
// }

// func k8sIngressPort(port *kusciaapi.IngressPort) v1alpha1.IngressPort {
// 	return v1alpha1.IngressPort{Port: port.Port}
// }

// func apiIngressPort(port v1alpha1.IngressPort) *kusciaapi.IngressPort {
// 	return &kusciaapi.IngressPort{Port: port.Port}
// }

func k8sContainer(container *kusciaapi.Container) (v1alpha1.Container, error) {
	k8sResource, err := k8sResourceRequirement(container.Resources)
	if err != nil {
		return v1alpha1.Container{}, err
	}
	return v1alpha1.Container{
		Name:               container.Name,
		Command:            container.Command,
		Args:               container.Args,
		WorkingDir:         container.WorkingDir,
		ConfigVolumeMounts: convertSlice(container.ConfigVolumeMounts, k8sConfigVolumeMount),
		Ports:              convertSlice(container.Ports, k8sContainerPort),
		EnvFrom:            convertSlice(container.EnvFrom, k8sEnvFromSource),
		Env:                convertSlice(container.Env, k8sEnvVar),
		Resources:          k8sResource,
		LivenessProbe:      k8sProbe(container.LivenessProbe),
		ReadinessProbe:     k8sProbe(container.ReadinessProbe),
		StartupProbe:       k8sProbe(container.StartupProbe),
		ImagePullPolicy:    v1.PullPolicy(container.ImagePullPolicy),
	}, nil
}

func apiContainer(container v1alpha1.Container) *kusciaapi.Container {
	return &kusciaapi.Container{
		Name:               container.Name,
		Command:            container.Command,
		Args:               container.Args,
		WorkingDir:         container.WorkingDir,
		ConfigVolumeMounts: convertSlice(container.ConfigVolumeMounts, apiConfigVolumeMount),
		Ports:              convertSlice(container.Ports, apiContainerPort),
		EnvFrom:            convertSlice(container.EnvFrom, apiEnvFromSource),
		Env:                convertSlice(container.Env, apiEnvVar),
		Resources:          apiResourceRequirement(container.Resources),
		LivenessProbe:      apiProbe(container.LivenessProbe),
		ReadinessProbe:     apiProbe(container.ReadinessProbe),
		StartupProbe:       apiProbe(container.StartupProbe),
		ImagePullPolicy:    string(container.ImagePullPolicy),
	}
}

func k8sConfigVolumeMount(configVolumeMount *kusciaapi.ConfigVolumeMount) v1alpha1.ConfigVolumeMount {
	return v1alpha1.ConfigVolumeMount{
		MountPath: configVolumeMount.MountPath,
		SubPath:   configVolumeMount.SubPath,
	}
}

func apiConfigVolumeMount(configVolumeMount v1alpha1.ConfigVolumeMount) *kusciaapi.ConfigVolumeMount {
	return &kusciaapi.ConfigVolumeMount{
		MountPath: configVolumeMount.MountPath,
		SubPath:   configVolumeMount.SubPath,
	}
}

func k8sContainerPort(port *kusciaapi.ContainerPort) v1alpha1.ContainerPort {
	return v1alpha1.ContainerPort{
		Name:     port.Name,
		Protocol: v1alpha1.PortProtocol(port.Protocol),
		Scope:    v1alpha1.PortScope(port.Scope),
	}
}

func apiContainerPort(port v1alpha1.ContainerPort) *kusciaapi.ContainerPort {
	return &kusciaapi.ContainerPort{
		Name:     port.Name,
		Protocol: string(port.Protocol),
		Scope:    string(port.Scope),
	}
}

func k8sEnvFromSource(env *kusciaapi.EnvFromSource) (ret v1.EnvFromSource) {
	ret.Prefix = env.GetPrefix()
	if env.ConfigMapRef != nil {
		ret.ConfigMapRef = &v1.ConfigMapEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: env.ConfigMapRef.GetName()}}
	}
	if env.SecretRef != nil {
		ret.SecretRef = &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: env.SecretRef.GetName()}}
	}
	return
}

func apiEnvFromSource(env v1.EnvFromSource) *kusciaapi.EnvFromSource {
	ret := &kusciaapi.EnvFromSource{}
	ret.Prefix = env.Prefix
	if env.ConfigMapRef != nil {
		ret.ConfigMapRef = &kusciaapi.ConfigMapEnvSource{Name: env.ConfigMapRef.LocalObjectReference.Name}
	}
	if env.SecretRef != nil {
		ret.SecretRef = &kusciaapi.SecretEnvSource{Name: env.SecretRef.LocalObjectReference.Name}
	}
	return ret
}

func k8sEnvVar(env *kusciaapi.EnvVar) v1.EnvVar {
	return v1.EnvVar{
		Name:  env.GetName(),
		Value: env.GetValue(),
	}
}

func apiEnvVar(env v1.EnvVar) *kusciaapi.EnvVar {
	return &kusciaapi.EnvVar{
		Name:  env.Name,
		Value: env.Value,
	}
}

func k8sResourceRequirement(resource *kusciaapi.ResourceRequirements) (v1.ResourceRequirements, error) {
	if resource == nil {
		return v1.ResourceRequirements{}, nil
	}
	limits := v1.ResourceList{}
	if resource.Limits != nil {
		if resource.Limits.Cpu != "" {
			q, err := k8sresource.ParseQuantity(resource.Limits.Cpu)
			if err != nil {
				return v1.ResourceRequirements{}, fmt.Errorf("can't parse limit cpu %s to resource quantity, %v", resource.Limits.Cpu, err)
			}
			limits[v1.ResourceCPU] = q
		}
		if resource.Limits.Memory != "" {
			q, err := k8sresource.ParseQuantity(resource.Limits.Memory)
			if err != nil {
				return v1.ResourceRequirements{}, fmt.Errorf("can't parse limit memory %s to resource quantity, %v", resource.Limits.Cpu, err)
			}
			limits[v1.ResourceMemory] = q
		}
	}
	requests := v1.ResourceList{}
	if resource.Requests != nil {
		if resource.Requests.Cpu != "" {
			q, err := k8sresource.ParseQuantity(resource.Requests.Cpu)
			if err != nil {
				return v1.ResourceRequirements{}, fmt.Errorf("can't parse request cpu %s to resource quantity, %v", resource.Limits.Cpu, err)
			}
			requests[v1.ResourceCPU] = q
		}
		if resource.Requests.Memory != "" {
			q, err := k8sresource.ParseQuantity(resource.Requests.Memory)
			if err != nil {
				return v1.ResourceRequirements{}, fmt.Errorf("can't parse request memory %s to resource quantity, %v", resource.Limits.Cpu, err)
			}
			requests[v1.ResourceMemory] = q
		}
	}

	return v1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}, nil
}

func apiResourceRequirement(resource v1.ResourceRequirements) *kusciaapi.ResourceRequirements {
	if len(resource.Limits) == 0 && len(resource.Requests) == 0 {
		return nil
	}
	ret := &kusciaapi.ResourceRequirements{}
	if len(resource.Limits) != 0 {
		limits := &kusciaapi.ResourceSpec{}
		for k, v := range resource.Limits {
			if k == v1.ResourceCPU {
				limits.Cpu = v.String()
			}
			if k == v1.ResourceMemory {
				limits.Memory = v.String()
			}
		}
		ret.Limits = limits
	}

	if len(resource.Requests) != 0 {
		requests := &kusciaapi.ResourceSpec{}
		for k, v := range resource.Requests {
			if k == v1.ResourceCPU {
				requests.Cpu = v.String()
			}
			if k == v1.ResourceMemory {
				requests.Memory = v.String()
			}
		}
		ret.Requests = requests
	}
	return ret
}

func k8sProbe(probe *kusciaapi.Probe) *v1.Probe {
	ret := &v1.Probe{}
	if probe == nil {
		return nil
	}
	if probe.Exec != nil {
		ret.Exec = &v1.ExecAction{
			Command: probe.Exec.Command,
		}
	}
	if probe.HttpGet != nil {
		ret.HTTPGet = &v1.HTTPGetAction{
			Path:        probe.HttpGet.GetPath(),
			Port:        intstr.FromString(probe.HttpGet.GetPort()),
			Host:        probe.HttpGet.GetHost(),
			Scheme:      v1.URIScheme(probe.HttpGet.GetScheme()),
			HTTPHeaders: convertSlice(probe.HttpGet.HttpHeaders, k8sHTTPHeader),
		}
	}
	if probe.TcpSocket != nil {
		ret.TCPSocket = &v1.TCPSocketAction{
			Port: intstr.FromString(probe.TcpSocket.GetPort()),
			Host: probe.TcpSocket.GetHost(),
		}
	}
	if probe.Grpc != nil {
		ret.GRPC = &v1.GRPCAction{
			Port: probe.Grpc.GetPort(),
		}
		if probe.Grpc.Service != "" {
			ret.GRPC.Service = &probe.Grpc.Service
		}
	}
	ret.InitialDelaySeconds = probe.InitialDelaySeconds
	ret.TimeoutSeconds = probe.TimeoutSeconds
	ret.PeriodSeconds = probe.PeriodSeconds
	ret.SuccessThreshold = probe.SuccessThreshold
	ret.FailureThreshold = probe.FailureThreshold
	if probe.TerminationGracePeriodSeconds != 0 {
		ret.TerminationGracePeriodSeconds = &probe.TerminationGracePeriodSeconds
	}

	return ret
}

func apiProbe(probe *v1.Probe) *kusciaapi.Probe {
	ret := &kusciaapi.Probe{}
	if probe == nil {
		return nil
	}
	if probe.Exec != nil {
		ret.Exec = &kusciaapi.ExecAction{
			Command: probe.Exec.Command,
		}
	}
	if probe.HTTPGet != nil {
		ret.HttpGet = &kusciaapi.HTTPGetAction{
			Path:        probe.HTTPGet.Path,
			Port:        probe.HTTPGet.Port.StrVal,
			Host:        probe.HTTPGet.Host,
			Scheme:      string(probe.HTTPGet.Scheme),
			HttpHeaders: convertSlice(probe.HTTPGet.HTTPHeaders, apiHTTPHeader),
		}
	}
	if probe.TCPSocket != nil {
		ret.TcpSocket = &kusciaapi.TCPSocketAction{
			Port: probe.TCPSocket.Port.StrVal,
			Host: probe.TCPSocket.Host,
		}
	}
	if probe.GRPC != nil {
		ret.Grpc = &kusciaapi.GRPCAction{
			Port: probe.GRPC.Port,
		}
		if probe.GRPC.Service != nil {
			ret.Grpc.Service = *probe.GRPC.Service
		}
	}
	ret.InitialDelaySeconds = probe.InitialDelaySeconds
	ret.TimeoutSeconds = probe.TimeoutSeconds
	ret.PeriodSeconds = probe.PeriodSeconds
	ret.SuccessThreshold = probe.SuccessThreshold
	ret.FailureThreshold = probe.FailureThreshold
	if probe.TerminationGracePeriodSeconds != nil {
		ret.TerminationGracePeriodSeconds = *probe.TerminationGracePeriodSeconds
	}
	return ret
}
func k8sHTTPHeader(header *kusciaapi.HTTPHeader) v1.HTTPHeader {
	return v1.HTTPHeader{
		Name:  header.GetName(),
		Value: header.GetValue(),
	}
}

func apiHTTPHeader(header v1.HTTPHeader) *kusciaapi.HTTPHeader {
	return &kusciaapi.HTTPHeader{
		Name:  header.Name,
		Value: header.Value,
	}
}

func convertSlice[S any, D any](src []S, convertFunc func(S) D) []D {
	dest := make([]D, len(src))
	for i, v := range src {
		dest[i] = convertFunc(v)
	}
	return dest
}

func convertSliceWithError[S any, D any](src []S, convertFunc func(S) (D, error)) ([]D, error) {
	var err error
	dest := make([]D, len(src))
	for i, v := range src {
		dest[i], err = convertFunc(v)
		if err != nil {
			return nil, err
		}
	}
	return dest, nil
}
