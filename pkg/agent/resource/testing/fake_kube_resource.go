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

package testing

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/secretflow/kuscia/pkg/agent/resource"
)

// FakeResourceManager returns an instance of the resource manager that will return the specified
// objects when its "GetX" methods are called. Objects can be any valid Kubernetes object
// (corev1.ConfigMap, corev1.Secret, ...).
func FakeResourceManager(namespace string, objects ...runtime.Object) *resource.KubeResourceManager {
	// Create a fake Kubernetes client that will list the specified objects.
	kubeClient := fake.NewSimpleClientset(objects...)
	// Create a shared informer factory from where we can grab informers and listers for configmaps, secrets.
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 30*time.Second)
	// Grab informers for configmaps and secrets.
	mInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	sInformer := kubeInformerFactory.Core().V1().Secrets()
	pInformer := kubeInformerFactory.Core().V1().Pods()

	// Start all the required informers.
	go mInformer.Informer().Run(wait.NeverStop)
	go sInformer.Informer().Run(wait.NeverStop)
	go pInformer.Informer().Run(wait.NeverStop)
	// Wait for the caches to be synced.
	if !cache.WaitForCacheSync(wait.NeverStop, mInformer.Informer().HasSynced, sInformer.Informer().HasSynced, pInformer.Informer().HasSynced) {
		panic("failed to wait for caches to be synced")
	}
	// Create a new instance of the resource manager using the listers for configmaps and secrets.
	rm := resource.NewResourceManager(kubeClient, namespace, pInformer.Lister().Pods(namespace), sInformer.Lister().Secrets(namespace), mInformer.Lister().ConfigMaps(namespace))
	return rm
}
