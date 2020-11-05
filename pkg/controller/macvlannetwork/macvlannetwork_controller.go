/*
Copyright 2020 NVIDIA

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

package macvlannetwork

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/pkg/apis/mellanox/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/state"
)

var log = logf.Log.WithName("controller_macvlannetwork")

// Add creates a new MacvlanNetwork Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	// Create state manager
	stateManager, err := state.NewManager(mellanoxv1alpha1.MacvlanNetworkCRDName, mgr.GetClient(), mgr.GetScheme())
	if err != nil {
		// Error creating stateManager
		log.V(consts.LogLevelError).Info("Error creating state manager.", "error:", err)
		panic("Failed to create State manager")
	}
	return &ReconcileMacvlanNetwork{client: mgr.GetClient(), scheme: mgr.GetScheme(), stateManager: stateManager}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("macvlannetwork-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MacvlanNetwork
	err = c.Watch(&source.Kind{Type: &mellanoxv1alpha1.MacvlanNetwork{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource DaemonSet and requeue the owner MacvlanNetwork
	ws := r.(*ReconcileMacvlanNetwork).stateManager.GetWatchSources()
	log.V(consts.LogLevelInfo).Info("Watch Sources", "Kind:", ws)
	for i := range ws {
		err = c.Watch(ws[i], &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &mellanoxv1alpha1.MacvlanNetwork{},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// blank assignment to verify that ReconcileMacvlanNetwork implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMacvlanNetwork{}

// ReconcileMacvlanNetwork reconciles a MacvlanNetwork object
type ReconcileMacvlanNetwork struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	stateManager state.Manager
}

// Reconcile reads that state of the cluster for a MacvlanNetwork object and makes changes based on the state read
// and what is in the MacvlanNetwork.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMacvlanNetwork) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MacvlanNetwork")

	// Fetch the MacvlanNetwork instance
	instance := &mellanoxv1alpha1.MacvlanNetwork{}
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

	managerStatus, err := r.stateManager.SyncState(instance, nil)
	r.updateCrStatus(instance, managerStatus)
	if err != nil {
		return reconcile.Result{}, err
	}

	if managerStatus.Status != state.SyncStateReady {
		return reconcile.Result{
			RequeueAfter: time.Duration(config.FromEnv().Controller.RequeueTimeSeconds) * time.Second,
		}, nil
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileMacvlanNetwork) updateCrStatus(cr *mellanoxv1alpha1.MacvlanNetwork, status state.Results) {
	cr.Status.State = mellanoxv1alpha1.State(status.StatesStatus[0].Status)

	// send status update request to k8s API
	log.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.Name, "namespace", cr.Namespace, "Result:", cr.Status)
	err := r.client.Status().Update(context.TODO(), cr)
	if err != nil {
		log.V(consts.LogLevelError).Info("Failed to update CR status", "error:", err)
	}
}
