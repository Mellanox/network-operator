package nicclusterpolicy

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"time"

	mellanoxv1alpha1 "github.com/Mellanox/nic-operator/pkg/apis/mellanox/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_nicclusterpolicy")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new NicClusterPolicy Controller and adds it to the Manager.
// The Manager will set fields on the Controller and Start it when the Manager is started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNicClusterPolicy{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("nicclusterpolicy-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource NicClusterPolicy
	err = c.Watch(&source.Kind{Type: &mellanoxv1alpha1.NicClusterPolicy{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner NicClusterPolicy
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mellanoxv1alpha1.NicClusterPolicy{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileNicClusterPolicy implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNicClusterPolicy{}

// ReconcileNicClusterPolicy reconciles a NicClusterPolicy object
type ReconcileNicClusterPolicy struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a NicClusterPolicy object and makes changes based on the state read
// and what is in the NicClusterPolicy.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNicClusterPolicy) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NicClusterPolicy")

	// Fetch the NicClusterPolicy instance
	instance := &mellanoxv1alpha1.NicClusterPolicy{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	stateToUpdate := mellanoxv1alpha1.StateNotReady
	defer func() {
		if instance.Status.State != stateToUpdate {
			// Update CR state
			reqLogger.Info("Updating CR", "Status", stateToUpdate)
			instance.Status.State = stateToUpdate
			err := r.client.Status().Update(context.TODO(), instance)
			if err != nil {
				reqLogger.Error(err, "Failed to update status")
			}
		}
	}()

	// get DS obj
	obj, err := newDriverDsForCR(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Check if this object already exists
	found := &appsv1.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, found)

	// handle case where OFED is no longer required
	if instance.Spec.OFEDDriver == nil {
		if err == nil {
			err := r.client.Delete(context.TODO(), found)
			if err != nil {
				return reconcile.Result{RequeueAfter: 5 * time.Second}, err
			}
		} else if errors.IsNotFound(err) {
			// Daemonset deleted
			reqLogger.Info("Daemonset deleted. status is set to complete.")
			reqLogger.Info("Reconcile Done for object")
			stateToUpdate = "Complete"
			return reconcile.Result{}, nil
		}
	}

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new object", "kind", obj.GetKind(), "Namespace", obj.GetNamespace(), "Name", obj.GetName())

		// Set NicClusterPolicy instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, obj, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		err = r.client.Create(context.TODO(), obj)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Object created successfully
		reqLogger.Info("Object created successfully")
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// DaemonSet already exists, check if its done setting up pods
	if found.Status.DesiredNumberScheduled == found.Status.CurrentNumberScheduled {
		// set status to Complete and dont requeue
		stateToUpdate = "Complete"
		reqLogger.Info("Reconcile Done for object")
		return reconcile.Result{}, nil
	}

	reqLogger.Info("DaemonSet already exists but is not yet running, requeue after 5 Sec",
		"Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
}

func newDriverDsForCR(cr *mellanoxv1alpha1.NicClusterPolicy) (*unstructured.Unstructured, error) {
	data, err := ioutil.ReadFile("manifests/stage-ofed-driver/0010_ofed-driver-ds.yaml")
	if err != nil {
		return nil, err
	}
	// bytes.NewBuffer() can be used for both read and write
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	// TODO: we should loop through until io.EOF in case of multiple objects per yaml file
	obj := unstructured.Unstructured{}
	err = decoder.Decode(&obj)
	if err != nil && err != io.EOF {
		return nil, err
	}
	obj.SetNamespace(cr.Namespace)
	return &obj, nil
}
