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

package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/secretflow/kuscia/pkg/common"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
	utilsres "github.com/secretflow/kuscia/pkg/utils/resources"
	proto "github.com/secretflow/kuscia/proto/api/v1alpha1/appconfig"
)

func makeTestPendingHandler() *PendingHandler {
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset(makeTestAppImageCase1(), makeTestKusciaTaskCase1())

	kubeInformersFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(kusciaClient, 0)
	nsInformer := kubeInformersFactory.Core().V1().Namespaces()
	appImageInformer := kusciaInformerFactory.Kuscia().V1alpha1().AppImages()

	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "domain-a",
		},
	}
	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "domain-b",
		},
	}

	nsInformer.Informer().GetStore().Add(&ns1)
	nsInformer.Informer().GetStore().Add(&ns2)
	appImageInformer.Informer().GetStore().Add(makeTestAppImageCase1())

	dep := &Dependencies{
		KubeClient:       kubeClient,
		KusciaClient:     kusciaClient,
		TrgLister:        kusciaInformerFactory.Kuscia().V1alpha1().TaskResourceGroups().Lister(),
		NamespacesLister: nsInformer.Lister(),
		PodsLister:       kubeInformersFactory.Core().V1().Pods().Lister(),
		ServicesLister:   kubeInformersFactory.Core().V1().Services().Lister(),
		AppImagesLister:  appImageInformer.Lister(),
	}

	return NewPendingHandler(dep)
}

func TestPendingHandler_Handle(t *testing.T) {
	handler := makeTestPendingHandler()
	kusciaTask := makeTestKusciaTaskCase1()

	_, err := handler.Handle(kusciaTask)
	assert.NoError(t, err)
	assert.Equal(t, kusciaapisv1alpha1.TaskPending, kusciaTask.Status.Phase)

	kusciaTask.Status.Conditions = nil
	kusciaTask.Spec.Parties[0].AppImageRef = "not-exist-image"
	_, err = handler.Handle(kusciaTask)
	assert.Error(t, err)
	createdCondition, _ := utilsres.GetKusciaTaskCondition(&kusciaTask.Status, kusciaapisv1alpha1.KusciaTaskCondResourceCreated, true)
	assert.Equal(t, v1.ConditionFalse, createdCondition.Status)

	podTests := []struct {
		namespace string
		name      string
	}{
		{
			namespace: "domain-a",
			name:      "kusciatask-001-server-0",
		},
		{
			namespace: "domain-a",
			name:      "kusciatask-001-server-1",
		},
		{
			namespace: "domain-b",
			name:      "kusciatask-001-client-0",
		},
		{
			namespace: "domain-b",
			name:      "kusciatask-001-client-1",
		},
	}

	for i, tt := range podTests {
		t.Run(fmt.Sprintf("PodTestCase %d", i), func(t *testing.T) {
			_, err := handler.kubeClient.CoreV1().Pods(tt.namespace).Get(context.Background(), tt.name, metav1.GetOptions{})
			assert.NoError(t, err)
		})
	}

	serviceTests := []struct {
		namespace string
		name      string
	}{
		{
			namespace: "domain-a",
			name:      "kusciatask-001-server-0-cluster",
		},
		{
			namespace: "domain-b",
			name:      "kusciatask-001-client-0-cluster",
		},
	}

	for i, tt := range serviceTests {
		t.Run(fmt.Sprintf("ServiceTestCase %d", i), func(t *testing.T) {
			_, err := handler.kubeClient.CoreV1().Services(tt.namespace).Get(context.Background(), tt.name, metav1.GetOptions{})
			assert.NoError(t, err)
		})
	}
}

func Test_mergeDeployTemplate(t *testing.T) {
	tests := []struct {
		baseTemplate  *kusciaapisv1alpha1.DeployTemplate
		partyTemplate *kusciaapisv1alpha1.PartyTemplate
		want          *kusciaapisv1alpha1.DeployTemplate
	}{
		{
			baseTemplate:  makeTestDeployTemplateCase2(),
			partyTemplate: makeTestPartyTemplateCase1(),
			want:          makeTestDeployTemplateCase3(),
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("TestCase %d", i), func(t *testing.T) {
			assert.Equal(t, tt.want, mergeDeployTemplate(tt.baseTemplate, tt.partyTemplate))
		})
	}
}

