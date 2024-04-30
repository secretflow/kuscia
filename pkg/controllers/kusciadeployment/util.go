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

//nolint:dulp
package kusciadeployment

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kusciav1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	proto "github.com/secretflow/kuscia/proto/api/v1alpha1/appconfig"
)

type KdStatusReason string

const (
	buildPartyKitInfoFailed       KdStatusReason = "BuildPartyKitInfoFailed"
	fillPartyClusterDefinesFailed KdStatusReason = "FillPartyClusterDefinesFailed"
	getSelfPartyKitInfoFailed     KdStatusReason = "GetSelfPartyKitInfoFailed"
	createConfigMapFailed         KdStatusReason = "CreateConfigMapFailed"
	createServiceFailed           KdStatusReason = "CreateServiceFailed"
	createDeploymentFailed        KdStatusReason = "CreateDeploymentFailed"
	retryProcessingFailed         KdStatusReason = "RetryProcessingFailed"
)

// PartyKitInfo defines kit for party.
type PartyKitInfo struct {
	kd                    *kusciav1alpha1.KusciaDeployment
	domainID              string
	role                  string
	deployTemplate        *kusciav1alpha1.KusciaDeploymentPartyTemplate
	configTemplatesCMName string
	configTemplates       map[string]string
	servicedPorts         []string
	portAccessDomains     map[string]string
	dkInfo                *DeploymentKitInfo
}

// NamedPorts defines port name and container's port mapping.
type NamedPorts map[string]kusciav1alpha1.ContainerPort
type PortService map[string]string

// DeploymentKitInfo defines kit for deployment.
type DeploymentKitInfo struct {
	deploymentName string
	image          string
	imageID        string
	ports          NamedPorts
	portService    PortService
	clusterDef     *proto.ClusterDefine
	allocatedPorts *proto.AllocatedPorts
}

func (c *Controller) buildPartyKitInfos(kd *kusciav1alpha1.KusciaDeployment) (map[string]*PartyKitInfo, error) {
	partyKitInfos := make(map[string]*PartyKitInfo)
	for _, party := range kd.Spec.Parties {
		kitInfo, err := c.buildPartyKitInfo(kd, &party)
		if err != nil {
			kd.Status.Phase = kusciav1alpha1.KusciaDeploymentPhaseFailed
			kd.Status.Reason = string(buildPartyKitInfoFailed)
			kd.Status.Message = fmt.Sprintf("failed to build domain %v kit info, %v", party.DomainID, err)
			return nil, err
		}
		key := party.DomainID + "/" + party.Role
		partyKitInfos[key] = kitInfo
	}

	if err := c.fillPartyClusterDefines(partyKitInfos); err != nil {
		kd.Status.Phase = kusciav1alpha1.KusciaDeploymentPhaseFailed
		kd.Status.Reason = string(fillPartyClusterDefinesFailed)
		kd.Status.Message = fmt.Sprintf("failed to fill party cluster defines, %v", err)
		return nil, err
	}

	selfParties, err := c.selfParties(kd)
	if err != nil {
		return nil, err
	}

	selfPartyKitInfos := make(map[string]*PartyKitInfo)
	for _, selfParty := range selfParties {
		key := selfParty.DomainID + "/" + selfParty.Role
		if partyKitInfos[key] == nil {
			err = fmt.Errorf("failed to get self party kit info for %v", key)
			kd.Status.Phase = kusciav1alpha1.KusciaDeploymentPhaseFailed
			kd.Status.Reason = string(getSelfPartyKitInfoFailed)
			kd.Status.Message = err.Error()
			return nil, err
		}
		selfPartyKitInfos[key] = partyKitInfos[key]
	}

	return selfPartyKitInfos, nil
}

