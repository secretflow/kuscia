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

package service

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	utils2 "github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

const (
	recreateDeploymentStrategyType      string = "Recreate"
	rollingUpdateDeploymentStrategyType string = "RollingUpdate"
)

type IServingService interface {
	CreateServing(ctx context.Context, request *kusciaapi.CreateServingRequest) *kusciaapi.CreateServingResponse
	QueryServing(ctx context.Context, request *kusciaapi.QueryServingRequest) *kusciaapi.QueryServingResponse
	BatchQueryServingStatus(ctx context.Context, request *kusciaapi.BatchQueryServingStatusRequest) *kusciaapi.BatchQueryServingStatusResponse
	UpdateServing(ctx context.Context, request *kusciaapi.UpdateServingRequest) *kusciaapi.UpdateServingResponse
	DeleteServing(ctx context.Context, request *kusciaapi.DeleteServingRequest) *kusciaapi.DeleteServingResponse
}

type servingService struct {
	Initiator    string
	kubeClient   kubernetes.Interface
	kusciaClient kusciaclientset.Interface
}

func NewServingService(config *config.KusciaAPIConfig) IServingService {
	return &servingService{
		Initiator:    config.Initiator,
		kubeClient:   config.KubeClient,
		kusciaClient: config.KusciaClient,
	}
}

func (s *servingService) CreateServing(ctx context.Context, request *kusciaapi.CreateServingRequest) *kusciaapi.CreateServingResponse {
	if request.ServingId == "" {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "serving id can not be empty"),
		}
	}

	if request.ServingInputConfig == "" {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "serving input config can not be empty"),
		}
	}

	if request.Initiator == "" {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "initiator can not be empty"),
		}
	}

	if s.Initiator != "" && s.Initiator != request.Initiator {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("initiator must be %s in P2P", request.Initiator)),
		}
	}

	if len(request.Parties) == 0 {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "parties can not be empty"),
		}
	}

	foundInitiator := false
	for i, party := range request.Parties {
		if party.AppImage == "" {
			return &kusciaapi.CreateServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("appimage can not be empty in parties[%d]", i)),
			}
		}
		if party.DomainId == "" {
			return &kusciaapi.CreateServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, fmt.Sprintf("domain id can not be empty in parties[%d]", i)),
			}
		}
		if party.DomainId == request.Initiator {
			foundInitiator = true
		}
	}
	if !foundInitiator {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "initiator should be one of the parties"),
		}
	}

	kdParties := make([]v1alpha1.KusciaDeploymentParty, len(request.Parties))
	for i, party := range request.Parties {
		kdParties[i] = v1alpha1.KusciaDeploymentParty{
			DomainID:    party.DomainId,
			AppImageRef: party.AppImage,
			Role:        party.Role,
		}

		s.fillKusciaDeploymentPartyReplicas(&kdParties[i], party.Replicas)
		strategy, err := s.buildKusciaDeploymentPartyStrategy(request.Parties[i])
		if err != nil {
			return &kusciaapi.CreateServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
			}
		}
		if strategy != nil {
			kdParties[i].Template.Strategy = strategy
		}

		containers, err := s.buildKusciaDeploymentPartyContainers(ctx, request.Parties[i])
		if err != nil {
			return &kusciaapi.CreateServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
			}
		}
		if len(containers) > 0 {
			kdParties[i].Template.Spec.Containers = containers
		}
	}

	kd := &v1alpha1.KusciaDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: request.ServingId,
			Labels: map[string]string{
				common.LabelKusciaDeploymentAppType: string(common.ServingAppType),
			},
		},
		Spec: v1alpha1.KusciaDeploymentSpec{
			Initiator:   request.Initiator,
			InputConfig: request.ServingInputConfig,
			Parties:     kdParties,
		},
	}

	if _, err := s.kusciaClient.KusciaV1alpha1().KusciaDeployments().Create(ctx, kd, metav1.CreateOptions{}); err != nil {
		return &kusciaapi.CreateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrCreateServing, err.Error()),
		}
	}

	return &kusciaapi.CreateServingResponse{
		Status: utils2.BuildSuccessResponseStatus(),
	}
}

func (s *servingService) fillKusciaDeploymentPartyReplicas(kdParty *v1alpha1.KusciaDeploymentParty, replicas *int32) {
	if replicas != nil {
		kdParty.Template.Replicas = replicas
	}
}