func Test_generatePortAccessDomains(t *testing.T) {
	parties := []kusciaapisv1alpha1.PartyInfo{
		{
			DomainID: "domain-a",
			Role:     "server",
		},
		{
			DomainID: "domain-b",
			Role:     "client",
		},
		{
			DomainID: "domain-c",
			Role:     "client",
		},
	}

	ports := NamedPorts{
		"port-10000": kusciaapisv1alpha1.ContainerPort{
			Name:     "port-10000",
			Port:     10000,
			Protocol: "HTTP",
			Scope:    kusciaapisv1alpha1.ScopeCluster,
		},
		"port-10001": kusciaapisv1alpha1.ContainerPort{
			Name:     "port-10001",
			Port:     10001,
			Protocol: "HTTP",
			Scope:    kusciaapisv1alpha1.ScopeDomain,
		},
	}

	tests := []struct {
		name                  string
		parties               []kusciaapisv1alpha1.PartyInfo
		networkPolicy         *kusciaapisv1alpha1.NetworkPolicy
		wantPortAccessDomains map[string][]string
	}{
		{
			name:    "domain-b,domain-c can access all port, domain-a only can access one port",
			parties: parties,
			networkPolicy: &kusciaapisv1alpha1.NetworkPolicy{
				Ingresses: []kusciaapisv1alpha1.Ingress{
					{
						From: kusciaapisv1alpha1.IngressFrom{
							Roles: []string{"client"},
						},
						Ports: []kusciaapisv1alpha1.IngressPort{
							{
								Port: "port-10000",
							},
							{
								Port: "port-10001",
							},
						},
					},
					{
						From: kusciaapisv1alpha1.IngressFrom{
							Roles: []string{"client", "server"},
						},
						Ports: []kusciaapisv1alpha1.IngressPort{
							{
								Port: "port-10002",
							},
						},
					},
				},
			},
			wantPortAccessDomains: map[string][]string{
				"port-10000": {"domain-b", "domain-c"},
				"port-10001": {"domain-b", "domain-c"},
				"port-10002": {"domain-a", "domain-b", "domain-c"},
			},
		},
		{
			name:          "domain-a, domain-b and domain-c can access cluster ports",
			parties:       parties,
			networkPolicy: nil,
			wantPortAccessDomains: map[string][]string{
				"port-10000": {"domain-a", "domain-b", "domain-c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessDomains := generatePortAccessDomains(tt.parties, tt.networkPolicy, ports)

			accessDomainsConverted := map[string][]string{}
			for port, domains := range accessDomains {
				domainSlice := strings.Split(domains, ",")
				sort.Strings(domainSlice)
				accessDomainsConverted[port] = domainSlice
			}

			assert.Equal(t, tt.wantPortAccessDomains, accessDomainsConverted)
		})
	}
}

func Test_generatePod(t *testing.T) {
	partyKit := &PartyKitInfo{
		kusciaTask:            makeTestKusciaTaskCase1(),
		domainID:              "domain-a",
		role:                  "server",
		image:                 "test-image:0.0.1",
		deployTemplate:        makeTestDeployTemplateCase1(),
		configTemplatesCMName: "kusciatask-001-configtemplate",
		configTemplates:       map[string]string{"task-config.config": "task_input_config: {{.TASK_INPUT_CONFIG}}"},
		servicedPorts:         []string{"cluster", "domain"},
		portAccessDomains:     map[string]string{},
		pods: []*PodKitInfo{
			{
				index:   0,
				podName: "kusciatask-001-server-0",
				ports: NamedPorts{
					"cluster": kusciaapisv1alpha1.ContainerPort{
						Name:  "cluster",
						Port:  10000,
						Scope: kusciaapisv1alpha1.ScopeCluster,
					},
					"domain": kusciaapisv1alpha1.ContainerPort{
						Name:  "domain",
						Port:  10001,
						Scope: kusciaapisv1alpha1.ScopeDomain,
					},
					"local": kusciaapisv1alpha1.ContainerPort{
						Name:  "local",
						Port:  10002,
						Scope: kusciaapisv1alpha1.ScopeLocal,
					},
				},
				clusterDef:     &proto.ClusterDefine{},
				allocatedPorts: &proto.AllocatedPorts{},
			},
		},
	}

	podKit := partyKit.pods[0]
	h := makeTestPendingHandler()
	pod, err := h.generatePod(partyKit, podKit)
	assert.NoError(t, err)

	rmEnv := func(pod *v1.Pod, envName string) {
		for i, c := range pod.Spec.Containers {
			for ii, env := range c.Env {
				if env.Name == envName {
					pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env[:ii], pod.Spec.Containers[i].Env[ii+1:]...)
					return
				}
			}
		}
	}

	wantPodYAML := `
metadata:
  creationTimestamp: null
  labels:
    kuscia.secretflow/communication-role-client: "true"
    kuscia.secretflow/communication-role-server: "true"
    kuscia.secretflow/controller: kusciatask
    kuscia.secretflow/pod-identity: ""
    kuscia.secretflow/task-resource-uid: ""
    kuscia.secretflow/task-uid: ""
    kuscia.secretflow/pod-role: server
  annotations:
    kuscia.secretflow/config-template-volumes: config-template
    kuscia.secretflow/initiator: ""
    kuscia.secretflow/task-id: kusciatask-001
    kuscia.secretflow/task-resource: ""
    kuscia.secretflow/task-resource-group: kusciatask-001
  name: kusciatask-001-server-0
  namespace: domain-a
spec:
  containers:
  - args:
    - -l
    command:
    - ls
    env:
    - name: HOME
      value: /root
    - name: KUSCIA_DOMAIN_ID
      value: domain-a
    - name: TASK_ID
      value: kusciatask-001
    - name: TASK_CLUSTER_DEFINE
      value: "{\"parties\":[], \"selfPartyIdx\":0, \"selfEndpointIdx\":0}"
    - name: ALLOCATED_PORTS
      value: "{\"ports\":[]}"
    - name: TASK_INPUT_CONFIG
      value: task input config
    image: test-image:0.0.1
    name: container-0
    ports:
    - containerPort: 10000
      name: cluster
      protocol: TCP
    - containerPort: 10001
      name: domain
      protocol: TCP
    - containerPort: 10002
      name: local
      protocol: TCP
    resources:
      limits:
        cpu: "2"
      requests:
        cpu: "1"
    terminationMessagePolicy: FallbackToLogsOnError
    ImagePullPolicy: IfNotPresent
    volumeMounts:
    - mountPath: /home/admin/conf/task-config.conf
      name: config-template
      subPath: task-config.conf
  schedulerName: "kuscia-scheduler"
  automountServiceAccountToken: false
  nodeSelector:
    kuscia.secretflow/namespace: domain-a
  restartPolicy: Always
  tolerations:
  - effect: NoSchedule
    key: kuscia.secretflow/agent
    operator: Exists
  volumes:
  - configMap:
      name: kusciatask-001-configtemplate
    name: config-template
status: {}
`
	wantPod := &v1.Pod{}
	assert.NoError(t, yaml.Unmarshal([]byte(wantPodYAML), wantPod))

	rmEnv(wantPod, "TASK_CLUSTER_DEFINE")
	rmEnv(wantPod, "ALLOCATED_PORTS")
	rmEnv(pod, "TASK_CLUSTER_DEFINE")
	rmEnv(pod, "ALLOCATED_PORTS")

	assert.Equal(t, wantPod, pod)
}

