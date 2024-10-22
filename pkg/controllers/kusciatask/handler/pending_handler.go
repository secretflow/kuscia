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

//nolint:dupl
package handler

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"

	"github.com/secretflow/kuscia/pkg/common"
	pkgport "github.com/secretflow/kuscia/pkg/controllers/portflake/port"
	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kuscialistersv1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	utilcom "github.com/secretflow/kuscia/pkg/utils/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	proto "github.com/secretflow/kuscia/proto/api/v1alpha1/appconfig"
)

// PendingHandler is used to handle kuscia task which phase is pending.
type PendingHandler struct {
	kubeClient       kubernetes.Interface
	kusciaClient     kusciaclientset.Interface
	trgLister        kuscialistersv1alpha1.TaskResourceGroupLister
	namespacesLister corelisters.NamespaceLister
	podsLister       corelisters.PodLister
	servicesLister   corelisters.ServiceLister
	configMapLister  corelisters.ConfigMapLister
	appImagesLister  kuscialistersv1alpha1.AppImageLister
}

type NamedPorts map[string]kusciaapisv1alpha1.ContainerPort
type PortService map[string]string

type PodKitInfo struct {
	index          int
	podName        string
	podIdentity    string
	ports          NamedPorts
	portService    PortService
	clusterDef     *proto.ClusterDefine
	allocatedPorts *proto.AllocatedPorts
}

type PartyKitInfo struct {
	kusciaTask            *kusciaapisv1alpha1.KusciaTask
	domainID              string
	role                  string
	image                 string
	imageID               string
	deployTemplate        *kusciaapisv1alpha1.DeployTemplate
	configTemplatesCMName string
	configTemplates       map[string]string
	servicedPorts         []string
	portAccessDomains     map[string]string
	minReservedPods       int
	pods                  []*PodKitInfo
	bandwidthLimit        []kusciaapisv1alpha1.BandwidthLimit
}

// NewPendingHandler returns a PendingHandler instance.
func NewPendingHandler(deps *Dependencies) *PendingHandler {
	return &PendingHandler{
		kubeClient:       deps.KubeClient,
		kusciaClient:     deps.KusciaClient,
		trgLister:        deps.TrgLister,
		namespacesLister: deps.NamespacesLister,
		podsLister:       deps.PodsLister,
		servicesLister:   deps.ServicesLister,
		configMapLister:  deps.ConfigMapLister,
		appImagesLister:  deps.AppImagesLister,
	}
}

// Handle is used to perform the real logic.
func (h *PendingHandler) Handle(kusciaTask *kusciaapisv1alpha1.KusciaTask) (needUpdate bool, err error) {
	now := metav1.Now().Rfc3339Copy()
	if needUpdate, err = h.prepareTaskResources(now, kusciaTask); needUpdate || err != nil {
		return needUpdate, err
	}

	curKtStatus := kusciaTask.Status.DeepCopy()
	h.initPartyTaskStatus(kusciaTask, curKtStatus)
	refreshKtResourcesStatus(h.kubeClient, h.podsLister, h.servicesLister, curKtStatus)
	if !reflect.DeepEqual(kusciaTask.Status, curKtStatus) {
		needUpdate = true
		kusciaTask.Status = *curKtStatus
		kusciaTask.Status.LastReconcileTime = &now
	}

	if updated := h.taskFailed(now, kusciaTask); updated {
		return updated, nil
	}

	if updated, err := h.taskRunning(now, kusciaTask); updated || err != nil {
		return updated, err
	}

	if updated, err := h.taskExpired(now, kusciaTask); updated || err != nil {
		return updated, err
	}
	return needUpdate, nil
}

func (h *PendingHandler) prepareTaskResources(now metav1.Time, kusciaTask *kusciaapisv1alpha1.KusciaTask) (needUpdate bool, err error) {
	if kusciaTask.Status.StartTime == nil {
		kusciaTask.Status.StartTime = &now
	}

	if kusciaTask.Status.Phase == "" {
		kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskPending
	}

	cond, found := utilsres.GetKusciaTaskCondition(&kusciaTask.Status, kusciaapisv1alpha1.KusciaTaskCondPortsAllocated, true)
	if !found {
		needUpdate, err = h.allocatePorts(kusciaTask)
		if err != nil {
			return false, err
		}
		if needUpdate {
			utilsres.SetKusciaTaskCondition(now, cond, v1.ConditionTrue, "", "")
			kusciaTask.Status.LastReconcileTime = &now
			return true, nil
		}
	}

	cond, found = utilsres.GetKusciaTaskCondition(&kusciaTask.Status, kusciaapisv1alpha1.KusciaTaskCondResourceCreated, true)
	if !found {
		latestKt, err := h.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(context.Background(),
			kusciaTask.Name,
			metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		latestCond, _ := utilsres.GetKusciaTaskCondition(&latestKt.Status, kusciaapisv1alpha1.KusciaTaskCondResourceCreated, false)
		if latestCond != nil && latestCond.Status == v1.ConditionTrue {
			return false, nil
		}

		if err = h.createTaskResources(kusciaTask); err != nil {
			needUpdate = utilsres.SetKusciaTaskCondition(now, cond, v1.ConditionFalse, "KusciaTaskCreateFailed", err.Error())
			return needUpdate, err
		}
		utilsres.SetKusciaTaskCondition(now, cond, v1.ConditionTrue, "", "")
		kusciaTask.Status.LastReconcileTime = &now
		return true, nil
	}

	return false, nil
}

func (h *PendingHandler) buildPartyKitInfos(kusciaTask *kusciaapisv1alpha1.KusciaTask) (map[string]*PartyKitInfo, map[string]*PartyKitInfo, error) {
	partyKitInfos := map[string]*PartyKitInfo{}
	selfPartyKitInfos := map[string]*PartyKitInfo{}
	for i, party := range kusciaTask.Spec.Parties {
		kit, err := h.buildPartyKitInfo(kusciaTask, &kusciaTask.Spec.Parties[i])
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build domain %v kit info, %v", party.DomainID, err)
		}

		partyKitInfos[party.DomainID+party.Role] = kit

		isPartner, err := utilsres.IsPartnerDomain(h.namespacesLister, kit.domainID)
		if err != nil {
			return nil, nil, err
		}
		if !isPartner {
			selfPartyKitInfos[party.DomainID+party.Role] = kit
		}
	}
	return partyKitInfos, selfPartyKitInfos, nil
}

