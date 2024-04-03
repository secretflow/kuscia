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

package pod

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeinformers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/kubelet/events"

	"github.com/secretflow/kuscia/pkg/agent/config"
	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/framework"
	"github.com/secretflow/kuscia/pkg/agent/kri"
	"github.com/secretflow/kuscia/pkg/agent/middleware/hook"
	"github.com/secretflow/kuscia/pkg/agent/provider/pod/kubebackend"
	"github.com/secretflow/kuscia/pkg/agent/resource"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/agent/utils/podutils"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/election"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	labelOwnerPodName = "kuscia.secretflow/owner-pod-name"
)

var (
	resourceMinLifeCycle = 30 * time.Second
	resourceNameLimit    = 253
)

type K8sProviderDependence struct {
	NodeName        string
	Namespace       string
	NodeIP          string
	BkClient        clientset.Interface
	PodSyncHandler  framework.SyncHandler
	ResourceManager *resource.KubeResourceManager
	K8sProviderCfg  *config.K8sProviderCfg
	Recorder        record.EventRecorder
}

type K8sProvider struct {
	bkClient          clientset.Interface
	namespace         string
	bkNamespace       string
	nodeName          string
	podDNSConfig      *v1.PodDNSConfig
	podDNSPolicy      string
	resolveConfigData string
	podSyncHandler    framework.SyncHandler
	resourceManager   *resource.KubeResourceManager
	backendPlugin     kubebackend.BackendPlugin

	kubeInformerFactory kubeinformers.SharedInformerFactory
	podLister           corelisters.PodNamespaceLister
	podsSynced          cache.InformerSynced
	configMapLister     corelisters.ConfigMapNamespaceLister
	configMapSynced     cache.InformerSynced
	secretLister        corelisters.SecretNamespaceLister
	secretSynced        cache.InformerSynced

	labelsToAdd      map[string]string
	annotationsToAdd map[string]string
	runtimeClassName string

	leaderElector election.Elector
	recorder      record.EventRecorder
}

func NewK8sProvider(dep *K8sProviderDependence) (*K8sProvider, error) {
	kp := &K8sProvider{
		bkClient:         dep.BkClient,
		namespace:        dep.Namespace,
		bkNamespace:      dep.K8sProviderCfg.Namespace,
		nodeName:         dep.NodeName,
		podDNSPolicy:     dep.K8sProviderCfg.DNS.Policy,
		podSyncHandler:   dep.PodSyncHandler,
		resourceManager:  dep.ResourceManager,
		labelsToAdd:      dep.K8sProviderCfg.LabelsToAdd,
		annotationsToAdd: dep.K8sProviderCfg.AnnotationsToAdd,
		runtimeClassName: dep.K8sProviderCfg.RuntimeClassName,
		recorder:         dep.Recorder,
	}

	if kp.podDNSPolicy == "" {
		kp.podDNSPolicy = "None"
	}

	if kp.labelsToAdd == nil {
		kp.labelsToAdd = map[string]string{}
	}
	if kp.annotationsToAdd == nil {
		kp.annotationsToAdd = map[string]string{}
	}

	if len(dep.K8sProviderCfg.DNS.Servers) == 0 {
		kp.podDNSConfig = &v1.PodDNSConfig{
			Nameservers: []string{dep.NodeIP},
		}
	} else {
		kp.podDNSConfig = &v1.PodDNSConfig{
			Nameservers: dep.K8sProviderCfg.DNS.Servers,
			Searches:    dep.K8sProviderCfg.DNS.Searches,
		}
	}

	if dep.K8sProviderCfg.DNS.ResolverConfig != "" {
		data, err := os.ReadFile(dep.K8sProviderCfg.DNS.ResolverConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to read resolver config file, detail-> %v", err)
		}
		kp.resolveConfigData = string(data)
	}

	backendPlugin := kubebackend.GetBackendPlugin(dep.K8sProviderCfg.Backend.Name)
	if backendPlugin == nil {
		return nil, fmt.Errorf("backend plugin %q not found", dep.K8sProviderCfg.Backend.Name)
	}
	kp.backendPlugin = backendPlugin

	if err := kp.backendPlugin.Init(&dep.K8sProviderCfg.Backend.Config); err != nil {
		return nil, fmt.Errorf("failed to init k8s backend %q, detail-> %v", dep.K8sProviderCfg.Backend.Name, err)
	}

	kp.kubeInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(
		dep.BkClient, time.Minute*5, kubeinformers.WithNamespace(dep.K8sProviderCfg.Namespace))
	kp.podLister = kp.kubeInformerFactory.Core().V1().Pods().Lister().Pods(dep.K8sProviderCfg.Namespace)

	podInformer := kp.kubeInformerFactory.Core().V1().Pods().Informer()
	kp.podsSynced = podInformer.HasSynced

	kp.configMapLister = kp.kubeInformerFactory.Core().V1().ConfigMaps().Lister().ConfigMaps(dep.K8sProviderCfg.Namespace)
	kp.configMapSynced = kp.kubeInformerFactory.Core().V1().ConfigMaps().Informer().HasSynced

	kp.secretLister = kp.kubeInformerFactory.Core().V1().Secrets().Lister().Secrets(dep.K8sProviderCfg.Namespace)
	kp.secretSynced = kp.kubeInformerFactory.Core().V1().Secrets().Informer().HasSynced

	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newObj interface{}) {
			kp.handleBackendPodChanged(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			kp.handleBackendPodChanged(obj)
		},
	})
	if err != nil {
		return nil, err
	}

	leaderElector := election.NewElector(
		kp.bkClient,
		kp.nodeName,
		election.WithNamespace(kp.bkNamespace),
		election.WithOnNewLeader(kp.onNewLeader),
		election.WithOnStartedLeading(kp.onStartedLeading),
		election.WithOnStoppedLeading(kp.onStoppedLeading))
	if leaderElector == nil {
		return nil, errors.New("failed to new leader elector")
	}
	kp.leaderElector = leaderElector

	return kp, nil
}