func (s *servingService) buildKusciaDeploymentPartyStrategy(party *kusciaapi.ServingParty) (*appsv1.DeploymentStrategy, error) {
	strategy := &appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &intstr.IntOrString{
				Type:   1,
				StrVal: "25%",
			},
			MaxSurge: &intstr.IntOrString{
				Type:   1,
				StrVal: "25%",
			},
		},
	}

	if party.UpdateStrategy == nil {
		return strategy, nil
	}

	if party.UpdateStrategy.Type != recreateDeploymentStrategyType && party.UpdateStrategy.Type != rollingUpdateDeploymentStrategyType {
		return nil, fmt.Errorf("update strategy type should be %v or %v for party/role %v/%v", recreateDeploymentStrategyType, rollingUpdateDeploymentStrategyType, party.DomainId, party.Role)
	}

	if party.UpdateStrategy.Type == recreateDeploymentStrategyType {
		strategy.Type = appsv1.RecreateDeploymentStrategyType
		strategy.RollingUpdate = nil
		return strategy, nil
	}

	if party.UpdateStrategy.MaxSurge != "" {
		maxSurge := intstr.Parse(party.UpdateStrategy.MaxSurge)
		strategy.RollingUpdate.MaxSurge = &maxSurge
	}

	if party.UpdateStrategy.MaxUnavailable != "" {
		maxUnavailable := intstr.Parse(party.UpdateStrategy.MaxUnavailable)
		strategy.RollingUpdate.MaxUnavailable = &maxUnavailable
	}

	return strategy, nil
}

func (s *servingService) buildKusciaDeploymentPartyContainers(ctx context.Context, party *kusciaapi.ServingParty) ([]v1alpha1.Container, error) {
	if len(party.Resources) == 0 {
		return nil, nil
	}

	partyTemplate, err := s.getAppImageTemplate(ctx, party.AppImage, party.Role)
	if err != nil {
		return nil, err
	}

	commonResource, ctrResource := s.buildServingPartyContainerResources(party)
	for ctrName := range ctrResource {
		found := false
		for _, ctr := range partyTemplate.Spec.Containers {
			if ctrName == ctr.Name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("container_name %v of party domainID/role %v/%v resources doesn't exist in app image %v",
				ctrName, party.DomainId, party.Role, party.AppImage)
		}
	}

	var containers []v1alpha1.Container
	for _, ctr := range partyTemplate.Spec.Containers {
		requestResource := corev1.ResourceList{}
		requestCPU := ""
		if ctrResource[ctr.Name] != nil && ctrResource[ctr.Name].MinCpu != "" {
			requestCPU = ctrResource[ctr.Name].MinCpu
		} else if commonResource != nil && commonResource.MinCpu != "" {
			requestCPU = commonResource.MinCpu
		}
		if err = s.fillKusciaDeploymentContainerResourceList(requestResource, requestCPU, corev1.ResourceCPU); err != nil {
			return nil, fmt.Errorf("resource min_cpu %v format is invalid for party domain_id/role %v/%v, %v", requestCPU, party.DomainId, party.Role, err)
		}

		requestMemory := ""
		if ctrResource[ctr.Name] != nil && ctrResource[ctr.Name].MinMemory != "" {
			requestMemory = ctrResource[ctr.Name].MinMemory
		} else if commonResource != nil && commonResource.MinMemory != "" {
			requestMemory = commonResource.MinMemory
		}
		if err = s.fillKusciaDeploymentContainerResourceList(requestResource, requestMemory, corev1.ResourceMemory); err != nil {
			return nil, fmt.Errorf("resource min_memory %v format is invalid for party domain_id/role %v/%v, %v", requestMemory, party.DomainId, party.Role, err)
		}

		limitResource := corev1.ResourceList{}
		limitCPU := ""
		if ctrResource[ctr.Name] != nil && ctrResource[ctr.Name].MaxCpu != "" {
			limitCPU = ctrResource[ctr.Name].MaxCpu
		} else if commonResource != nil && commonResource.MaxCpu != "" {
			limitCPU = commonResource.MaxCpu
		}
		if err = s.fillKusciaDeploymentContainerResourceList(limitResource, limitCPU, corev1.ResourceCPU); err != nil {
			return nil, fmt.Errorf("resource max_cpu %v format is invalid for party domain_id/role %v/%v, %v", limitCPU, party.DomainId, party.Role, err)
		}

		limitMemory := ""
		if ctrResource[ctr.Name] != nil && ctrResource[ctr.Name].MaxMemory != "" {
			limitMemory = ctrResource[ctr.Name].MaxMemory
		} else if commonResource != nil && commonResource.MaxMemory != "" {
			limitMemory = commonResource.MaxMemory
		}
		if err = s.fillKusciaDeploymentContainerResourceList(limitResource, limitMemory, corev1.ResourceMemory); err != nil {
			return nil, fmt.Errorf("resource max_memory %v format is invalid for party domain_id/role %v/%v, %v", limitMemory, party.DomainId, party.Role, err)
		}

		if len(requestResource) == 0 && len(limitResource) == 0 {
			return nil, nil
		}

		containers = append(containers, v1alpha1.Container{
			Name: ctr.Name,
			Resources: corev1.ResourceRequirements{
				Requests: requestResource,
				Limits:   limitResource,
			},
		})
	}
	return containers, nil
}

