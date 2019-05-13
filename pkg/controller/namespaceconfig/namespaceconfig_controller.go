package namespaceconfig

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	multierror "github.com/hashicorp/go-multierror"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
)

const operatorLabel = "namespace-config-operator.redhat-cop.io_owner"
const finalizer = "namespace-config-operator"

var log = logf.Log.WithName("controller_namespaceconfig")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new NamespaceConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {

	return &ReconcileNamespaceConfig{
		ReconcilerBase:  util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme()),
		DiscoveryClient: *discovery.NewDiscoveryClientForConfigOrDie(mgr.GetConfig()),
		Config:          *mgr.GetConfig(),
		recorder:        mgr.GetRecorder("namespaceconfiguration-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespaceconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource NamespaceConfig
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.NamespaceConfig{}}, &handler.EnqueueRequestForObject{}, resourceGenerationOrFinalizerChangedPredicate{})
	if err != nil {
		return err
	}

	namespaceToNamespaceConfig := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			res := []reconcile.Request{}
			ns := a.Object.(*corev1.Namespace)
			client := mgr.GetClient()
			ncl, err := findApplicableNameSpaceConfigs(*ns, &client)
			if err != nil {
				log.Error(err, "unable to find applicable NamespaceConfig for namespace", "namespace", ns.Name)
				return []reconcile.Request{}
			}
			for _, namespaceconfig := range ncl {
				res = append(res, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      namespaceconfig.GetName(),
						Namespace: namespaceconfig.GetNamespace(),
					},
				})
			}
			return res
		})

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner NamespaceConfig
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: namespaceToNamespaceConfig,
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileNamespaceConfig{}

// ReconcileNamespaceConfig reconciles a NamespaceConfig object
type ReconcileNamespaceConfig struct {
	util.ReconcilerBase
	discovery.DiscoveryClient
	rest.Config
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a NamespaceConfig object and makes changes based on the state read
// and what is in the NamespaceConfig.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNamespaceConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NamespaceConfig")

	// Fetch the NamespaceConfig instance
	instance := &redhatcopv1alpha1.NamespaceConfig{}
	err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// if isNotValid(instance) {
	// 	//record event
	// 	//update status
	// 	//return with time
	// }

	// if !isInitliazed(instance) {
	// 	err := r.GetClient().Update(context.TODO(), instance)
	// 	if err != nil {
	// 		log.Error(err, "unable to update instance", "instance", instance)
	// 		return reconcile.Result{}, err
	// 	}
	// 	return reconcile.Result{}, nil
	// }

	//namespaces selected by this instance
	namespaces, err := r.getSelectedNamespaces(instance)
	if err != nil {
		log.Error(err, "unable to retrieve the list of selected namespaces", "selector", instance.Spec.Selector)
		return r.manageError(err, instance)
	}
	ownerLabelValue := instance.GetNamespace() + "-" + instance.GetName()
	ownerSelector := operatorLabel + "=" + ownerLabelValue
	// object defined by this instance
	objects, err := getObjects(instance)
	if util.IsBeingDeleted(instance) {
		if !util.HasFinalizer(instance, finalizer) {
			return reconcile.Result{}, nil
		}
		resources, _, err := r.getKnowTypes()
		if err == nil {
			//log.Info("known types", "known types", resources)
			labeledObjects := r.findAllLabeledObjects(resources, ownerSelector)
			//log.Info("deleting labeled objects", "objs", labeledObjects)
			for _, obj := range labeledObjects {
				r.deleteObject(obj)
			}
			util.RemoveFinalizer(instance, finalizer)
			err := r.GetClient().Update(context.TODO(), instance)
			if err != nil {
				log.Error(err, "unable to update instance", "instance", instance)
				return r.manageError(err, instance)
			}
			return reconcile.Result{}, nil
		}
	}
	if !util.HasFinalizer(instance, finalizer) {
		util.AddFinalizer(instance, finalizer)
		err := r.GetClient().Update(context.TODO(), instance)
		if err != nil {
			log.Error(err, "unable to update instance", "instance", instance)
			return r.manageError(err, instance)
		}
		return reconcile.Result{}, nil
	}
	// know types in this cluster
	resources, _, err := r.getKnowTypes()
	if err == nil {
		// object owned by this instance (will include legit and illegit ones)
		labeledObjects := r.findAllLabeledObjects(resources, ownerSelector)
		r.deleteObjectsOnControlledNamespaces(labeledObjects, objects, namespaces)
		r.deleteObjectsOnUncontrolledNamespaces(labeledObjects, namespaces)
	} else {
		log.Error(err, "unable to retrive known types, ignoring delete phase ...")
	}
	var err1 *multierror.Error
	for _, ns := range namespaces {
		err := r.applyConfigToNamespace(objects, ns, ownerLabelValue)
		if err != nil {
			err1 = multierror.Append(err1, err)
		}
	}
	if err1.ErrorOrNil() != nil {
		return r.manageError(err1.ErrorOrNil(), instance)
	}
	return r.manageSuccess(instance)
}