func (kp *K8sProvider) Start(ctx context.Context) error {
	nlog.Info("Starting k8s provider")

	go kp.kubeInformerFactory.Start(ctx.Done())

	nlog.Info("Waiting for informer cache to sync")
	if !cache.WaitForCacheSync(ctx.Done(), kp.podsSynced, kp.configMapSynced, kp.secretSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	kp.leaderElector.Run(ctx)

	<-ctx.Done()
	nlog.Info("K8s provider exited")

	return nil
}

func (kp *K8sProvider) Stop() {
	return
}

func (kp *K8sProvider) SyncPod(ctx context.Context, pod *v1.Pod, podStatus *pkgcontainer.PodStatus, reasonCache *kri.ReasonCache) (retErr error) {
	nlog.Infof("Sync pod %v", format.Pod(pod))

	defer func() {
		if retErr != nil {
			kp.recorder.Eventf(pod, v1.EventTypeWarning, events.FailedCreatePodSandBox, "Failed to sync pod to k8s: %v", retErr)
		}
	}()

	newPod := pod.DeepCopy()
	newPod.ObjectMeta = *kp.normalizeMeta(pod.UID, &pod.ObjectMeta)

	if _, err := kp.podLister.Get(pod.Name); err == nil {
		nlog.Infof("Pod %q is already exist, skipping", pod.Name)
		return nil
	}

	newPod.Spec.DNSPolicy = v1.DNSPolicy(kp.podDNSPolicy)
	newPod.Spec.DNSConfig = kp.podDNSConfig
	newPod.Spec.NodeName = ""
	newPod.Spec.NodeSelector = nil
	newPod.Spec.SchedulerName = ""

	if newPod.Spec.RuntimeClassName == nil && kp.runtimeClassName != "" {
		newPod.Spec.RuntimeClassName = &kp.runtimeClassName
	}

	configMaps := map[string]*v1.ConfigMap{}
	secrets := map[string]*v1.Secret{}
	for _, v := range newPod.Spec.Volumes {
		if v.ConfigMap != nil {
			cm, err := kp.resourceManager.GetConfigMap(v.ConfigMap.Name)
			if err != nil {
				return fmt.Errorf("failed to get configmap %s: %v", v.ConfigMap.Name, err)
			}
			configMaps[cm.Name] = cm
		}
		if v.Secret != nil {
			secret, err := kp.resourceManager.GetSecret(v.Secret.SecretName)
			if err != nil {
				return fmt.Errorf("failed to get secret %v: %v", v.Secret.SecretName, err)
			}
			secrets[secret.Name] = secret
		}
	}

	if kp.resolveConfigData != "" {
		resolveCM := kp.mountResolveConfig(newPod)
		configMaps[resolveCM.Name] = resolveCM
	}

	hookCtx := &hook.K8sProviderSyncPodContext{
		Pod:             pod,
		BkPod:           newPod,
		ResourceManager: kp.resourceManager,
		Configmaps:      []*v1.ConfigMap{},
		Secrets:         []*v1.Secret{},
	}
	if err := hook.Execute(hookCtx); err != nil {
		return fmt.Errorf("failed to execute hook for pod %v", format.Pod(pod))
	}

	for _, cm := range hookCtx.Configmaps {
		configMaps[cm.Name] = cm
	}
	for _, secret := range hookCtx.Secrets {
		secrets[secret.Name] = secret
	}

	for _, cm := range configMaps {
		newCM := cm.DeepCopy()
		newCM.ObjectMeta = *kp.normalizeMeta(pod.UID, &cm.ObjectMeta)
		normalizeSubResourceMeta(&newCM.ObjectMeta, newPod.Name)

		_, err := createSubResource[*v1.ConfigMap](ctx, newCM, kp.configMapLister, kp.bkClient.CoreV1().ConfigMaps(kp.bkNamespace))
		if err != nil {
			return fmt.Errorf("failed to create configmap %v, detail-> %v", newCM.Name, err)
		}
		for i, v := range newPod.Spec.Volumes {
			if v.ConfigMap != nil && v.ConfigMap.Name == cm.Name {
				newPod.Spec.Volumes[i].ConfigMap.Name = newCM.Name
			}
		}
	}

	for _, secret := range secrets {
		newSecret := secret.DeepCopy()
		newSecret.ObjectMeta = *kp.normalizeMeta(pod.UID, &secret.ObjectMeta)
		normalizeSubResourceMeta(&newSecret.ObjectMeta, newPod.Name)

		_, err := createSubResource[*v1.Secret](ctx, newSecret, kp.secretLister, kp.bkClient.CoreV1().Secrets(kp.bkNamespace))
		if err != nil {
			return fmt.Errorf("failed to create secret %v, detail-> %v", newSecret.Name, err)
		}
		for i, v := range newPod.Spec.Volumes {
			if v.Secret != nil && v.Secret.SecretName == secret.Name {
				newPod.Spec.Volumes[i].Secret.SecretName = newSecret.Name
			}
		}
	}

	for k, v := range kp.labelsToAdd {
		newPod.Labels[k] = v
	}
	for k, v := range kp.annotationsToAdd {
		newPod.Annotations[k] = v
	}

	kp.backendPlugin.PreSyncPod(newPod)

	_, err := kp.applyPod(ctx, newPod)
	if err != nil {
		return fmt.Errorf("failed to apply pod %v, detail-> %v", format.Pod(newPod), err)
	}

	return nil
}

func (kp *K8sProvider) normalizeMeta(sourcePodUID types.UID, meta *metav1.ObjectMeta) *metav1.ObjectMeta {
	newMeta := &metav1.ObjectMeta{
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
	for k, v := range meta.Labels {
		newMeta.Labels[k] = v
	}
	newMeta.Labels[common.LabelNodeNamespace] = kp.namespace
	newMeta.Labels[common.LabelNodeName] = kp.nodeName
	newMeta.Labels[common.LabelPodUID] = string(sourcePodUID)
	for k, v := range meta.Annotations {
		newMeta.Annotations[k] = v
	}
	newMeta.Namespace = kp.bkNamespace
	newMeta.Name = meta.Name

	return newMeta
}

func normalizeSubResourceMeta(meta *metav1.ObjectMeta, ownerPodName string) {
	name := fmt.Sprintf("%s-%s", ownerPodName, meta.Name)
	if len(name) > resourceNameLimit {
		hash := sha256.Sum256([]byte(name))
		name = fmt.Sprintf("%x", hash)

		if len(name) > resourceNameLimit {
			name = name[:resourceNameLimit]
		}
	}

	meta.Name = name
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	meta.Labels[labelOwnerPodName] = ownerPodName
}

func (kp *K8sProvider) mountResolveConfig(bkPod *v1.Pod) *v1.ConfigMap {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kp.namespace,
			Name:      "resolv-config",
		},
		Data: map[string]string{
			"resolv.conf": kp.resolveConfigData,
		},
	}

	bkPod.Spec.Volumes = append(bkPod.Spec.Volumes, v1.Volume{
		Name: "resolv-config",
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: cm.Name,
				},
			},
		},
	})

	for i := range bkPod.Spec.Containers {
		c := &bkPod.Spec.Containers[i]
		c.VolumeMounts = append(c.VolumeMounts, v1.VolumeMount{
			Name:      "resolv-config",
			MountPath: "/etc/resolv.conf",
			SubPath:   "resolv.conf",
		})
	}

	return cm
}