func (s *servingService) fillKusciaDeploymentContainerResourceList(resource corev1.ResourceList, quantity string, resourceType corev1.ResourceName) error {
	if quantity == "" {
		return nil
	}

	q, err := k8sresource.ParseQuantity(quantity)
	if err != nil {
		return err
	}
	resource[resourceType] = q
	return nil
}

func (s *servingService) getAppImageTemplate(ctx context.Context, appImageName, role string) (*v1alpha1.DeployTemplate, error) {
	appImage, err := s.getAppImage(ctx, appImageName)
	if err != nil {
		return nil, err
	}

	partyTemplate, err := resources.SelectDeployTemplate(appImage.Spec.DeployTemplates, role)
	if err != nil {
		return nil, fmt.Errorf("can not get deploy template from appimage %v", appImage)
	}
	return partyTemplate.DeepCopy(), nil
}

func (s *servingService) getAppImage(ctx context.Context, name string) (*v1alpha1.AppImage, error) {
	appImage, err := s.kusciaClient.KusciaV1alpha1().AppImages().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can not get appimage %v from cluster, %v", name, err)
	}
	return appImage, nil
}

func (s *servingService) buildServingPartyContainerResources(party *kusciaapi.ServingParty) (*kusciaapi.Resource, map[string]*kusciaapi.Resource) {
	var commonResource *kusciaapi.Resource
	ctrResource := make(map[string]*kusciaapi.Resource)
	for j, resource := range party.Resources {
		if resource.ContainerName == "" {
			if commonResource == nil {
				commonResource = &kusciaapi.Resource{}
			}
			commonResource = party.Resources[j]
			continue
		}
		ctrResource[resource.ContainerName] = party.Resources[j]
	}
	return commonResource, ctrResource
}

func (s *servingService) buildServingUpdateStrategy(kdParty *v1alpha1.KusciaDeploymentParty) *kusciaapi.UpdateStrategy {
	if kdParty.Template.Strategy == nil {
		return nil
	}

	strategy := kdParty.Template.Strategy
	updateStrategy := &kusciaapi.UpdateStrategy{
		Type: string(strategy.Type),
	}

	if strategy.RollingUpdate != nil {
		updateStrategy.MaxSurge = strategy.RollingUpdate.MaxSurge.String()
		updateStrategy.MaxUnavailable = strategy.RollingUpdate.MaxUnavailable.String()
	}
	return updateStrategy
}

func (s *servingService) buildServingResources(ctx context.Context, kd *v1alpha1.KusciaDeployment, kdParty *v1alpha1.KusciaDeploymentParty, partyTemplate *v1alpha1.DeployTemplate) ([]*kusciaapi.Resource, error) {
	var resources []*kusciaapi.Resource
	for i := range kdParty.Template.Spec.Containers {
		resources = s.buildServingResource(kdParty.Template.Spec.Containers[i].Name, kdParty.Template.Spec.Containers[i].Resources, resources)
	}
	if len(resources) == len(partyTemplate.Spec.Containers) {
		return resources, nil
	}

	deployment, err := s.getPartyDeployment(ctx, kd, kdParty.DomainID, kdParty.Role)
	if err != nil {
		return nil, err
	}
	if deployment != nil {
		for _, ctr := range deployment.Spec.Template.Spec.Containers {
			resources = s.buildServingResource(ctr.Name, ctr.Resources, resources)
		}
		if len(resources) == len(partyTemplate.Spec.Containers) {
			return resources, nil
		}
	}

	for _, ctr := range partyTemplate.Spec.Containers {
		resources = s.buildServingResource(ctr.Name, ctr.Resources, resources)
	}
	return resources, nil
}