func getObjects(namespaceconfig *redhatcopv1alpha1.NamespaceConfig) ([]unstructured.Unstructured, error) {
	objs := []unstructured.Unstructured{}
	for _, raw := range namespaceconfig.Spec.Resources {
		bb, err := yaml.YAMLToJSON(raw.Raw)
		if err != nil {
			log.Error(err, "Error transforming yaml to json", "raw", raw.Raw)
			return []unstructured.Unstructured{}, err
		}
		obj := unstructured.Unstructured{}
		err = json.Unmarshal(bb, &obj)
		if err != nil {
			log.Error(err, "Error unmarshalling json manifest", "manifest", string(bb))
			return []unstructured.Unstructured{}, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}

func (r *ReconcileNamespaceConfig) getSelectedNamespaces(namespaceconfig *redhatcopv1alpha1.NamespaceConfig) ([]corev1.Namespace, error) {
	nl := corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.Selector)
	if err != nil {
		log.Error(err, "unable to create selector from label selector", "selector", &namespaceconfig.Spec.Selector)
		return []corev1.Namespace{}, err
	}
	err = r.GetClient().List(context.TODO(), &client.ListOptions{LabelSelector: selector}, &nl)
	if err != nil {
		log.Error(err, "unable to list namespaces with selector", "selector", selector)
		return []corev1.Namespace{}, err
	}
	return nl.Items, nil
}

func (r *ReconcileNamespaceConfig) applyConfigToNamespace(objs []unstructured.Unstructured, namespace corev1.Namespace, label string) error {
	for _, obj := range objs {
		objIntf, err := r.getDynamicClientOnUnstructured(obj)
		if err != nil {
			log.Error(err, "unable to get dynamic client on object", "object", obj)
			return err
		}
		namespacedObjIntf := objIntf.Namespace(namespace.GetName())

		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[operatorLabel] = label
		obj.SetLabels(labels)
		err = createOrUpdate(&namespacedObjIntf, &obj)
		if err != nil {
			return err
		}
	}
	return nil
}

func createOrUpdate(client *dynamic.ResourceInterface, obj *unstructured.Unstructured) error {

	obj2, err := (*client).Get(obj.GetName(), metav1.GetOptions{})

	if apierrors.IsNotFound(err) {
		_, err = (*client).Create(obj, metav1.CreateOptions{})
		if err != nil {
			log.Error(err, "unable to create object", "object", obj)
		}
		return err
	}
	if err == nil {
		obj.SetResourceVersion(obj2.GetResourceVersion())
		_, err = (*client).Update(obj, metav1.UpdateOptions{})
		if err != nil {
			log.Error(err, "unable to update object", "object", obj)
		}
		return err

	}
	log.Error(err, "unable to lookup object", "object", obj)
	return err

}

func findApplicableNameSpaceConfigs(namespace corev1.Namespace, c *client.Client) ([]redhatcopv1alpha1.NamespaceConfig, error) {
	//find all the namespaceconfig
	result := []redhatcopv1alpha1.NamespaceConfig{}
	ncl := redhatcopv1alpha1.NamespaceConfigList{}
	err := (*c).List(context.TODO(), &client.ListOptions{}, &ncl)
	if err != nil {
		log.Error(err, "unable to retrieve the list of namespace configs")
		return []redhatcopv1alpha1.NamespaceConfig{}, err
	}
	//for each namespaceconfig see if it selects the namespace
	for _, nc := range ncl.Items {
		selector, err := metav1.LabelSelectorAsSelector(&nc.Spec.Selector)
		if err != nil {
			log.Error(err, "unable to create selector from label selector", "selector", &nc.Spec.Selector)
			return []redhatcopv1alpha1.NamespaceConfig{}, err
		}
		if selector.Matches(labels.Set(namespace.Labels)) {
			result = append(result, nc)
		}
	}
	return result, nil
}

func (r *ReconcileNamespaceConfig) deleteObjectsOnControlledNamespaces(existingObjects []unstructured.Unstructured, requestedObjects []unstructured.Unstructured, namespaces []corev1.Namespace) {
	for _, ns := range namespaces {
		objInNamespace := getObjectsInNamespace(existingObjects, ns)
		toBeDeletedObjects := leftOuterJoin(objInNamespace, requestedObjects)
		for _, obj := range toBeDeletedObjects {
			r.deleteObject(obj)
		}
	}
}

func leftOuterJoin(left []unstructured.Unstructured, right []unstructured.Unstructured) []unstructured.Unstructured {
	res := []unstructured.Unstructured{}
	for _, leftObj := range left {
		for _, rightObj := range right {
			if sameObj(leftObj, rightObj) {
				res = append(res, leftObj)
			}
		}
	}
	return res
}

func sameObj(left unstructured.Unstructured, right unstructured.Unstructured) bool {
	return left.GetName() == right.GetName() &&
		left.GetNamespace() == right.GetNamespace() &&
		left.GetObjectKind().GroupVersionKind().GroupKind() == right.GetObjectKind().GroupVersionKind().GroupKind()
}

func getObjectsInNamespace(existingObjects []unstructured.Unstructured, namespace corev1.Namespace) []unstructured.Unstructured {
	res := []unstructured.Unstructured{}
	for _, obj := range existingObjects {
		if obj.GetNamespace() == namespace.Name {
			res = append(res, obj)
		}
	}
	return res
}

func (r *ReconcileNamespaceConfig) deleteObject(obj unstructured.Unstructured) {
	objIntf, err := r.getDynamicClientOnUnstructured(obj)
	if err != nil {
		log.Error(err, "unable to get dynamic client on obj, ignoring...", "obj", obj)
		return
	}
	namespacedObjIntf := objIntf.Namespace(obj.GetNamespace())
	err = namespacedObjIntf.Delete(obj.GetName(), &metav1.DeleteOptions{})
	if err != nil {
		log.Error(err, "unable to delete obj, ignoring...", "obj", obj)
		return
	}
}

func (r *ReconcileNamespaceConfig) deleteObjectsOnUncontrolledNamespaces(objects []unstructured.Unstructured, namespaces []corev1.Namespace) {
	for _, obj := range objects {
		if !isNamespaceInSet(namespaces, obj.GetNamespace()) {
			r.deleteObject(obj)
		}
	}
}

func isNamespaceInSet(namespaces []corev1.Namespace, namespace string) bool {
	for _, ns := range namespaces {
		if ns.Name == namespace {
			return true
		}
	}
	return false
}

func (r *ReconcileNamespaceConfig) findAllLabeledObjects(resources []metav1.APIResource, selector string) []unstructured.Unstructured {
	objs := []unstructured.Unstructured{}
	for _, res := range resources {
		resIntf, err := r.getDynamicClientOnType(res)
		if err != nil {
			log.Error(err, "unable to get dynamic client on type, ignoring...", "resource", res)
			continue
		}
		//log.Info("listing", "reource", res, "label selector", selector)
		unstructs, err := resIntf.List(metav1.ListOptions{
			LabelSelector: selector,
		})
		//log.Info("found", "objs", unstructs)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "unable to list resources ignoring...", "resource", res)
			}
			continue
		}
		objs = append(objs, unstructs.Items...)
	}
	return objs
}