func (c *Controller) buildPartyKitInfo(kd *kusciav1alpha1.KusciaDeployment, party *kusciav1alpha1.KusciaDeploymentParty) (*PartyKitInfo, error) {
	_, err := c.namespaceLister.Get(party.DomainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace %v, %v", party.DomainID, err)
	}

	appImage, err := c.appImageLister.Get(party.AppImageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get appImage %q, %v", party.AppImageRef, err)
	}

	baseDeployTemplate, err := utilsres.SelectDeployTemplate(appImage.Spec.DeployTemplates, party.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to select appropriate deploy template from appImage %q for party %v/%v, %v", appImage.Name, party.DomainID, party.Role, err)
	}

	deployTemplate := mergeDeployTemplate(baseDeployTemplate, &party.Template)

	ports, err := mergeContainersPorts(deployTemplate.Spec.Containers)
	if err != nil {
		return nil, fmt.Errorf("failed to merge ports in deploy template '%v/%v' for party %v, %v", baseDeployTemplate.Name, baseDeployTemplate.Role, party.DomainID, err)
	}

	servicedPorts := generateServicedPorts(ports)
	deployName := generateDeploymentName(kd.Name, party.Role)
	portService := generatePortServices(deployName, servicedPorts)

	dkInfo := &DeploymentKitInfo{
		deploymentName: deployName,
		image:          fmt.Sprintf("%s:%s", appImage.Spec.Image.Name, appImage.Spec.Image.Tag),
		imageID:        appImage.Spec.Image.ID,
		ports:          ports,
		portService:    portService,
	}

	kit := &PartyKitInfo{
		kd:                    kd,
		domainID:              party.DomainID,
		role:                  party.Role,
		deployTemplate:        deployTemplate,
		configTemplatesCMName: generateConfigMapName(deployName),
		configTemplates:       appImage.Spec.ConfigTemplates,
		servicedPorts:         servicedPorts,
		dkInfo:                dkInfo,
	}

	if len(kd.Spec.Parties) > 1 {
		kit.portAccessDomains = generatePortAccessDomains(kd.Spec.Parties, baseDeployTemplate.NetworkPolicy, ports)
	}

	return kit, nil
}

func mergeDeployTemplate(appImageTemplate *kusciav1alpha1.DeployTemplate, partyTemplate *kusciav1alpha1.KusciaDeploymentPartyTemplate) *kusciav1alpha1.KusciaDeploymentPartyTemplate {
	template := partyTemplate.DeepCopy()

	if partyTemplate.Replicas == nil {
		if appImageTemplate.Replicas != nil {
			template.Replicas = appImageTemplate.Replicas
		} else {
			var replicas int32 = 1
			template.Replicas = &replicas
		}
	}

	template.Spec = appImageTemplate.Spec
	for _, c := range partyTemplate.Spec.Containers {
		for i, cc := range template.Spec.Containers {
			if c.Name == cc.Name {
				if len(c.Resources.Requests) > 0 {
					template.Spec.Containers[i].Resources.Requests = c.Resources.Requests.DeepCopy()
				}

				if len(c.Resources.Limits) > 0 {
					template.Spec.Containers[i].Resources.Limits = c.Resources.Limits.DeepCopy()
				}
			}
		}
	}

	if partyTemplate.Spec.Affinity != nil {
		template.Spec.Affinity = partyTemplate.Spec.Affinity.DeepCopy()
	}
	return template
}

func (c *Controller) fillPartyClusterDefines(partyKitInfos map[string]*PartyKitInfo) error {
	parties, err := c.generateClusterDefineParties(partyKitInfos)
	if err != nil {
		return err
	}

	for _, kitInfo := range partyKitInfos {
		if err = fillPartyClusterDefine(kitInfo, parties); err != nil {
			return err
		}
	}
	return nil
}

func fillPartyClusterDefine(kitInfo *PartyKitInfo, parties []*proto.Party) error {
	var selfPartyIndex *int
	for i, party := range parties {
		if party.Name == kitInfo.domainID && party.Role == kitInfo.role {
			selfPartyIndex = &i
			break
		}
	}

	if selfPartyIndex == nil {
		return fmt.Errorf("party '%v/%v' is not found", kitInfo.domainID, kitInfo.role)
	}

	fillClusterDefine(kitInfo.dkInfo, parties, *selfPartyIndex, 0)
	fillAllocatedPorts(kitInfo.dkInfo)
	return nil
}