// createTaskResources generate taskResources by submit a trg.
// in this function, partyKitInfos is created for each domain, however pods are only created for local domain.
// actually, just a subset of partyKitInfos which are selfControlled will be used to create trg.Spec.Parties
// a party is selfControlled indicates the task resource of the party is controlled by a trg in self cluster :
// 1) each party is controlled in initiator cluster
// 2) only local party is controlled in participant cluster
func (h *PendingHandler) createTaskResources(kusciaTask *kusciaapisv1alpha1.KusciaTask) error {
	partyKitInfos, selfPartyKitInfos, err := h.buildPartyKitInfos(kusciaTask)
	if err != nil {
		return err
	}

	if err = buildPodAllocatePorts(kusciaTask, selfPartyKitInfos); err != nil {
		return err
	}

	parties := generateParties(partyKitInfos)

	for _, partyKitInfo := range partyKitInfos {
		fillPartyClusterDefine(partyKitInfo, parties)
	}

	podStatuses := make(map[string]*kusciaapisv1alpha1.PodStatus)
	serviceStatuses := make(map[string]*kusciaapisv1alpha1.ServiceStatus)
	for _, partyKitInfo := range selfPartyKitInfos {
		ps, ss, err := h.createResourceForParty(partyKitInfo)
		if err != nil {
			return fmt.Errorf("failed to create resource for party '%v/%v', %v", partyKitInfo.domainID, partyKitInfo.role, err)
		}

		for key, v := range ps {
			podStatuses[key] = v
		}

		for key, v := range ss {
			serviceStatuses[key] = v
		}
	}
	kusciaTask.Status.PodStatuses = podStatuses
	kusciaTask.Status.ServiceStatuses = serviceStatuses

	// just a subset of partyKitInfos which are selfControlled will be used to create trg.Spec.Parties
	if err := h.createTaskResourceGroup(kusciaTask, partyKitInfos); err != nil {
		return fmt.Errorf("failed to create task resource group for kuscia task %v, %v", kusciaTask.Name, err.Error())
	}
	return nil
}

func (h *PendingHandler) initPartyTaskStatus(kusciaTask *kusciaapisv1alpha1.KusciaTask, ktStatus *kusciaapisv1alpha1.KusciaTaskStatus) {
	for _, party := range kusciaTask.Spec.Parties {
		setPartyTaskStatus(party, ktStatus)
	}
}

func setPartyTaskStatus(partyInfo kusciaapisv1alpha1.PartyInfo, ktStatus *kusciaapisv1alpha1.KusciaTaskStatus) {
	for _, ptStatus := range ktStatus.PartyTaskStatus {
		if ptStatus.DomainID == partyInfo.DomainID && ptStatus.Role == partyInfo.Role {
			return
		}
	}

	ktStatus.PartyTaskStatus = append(ktStatus.PartyTaskStatus, kusciaapisv1alpha1.PartyTaskStatus{
		DomainID: partyInfo.DomainID,
		Role:     partyInfo.Role,
		Phase:    kusciaapisv1alpha1.TaskPending,
	})
}

func (h *PendingHandler) taskFailed(now metav1.Time, kusciaTask *kusciaapisv1alpha1.KusciaTask) bool {
	var failedParty []string
	for _, pts := range kusciaTask.Status.PartyTaskStatus {
		if pts.Phase == kusciaapisv1alpha1.TaskFailed {
			failedParty = append(failedParty, buildPartyKey(pts.DomainID, pts.Role))
		}
	}

	minReservedMembers := kusciaTask.Spec.ScheduleConfig.MinReservedMembers
	if minReservedMembers <= 0 {
		minReservedMembers = len(kusciaTask.Spec.Parties)
	}

	remainingParty := len(kusciaTask.Spec.Parties) - len(failedParty)
	if minReservedMembers > remainingParty {
		kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskFailed
		kusciaTask.Status.Message = fmt.Sprintf("The remaining no-failed party task counts %v are less than the task success threshold %v. failed party[%v]",
			remainingParty, minReservedMembers, strings.Join(failedParty, ","))
		kusciaTask.Status.LastReconcileTime = &now
		return true
	}
	return false
}

func (h *PendingHandler) taskRunning(now metav1.Time, kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	// if self cluster is not participant, skip creating related sub resources.
	// waiting for other parties to run the task.
	asParticipant, err := selfClusterAsParticipant(h.namespacesLister, kusciaTask)
	if err != nil {
		return false, err
	}
	if !asParticipant {
		for _, status := range kusciaTask.Status.PartyTaskStatus {
			if status.Phase != "" && status.Phase != kusciaapisv1alpha1.TaskPending {
				kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskRunning
				kusciaTask.Status.LastReconcileTime = &now
				return true, nil
			}
		}
	}

	// Check if there is a pod in running status,
	// If there is, it indicates that the task status can be converted to running
	for _, podStatus := range kusciaTask.Status.PodStatuses {
		pod, _ := h.podsLister.Pods(podStatus.Namespace).Get(podStatus.PodName)
		if pod != nil && pod.Status.Phase != v1.PodPending {
			kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskRunning
			kusciaTask.Status.LastReconcileTime = &now
			return true, nil
		}
	}
	return false, nil
}

