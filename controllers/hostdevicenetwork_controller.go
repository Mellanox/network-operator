/*
Copyright 2021 NVIDIA

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

// Package controllers contains controllers for network operations.
package controllers //nolint:dupl

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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

// HostDeviceNetworkReconciler reconciles a HostDeviceNetwork object
type HostDeviceNetworkReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	MigrationCh chan struct{}

	stateManager state.Manager
}

//nolint:lll
// +kubebuilder:rbac:groups=mellanox.com,resources=hostdevicenetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mellanox.com,resources=hostdevicenetworks/finalizers,verbs=update
// +kubebuilder:rbac:groups=mellanox.com,resources=hostdevicenetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//nolint:dupl
func (r *HostDeviceNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Wait for migration flow to finish
	select {
	case <-r.MigrationCh:
	case <-ctx.Done():
		return ctrl.Result{}, fmt.Errorf("canceled")
	}
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling HostDeviceNetwork")

	// Fetch the HostDeviceNetwork instance
	instance := &mellanoxcomv1alpha1.HostDeviceNetwork{}
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
	r.updateCrStatus(ctx, instance, managerStatus)
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

//nolint:dupl
func (r *HostDeviceNetworkReconciler) updateCrStatus(
	ctx context.Context, cr *mellanoxcomv1alpha1.HostDeviceNetwork, status state.Results) {
	reqLogger := log.FromContext(ctx)
NextResult:
	for _, stateStatus := range status.StatesStatus {
		// basically iterate over results and add/update crStatus.AppliedStates
		for i := range cr.Status.AppliedStates {
			if cr.Status.AppliedStates[i].Name == stateStatus.StateName {
				cr.Status.AppliedStates[i].State = mellanoxcomv1alpha1.State(stateStatus.Status)
				continue NextResult
			}
		}
		cr.Status.AppliedStates = append(cr.Status.AppliedStates, mellanoxcomv1alpha1.AppliedState{
			Name:  stateStatus.StateName,
			State: mellanoxcomv1alpha1.State(stateStatus.Status),
		})
	}
	// Update global State
	cr.Status.State = mellanoxcomv1alpha1.State(status.Status)

	if cr.Status.State == state.SyncStateReady {
		netAttachDef := &netattdefv1.NetworkAttachmentDefinition{}
		err := r.Get(ctx,
			types.NamespacedName{
				Name:      cr.Name,
				Namespace: cr.Spec.NetworkNamespace,
			}, netAttachDef)

		if err != nil {
			reqLogger.V(consts.LogLevelError).Error(err, "Can not retrieve NetworkAttachmentDefinition object")
		} else {
			cr.Status.HostDeviceNetworkAttachmentDef = utils.GetNetworkAttachmentDefLink(netAttachDef)
		}
	}

	// send status update request to k8s API
	reqLogger.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.Name, "namespace", cr.Namespace, "Result:", cr.Status)
	err := r.Status().Update(ctx, cr)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to update CR status")
	}
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl
func (r *HostDeviceNetworkReconciler) SetupWithManager(mgr ctrl.Manager, setupLog logr.Logger) error {
	// Create state manager
	stateManager, err := state.NewManager(mellanoxcomv1alpha1.HostDeviceNetworkCRDName, mgr.GetClient(),
		setupLog.WithName("StateManager"))
	if err != nil {
		// Error creating stateManager
		setupLog.V(consts.LogLevelError).Error(err, "Error creating state manager.")
		panic("Failed to create State manager")
	}
	r.stateManager = stateManager

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxcomv1alpha1.HostDeviceNetwork{}).
		// Watch for changes to primary resource HostDeviceNetwork
		Watches(&mellanoxcomv1alpha1.HostDeviceNetwork{}, &handler.EnqueueRequestForObject{})

	// Watch for changes to secondary resource DaemonSet and requeue the owner HostDeviceNetwork
	ws := stateManager.GetWatchSources()
	for kindName := range ws {
		setupLog.V(consts.LogLevelInfo).Info("Watching", "Kind", kindName)
		builder = builder.Watches(ws[kindName], handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(),
			&mellanoxcomv1alpha1.HostDeviceNetwork{}, handler.OnlyControllerOwner()))
	}

	return builder.Complete(r)
}
