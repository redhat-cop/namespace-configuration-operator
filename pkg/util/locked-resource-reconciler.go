package util

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/redhat-cop/operator-utils/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

//workaround to weird secret behavior
var serviceAccountGVK = schema.GroupVersionKind{
	Version: "v1",
	Kind:    "ServiceAccount",
}
var secretGVK = schema.GroupVersionKind{
	Version: "v1",
	Kind:    "Secret",
}
var configmapGVK = schema.GroupVersionKind{
	Version: "v1",
	Kind:    "ConfigMap",
}
var roleGVK = schema.GroupVersionKind{
	Version: "v1",
	Group:   "rbac.authorization.k8s.io",
	Kind:    "Role",
}
var roleBindingGVK = schema.GroupVersionKind{
	Version: "v1",
	Group:   "rbac.authorization.k8s.io",
	Kind:    "RoleBinding",
}
var openShiftTemplateGVK = schema.GroupVersionKind{
	Version: "v1",
	Group:   "template.openshift.io",
	Kind:    "Template",
}

type LockedObjectReconciler struct {
	Object unstructured.Unstructured
	util.ReconcilerBase
}

// NewReconciler returns a new reconcile.Reconciler
func NewLockedObjectReconciler(mgr manager.Manager, object unstructured.Unstructured) (reconcile.Reconciler, error) {
	value, ok := object.UnstructuredContent()["spec"]
	log.Info("NewLockedObjectReconciler called on", "type", object.GetObjectKind().GroupVersionKind().String(), "result", ok, "value", value)

	if !ok {
		if !reflect.DeepEqual(object.GetObjectKind().GroupVersionKind(), secretGVK) &&
			!reflect.DeepEqual(object.GetObjectKind().GroupVersionKind(), configmapGVK) &&
			!reflect.DeepEqual(object.GetObjectKind().GroupVersionKind(), roleGVK) &&
			!reflect.DeepEqual(object.GetObjectKind().GroupVersionKind(), roleBindingGVK) &&
			!reflect.DeepEqual(object.GetObjectKind().GroupVersionKind(), serviceAccountGVK) &&
			!reflect.DeepEqual(object.GetObjectKind().GroupVersionKind(), openShiftTemplateGVK) {
			err := errors.New("non-standard resources (without the spec field) are not supported")
			log.Error(err, "non-standard resources (without the spec field) are not supported", "type", object.GetObjectKind().GroupVersionKind())
			return &LockedObjectReconciler{}, err
		}
	}

	reconciler := &LockedObjectReconciler{
		ReconcilerBase: util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor("controller_locked_object_"+GetKeyFromObject(&object))),
		Object:         object,
	}

	controller, err := controller.New("controller_locked_object_"+GetKeyFromObject(&object), mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return &LockedObjectReconciler{}, err
	}

	gvk := object.GetObjectKind().GroupVersionKind()
	groupVersion := schema.GroupVersion{Group: gvk.Group, Version: gvk.Version}

	mgr.GetScheme().AddKnownTypes(groupVersion, &object)

	err = controller.Watch(&source.Kind{Type: &object}, &handler.EnqueueRequestForObject{}, &resourceModifiedPredicate{
		name:      object.GetName(),
		namespace: object.GetNamespace(),
	})
	if err != nil {
		return &LockedObjectReconciler{}, err
	}

	return reconciler, nil
}