func makeTestAppImageCase1() *kusciaapisv1alpha1.AppImage {
	return &kusciaapisv1alpha1.AppImage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-image-1",
		},
		Spec: kusciaapisv1alpha1.AppImageSpec{
			Image: kusciaapisv1alpha1.AppImageInfo{
				Name: "test-image",
				Tag:  "1.0.0",
			},
			DeployTemplates: []kusciaapisv1alpha1.DeployTemplate{
				*makeTestDeployTemplateCase1(),
			},
		},
	}
}

func makeTestKusciaTaskCase1() *kusciaapisv1alpha1.KusciaTask {
	return &kusciaapisv1alpha1.KusciaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kusciatask-001",
			Namespace: common.KusciaCrossDomain,
		},
		Spec: kusciaapisv1alpha1.KusciaTaskSpec{
			TaskInputConfig: "task input config",
			Parties: []kusciaapisv1alpha1.PartyInfo{
				{
					DomainID:    "domain-a",
					AppImageRef: "test-image-1",
					Role:        "server",
					Template: kusciaapisv1alpha1.PartyTemplate{
						Spec: kusciaapisv1alpha1.PodSpec{
							Containers: []kusciaapisv1alpha1.Container{
								{
									Name:    "container-0",
									Command: []string{"pwd"},
								},
							},
						},
					},
				},
				{
					DomainID:    "domain-b",
					AppImageRef: "test-image-1",
					Role:        "client",
					Template: kusciaapisv1alpha1.PartyTemplate{
						Spec: kusciaapisv1alpha1.PodSpec{
							Containers: []kusciaapisv1alpha1.Container{
								{
									Name:    "container-0",
									Command: []string{"whoami"},
								},
							},
						},
					},
				},
			},
		},
	}
}