func (s *servingService) getPartyDeployment(ctx context.Context, kd *v1alpha1.KusciaDeployment, domainID, role string) (*appsv1.Deployment, error) {
	if kd.Status.PartyDeploymentStatuses == nil {
		return nil, nil
	}

	partyDeployStatus, ok := kd.Status.PartyDeploymentStatuses[domainID]
	if !ok {
		return nil, nil
	}

	for deployName, status := range partyDeployStatus {
		if status.Role == role {
			deployment, err := s.kubeClient.AppsV1().Deployments(domainID).Get(ctx, deployName, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, err
			}
			return deployment, nil
		}
	}
	return nil, nil
}

func (s *servingService) buildServingResource(ctrName string, ctrResources corev1.ResourceRequirements, resources []*kusciaapi.Resource) []*kusciaapi.Resource {
	for _, res := range resources {
		if res.ContainerName == ctrName {
			return resources
		}
	}

	res := &kusciaapi.Resource{}
	res.ContainerName = ctrName
	for k, v := range ctrResources.Requests {
		switch k {
		case corev1.ResourceCPU:
			res.MinCpu = v.String()
		case corev1.ResourceMemory:
			res.MinMemory = v.String()
		default:
		}
	}

	for k, v := range ctrResources.Limits {
		switch k {
		case corev1.ResourceCPU:
			res.MaxCpu = v.String()
		case corev1.ResourceMemory:
			res.MaxMemory = v.String()
		default:
		}
	}
	return append(resources, res)
}

func (s *servingService) buildServingStatusDetail(ctx context.Context, kd *v1alpha1.KusciaDeployment) (*kusciaapi.ServingStatusDetail, error) {
	if len(kd.Status.PartyDeploymentStatuses) == 0 {
		return nil, nil
	}

	var partyStatuses []*kusciaapi.PartyServingStatus
	for domainID, partyDeploymentStatus := range kd.Status.PartyDeploymentStatuses {
		for deploymentName, statusInfo := range partyDeploymentStatus {
			services, err := s.kubeClient.CoreV1().Services(domainID).List(ctx, metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labels.Set{common.LabelKubernetesDeploymentName: deploymentName, common.LabelPortScope: string(v1alpha1.ScopeCluster)}).String(),
			})
			if err != nil {
				return nil, err
			}

			var endpoints []*kusciaapi.Endpoint
			for _, svc := range services.Items {
				endpoints = append(endpoints, &kusciaapi.Endpoint{Endpoint: fmt.Sprintf("%v.%v.svc", svc.Name, svc.Namespace)})
			}

			partyStatuses = append(partyStatuses, &kusciaapi.PartyServingStatus{
				DomainId:            domainID,
				Role:                statusInfo.Role,
				State:               string(statusInfo.Phase),
				Replicas:            statusInfo.Replicas,
				AvailableReplicas:   statusInfo.AvailableReplicas,
				UnavailableReplicas: statusInfo.UnavailableReplicas,
				UpdatedReplicas:     statusInfo.UpdatedReplicas,
				CreateTime:          utils.TimeRfc3339String(statusInfo.CreationTimestamp),
				Endpoints:           endpoints,
			})
		}
	}

	return &kusciaapi.ServingStatusDetail{
		State:            string(kd.Status.Phase),
		Message:          kd.Status.Message,
		Reason:           kd.Status.Reason,
		TotalParties:     int32(kd.Status.TotalParties),
		AvailableParties: int32(kd.Status.AvailableParties),
		CreateTime:       utils.TimeRfc3339String(&kd.ObjectMeta.CreationTimestamp),
		PartyStatuses:    partyStatuses,
	}, nil
}