func (h *PendingHandler) taskExpired(now metav1.Time, kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	trg, err := getTaskResourceGroup(context.Background(), kusciaTask.Name, h.trgLister, h.kusciaClient)
	if err != nil {
		return false, fmt.Errorf("get task resource group %v failed, %v", kusciaTask.Name, err)
	}

	if trg.Status.Phase == kusciaapisv1alpha1.TaskResourceGroupPhaseFailed {
		kusciaTask.Status.Phase = kusciaapisv1alpha1.TaskFailed
		kusciaTask.Status.Message = generateMessageBy(trg)
		kusciaTask.Status.LastReconcileTime = &now
		return true, nil
	}

	return false, nil
}

func generateMessageBy(trg *kusciaapisv1alpha1.TaskResourceGroup) string {
	message := ""
	for _, cond := range trg.Status.Conditions {
		if cond.Status == v1.ConditionFalse {
			message += cond.Reason + ","
		}
	}
	return message
}

func (h *PendingHandler) buildPartyKitInfo(kusciaTask *kusciaapisv1alpha1.KusciaTask, party *kusciaapisv1alpha1.PartyInfo) (*PartyKitInfo, error) {
	kit := &PartyKitInfo{
		kusciaTask: kusciaTask,
		domainID:   party.DomainID,
		role:       party.Role,
	}

	asParticipant, err := selfClusterAsParticipant(h.namespacesLister, kusciaTask)
	if err != nil {
		return nil, err
	}
	if !asParticipant {
		return kit, nil
	}

	appImage, err := h.appImagesLister.Get(party.AppImageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get appImage %q from cache, %v", party.AppImageRef, err)
	}

	baseDeployTemplate, err := utilsres.SelectDeployTemplate(appImage.Spec.DeployTemplates, party.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to select appropriate deploy template from appImage %q for party %v/%v, %v", appImage.Name, party.DomainID, party.Role, err)
	}

	deployTemplate := mergeDeployTemplate(baseDeployTemplate, &party.Template)

	replicas := 1
	if deployTemplate.Replicas != nil {
		replicas = int(*deployTemplate.Replicas)
	}

	pods := make([]*PodKitInfo, replicas)
	ports, err := mergeContainersPorts(deployTemplate.Spec.Containers)
	if err != nil {
		return nil, fmt.Errorf("failed to merge ports in deploy template '%v/%v', %v", deployTemplate.Name, deployTemplate.Role, err)
	}

	servicedPorts := generateServicedPorts(ports)
	for index := 0; index < replicas; index++ {
		podName := generatePodName(kusciaTask.Name, party.Role, index)
		podIdentity := generatePodIdentity(string(kusciaTask.UID), party.Role, index)
		portService := generatePortServices(podName, servicedPorts)

		pods[index] = &PodKitInfo{
			index:       index,
			podName:     podName,
			podIdentity: podIdentity,
			ports:       ports,
			portService: portService,
		}
	}

	minReservedPods := party.MinReservedPods
	if minReservedPods <= 0 {
		minReservedPods = replicas
	}

	kit.image = fmt.Sprintf("%s:%s", appImage.Spec.Image.Name, appImage.Spec.Image.Tag)
	kit.imageID = appImage.Spec.Image.ID
	kit.deployTemplate = deployTemplate
	kit.configTemplates = appImage.Spec.ConfigTemplates
	kit.servicedPorts = servicedPorts
	kit.minReservedPods = minReservedPods
	kit.pods = pods
	kit.bandwidthLimit = party.BandwidthLimit

	// Todo: Consider how to limit the communication between single-party jobs between multiple parties.
	if len(kusciaTask.Spec.Parties) > 1 {
		kit.portAccessDomains = generatePortAccessDomains(kusciaTask.Spec.Parties, deployTemplate.NetworkPolicy, ports)
	}
	return kit, nil
}

func generatePodIdentity(taskUID string, role string, index int) string {
	if role == "" {
		return fmt.Sprintf("%s-%d", taskUID, index)
	}
	return fmt.Sprintf("%s-%s-%d", taskUID, role, index)
}

func generatePodName(taskName string, role string, index int) string {
	if role == "" {
		return fmt.Sprintf("%s-%d", taskName, index)
	}
	return fmt.Sprintf("%s-%s-%d", taskName, role, index)
}

func mergeDeployTemplate(baseTemplate *kusciaapisv1alpha1.DeployTemplate, partyTemplate *kusciaapisv1alpha1.PartyTemplate) *kusciaapisv1alpha1.DeployTemplate {
	template := baseTemplate.DeepCopy()

	if partyTemplate.Replicas != nil {
		template.Replicas = partyTemplate.Replicas
	}

	if partyTemplate.Spec.RestartPolicy != "" {
		template.Spec.RestartPolicy = partyTemplate.Spec.RestartPolicy
	}

	for i := range template.Spec.Containers {
		dstCtr := &template.Spec.Containers[i]

		if i >= len(partyTemplate.Spec.Containers) {
			break
		}
		srcCtr := &partyTemplate.Spec.Containers[i]

		if srcCtr.Name != "" {
			dstCtr.Name = srcCtr.Name
		}

		if len(srcCtr.Command) > 0 || len(srcCtr.Args) > 0 {
			dstCtr.Command = srcCtr.Command
			dstCtr.Args = srcCtr.Args
		}

		if len(srcCtr.Env) > 0 {
			dstCtr.Env = append(dstCtr.Env, srcCtr.Env...)
		}

		for name, quantity := range srcCtr.Resources.Requests {
			if dstCtr.Resources.Requests == nil {
				dstCtr.Resources.Requests = v1.ResourceList{}
			}
			dstCtr.Resources.Requests[name] = quantity
		}

		for name, quantity := range srcCtr.Resources.Limits {
			if dstCtr.Resources.Limits == nil {
				dstCtr.Resources.Limits = v1.ResourceList{}
			}
			dstCtr.Resources.Limits[name] = quantity
		}
	}

	return template
}