func (r *ReconcileNamespaceConfig) getKnowTypes() (namespaced []metav1.APIResource, clusterLevel []metav1.APIResource, err error) {
	resListArray, err := r.DiscoveryClient.ServerPreferredResources()
	if err != nil {
		log.Error(err, "unable server preferred resources")
		return namespaced, clusterLevel, err
	}
	for _, resList := range resListArray {
		gv, err := schema.ParseGroupVersion(resList.GroupVersion)
		if err != nil {
			log.Error(err, "unable to parse groupversion from, ignoring...", "groupversion", resList.GroupVersion)
			continue
		}
		for _, res := range resList.APIResources {
			if strings.Contains(res.Name, "/") {
				//ignoring subresources
				continue
			}
			res.Group = gv.Group
			res.Version = gv.Version
			if res.Namespaced {
				namespaced = append(namespaced, res)
			} else {
				clusterLevel = append(clusterLevel, res)
			}
		}
	}
	return namespaced, clusterLevel, nil
}

func (r *ReconcileNamespaceConfig) getDynamicClientOnType(resource metav1.APIResource) (dynamic.NamespaceableResourceInterface, error) {
	return r.getDynamicClientOnGVR(schema.GroupVersionResource{
		Group:    resource.Group,
		Version:  resource.Version,
		Resource: resource.Name,
	})
}