func fillClusterDefine(dkInfo *DeploymentKitInfo, parties []*proto.Party, partyIndex int, endpointIndex int) {
	dkInfo.clusterDef = &proto.ClusterDefine{
		Parties:         parties,
		SelfPartyIdx:    int32(partyIndex),
		SelfEndpointIdx: int32(endpointIndex),
	}
}

func fillAllocatedPorts(dkInfo *DeploymentKitInfo) {
	resPorts := make([]*proto.Port, 0, len(dkInfo.ports))
	for _, port := range dkInfo.ports {
		resPorts = append(resPorts, &proto.Port{
			Name:     port.Name,
			Port:     port.Port,
			Scope:    string(port.Scope),
			Protocol: string(port.Protocol),
		})
	}

	dkInfo.allocatedPorts = &proto.AllocatedPorts{Ports: resPorts}
}

func (c *Controller) generateClusterDefineParties(partyKitInfos map[string]*PartyKitInfo) ([]*proto.Party, error) {
	var parties []*proto.Party
	for _, kitInfo := range partyKitInfos {
		party, err := c.generateClusterDefineParty(kitInfo)
		if err != nil {
			return nil, err
		}
		parties = append(parties, party)
	}

	return parties, nil
}

func (c *Controller) generateClusterDefineParty(kitInfo *PartyKitInfo) (*proto.Party, error) {
	var partyServices []*proto.Service
	for _, portName := range kitInfo.servicedPorts {
		var endpoints []string
		endpointAddress := ""
		if kitInfo.dkInfo.portService[portName] != "" {
			isPartnerDomain, err := c.isPartnerDomain(kitInfo.domainID)
			if err != nil {
				return nil, err
			}

			if isPartnerDomain && kitInfo.dkInfo.ports[portName].Scope != kusciav1alpha1.ScopeCluster {
				continue
			}

			switch kitInfo.dkInfo.ports[portName].Scope {
			case kusciav1alpha1.ScopeDomain:
				endpointAddress = fmt.Sprintf("%s.%s.svc:%d", kitInfo.dkInfo.portService[portName], kitInfo.domainID, kitInfo.dkInfo.ports[portName].Port)
			case kusciav1alpha1.ScopeCluster:
				endpointAddress = fmt.Sprintf("%s.%s.svc", kitInfo.dkInfo.portService[portName], kitInfo.domainID)
			default:
				continue
			}
		}

		endpoints = append(endpoints, endpointAddress)
		partyService := &proto.Service{
			PortName:  portName,
			Endpoints: endpoints,
		}
		partyServices = append(partyServices, partyService)
	}

	party := &proto.Party{
		Name:     kitInfo.domainID,
		Role:     kitInfo.role,
		Services: partyServices,
	}

	return party, nil
}

func mergeContainersPorts(containers []kusciav1alpha1.Container) (NamedPorts, error) {
	ports := NamedPorts{}
	for _, container := range containers {
		for _, port := range container.Ports {
			if _, ok := ports[port.Name]; ok {
				return nil, fmt.Errorf("duplicate port %q", port.Name)
			}

			ports[port.Name] = port
		}
	}

	return ports, nil
}

func generateServicedPorts(ports NamedPorts) []string {
	var servicedPorts []string
	for _, port := range ports {
		if port.Scope != kusciav1alpha1.ScopeCluster && port.Scope != kusciav1alpha1.ScopeDomain {
			continue
		}

		servicedPorts = append(servicedPorts, port.Name)
	}

	return servicedPorts
}

func generatePortServices(deploymentName string, servicedPorts []string) PortService {
	portService := PortService{}

	for _, portName := range servicedPorts {
		portService[portName] = utilsres.GenerateServiceName(deploymentName, portName)
	}

	return portService
}