func mergeContainersPorts(containers []kusciaapisv1alpha1.Container) (NamedPorts, error) {
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
		if port.Scope != kusciaapisv1alpha1.ScopeCluster && port.Scope != kusciaapisv1alpha1.ScopeDomain {
			continue
		}

		servicedPorts = append(servicedPorts, port.Name)
	}

	return servicedPorts
}

func generatePortServices(podName string, servicedPorts []string) map[string]string {
	portService := PortService{}

	for _, portName := range servicedPorts {
		portService[portName] = utilsres.GenerateServiceName(podName, portName)
	}

	return portService
}

// generatePortAccessDomains generates domain list with access permission according to the role that has access to a port.
func generatePortAccessDomains(parties []kusciaapisv1alpha1.PartyInfo, networkPolicy *kusciaapisv1alpha1.NetworkPolicy, ports NamedPorts) map[string]string {
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
			if port.Scope == kusciaapisv1alpha1.ScopeCluster {
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

func (h *PendingHandler) allocatePorts(kusciaTask *kusciaapisv1alpha1.KusciaTask) (bool, error) {
	_, selfPartyKitInfos, err := h.buildPartyKitInfos(kusciaTask)
	if err != nil {
		return false, err
	}

	allocatedPorts := kusciaTask.Status.AllocatedPorts
	if len(allocatedPorts) > 0 {
		return false, nil
	}

	needCounts := map[string]int{}
	for _, partyKit := range selfPartyKitInfos {
		ns := partyKit.domainID
		count := needCounts[ns]
		for _, pod := range partyKit.pods {
			count += len(pod.ports)
		}
		needCounts[ns] = count
	}

	retPorts, err := pkgport.AllocatePort(needCounts)
	if err != nil {
		return false, err
	}

	for _, partyKit := range selfPartyKitInfos {
		ns := partyKit.domainID
		ports, ok := retPorts[ns]
		if !ok {
			return false, fmt.Errorf("allocated ports not found for domain %s", ns)
		}
		index := 0
		partyPorts := kusciaapisv1alpha1.PartyAllocatedPorts{
			DomainID:  partyKit.domainID,
			Role:      partyKit.role,
			NamedPort: map[string]int32{},
		}

		for _, pod := range partyKit.pods {
			for portName := range pod.ports {
				if index >= len(ports) {
					return false, fmt.Errorf("allocated ports are not enough for domain %s", ns)
				}

				partyPorts.NamedPort[buildPortKey(pod.podName, portName)] = ports[index]
				index++
			}
		}

		allocatedPorts = append(allocatedPorts, partyPorts)
	}

	kusciaTask.Status.AllocatedPorts = allocatedPorts
	return true, nil
}

func buildPodAllocatePorts(kusciaTask *kusciaapisv1alpha1.KusciaTask, partyKitInfos map[string]*PartyKitInfo) error {
	for _, partyKit := range partyKitInfos {
		var partyPorts *kusciaapisv1alpha1.PartyAllocatedPorts
		for _, ports := range kusciaTask.Status.AllocatedPorts {
			if ports.DomainID == partyKit.domainID && ports.Role == partyKit.role {
				partyPorts = &ports
				break
			}
		}
		if partyPorts == nil {
			return fmt.Errorf("allocated ports not found for party %s/%s", partyKit.domainID, partyKit.role)
		}

		for _, pod := range partyKit.pods {
			if err := fillPodAllocatedPorts(partyPorts, pod); err != nil {
				return fmt.Errorf("failed to fill allocated ports for party %s/%s, detail->%v", partyKit.domainID, partyKit.role, err)
			}
		}
	}
	return nil
}

func fillPodAllocatedPorts(partyPorts *kusciaapisv1alpha1.PartyAllocatedPorts, pod *PodKitInfo) error {
	resPorts := make([]*proto.Port, 0, len(pod.ports))
	for i, port := range pod.ports {
		portKey := buildPortKey(pod.podName, port.Name)
		realPort, ok := partyPorts.NamedPort[portKey]
		if !ok {
			return fmt.Errorf("not found allocated port for %v", portKey)
		}

		resPorts = append(resPorts, &proto.Port{
			Name:     port.Name,
			Port:     realPort,
			Scope:    string(port.Scope),
			Protocol: string(port.Protocol),
		})
		port.Port = realPort
		pod.ports[i] = port
	}

	pod.allocatedPorts = &proto.AllocatedPorts{Ports: resPorts}
	return nil
}

func buildPortKey(podName, portName string) string {
	return fmt.Sprintf("%s/%s", podName, portName)
}

func generateParty(kitInfo *PartyKitInfo) *proto.Party {
	var partyServices []*proto.Service

	for _, portName := range kitInfo.servicedPorts {
		endpoints := make([]string, 0, len(kitInfo.pods))

		for _, pod := range kitInfo.pods {
			endpointAddress := ""
			if pod.portService[portName] != "" {
				if pod.ports[portName].Scope == kusciaapisv1alpha1.ScopeDomain {
					endpointAddress = fmt.Sprintf("%s.%s.svc:%d", pod.portService[portName], kitInfo.domainID, pod.ports[portName].Port)
				} else {
					endpointAddress = fmt.Sprintf("%s.%s.svc", pod.portService[portName], kitInfo.domainID)
				}
			}

			endpoints = append(endpoints, endpointAddress)
		}

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

	return party
}

func generateParties(partyKitInfos map[string]*PartyKitInfo) []*proto.Party {
	var parties []*proto.Party

	for _, kitInfo := range partyKitInfos {
		party := generateParty(kitInfo)
		parties = append(parties, party)
	}

	return parties
}

func fillPartyClusterDefine(kitInfo *PartyKitInfo, parties []*proto.Party) {
	var selfPartyIndex *int

	for i, party := range parties {
		if party.Name == kitInfo.domainID && party.Role == kitInfo.role {
			selfPartyIndex = &i
			break
		}
	}

	if selfPartyIndex == nil {
		nlog.Errorf("Not found party '%v/%v', unexpected!", kitInfo.domainID, kitInfo.role)
		return
	}

	for i, podKit := range kitInfo.pods {
		fillPodClusterDefine(podKit, parties, *selfPartyIndex, i)
	}
}

func fillPodClusterDefine(pod *PodKitInfo, parties []*proto.Party, partyIndex int, endpointIndex int) {
	pod.clusterDef = &proto.ClusterDefine{
		Parties:         parties,
		SelfPartyIdx:    int32(partyIndex),
		SelfEndpointIdx: int32(endpointIndex),
	}
}

func (h *PendingHandler) createResourceForParty(partyKit *PartyKitInfo) (map[string]*kusciaapisv1alpha1.PodStatus, map[string]*kusciaapisv1alpha1.ServiceStatus, error) {
	podStatuses := map[string]*kusciaapisv1alpha1.PodStatus{}
	serviceStatuses := map[string]*kusciaapisv1alpha1.ServiceStatus{}

	if len(partyKit.configTemplates) > 0 {
		configMap := generateConfigMap(partyKit)
		if err := h.submitConfigMap(configMap); err != nil {
			return nil, nil, fmt.Errorf("failed to submit configmap %q, %v", configMap.Name, err)
		}
	}

	for _, podKit := range partyKit.pods {
		pod, err := h.generatePod(partyKit, podKit)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate pod %q spec, %v", podKit.podName, err)
		}

		pod, err = h.submitPod(pod)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to submit pod %q, %v", podKit.podName, err)
		}

		podStatuses[pod.Namespace+"/"+pod.Name] = &kusciaapisv1alpha1.PodStatus{
			PodName:   pod.Name,
			PodPhase:  pod.Status.Phase,
			Namespace: pod.ObjectMeta.Namespace,
			NodeName:  pod.Spec.NodeName,
			Message:   pod.Status.Message,
			Reason:    pod.Status.Reason,
		}

		for portName, serviceName := range podKit.portService {
			ctrPort, ok := podKit.ports[portName]
			if !ok {
				return nil, nil, fmt.Errorf("not found container port %q in pod %q", portName, podKit.podName)
			}

			service, err := generateServices(partyKit, pod, serviceName, ctrPort)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to generate service %q, %v", serviceName, err)
			}

			if err = h.submitService(service, pod); err != nil {
				return nil, nil, fmt.Errorf("failed to submit service %q, %v", serviceName, err)
			}

			serviceStatuses[service.Namespace+"/"+service.Name] = &kusciaapisv1alpha1.ServiceStatus{
				Namespace:   service.Namespace,
				ServiceName: service.Name,
				PortName:    portName,
				PortNumber:  ctrPort.Port,
				Scope:       ctrPort.Scope,
			}
		}
	}

	return podStatuses, serviceStatuses, nil
}

