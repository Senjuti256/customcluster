/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	sdedevv1alpha1 "github.com/Senjuti256/customcluster/pkg/apis/sde.dev/v1alpha1"
	versioned "github.com/Senjuti256/customcluster/pkg/client/clientset/versioned"
	internalinterfaces "github.com/Senjuti256/customcluster/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/Senjuti256/customcluster/pkg/client/listers/sde.dev/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CustomclusterInformer provides access to a shared informer and lister for
// Customclusters.
type CustomclusterInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.CustomclusterLister
}

type customclusterInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCustomclusterInformer constructs a new informer for Customcluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCustomclusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCustomclusterInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredCustomclusterInformer constructs a new informer for Customcluster type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCustomclusterInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SamplecontrollerV1alpha1().Customclusters(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SamplecontrollerV1alpha1().Customclusters(namespace).Watch(context.TODO(), options)
			},
		},
		&sdedevv1alpha1.Customcluster{},
		resyncPeriod,
		indexers,
	)
}

func (f *customclusterInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCustomclusterInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *customclusterInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&sdedevv1alpha1.Customcluster{}, f.defaultInformer)
}

func (f *customclusterInformer) Lister() v1alpha1.CustomclusterLister {
	return v1alpha1.NewCustomclusterLister(f.Informer().GetIndexer())
}