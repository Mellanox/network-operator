/*
2022 NVIDIA CORPORATION & AFFILIATES

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

package controllers //nolint:dupl

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mellanoxcomv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/utils"
)

// IPoIBNetworkReconciler reconciles a IPoIBNetwork object
type IPoIBNetworkReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	MigrationCh chan struct{}

	stateManager state.Manager
}

//nolint:lll
// +kubebuilder:rbac:groups=mellanox.com,resources=ipoibnetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mellanox.com,resources=ipoibnetworks/finalizers,verbs=update
// +kubebuilder:rbac:groups=mellanox.com,resources=ipoibnetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//nolint:dupl
func (r *IPoIBNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Wait for migration flow to finish
	select {
	case <-r.MigrationCh:
	case <-ctx.Done():
		return ctrl.Result{}, fmt.Errorf("canceled")
	}
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling IPoIBNetwork")

	// Fetch the IPoIBNetwork instance
	instance := &mellanoxcomv1alpha1.IPoIBNetwork{}
	err := r.Get(ctx, req.NamespacedName, instance)
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

	managerStatus := r.stateManager.SyncState(ctx, instance, nil)
	err = r.updateCrStatus(ctx, instance, managerStatus)
	if err != nil {
		return reconcile.Result{}, err
	}

	if managerStatus.Status != state.SyncStateReady {
		return reconcile.Result{
			RequeueAfter: time.Duration(config.FromEnv().Controller.RequeueTimeSeconds) * time.Second,
		}, nil
	}

	return ctrl.Result{}, nil
}

func (r *IPoIBNetworkReconciler) updateCrStatus(
	ctx context.Context, cr *mellanoxcomv1alpha1.IPoIBNetwork, status state.Results) error {
	reqLogger := log.FromContext(ctx)
	cr.Status.State = mellanoxcomv1alpha1.State(status.StatesStatus[0].Status)
	if status.StatesStatus[0].ErrInfo != nil {
		cr.Status.Reason = status.StatesStatus[0].ErrInfo.Error()
	}

	var err error

	if cr.Status.State == state.SyncStateReady {
		netAttachDef := &netattdefv1.NetworkAttachmentDefinition{}
		getErr := r.Get(ctx,
			types.NamespacedName{
				Name:      cr.Name,
				Namespace: cr.Spec.NetworkNamespace,
			}, netAttachDef)

		if getErr != nil {
			reqLogger.V(consts.LogLevelError).Error(getErr, "Can not retrieve NetworkAttachmentDefinition object")
			err = getErr
		} else {
			cr.Status.IPoIBNetworkAttachmentDef = utils.GetNetworkAttachmentDefLink(netAttachDef)
		}
	}

	// send status update request to k8s API
	reqLogger.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.Name, "namespace", cr.Namespace, "Result:", cr.Status)
	updateErr := r.Status().Update(context.TODO(), cr)
	if updateErr != nil {
		reqLogger.V(consts.LogLevelError).Error(updateErr, "Failed to update CR status")
		err = updateErr
	}

	return err
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl
func (r *IPoIBNetworkReconciler) SetupWithManager(mgr ctrl.Manager, setupLog logr.Logger) error {
	// Create state manager
	stateManager, err := state.NewManager(mellanoxcomv1alpha1.IPoIBNetworkCRDName, mgr.GetClient(),
		setupLog.WithName("StateManager"))
	if err != nil {
		// Error creating stateManager
		setupLog.V(consts.LogLevelError).Error(err, "Error creating state manager.")
		panic("Failed to create State manager")
	}
	r.stateManager = stateManager

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxcomv1alpha1.IPoIBNetwork{}).
		// Watch for changes to primary resource IPoIBNetwork
		Watches(&mellanoxcomv1alpha1.IPoIBNetwork{}, &handler.EnqueueRequestForObject{})

	// Watch for changes to secondary resource DaemonSet and requeue the owner IPoIBNetwork
	ws := stateManager.GetWatchSources()
	for kindName := range ws {
		setupLog.V(consts.LogLevelInfo).Info("Watching", "Kind", kindName)
		builder = builder.Watches(ws[kindName], handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(),
			&mellanoxcomv1alpha1.IPoIBNetwork{}, handler.OnlyControllerOwner()))
	}

	return builder.Complete(r)
}