func (h *PendingHandler) createTaskResourceGroup(kusciaTask *kusciaapisv1alpha1.KusciaTask, partyKitInfos map[string]*PartyKitInfo) error {
	trg, err := h.generateTaskResourceGroup(kusciaTask, partyKitInfos)
	if err != nil {
		return err
	}
	return h.submitTaskResourceGroup(trg)
}

// generateTaskResourceGroup use selfControlled partyKitInfos to create trg.Spec.Parties
// and adjust trg.Spec.MinReservedMembers by minus the amount of out of controlled parties
func (h *PendingHandler) generateTaskResourceGroup(kusciaTask *kusciaapisv1alpha1.KusciaTask, partyKitInfos map[string]*PartyKitInfo) (*kusciaapisv1alpha1.TaskResourceGroup, error) {
	var (
		resourceReservedSeconds = defaultResourceReservedSeconds
		lifeCycleSeconds        = defaultLifecycleSeconds
		retryIntervalSeconds    = defaultRetryIntervalSeconds
	)

	if kusciaTask.Spec.ScheduleConfig.ResourceReservedSeconds > 0 {
		resourceReservedSeconds = kusciaTask.Spec.ScheduleConfig.ResourceReservedSeconds
	}

	if kusciaTask.Spec.ScheduleConfig.LifecycleSeconds > 0 {
		lifeCycleSeconds = kusciaTask.Spec.ScheduleConfig.LifecycleSeconds
	}

	if kusciaTask.Spec.ScheduleConfig.RetryIntervalSeconds > 0 {
		retryIntervalSeconds = kusciaTask.Spec.ScheduleConfig.RetryIntervalSeconds
	}

	var trgParties, outOfControlledParties []kusciaapisv1alpha1.TaskResourceGroupParty
	for _, partyKitInfo := range partyKitInfos {
		isPartner, err := utilsres.IsPartnerDomain(h.namespacesLister, partyKitInfo.domainID)
		if err != nil {
			return nil, err
		}
		if isPartner {
			outOfControlledParties = append(outOfControlledParties, kusciaapisv1alpha1.TaskResourceGroupParty{
				Role:            partyKitInfo.role,
				DomainID:        partyKitInfo.domainID,
				MinReservedPods: 1,
			})
			continue
		}

		var trgPartyPods []kusciaapisv1alpha1.TaskResourceGroupPartyPod
		if !isPartner {
			for _, ps := range partyKitInfo.pods {
				trgPartyPods = append(trgPartyPods, kusciaapisv1alpha1.TaskResourceGroupPartyPod{Name: ps.podName})
			}
		}

		trgParties = append(trgParties, kusciaapisv1alpha1.TaskResourceGroupParty{
			Role:            partyKitInfo.role,
			DomainID:        partyKitInfo.domainID,
			MinReservedPods: partyKitInfo.minReservedPods,
			Pods:            trgPartyPods,
		})
	}

	minReservedMembers := kusciaTask.Spec.ScheduleConfig.MinReservedMembers
	allAvailableParty := len(trgParties) + len(outOfControlledParties)
	if minReservedMembers <= 0 || minReservedMembers > allAvailableParty {
		minReservedMembers = allAvailableParty
	}

	var jobID, taskAlias string
	if kusciaTask.Annotations != nil {
		jobID = kusciaTask.Annotations[common.JobIDAnnotationKey]
		taskAlias = kusciaTask.Annotations[common.TaskAliasAnnotationKey]
	}

	var jobUID string
	if kusciaTask.Labels != nil {
		jobUID = kusciaTask.Labels[common.LabelJobUID]
	}

	trg := &kusciaapisv1alpha1.TaskResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: kusciaTask.Name,
			Annotations: map[string]string{
				common.InitiatorAnnotationKey:               kusciaTask.Annotations[common.InitiatorAnnotationKey],
				common.SelfClusterAsInitiatorAnnotationKey:  kusciaTask.Annotations[common.SelfClusterAsInitiatorAnnotationKey],
				common.JobIDAnnotationKey:                   jobID,
				common.TaskIDAnnotationKey:                  kusciaTask.Name,
				common.TaskAliasAnnotationKey:               taskAlias,
				common.InterConnKusciaPartyAnnotationKey:    kusciaTask.Annotations[common.InterConnKusciaPartyAnnotationKey],
				common.InterConnBFIAPartyAnnotationKey:      kusciaTask.Annotations[common.InterConnBFIAPartyAnnotationKey],
				common.KusciaPartyMasterDomainAnnotationKey: kusciaTask.Annotations[common.KusciaPartyMasterDomainAnnotationKey],
			},
			Labels: map[string]string{
				common.LabelController: common.ControllerKusciaTask,
				common.LabelJobUID:     jobUID,
				common.LabelTaskUID:    string(kusciaTask.UID),
			},
		},
		Spec: kusciaapisv1alpha1.TaskResourceGroupSpec{
			MinReservedMembers:      minReservedMembers,
			ResourceReservedSeconds: resourceReservedSeconds,
			LifecycleSeconds:        lifeCycleSeconds,
			RetryIntervalSeconds:    retryIntervalSeconds,
			Initiator:               kusciaTask.Spec.Initiator,
			Parties:                 trgParties,
			OutOfControlledParties:  outOfControlledParties,
		},
	}

	return trg, nil
}