func (kp *K8sProvider) applyPod(ctx context.Context, pod *v1.Pod) (*v1.Pod, error) {
	newPod, err := kp.podLister.Get(pod.Name)
	if err == nil {
		nlog.Infof("Pod %v already exists, skip applying", format.Pod(pod))
		return newPod, nil
	} else if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get pod %v, detail-> %v", format.Pod(pod), err)
	}

	newPod, err = kp.bkClient.CoreV1().Pods(kp.bkNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod %v, detail-> %v", format.Pod(pod), err)
	}

	nlog.Infof("Create pod %v successfully", format.Pod(pod))

	return newPod, nil
}

func (kp *K8sProvider) KillPod(ctx context.Context, pod *v1.Pod, runningPod pkgcontainer.Pod, gracePeriodOverride *int64) error {
	if err := kp.deleteBackendPod(ctx, kp.bkNamespace, pod.Name, gracePeriodOverride); err != nil {
		return fmt.Errorf("failed to kill pod %q, detail-> %v", format.Pod(pod), err)
	}

	nlog.Infof("Pod %q killed successfully", format.Pod(pod))

	return nil
}

func (kp *K8sProvider) deleteBackendPod(ctx context.Context, namespace, name string, gracePeriodOverride *int64) error {
	err := kp.bkClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: gracePeriodOverride})
	if k8serrors.IsNotFound(err) {
		nlog.Infof("Backend pod %v/%v not found, skip killing", namespace, name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete backend pod %v/%v, detail-> %v", namespace, name, err)
	}

	return nil
}

