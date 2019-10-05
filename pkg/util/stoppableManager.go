package util

import (
	"time"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_locked_object")

type StoppableManager struct {
	manager.Manager
	stopChannel chan struct{}
}

func (sm *StoppableManager) Stop() {
	close(sm.stopChannel)
}

func (sm *StoppableManager) Start() {
	go sm.Manager.Start(sm.stopChannel)
}

func NewStoppableManager(parentManager manager.Manager) (StoppableManager, error) {
	manager, err := manager.New(parentManager.GetConfig(), manager.Options{})
	if err != nil {
		return StoppableManager{}, err
	}
	return StoppableManager{
		Manager:     manager,
		stopChannel: make(chan struct{}),
	}, nil
}

type LockedObjectReconciler struct {
	Object unstructured.Unstructured
	util.ReconcilerBase
}

// NewReconciler returns a new reconcile.Reconciler
func NewLockedObjectReconciler(mgr manager.Manager, object unstructured.Unstructured) (reconcile.Reconciler, error) {
	reconciler := &LockedObjectReconciler{
		ReconcilerBase: util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetRecorder("controller_locked_object_"+GetKeyFromObject(&object))),
		Object:         object,
	}

	controller, err := controller.New("controller_locked_object_"+GetKeyFromObject(&object), mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return &LockedObjectReconciler{}, err
	}
	// Watch for changes to the object
	informer, err := NewNamedInstanceGenericSharedIndexInformer(mgr.GetConfig(), &object, time.Minute)
	if err != nil {
		return &LockedObjectReconciler{}, err
	}
	err = controller.Watch(&source.Informer{Informer: informer},
		&handler.EnqueueRequestForObject{},
		resourceModifiedPredicate{})
	if err != nil {
		return &LockedObjectReconciler{}, err
	}

	return reconciler, nil
}

func (lor *LockedObjectReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info("reconcile called for", "object", GetKeyFromObject(&lor.Object))
	err := lor.CreateOrUpdateResource(nil, "", &lor.Object)
	return reconcile.Result{}, err
}

func GetKeyFromObject(object *unstructured.Unstructured) string {
	return object.GroupVersionKind().String() + "/" + object.GetNamespace() + "/" + object.GetName()
}

type resourceModifiedPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating resource version change
func (resourceModifiedPredicate) Update(e event.UpdateEvent) bool {
	log.Info("update called for", "event", e)
	if e.MetaOld == nil {
		log.Error(nil, "UpdateEvent has no old metadata", "event", e)
		return false
	}
	if e.ObjectOld == nil {
		log.Error(nil, "UpdateEvent has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		log.Error(nil, "UpdateEvent has no new runtime object for update", "event", e)
		return false
	}
	if e.MetaNew == nil {
		log.Error(nil, "UpdateEvent has no new metadata", "event", e)
		return false
	}
	return true
}

func (resourceModifiedPredicate) Create(e event.CreateEvent) bool {
	log.Info("create called for", "event", e)
	return false
}

func (resourceModifiedPredicate) Delete(e event.DeleteEvent) bool {
	log.Info("delete called for", "event", e)
	return true
}