func generateConfigMap(partyKit *PartyKitInfo) *v1.ConfigMap {
	labels := map[string]string{
		common.LabelController: common.ControllerKusciaTask,
		common.LabelTaskUID:    string(partyKit.kusciaTask.UID),
	}

	annotations := map[string]string{
		common.InitiatorAnnotationKey: partyKit.kusciaTask.Spec.Initiator,
		common.TaskIDAnnotationKey:    partyKit.kusciaTask.Name,
	}

	var protocolType string
	if partyKit.kusciaTask.Labels != nil {
		protocolType = partyKit.kusciaTask.Labels[common.LabelInterConnProtocolType]
	}

	if protocolType != "" {
		labels[common.LabelInterConnProtocolType] = protocolType
	}

	partyKit.configTemplatesCMName = fmt.Sprintf("%s-configtemplate", partyKit.kusciaTask.Name)
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        partyKit.configTemplatesCMName,
			Namespace:   partyKit.domainID,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: partyKit.configTemplates,
	}
}

func generateKusciaConfigMap(partyKit *PartyKitInfo, podKit *PodKitInfo) *v1.ConfigMap {
	labels := map[string]string{
		common.LabelController: common.ControllerKusciaTask,
		common.LabelTaskUID:    string(partyKit.kusciaTask.UID),
	}

	annotations := map[string]string{
		common.InitiatorAnnotationKey: partyKit.kusciaTask.Spec.Initiator,
		common.TaskIDAnnotationKey:    partyKit.kusciaTask.Name,
	}

	var protocolType string
	if partyKit.kusciaTask.Labels != nil {
		protocolType = partyKit.kusciaTask.Labels[common.LabelInterConnProtocolType]
	}

	if protocolType != "" {
		labels[common.LabelInterConnProtocolType] = protocolType
	}

	protoJSONOptions := protojson.MarshalOptions{EmitUnpopulated: true}
	clusterDefine, _ := protoJSONOptions.Marshal(podKit.clusterDef)
	allocatedPorts, _ := protoJSONOptions.Marshal(podKit.allocatedPorts)

	confMap := make(map[string]string)
	confMap[common.EnvDomainID] = partyKit.domainID
	confMap[common.EnvTaskID] = partyKit.kusciaTask.Name
	confMap[common.EnvTaskClusterDefine] = string(clusterDefine)
	confMap[common.EnvAllocatedPorts] = string(allocatedPorts)
	confMapBinaryData := make(map[string][]byte)
	// compress task input config
	if compressInputConf, err := utilcom.CompressString(partyKit.kusciaTask.Spec.TaskInputConfig); err != nil {
		nlog.Warnf("Compress taskInputConfig failed, pod: %s, error: %s.", podKit.podName, err.Error())
		confMap[common.EnvTaskInputConfig] = partyKit.kusciaTask.Spec.TaskInputConfig
	} else {
		// set compress value to the binary data of configMap
		confMapBinaryData[common.EnvTaskInputConfig] = compressInputConf
		// set the annotation to indicate which value was compressed
		annotations[common.ConfigValueCompressFieldsNameAnnotationKey] = utilcom.SliceToAnnotationString([]string{common.EnvTaskInputConfig})
	}

	confMapName := fmt.Sprintf(common.KusciaGenerateConfigMapFormat, partyKit.kusciaTask.Name) // kuscia-generate-conf
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        confMapName,
			Namespace:   partyKit.domainID,
			Labels:      labels,
			Annotations: annotations,
		},
		Data:       confMap,
		BinaryData: confMapBinaryData,
	}
}

