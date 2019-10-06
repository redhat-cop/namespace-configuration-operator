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
	"strings"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func NewNamedInstanceGenericSharedIndexInformer(config *rest.Config, obj runtime.Unstructured, defaultEventHandlerResyncPeriod time.Duration) (cache.SharedIndexInformer, error) {

	lw, err := NewGenericListerWatcher(config, obj)
	if err != nil {
		return nil, err
	}
	return cache.NewSharedIndexInformer(lw, obj, defaultEventHandlerResyncPeriod, cache.Indexers{}), nil
}

type GenericListerWatcher struct {
	unstructured      unstructured.Unstructured
	resourceInterface dynamic.ResourceInterface
	name              string
}

func (glw *GenericListerWatcher) List(opts metav1.ListOptions) (runtime.Object, error) {
	log.Info("list function called")
	return glw.resourceInterface.List(opts)
}

func (glw *GenericListerWatcher) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	log.Info("watch function called")
	opts.Watch = true
	return glw.resourceInterface.Watch(opts)
}

func NewGenericListerWatcher(config *rest.Config, obj runtime.Unstructured) (*GenericListerWatcher, error) {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.New("unable to convert unstructured runtime object to unstructured object")
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	rvk, err := getGVR(unstructured, config)
	if err != nil {
		return nil, err
	}

	resourceInterface := client.Resource(rvk).Namespace(unstructured.GetNamespace())

	res, err := resourceInterface.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	log.Info("res", "res", res)

	return &GenericListerWatcher{
		resourceInterface: resourceInterface,
		name:              unstructured.GetName(),
		unstructured:      *unstructured.DeepCopy(),
	}, nil
}

func getGVR(obj *unstructured.Unstructured, config *rest.Config) (schema.GroupVersionResource, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	res := metav1.APIResource{}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	resList, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		log.Error(err, "unable to retrieve resouce list for:", "groupversion", gvk.GroupVersion().String())
		return schema.GroupVersionResource{}, err
	}
	for _, resource := range resList.APIResources {
		if resource.Kind == gvk.Kind && !strings.Contains(resource.Name, "/") {
			res = resource
			res.Group = gvk.Group
			res.Version = gvk.Version
			break
		}
	}
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: res.Name,
	}, nil
}