func generatePortAccessDomains(parties []kusciav1alpha1.KusciaDeploymentParty, networkPolicy *kusciav1alpha1.NetworkPolicy, ports NamedPorts) map[string]string {
	portAccessDomains := map[string]string{}
	if networkPolicy == nil {
		domainMap := map[string]struct{}{}
		for _, party := range parties {
			domainMap[party.DomainID] = struct{}{}
		}

		domainSlice := make([]string, 0, len(domainMap))
		for domain := range domainMap {
			domainSlice = append(domainSlice, domain)
		}

		for _, port := range ports {
			if port.Scope == kusciav1alpha1.ScopeCluster {
				portAccessDomains[port.Name] = strings.Join(domainSlice, ",")
			}
		}
	} else {
		roleDomains := map[string][]string{}
		for _, party := range parties {
			if domains, ok := roleDomains[party.Role]; ok {
				roleDomains[party.Role] = append(domains, party.DomainID)
			} else {
				roleDomains[party.Role] = []string{party.DomainID}
			}
		}

		portAccessRoles := map[string][]string{}
		for _, item := range networkPolicy.Ingresses {
			for _, port := range item.Ports {
				if domains, ok := portAccessRoles[port.Port]; ok {
					portAccessRoles[port.Port] = append(domains, item.From.Roles...)
				} else {
					portAccessRoles[port.Port] = item.From.Roles
				}
			}
		}

		for port, roles := range portAccessRoles {
			domainMap := map[string]struct{}{}
			for _, role := range roles {
				for _, domain := range roleDomains[role] {
					domainMap[domain] = struct{}{}
				}
			}
			domainSlice := make([]string, 0, len(domainMap))
			for domain := range domainMap {
				domainSlice = append(domainSlice, domain)
			}
			portAccessDomains[port] = strings.Join(domainSlice, ",")
		}
	}
	return portAccessDomains
}

func generateConfigMapName(deploymentName string) string {
	return fmt.Sprintf("%s-configtemplate", deploymentName)
}

func generateDeploymentName(kdName, role string) string {
	if role == "" {
		return fmt.Sprintf("%s", kdName)
	}
	return fmt.Sprintf("%s-%s", kdName, role)
}

func (c *Controller) isPartnerDomain(domainId string) (bool, error) {
	partyDomain, err := c.domainLister.Get(domainId)
	if err != nil {
		return false, err
	}
	if partyDomain.Spec.Role != "" {
		return true, nil
	}
	return false, nil
}

func (c *Controller) handleError(ctx context.Context,
	partyKitInfos map[string]*PartyKitInfo,
	preKdStatus *kusciav1alpha1.KusciaDeploymentStatus,
	kd *kusciav1alpha1.KusciaDeployment,
	err error) error {
	if kd.Status.Phase == kusciav1alpha1.KusciaDeploymentPhaseFailed {
		if c.refreshPartyDeploymentStatuses(kd, partyKitInfos) || !reflect.DeepEqual(preKdStatus, kd.Status) {
			return c.updateKusciaDeploymentStatus(ctx, kd)
		}
		return nil
	}
	return err
}

func (c *Controller) updateKusciaDeploymentStatus(ctx context.Context, kd *kusciav1alpha1.KusciaDeployment) (err error) {
	now := metav1.Now()
	kd.Status.LastReconcileTime = &now
	if kd.Status.TotalParties == 0 {
		kd.Status.TotalParties = len(kd.Spec.Parties)
	}

	_, err = c.kusciaClient.KusciaV1alpha1().KusciaDeployments(kd.Namespace).UpdateStatus(ctx, kd, metav1.UpdateOptions{})
	if err != nil && !k8serrors.IsConflict(err) {
		return fmt.Errorf("failed to updating kuscia deployment %v status, %v", kd.Name, err)
	}

	return nil
}