func (kp *K8sProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	return nil
}

// CleanupPods deletes the subresource owned by cleaned pods.
func (kp *K8sProvider) CleanupPods(ctx context.Context, pods []*v1.Pod, runningPods []*pkgcontainer.Pod, possiblyRunningPods map[types.UID]sets.Empty) error {
	return nil
}

func (kp *K8sProvider) RefreshPodStatus(pod *v1.Pod, podStatus *v1.PodStatus) {
	bkPod, err := kp.podLister.Get(pod.Name)
	if k8serrors.IsNotFound(err) {
		nlog.Warnf("Pod %v not found, skip updating status", format.Pod(pod))
		return
	}
	if err != nil {
		nlog.Errorf("Failed to get pod %v: %v", format.Pod(pod), err)
		return
	}

	podStatus.ContainerStatuses = make([]v1.ContainerStatus, len(bkPod.Status.ContainerStatuses))
	podStatus.InitContainerStatuses = make([]v1.ContainerStatus, len(bkPod.Status.InitContainerStatuses))
	for i := range bkPod.Status.ContainerStatuses {
		podStatus.ContainerStatuses[i] = *bkPod.Status.ContainerStatuses[i].DeepCopy()
	}
	for i := range bkPod.Status.InitContainerStatuses {
		podStatus.InitContainerStatuses[i] = *bkPod.Status.InitContainerStatuses[i].DeepCopy()
	}
	copyPodCondition(&bkPod.Status, podStatus, v1.PodReady)
	copyPodCondition(&bkPod.Status, podStatus, v1.ContainersReady)

	nlog.Infof("Refresh pod %q status: %+v", format.Pod(pod), podStatus)
}

func copyPodCondition(srcPodStatus, dstPodStatus *v1.PodStatus, conditionType v1.PodConditionType) bool {
	for i := range srcPodStatus.Conditions {
		if srcPodStatus.Conditions[i].Type == conditionType {
			podutils.UpdateCondition(dstPodStatus, conditionType, srcPodStatus.Conditions[i])
			return true
		}
	}
	return false
}

func (kp *K8sProvider) GetPodStatus(ctx context.Context, pod *v1.Pod) (*pkgcontainer.PodStatus, error) {
	podStatus := &pkgcontainer.PodStatus{
		Namespace: kp.bkNamespace,
		Name:      pod.Name,
	}

	if pod.Labels != nil {
		podStatus.ID = types.UID(pod.Labels[common.LabelPodUID])
	}

	bkPod, err := kp.podLister.Get(pod.Name)
	if k8serrors.IsNotFound(err) {
		return podStatus, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %v: %v", format.Pod(pod), err)
	}

	bkPodStatus := bkPod.Status

	for _, ip := range bkPodStatus.PodIPs {
		podStatus.IPs = append(podStatus.IPs, ip.IP)
	}

	for _, bkcs := range bkPodStatus.ContainerStatuses {
		cs := &pkgcontainer.Status{
			ID:           pkgcontainer.CtrID{ID: bkcs.ContainerID},
			Name:         bkcs.Name,
			RestartCount: int(bkcs.RestartCount),
			Image:        bkcs.Image,
			ImageID:      bkcs.ImageID,
		}

		if bkcs.State.Running != nil {
			cs.State = pkgcontainer.ContainerStateRunning
			cs.StartedAt = bkcs.State.Running.StartedAt.Time
		} else if bkcs.State.Terminated != nil {
			cs.State = pkgcontainer.ContainerStateExited
			cs.StartedAt = bkcs.State.Terminated.StartedAt.Time
			cs.FinishedAt = bkcs.State.Terminated.FinishedAt.Time
			cs.ExitCode = int(bkcs.State.Terminated.ExitCode)
			cs.Reason = bkcs.State.Terminated.Reason
			cs.Message = bkcs.State.Terminated.Message
		} else {
			cs.State = pkgcontainer.ContainerStateCreated
		}

		podStatus.ContainerStatuses = append(podStatus.ContainerStatuses, cs)
	}

	return podStatus, nil
}

