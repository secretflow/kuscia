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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	kusciav1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	versioned "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	internalinterfaces "github.com/secretflow/kuscia/pkg/crd/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/secretflow/kuscia/pkg/crd/listers/kuscia/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// KusciaBetaTaskInformer provides access to a shared informer and lister for
// KusciaBetaTasks.
type KusciaBetaTaskInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.KusciaBetaTaskLister
}

type kusciaBetaTaskInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewKusciaBetaTaskInformer constructs a new informer for KusciaBetaTask type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewKusciaBetaTaskInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredKusciaBetaTaskInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredKusciaBetaTaskInformer constructs a new informer for KusciaBetaTask type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredKusciaBetaTaskInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KusciaV1alpha1().KusciaBetaTasks(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KusciaV1alpha1().KusciaBetaTasks(namespace).Watch(context.TODO(), options)
			},
		},
		&kusciav1alpha1.KusciaBetaTask{},
		resyncPeriod,
		indexers,
	)
}

func (f *kusciaBetaTaskInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredKusciaBetaTaskInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *kusciaBetaTaskInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kusciav1alpha1.KusciaBetaTask{}, f.defaultInformer)
}

func (f *kusciaBetaTaskInformer) Lister() v1alpha1.KusciaBetaTaskLister {
	return v1alpha1.NewKusciaBetaTaskLister(f.Informer().GetIndexer())
}