func (h *PendingHandler) submitConfigMap(cm *v1.ConfigMap) error {
	_, err := h.configMapLister.ConfigMaps(cm.Namespace).Get(cm.Name)
	if k8serrors.IsNotFound(err) {
		_, err = h.kubeClient.CoreV1().ConfigMaps(cm.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	}
	// If an error occurs during Get/Create, we'll requeue the item, so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (h *PendingHandler) generatePod(partyKit *PartyKitInfo, podKit *PodKitInfo) (*v1.Pod, error) {
	labels := map[string]string{
		common.LabelController:              common.ControllerKusciaTask,
		common.LabelCommunicationRoleServer: "true",
		common.LabelCommunicationRoleClient: "true",
		labelKusciaTaskPodIdentity:          podKit.podIdentity,
		kusciaapisv1alpha1.TaskResourceUID:  "",
		common.LabelTaskUID:                 string(partyKit.kusciaTask.UID),
		labelKusciaTaskPodRole:              partyKit.role,
	}

	annotations := map[string]string{
		common.InitiatorAnnotationKey:         partyKit.kusciaTask.Spec.Initiator,
		common.TaskIDAnnotationKey:            partyKit.kusciaTask.Name,
		common.TaskResourceGroupAnnotationKey: partyKit.kusciaTask.Name,
		kusciaapisv1alpha1.TaskResourceKey:    "",
	}

	var protocolType string
	if partyKit.kusciaTask.Labels != nil {
		protocolType = partyKit.kusciaTask.Labels[common.LabelInterConnProtocolType]
	}

	if protocolType != "" {
		labels[common.LabelInterConnProtocolType] = protocolType
	}

	restartPolicy := v1.RestartPolicyNever
	if partyKit.deployTemplate.Spec.RestartPolicy != "" {
		restartPolicy = partyKit.deployTemplate.Spec.RestartPolicy
	}

	ns, err := h.namespacesLister.Get(partyKit.domainID)
	if err != nil {
		return nil, err
	}

	schedulerName := common.KusciaSchedulerName
	if ns.Labels != nil && ns.Labels[common.LabelDomainRole] == string(kusciaapisv1alpha1.Partner) {
		schedulerName = fmt.Sprintf("%v-%v", partyKit.domainID, schedulerName)
	}

	automountServiceAccountToken := false
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podKit.podName,
			Namespace:   partyKit.domainID,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1.PodSpec{
			RestartPolicy: restartPolicy,
			Tolerations: []v1.Toleration{
				{
					Key:      common.KusciaTaintTolerationKey,
					Operator: v1.TolerationOpExists,
					Effect:   v1.TaintEffectNoSchedule,
				},
			},
			NodeSelector: map[string]string{
				common.LabelNodeNamespace: partyKit.domainID,
			},
			SchedulerName:                schedulerName,
			AutomountServiceAccountToken: &automountServiceAccountToken,
		},
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	if partyKit.imageID != "" {
		pod.Annotations[common.ImageIDAnnotationKey] = partyKit.imageID
	}

	needConfigTemplateVolume := false
	for _, ctr := range partyKit.deployTemplate.Spec.Containers {
		if ctr.ImagePullPolicy == "" {
			ctr.ImagePullPolicy = v1.PullIfNotPresent
		}

		resCtr := v1.Container{
			Name:                     ctr.Name,
			Image:                    partyKit.image,
			Command:                  ctr.Command,
			Args:                     ctr.Args,
			WorkingDir:               ctr.WorkingDir,
			Env:                      ctr.Env,
			EnvFrom:                  ctr.EnvFrom,
			Resources:                ctr.Resources,
			LivenessProbe:            ctr.LivenessProbe,
			ReadinessProbe:           ctr.ReadinessProbe,
			StartupProbe:             ctr.StartupProbe,
			ImagePullPolicy:          ctr.ImagePullPolicy,
			SecurityContext:          ctr.SecurityContext,
			TerminationMessagePolicy: v1.TerminationMessageFallbackToLogsOnError,
		}

		for _, port := range ctr.Ports {
			namedPort, ok := podKit.ports[port.Name]
			if !ok {
				return nil, fmt.Errorf("port %s is not allocated for pod %s", port.Name, pod.Name)
			}
			resPort := v1.ContainerPort{
				Name:          port.Name,
				ContainerPort: namedPort.Port,
				Protocol:      v1.ProtocolTCP,
			}

			resCtr.Ports = append(resCtr.Ports, resPort)
		}

		portNumberEnvs := buildPortNumberEnvs(podKit.allocatedPorts) // todo : remove it , now scql use it ,20240829
		if len(portNumberEnvs) > 0 {
			resCtr.Env = append(resCtr.Env, portNumberEnvs...)
		}

		if len(ctr.ConfigVolumeMounts) > 0 && partyKit.configTemplatesCMName != "" {
			needConfigTemplateVolume = true
			for _, vm := range ctr.ConfigVolumeMounts {
				resCtr.VolumeMounts = append(resCtr.VolumeMounts, v1.VolumeMount{
					Name:      configTemplateVolumeName,
					MountPath: vm.MountPath,
					SubPath:   vm.SubPath,
				})
			}
		}

		pod.Spec.Containers = append(pod.Spec.Containers, resCtr)
	}

	if needConfigTemplateVolume {
		// set the config(such as allocatePorts , clusterDefine, taskInputConfig) generated by kuscia to configMap
		// transport config via configMap instead of ENV value
		confMap := generateKusciaConfigMap(partyKit, podKit)
		err = h.submitConfigMap(confMap)
		if err != nil {
			nlog.Errorf("Submit task configMap failed, taskID: %s, error: %s.", partyKit.kusciaTask.Name, err.Error())
			return nil, err
		}
		// set the configMap which contain template values to the annotation of pod
		pod.Annotations[common.ConfigTemplateValueAnnotationKey] = confMap.Name

		pod.Annotations[common.ConfigTemplateVolumesAnnotationKey] = configTemplateVolumeName
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: configTemplateVolumeName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: partyKit.configTemplatesCMName,
					},
				},
			},
		})
	}

	return pod, nil
}