func makeTestDeployTemplateCase1() *kusciaapisv1alpha1.DeployTemplate {
	dt := &kusciaapisv1alpha1.DeployTemplate{
		Name:     "abc",
		Role:     "server,client",
		Replicas: new(int32),
		Spec: kusciaapisv1alpha1.PodSpec{
			RestartPolicy: v1.RestartPolicyAlways,
			Containers: []kusciaapisv1alpha1.Container{
				{
					Name:    "container-0",
					Command: []string{"ls"},
					Args:    []string{"-l"},
					Env: []v1.EnvVar{
						{
							Name:  "HOME",
							Value: "/root",
						},
					},
					Ports: []kusciaapisv1alpha1.ContainerPort{
						{
							Name:  "cluster",
							Port:  10000,
							Scope: kusciaapisv1alpha1.ScopeCluster,
						},
						{
							Name:  "domain",
							Port:  10001,
							Scope: kusciaapisv1alpha1.ScopeDomain,
						},
						{
							Name:  "local",
							Port:  10002,
							Scope: kusciaapisv1alpha1.ScopeLocal,
						},
					},
					ConfigVolumeMounts: []kusciaapisv1alpha1.ConfigVolumeMount{
						{
							MountPath: "/home/admin/conf/task-config.conf",
							SubPath:   "task-config.conf",
						},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("2"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
		},
	}
	*dt.Replicas = 2

	return dt
}

func makeTestDeployTemplateCase2() *kusciaapisv1alpha1.DeployTemplate {
	dt := &kusciaapisv1alpha1.DeployTemplate{
		Name:     "abc",
		Role:     "server,client",
		Replicas: new(int32),
		Spec: kusciaapisv1alpha1.PodSpec{
			RestartPolicy: v1.RestartPolicyAlways,
			Containers: []kusciaapisv1alpha1.Container{
				{
					Name:    "container-0",
					Command: []string{"ls"},
					Args:    []string{"-l"},
					Env: []v1.EnvVar{
						{
							Name:  "HOME",
							Value: "/root",
						},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("2"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
		},
	}
	*dt.Replicas = 1

	return dt
}

func makeTestDeployTemplateCase3() *kusciaapisv1alpha1.DeployTemplate {
	dt := &kusciaapisv1alpha1.DeployTemplate{
		Name:     "abc",
		Role:     "server,client",
		Replicas: new(int32),
		Spec: kusciaapisv1alpha1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []kusciaapisv1alpha1.Container{
				{
					Name:    "container-0",
					Command: []string{""},
					Env: []v1.EnvVar{
						{
							Name:  "HOME",
							Value: "/root",
						},
						{
							Name:  "ROLE",
							Value: "server",
						},
					},
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("2"),
						},
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("2"),
						},
					},
				},
			},
		},
	}
	*dt.Replicas = 2

	return dt
}

func makeTestPartyTemplateCase1() *kusciaapisv1alpha1.PartyTemplate {
	dt := &kusciaapisv1alpha1.PartyTemplate{
		Replicas: new(int32),
		Spec: kusciaapisv1alpha1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []kusciaapisv1alpha1.Container{
				{
					Name:    "container-0",
					Command: []string{""},
					Env: []v1.EnvVar{
						{
							Name:  "ROLE",
							Value: "server",
						},
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: resource.MustParse("2"),
						},
					},
				},
			},
		},
	}
	*dt.Replicas = 2

	return dt
}
