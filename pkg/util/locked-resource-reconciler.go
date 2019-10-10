package util

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/redhat-cop/operator-utils/pkg/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

// NewReconciler returns a new reconcile.Reconciler
func NewLockedObjectReconciler(mgr manager.Manager, object unstructured.Unstructured) (reconcile.Reconciler, error) {
	_, ok := object.UnstructuredContent()["spec"]
	if !ok {
		switch object.GetObjectKind().GroupVersionKind() {
		case (&corev1.Secret{}).GetObjectKind().GroupVersionKind():
			break
		case (&corev1.ConfigMap{}).GetObjectKind().GroupVersionKind():
			break
		case (&rbacv1.RoleBinding{}).GetObjectKind().GroupVersionKind():
			break
		case (&rbacv1.Role{}).GetObjectKind().GroupVersionKind():
			break
		case (&rbacv1.ClusterRoleBinding{}).GetObjectKind().GroupVersionKind():
			break
		case (&rbacv1.ClusterRole{}).GetObjectKind().GroupVersionKind():
			break
		default:
			return &LockedObjectReconciler{}, errors.New("non-standard resources (without the spec field) are not supported")
		}
	}

	reconciler := &LockedObjectReconciler{
		ReconcilerBase: util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetRecorder("controller_locked_object_"+GetKeyFromObject(&object))),
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
		switch instance.GetObjectKind().GroupVersionKind() {
		case (&corev1.Secret{}).GetObjectKind().GroupVersionKind():
			return lor.secretSpecialHandling(instance)
		case (&corev1.ConfigMap{}).GetObjectKind().GroupVersionKind():
			return lor.configmapSpecialHandling(instance)
		case (&rbacv1.RoleBinding{}).GetObjectKind().GroupVersionKind():
			return lor.roleBindingSpecialHandling(instance)
		case (&rbacv1.Role{}).GetObjectKind().GroupVersionKind():
			return lor.roleSpecialHandling(instance)
		case (&rbacv1.ClusterRoleBinding{}).GetObjectKind().GroupVersionKind():
			return lor.roleBindingSpecialHandling(instance)
		case (&rbacv1.ClusterRole{}).GetObjectKind().GroupVersionKind():
			return lor.roleSpecialHandling(instance)
		default:
			return reconcile.Result{}, errors.New("non-standard resources (without the spec field) are not supported")
		}
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
	log.Info("update called for", "event", e)
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
	log.Info("create called for", "event", e)
	//log.Info("type", "type", reflect.TypeOf(e.Object).String())
	return false
}

func (p *resourceModifiedPredicate) Delete(e event.DeleteEvent) bool {
	log.Info("delete called for", "event", e)
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
	if reflect.DeepEqual(instance.UnstructuredContent()["roleref"], lor.Object.UnstructuredContent()["roleref"]) {
		instance.UnstructuredContent()["roleref"] = lor.Object.UnstructuredContent()["roleref"]
		tobeupdated = true
	}
	if reflect.DeepEqual(instance.UnstructuredContent()["subjects"], lor.Object.UnstructuredContent()["subjects"]) {
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