func buildPortNumberEnvs(allocatedPorts *proto.AllocatedPorts) []v1.EnvVar {
	if allocatedPorts == nil {
		return nil
	}

	portNumberEnvs := make([]v1.EnvVar, 0)
	for _, portInfo := range allocatedPorts.Ports {
		if portInfo == nil {
			continue
		}

		portNumberEnvs = append(portNumberEnvs, v1.EnvVar{
			Name:  strings.ToUpper(strings.ReplaceAll(fmt.Sprintf(common.EnvPortNumber, portInfo.Name), "-", "_")),
			Value: strconv.Itoa(int(portInfo.Port)),
		})
	}
	return portNumberEnvs
}

func (h *PendingHandler) submitPod(pod *v1.Pod) (*v1.Pod, error) {
	listerPod, err := h.podsLister.Pods(pod.Namespace).Get(pod.Name)
	if k8serrors.IsNotFound(err) {
		listerPod, err = h.kubeClient.CoreV1().Pods(pod.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	}

	// If an error occurs during Get/Create, we'll requeue the item, so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	return listerPod, nil
}

func generateServices(partyKit *PartyKitInfo, pod *v1.Pod, serviceName string, port kusciaapisv1alpha1.ContainerPort) (*v1.Service, error) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: pod.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(pod, v1.SchemeGroupVersion.WithKind("Pod")),
			},
		},
		Spec: v1.ServiceSpec{
			Type:      v1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Selector:  map[string]string{labelKusciaTaskPodIdentity: pod.Labels[labelKusciaTaskPodIdentity]},
			Ports: []v1.ServicePort{
				{
					Name:     port.Name,
					Port:     port.Port,
					Protocol: v1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: port.Name,
					},
				},
			},
			PublishNotReadyAddresses: true,
		},
	}

	svc.Labels = map[string]string{
		common.LabelPortName:  port.Name,
		common.LabelPortScope: string(port.Scope),
		common.LabelTaskUID:   string(partyKit.kusciaTask.UID),
	}

	var protocolType string
	if partyKit.kusciaTask.Labels != nil {
		protocolType = partyKit.kusciaTask.Labels[common.LabelInterConnProtocolType]
	}

	if protocolType != "" {
		svc.Labels[common.LabelInterConnProtocolType] = protocolType
	}

	if port.Scope != kusciaapisv1alpha1.ScopeDomain {
		svc.Labels[common.LabelLoadBalancer] = string(common.DomainRouteLoadBalancer)
	}

	svc.Annotations = map[string]string{
		common.InitiatorAnnotationKey:    partyKit.kusciaTask.Spec.Initiator,
		common.ProtocolAnnotationKey:     string(port.Protocol),
		common.AccessDomainAnnotationKey: partyKit.portAccessDomains[port.Name],
		common.TaskIDAnnotationKey:       partyKit.kusciaTask.Name,
	}

	for _, limit := range partyKit.bandwidthLimit {
		key := fmt.Sprintf("%s%s", common.TaskBandwidthLimitAnnotationPrefix, limit.DestinationID)
		svc.Annotations[key] = strconv.Itoa(int(limit.LimitKBps))
	}

	return svc, nil
}

func (h *PendingHandler) submitService(service *v1.Service, pod *v1.Pod) error {
	listerService, err := h.servicesLister.Services(service.Namespace).Get(service.Name)
	if k8serrors.IsNotFound(err) {
		listerService, err = h.kubeClient.CoreV1().Services(service.Namespace).Create(context.Background(), service, metav1.CreateOptions{})
	}
	// If an error occurs during Get/Create, we'll requeue the item, so we
	// can attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}
	// If the Service is not controlled by this Pod resource, we
	// should log a warning to the event recorder and return.
	if !metav1.IsControlledBy(listerService, pod) {
		return fmt.Errorf("service %q already exists and is not owned by %s", service.Name, pod.Name)
	}

	return nil
}

func (h *PendingHandler) submitTaskResourceGroup(trg *kusciaapisv1alpha1.TaskResourceGroup) error {
	_, err := h.trgLister.Get(trg.Name)
	if k8serrors.IsNotFound(err) {
		_, err = h.kusciaClient.KusciaV1alpha1().TaskResourceGroups().Create(context.Background(), trg, metav1.CreateOptions{})
	}

	if err != nil {
		return err
	}

	return nil
}