func (lor *LockedObjectReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info("reconcile called for", "object", GetKeyFromObject(&lor.Object), "request", request)
	//err := lor.CreateOrUpdateResource(nil, "", &lor.Object)

	// Fetch the  instance
	//instance := &unstructured.Unstructured{}
	client, err := lor.GetDynamicClientOnUnstructured(lor.Object)
	if err != nil {
		return reconcile.Result{}, err
	}
	instance, err := client.Get(lor.Object.GetName(), v1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			// if not found we have to recreate it.
			err = lor.CreateOrUpdateResource(nil, "", &lor.Object)
			if err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	_, ok := instance.UnstructuredContent()["spec"]
	if !ok {
		if reflect.DeepEqual(instance.GetObjectKind().GroupVersionKind(), secretGVK) {
			return lor.secretSpecialHandling(instance)
		}
		if reflect.DeepEqual(instance.GetObjectKind().GroupVersionKind(), configmapGVK) {
			return lor.configmapSpecialHandling(instance)
		}
		if reflect.DeepEqual(instance.GetObjectKind().GroupVersionKind(), roleGVK) {
			return lor.roleSpecialHandling(instance)
		}
		if reflect.DeepEqual(instance.GetObjectKind().GroupVersionKind(), roleBindingGVK) {
			return lor.roleBindingSpecialHandling(instance)
		}
		if reflect.DeepEqual(instance.GetObjectKind().GroupVersionKind(), serviceAccountGVK) {
			return lor.serviceAccountSpecialHandling(instance)
		}
		if reflect.DeepEqual(instance.GetObjectKind().GroupVersionKind(), openShiftTemplateGVK) {
			return lor.openShiftTemplateSpecialHandling(instance)
		}

		return reconcile.Result{}, errors.New("non-standard resources (without the spec field) are not supported")
	}

	if !reflect.DeepEqual(instance.UnstructuredContent()["spec"], lor.Object.UnstructuredContent()["spec"]) {
		instance.UnstructuredContent()["spec"] = lor.Object.UnstructuredContent()["spec"]
		err = lor.CreateOrUpdateResource(nil, "", instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func GetKeyFromObject(object *unstructured.Unstructured) string {
	return object.GroupVersionKind().String() + "/" + object.GetNamespace() + "/" + object.GetName()
}

type resourceModifiedPredicate struct {
	name      string
	namespace string
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating resource version change
func (p *resourceModifiedPredicate) Update(e event.UpdateEvent) bool {
	//log.Info("update called for", "event", e)
	if e.MetaNew.GetNamespace() == p.namespace && e.MetaNew.GetName() == p.name {
		// log.Info("type", "type", reflect.TypeOf(e.ObjectNew).String())
		// old, ok := e.ObjectOld.(*unstructured.Unstructured)
		// if !ok {
		// 	return false
		// }
		// new, ok := e.ObjectNew.(*unstructured.Unstructured)
		// //TODO add special handling for object that don't follow the spec convention
		// spec_old := old.UnstructuredContent()["spec"]
		// spec_new := new.UnstructuredContent()["spec"]
		// return !reflect.DeepEqual(spec_old, spec_new)
		return true
	}
	return false
}

func (p *resourceModifiedPredicate) Create(e event.CreateEvent) bool {
	//log.Info("create called for", "event", e)
	//log.Info("type", "type", reflect.TypeOf(e.Object).String())
	return false
}

func (p *resourceModifiedPredicate) Delete(e event.DeleteEvent) bool {
	//log.Info("delete called for", "event", e)
	if e.Meta.GetNamespace() == p.namespace && e.Meta.GetName() == p.name {
		return true
	}
	return false
}

func (lor *LockedObjectReconciler) secretSpecialHandling(instance *unstructured.Unstructured) (reconcile.Result, error) {
	tobeupdated := false
	if !reflect.DeepEqual(instance.UnstructuredContent()["data"], lor.Object.UnstructuredContent()["data"]) {
		instance.UnstructuredContent()["data"] = lor.Object.UnstructuredContent()["data"]
		tobeupdated = true
	}
	if !reflect.DeepEqual(instance.UnstructuredContent()["type"], lor.Object.UnstructuredContent()["type"]) {
		instance.UnstructuredContent()["type"] = lor.Object.UnstructuredContent()["type"]
		tobeupdated = true
	}
	if tobeupdated {
		err := lor.CreateOrUpdateResource(nil, "", instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (lor *LockedObjectReconciler) configmapSpecialHandling(instance *unstructured.Unstructured) (reconcile.Result, error) {
	if !reflect.DeepEqual(instance.UnstructuredContent()["data"], lor.Object.UnstructuredContent()["data"]) {
		instance.UnstructuredContent()["data"] = lor.Object.UnstructuredContent()["data"]
		err := lor.CreateOrUpdateResource(nil, "", instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (lor *LockedObjectReconciler) roleSpecialHandling(instance *unstructured.Unstructured) (reconcile.Result, error) {
	tobeupdated := false
	if !reflect.DeepEqual(instance.UnstructuredContent()["rules"], lor.Object.UnstructuredContent()["rules"]) {
		instance.UnstructuredContent()["rules"] = lor.Object.UnstructuredContent()["rules"]
		tobeupdated = true
	}
	if !reflect.DeepEqual(instance.UnstructuredContent()["aggregationRule"], lor.Object.UnstructuredContent()["aggregationRule"]) {
		instance.UnstructuredContent()["aggregationRule"] = lor.Object.UnstructuredContent()["aggregationRule"]
		tobeupdated = true
	}
	if tobeupdated {
		err := lor.CreateOrUpdateResource(nil, "", instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (lor *LockedObjectReconciler) roleBindingSpecialHandling(instance *unstructured.Unstructured) (reconcile.Result, error) {
	tobeupdated := false
	// if reflect.DeepEqual(instance.UnstructuredContent()["roleref"], lor.Object.UnstructuredContent()["roleref"]) {
	// 	instance.UnstructuredContent()["roleref"] = lor.Object.UnstructuredContent()["roleref"]
	// 	tobeupdated = true
	// }
	if !reflect.DeepEqual(instance.UnstructuredContent()["subjects"], lor.Object.UnstructuredContent()["subjects"]) {
		instance.UnstructuredContent()["subjects"] = lor.Object.UnstructuredContent()["subjects"]
		tobeupdated = true
	}
	if tobeupdated {
		err := lor.CreateOrUpdateResource(nil, "", instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (lor *LockedObjectReconciler) openShiftTemplateSpecialHandling(instance *unstructured.Unstructured) (reconcile.Result, error) {
	log.Info("Change Detected to Template Resource")
	tobeupdated := false
	if !reflect.DeepEqual(instance.UnstructuredContent()["objects"], lor.Object.UnstructuredContent()["objects"]) {
		instance.UnstructuredContent()["objects"] = lor.Object.UnstructuredContent()["objects"]
		tobeupdated = true
	}
	if !reflect.DeepEqual(instance.UnstructuredContent()["parameters"], lor.Object.UnstructuredContent()["parameters"]) {
		instance.UnstructuredContent()["parameters"] = lor.Object.UnstructuredContent()["parameters"]
		tobeupdated = true
	}
	if !reflect.DeepEqual(instance.GetAnnotations(), lor.Object.GetAnnotations()) {
		instance.SetAnnotations(lor.Object.GetAnnotations())
		tobeupdated = true
	}
	if tobeupdated {
		err := lor.CreateOrUpdateResource(nil, "", instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (lor *LockedObjectReconciler) serviceAccountSpecialHandling(instance *unstructured.Unstructured) (reconcile.Result, error) {
	//service accounts are essentially read only
	return reconcile.Result{}, nil
}
