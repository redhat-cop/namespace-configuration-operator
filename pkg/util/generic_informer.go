/*
Copyright 2019 The Kubernetes Authors.

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

package util

import (
	"time"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/kubefed/pkg/client/generic/scheme"
)

func NewNamedInstanceGenericSharedIndexInformer(config *rest.Config, obj runtime.Object, defaultEventHandlerResyncPeriod time.Duration) (toolscache.SharedIndexInformer, error) {
	metaobj, ok := obj.(metav1.Object)
	if !ok {
		return nil, errors.New("unable to convert runtime object to meta object")
	}
	namespace := metaobj.GetNamespace()
	//name := metaobj.GetName()
	gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
	if err != nil {
		return nil, err
	}

	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		return nil, errors.Wrap(err, "Could not create RESTMapper from config")
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	client, err := apiutil.RESTClientForGVK(gvk, config, scheme.Codecs)
	if err != nil {
		return nil, err
	}

	// listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
	// listObj, err := scheme.Scheme.New(listGVK)
	// if err != nil {
	// 	return nil, err
	// }
	lw := cache.NewListWatchFromClient(client, mapping.Resource.Resource, namespace, nil)
	// lw := cache.ListWatch{
	// 	ListFunc: func(opts metav1.ListOptions) (pkgruntime.Object, error) {
	// 		log.Info("list function called")
	// 		res := listObj.DeepCopyObject()
	// 		isNamespaceScoped := namespace != "" && mapping.Scope.Name() != meta.RESTScopeNameRoot
	// 		err := client.Get().Name(name).NamespaceIfScoped(namespace, isNamespaceScoped).Resource(mapping.Resource.Resource).VersionedParams(&opts, scheme.ParameterCodec).Do().Into(res)
	// 		return res, err
	// 	},
	// 	WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
	// 		// Watch needs to be set to true separately
	// 		log.Info("list function called")
	// 		opts.Watch = true
	// 		isNamespaceScoped := namespace != "" && mapping.Scope.Name() != meta.RESTScopeNameRoot
	// 		return client.Get().Name(name).NamespaceIfScoped(namespace, isNamespaceScoped).Resource(mapping.Resource.Resource).VersionedParams(&opts, scheme.ParameterCodec).Watch()
	// 	},
	// }
	return toolscache.NewSharedIndexInformer(lw, obj, defaultEventHandlerResyncPeriod, toolscache.Indexers{}), nil
}