func (s *servingService) QueryServing(ctx context.Context, request *kusciaapi.QueryServingRequest) *kusciaapi.QueryServingResponse {
	servingID := request.ServingId
	if servingID == "" {
		return &kusciaapi.QueryServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "serving id can not be empty"),
		}
	}

	kd, err := s.kusciaClient.KusciaV1alpha1().KusciaDeployments().Get(ctx, servingID, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.QueryServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrQueryServing, err.Error()),
		}
	}

	servingParties := make([]*kusciaapi.ServingParty, len(kd.Spec.Parties))
	for i, party := range kd.Spec.Parties {
		partyTemplate, err := s.getAppImageTemplate(ctx, party.AppImageRef, party.Role)
		if err != nil {
			return &kusciaapi.QueryServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrQueryServing, err.Error()),
			}
		}

		resources, err := s.buildServingResources(ctx, kd, &kd.Spec.Parties[i], partyTemplate)
		if err != nil {
			return &kusciaapi.QueryServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrQueryServing, err.Error()),
			}
		}
		servingParties[i] = &kusciaapi.ServingParty{
			AppImage:       party.AppImageRef,
			Role:           party.Role,
			DomainId:       party.DomainID,
			Replicas:       party.Template.Replicas,
			UpdateStrategy: s.buildServingUpdateStrategy(&kd.Spec.Parties[i]),
			Resources:      resources,
		}
	}

	status, err := s.buildServingStatusDetail(ctx, kd)
	if err != nil {
		return &kusciaapi.QueryServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrQueryServing, err.Error()),
		}
	}

	return &kusciaapi.QueryServingResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data: &kusciaapi.QueryServingResponseData{
			ServingInputConfig: kd.Spec.InputConfig,
			Initiator:          kd.Spec.Initiator,
			Parties:            servingParties,
			Status:             status,
		},
	}
}

func (s *servingService) BatchQueryServingStatus(ctx context.Context, request *kusciaapi.BatchQueryServingStatusRequest) *kusciaapi.BatchQueryServingStatusResponse {
	servingIDs := request.ServingIds
	if len(servingIDs) == 0 {
		return &kusciaapi.BatchQueryServingStatusResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "serving ids can not be empty"),
		}
	}

	for _, servingID := range servingIDs {
		if servingID == "" {
			return &kusciaapi.BatchQueryServingStatusResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "serving id can not be empty"),
			}
		}
	}

	servingStatuses := make([]*kusciaapi.ServingStatus, len(servingIDs))
	for i, servingID := range servingIDs {
		servingStatusDetail, err := s.buildServingStatusByID(ctx, servingID)
		if err != nil {
			return &kusciaapi.BatchQueryServingStatusResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrQueryServingStatus, err.Error()),
			}
		}
		servingStatuses[i] = &kusciaapi.ServingStatus{
			ServingId: servingID,
			Status:    servingStatusDetail,
		}
	}

	return &kusciaapi.BatchQueryServingStatusResponse{
		Status: utils2.BuildSuccessResponseStatus(),
		Data: &kusciaapi.BatchQueryServingStatusResponseData{
			Servings: servingStatuses,
		},
	}

}

func (s *servingService) buildServingStatusByID(ctx context.Context, servingID string) (*kusciaapi.ServingStatusDetail, error) {
	kd, err := s.kusciaClient.KusciaV1alpha1().KusciaDeployments().Get(ctx, servingID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	statusDetail, err := s.buildServingStatusDetail(ctx, kd)
	if err != nil {
		return nil, err
	}

	return statusDetail, nil
}

func (s *servingService) UpdateServing(ctx context.Context, request *kusciaapi.UpdateServingRequest) *kusciaapi.UpdateServingResponse {
	if request.ServingId == "" {
		return &kusciaapi.UpdateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "serving id can not be empty"),
		}
	}

	if request.ServingInputConfig == "" && len(request.Parties) == 0 {
		return &kusciaapi.UpdateServingResponse{
			Status: utils2.BuildSuccessResponseStatus(),
		}
	}

	kd, err := s.kusciaClient.KusciaV1alpha1().KusciaDeployments().Get(ctx, request.ServingId, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.UpdateServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrUpdateServing, err.Error()),
		}
	}

	needUpdate := false
	kdCopy := kd.DeepCopy()
	if request.ServingInputConfig != "" && request.ServingInputConfig != kdCopy.Spec.InputConfig {
		nlog.Infof("Serving %v input config updated from %v to %v", request.ServingId, kdCopy.Spec.InputConfig,
			request.ServingInputConfig)
		needUpdate = true
		kdCopy.Spec.InputConfig = request.ServingInputConfig
	}

	for i := range request.Parties {
		updated, err := s.updateKusciaDeploymentParty(ctx, request.ServingId, kdCopy, request.Parties[i])
		if err != nil {
			return &kusciaapi.UpdateServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, err.Error()),
			}
		}
		if updated {
			needUpdate = true
		}
	}

	if needUpdate {
		_, err = s.kusciaClient.KusciaV1alpha1().KusciaDeployments().Update(ctx, kdCopy, metav1.UpdateOptions{})
		if err != nil {
			return &kusciaapi.UpdateServingResponse{
				Status: utils2.BuildErrorResponseStatus(errorcode.ErrUpdateServing, err.Error()),
			}
		}
	}
	return &kusciaapi.UpdateServingResponse{
		Status: utils2.BuildSuccessResponseStatus(),
	}
}