func (r *ReconcileNamespaceConfig) getDynamicClientOnGVR(gkv schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
	intf, err := dynamic.NewForConfig(&r.Config)
	if err != nil {
		log.Error(err, "unable to get dynamic client")
		return nil, err
	}
	res := intf.Resource(gkv)
	return res, nil
}

func (r *ReconcileNamespaceConfig) getDynamicClientOnUnstructured(obj unstructured.Unstructured) (dynamic.NamespaceableResourceInterface, error) {
	apiRes, err := r.getAPIReourceForUnstructured(obj)
	if err != nil {
		log.Error(err, "unable to get apiresource from unstructured", "unstructured", obj)
		return nil, err
	}
	return r.getDynamicClientOnType(apiRes)
}

func (r *ReconcileNamespaceConfig) getAPIReourceForUnstructured(obj unstructured.Unstructured) (metav1.APIResource, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	res := metav1.APIResource{}
	resList, err := r.DiscoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		log.Error(err, "unable to retrieve resouce list for:", "groupversion", gvk.GroupVersion().String())
		return res, err
	}
	for _, resource := range resList.APIResources {
		if resource.Kind == gvk.Kind && !strings.Contains(resource.Name, "/") {
			res = resource
			res.Group = gvk.Group
			res.Version = gvk.Version
			break
		}
	}
	return res, nil

}

func (r *ReconcileNamespaceConfig) manageError(issue error, instance *redhatcopv1alpha1.NamespaceConfig) (reconcile.Result, error) {
	lastUpdate := instance.Status.LastUpdate.Time
	lastStatus := instance.Status.Status
	r.recorder.Event(instance, "Warning", "ProcessingError", issue.Error())
	status := redhatcopv1alpha1.NamespaceConfigStatus{
		LastUpdate: metav1.Now(),
		Reason:     issue.Error(),
		Status:     "Failure",
	}
	instance.Status = status
	err := r.GetClient().Status().Update(context.Background(), instance)
	if err != nil {
		log.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: time.Second,
			Requeue:      true,
		}, nil
	}
	var retryInterval time.Duration
	if instance.Status.LastUpdate.IsZero() || lastStatus == "Success" {
		retryInterval = time.Second
	} else {
		retryInterval = status.LastUpdate.Sub(lastUpdate).Round(time.Second)
	}
	return reconcile.Result{
		RequeueAfter: time.Duration(math.Min(float64(retryInterval.Nanoseconds()*2), float64(time.Hour.Nanoseconds()*6))),
		Requeue:      true,
	}, nil
}

func (r *ReconcileNamespaceConfig) manageSuccess(instance *redhatcopv1alpha1.NamespaceConfig) (reconcile.Result, error) {
	status := redhatcopv1alpha1.NamespaceConfigStatus{
		LastUpdate: metav1.Now(),
		Reason:     "",
		Status:     "Success",
	}
	instance.Status = status
	err := r.GetClient().Status().Update(context.Background(), instance)
	if err != nil {
		log.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: time.Second,
			Requeue:      true,
		}, nil
	}
	return reconcile.Result{}, nil
}

type resourceGenerationOrFinalizerChangedPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating resource version change
func (resourceGenerationOrFinalizerChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.MetaOld == nil {
		log.Error(nil, "UpdateEvent has no old metadata", "event", e)
		return false
	}
	if e.ObjectOld == nil {
		log.Error(nil, "GenericEvent has no old runtime object to update", "event", e)
		return false
	}
	if e.ObjectNew == nil {
		log.Error(nil, "GenericEvent has no new runtime object for update", "event", e)
		return false
	}
	if e.MetaNew == nil {
		log.Error(nil, "UpdateEvent has no new metadata", "event", e)
		return false
	}
	if e.MetaNew.GetGeneration() == e.MetaOld.GetGeneration() && reflect.DeepEqual(e.MetaNew.GetFinalizers(), e.MetaOld.GetFinalizers()) {
		return false
	}
	return true
}