func (kp *K8sProvider) GetPods(ctx context.Context, all bool) ([]*pkgcontainer.Pod, error) {
	pods, err := kp.podLister.List(labels.SelectorFromSet(labels.Set{common.LabelNodeName: kp.nodeName}))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	var runningPods []*pkgcontainer.Pod
	for _, p := range pods {
		runningPod := &pkgcontainer.Pod{
			ID:        p.UID,
			Name:      p.Name,
			Namespace: kp.bkNamespace,
		}

		if p.Labels != nil {
			runningPod.ID = types.UID(p.Labels[common.LabelPodUID])
		}

		for _, c := range p.Spec.Containers {
			container := &pkgcontainer.Container{
				Name:  c.Name,
				Image: c.Image,
			}
			runningPod.Containers = append(runningPod.Containers, container)
		}

		runningPods = append(runningPods, runningPod)
	}

	return runningPods, nil
}

func (kp *K8sProvider) handleBackendPodChanged(obj interface{}) {
	var (
		object metav1.Object
		ok     bool
	)

	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			nlog.Errorf("Error decoding object, invalid type %T", obj)
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			nlog.Errorf("Error decoding object tombstone, invalid type %T", tombstone.Obj)
			return
		}
		nlog.Debugf("Recovered deleted object %q from tombstone", object.GetName())
	}

	podLabels := object.GetLabels()
	if podLabels == nil {
		nlog.Errorf("Pod %s has no labels, skipping", object.GetName())
		return
	}

	if podLabels[common.LabelNodeName] != kp.nodeName {
		nlog.Debugf("Pod %s is not running on this node (%v != %v), skipping", object.GetName(), podLabels[common.LabelNodeName], kp.nodeName)
		return
	}

	kp.podSyncHandler.HandlePodSyncByUID(types.UID(podLabels[common.LabelPodUID]))
}

type resourceLister[T metav1.Object] interface {
	Get(name string) (T, error)
}

type resourceStub[T metav1.Object] interface {
	Create(ctx context.Context, object T, opts metav1.CreateOptions) (T, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
}

func createSubResource[T metav1.Object](ctx context.Context, object T, lister resourceLister[T], stub resourceStub[T]) (T, error) {
	oldObject, err := lister.Get(object.GetName())
	if err == nil {
		nlog.Infof("Resource(%T) %v already exists, skip applying", object, object.GetName())
		return oldObject, nil
	} else if !k8serrors.IsNotFound(err) {
		return oldObject, fmt.Errorf("failed to get resource(%T) %v, detail-> %v", object, object.GetName(), err)
	}

	newObject, err := stub.Create(ctx, object, metav1.CreateOptions{})
	if err != nil {
		return newObject, fmt.Errorf("failed to create resource(%T) %v, detail-> %v", object, object.GetName(), err)
	}

	nlog.Infof("Create resource(%T) %v successfully", object, object.GetName())

	return newObject, nil
}

func cleanupSubResource[T metav1.Object](ctx context.Context, object T, podGetter corev1client.PodInterface, stub resourceStub[T]) error {
	if time.Since(object.GetCreationTimestamp().Time) < resourceMinLifeCycle {
		return nil
	}

	objLabels := object.GetLabels()
	if objLabels == nil {
		return nil
	}

	ownerPodName, ok := objLabels[labelOwnerPodName]
	if !ok {
		return nil
	}

	_, err := podGetter.Get(ctx, ownerPodName, metav1.GetOptions{})
	if err == nil {
		return nil
	} else if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get owner pod %s, detail-> %v", ownerPodName, err)
	}

	if err := stub.Delete(ctx, object.GetName(), metav1.DeleteOptions{}); err != nil {
		return err
	}

	nlog.Infof("Cleanup resource(%T) %v successfully", object, object.GetName())

	return nil
}