func (s *servingService) updateKusciaDeploymentParty(ctx context.Context, servingID string, kd *v1alpha1.KusciaDeployment, party *kusciaapi.ServingParty) (bool, error) {
	if party == nil {
		return false, nil
	}

	findParty := false
	needUpdate := false
	for i, kdParty := range kd.Spec.Parties {
		if party.DomainId == kdParty.DomainID && party.Role == kdParty.Role {
			findParty = true
			if party.AppImage != "" && party.AppImage != kdParty.AppImageRef {
				nlog.Infof("Serving %v party domainID/role %v/%v appimage updated from %v to %v", servingID, kdParty.DomainID,
					kdParty.Role, kdParty.AppImageRef, party.AppImage)
				if _, err := s.getAppImage(ctx, party.AppImage); err != nil {
					return false, err
				}
				needUpdate = true
				kd.Spec.Parties[i].AppImageRef = party.AppImage
			}

			if party.Replicas != nil {
				if kdParty.Template.Replicas == nil || *party.Replicas != *kdParty.Template.Replicas {
					nlog.Infof("Serving %v party domainID/role %v/%v replicas updated from %v to %v", servingID, kdParty.DomainID,
						kdParty.Role, s.printReplicas(kdParty.Template.Replicas), s.printReplicas(party.Replicas))
					needUpdate = true
					kd.Spec.Parties[i].Template.Replicas = party.Replicas
				}
			}

			if party.UpdateStrategy != nil {
				newPartyStrategy, err := s.buildKusciaDeploymentPartyStrategy(party)
				if err != nil {
					return false, err
				}

				if !reflect.DeepEqual(newPartyStrategy, kdParty.Template.Strategy) {
					nlog.Infof("Serving %v party domainID/role %v/%v strategy updated from %v to %v", servingID, kdParty.DomainID,
						kdParty.Role, kdParty.Template.Strategy.String(), newPartyStrategy.String())
					needUpdate = true
					kd.Spec.Parties[i].Template.Strategy = newPartyStrategy
				}
			}

			if len(party.Resources) > 0 {
				if len(kdParty.Template.Spec.Containers) == 0 {
					containers, err := s.buildKusciaDeploymentPartyContainers(ctx, party)
					if err != nil {
						return false, err
					}
					if len(containers) > 0 {
						nlog.Infof("Serving %v party domainID/role %v/%v resources updated from empty to %v", servingID, kdParty.DomainID,
							kdParty.Role, s.printContainersResource(containers))
						needUpdate = true
						kd.Spec.Parties[i].Template.Spec.Containers = containers
					}
				} else {
					commonResource, ctrResource := s.buildServingPartyContainerResources(party)
					for k, ctr := range kdParty.Template.Spec.Containers {
						var partyResource *kusciaapi.Resource
						if res, ok := ctrResource[ctr.Name]; ok {
							partyResource = res
						} else if commonResource != nil {
							partyResource = commonResource
						}

						if partyResource != nil {
							preResources := kd.Spec.Parties[i].Template.Spec.Containers[k].Resources.DeepCopy()
							updated, err := s.updateContainerResource(partyResource, kd.Spec.Parties[i].Template.Spec.Containers[k].Resources)
							if err != nil {
								return false, err
							}
							if updated {
								nlog.Infof("Serving %v party domainID/role %v/%v container %v resources updated from %v to %v", servingID, kdParty.DomainID,
									kdParty.Role, ctr.Name, s.printContainerResource(*preResources), partyResource.String())
								needUpdate = true
							}
						}
					}
				}
			}
			break
		}
	}

	if !findParty {
		return false, fmt.Errorf("party domain_id/role %v/%v does not exist", party.DomainId, party.Role)
	}

	return needUpdate, nil
}

func (s *servingService) printReplicas(replicas *int32) int32 {
	if replicas == nil {
		return 0
	}
	return *replicas
}

func (s *servingService) updateContainerResource(servingPartyResource *kusciaapi.Resource, ctrResource corev1.ResourceRequirements) (bool, error) {
	updated := false
	if servingPartyResource.MinCpu != "" {
		minCPU, err := k8sresource.ParseQuantity(servingPartyResource.MinCpu)
		if err != nil {
			return false, err
		}
		if ctrResource.Requests.Cpu() == nil || !minCPU.Equal(*ctrResource.Requests.Cpu()) {
			ctrResource.Requests[corev1.ResourceCPU] = minCPU
			updated = true
		}
	}

	if servingPartyResource.MinMemory != "" {
		minMemory, err := k8sresource.ParseQuantity(servingPartyResource.MinMemory)
		if err != nil {
			return false, err
		}
		if ctrResource.Requests.Memory() == nil || !minMemory.Equal(*ctrResource.Requests.Memory()) {
			ctrResource.Requests[corev1.ResourceMemory] = minMemory
			updated = true
		}
	}

	if servingPartyResource.MaxCpu != "" {
		maxCPU, err := k8sresource.ParseQuantity(servingPartyResource.MaxCpu)
		if err != nil {
			return false, err
		}
		if ctrResource.Limits.Cpu() == nil || !maxCPU.Equal(*ctrResource.Limits.Cpu()) {
			ctrResource.Limits[corev1.ResourceCPU] = maxCPU
			updated = true
		}
	}

	if servingPartyResource.MaxMemory != "" {
		maxMemory, err := k8sresource.ParseQuantity(servingPartyResource.MaxMemory)
		if err != nil {
			return false, err
		}
		if ctrResource.Limits.Memory() == nil || !maxMemory.Equal(*ctrResource.Limits.Memory()) {
			ctrResource.Limits[corev1.ResourceMemory] = maxMemory
			updated = true
		}
	}

	return updated, nil
}

func (s *servingService) printContainerResource(rr corev1.ResourceRequirements) string {
	output := "container resources "
	for k, v := range rr.Requests {
		output += fmt.Sprintf("requests[%v]: %v, ", k, v.String())
	}

	for k, v := range rr.Limits {
		output += fmt.Sprintf("limits[%v]: %v, ", k, v.String())
	}
	return output
}

func (s *servingService) printContainersResource(containers []v1alpha1.Container) string {
	output := ""
	for _, ctr := range containers {
		ctrOut := fmt.Sprintf("container[%v] resources ", ctr.Name)
		for k, v := range ctr.Resources.Requests {
			ctrOut += fmt.Sprintf("requests[%v]: %v, ", k, v.String())
		}

		for k, v := range ctr.Resources.Limits {
			ctrOut += fmt.Sprintf("limits[%v]: %v, ", k, v.String())
		}
		output += ctrOut
	}
	return output
}

func (s *servingService) DeleteServing(ctx context.Context, request *kusciaapi.DeleteServingRequest) *kusciaapi.DeleteServingResponse {
	servingID := request.ServingId
	if servingID == "" {
		return &kusciaapi.DeleteServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrRequestValidate, "serving id can not be empty"),
		}
	}

	err := s.kusciaClient.KusciaV1alpha1().KusciaDeployments().Delete(ctx, servingID, metav1.DeleteOptions{})
	if err != nil {
		return &kusciaapi.DeleteServingResponse{
			Status: utils2.BuildErrorResponseStatus(errorcode.ErrDeleteServing, err.Error()),
		}
	}
	return &kusciaapi.DeleteServingResponse{
		Status: utils2.BuildSuccessResponseStatus(),
	}
}
