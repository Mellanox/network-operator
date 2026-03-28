/*
Copyright 2026 NVIDIA

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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/state"
)

// NicNodePolicyReconciler reconciles a NicNodePolicy object
type NicNodePolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ClusterTypeProvider      clustertype.Provider
	DocaDriverImagesProvider docadriverimages.Provider

	stateManager state.Manager
}

//nolint:lll
// +kubebuilder:rbac:groups=mellanox.com,resources=nicnodepolicies;nicnodepolicies/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mellanox.com,resources=nicnodepolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NicNodePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Reconciling NicNodePolicy")

	// Fetch the NicNodePolicy instance
	instance := &mellanoxv1alpha1.NicNodePolicy{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.V(consts.LogLevelError).Error(err, "Error occurred on GET CRD request from API server.")
		return reconcile.Result{}, err
	}

	// Create a new State service catalog
	sc := state.NewInfoCatalog()
	sc.Add(state.InfoTypeClusterType, r.ClusterTypeProvider)

	if instance.Spec.OFEDDriver != nil {
		reqLogger.V(consts.LogLevelInfo).Info("Creating Node info provider")
		nodeList := &corev1.NodeList{}
		err = r.List(ctx, nodeList, nodeinfo.MellanoxNICListOptions...)
		if err != nil {
			reqLogger.V(consts.LogLevelError).Error(err, "Error occurred on LIST nodes request from API server.")
			return reconcile.Result{}, err
		}
		nodePtrList := make([]*corev1.Node, len(nodeList.Items))
		for i := range nodePtrList {
			nodePtrList[i] = &nodeList.Items[i]
		}
		sc.Add(state.InfoTypeNodeInfo, nodeinfo.NewProvider(nodePtrList))
		r.DocaDriverImagesProvider.SetImageSpec(&instance.Spec.OFEDDriver.ImageSpec)
		sc.Add(state.InfoTypeDocaDriverImage, r.DocaDriverImagesProvider)
	} else {
		r.DocaDriverImagesProvider.SetImageSpec(nil)
	}

	// Sync state and update status
	managerStatus := r.stateManager.SyncState(ctx, instance, sc)
	r.updateCrStatus(ctx, instance, managerStatus)

	if managerStatus.Status != state.SyncStateReady {
		return r.requeue()
	}

	return ctrl.Result{}, nil
}

// requeue triggers resync with configured requeue delay
func (r *NicNodePolicyReconciler) requeue() (reconcile.Result, error) {
	return reconcile.Result{
		RequeueAfter: time.Duration(config.FromEnv().Controller.RequeueTimeSeconds) * time.Second,
	}, nil
}

//nolint:dupl
func (r *NicNodePolicyReconciler) updateCrStatus(
	ctx context.Context, cr *mellanoxv1alpha1.NicNodePolicy, status state.Results) {
	reqLogger := log.FromContext(ctx)
NextResult:
	for _, stateStatus := range status.StatesStatus {
		for i := range cr.Status.AppliedStates {
			if cr.Status.AppliedStates[i].Name == stateStatus.StateName {
				cr.Status.AppliedStates[i].State = mellanoxv1alpha1.State(stateStatus.Status)
				if stateStatus.ErrInfo != nil {
					cr.Status.AppliedStates[i].Message = stateStatus.ErrInfo.Error()
				} else {
					cr.Status.AppliedStates[i].Message = ""
				}
				continue NextResult
			}
		}
		cr.Status.AppliedStates = append(cr.Status.AppliedStates, mellanoxv1alpha1.AppliedState{
			Name:  stateStatus.StateName,
			State: mellanoxv1alpha1.State(stateStatus.Status),
		})
	}
	// Update global State
	cr.Status.State = mellanoxv1alpha1.State(status.Status)

	reqLogger.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.Name, "namespace", cr.Namespace, "Result:", cr.Status)
	err := r.Status().Update(ctx, cr)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to update CR status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *NicNodePolicyReconciler) SetupWithManager(mgr ctrl.Manager, setupLog logr.Logger) error {
	// Create state manager
	stateManager, err := state.NewManager(mellanoxv1alpha1.NicNodePolicyCRDName, mgr.GetClient(),
		setupLog.WithName("StateManager"))
	if err != nil {
		setupLog.V(consts.LogLevelError).Error(err, "Error creating state manager.")
		return fmt.Errorf("failed to create state manager: %w", err)
	}
	r.stateManager = stateManager

	bld := ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxv1alpha1.NicNodePolicy{})

	ws := stateManager.GetWatchSources()
	for kindName := range ws {
		setupLog.V(consts.LogLevelInfo).Info("Watching", "Kind", kindName)
		bld = bld.Watches(ws[kindName], handler.EnqueueRequestForOwner(
			mgr.GetScheme(), mgr.GetRESTMapper(), &mellanoxv1alpha1.NicNodePolicy{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(IgnoreSameContentPredicate{}))
	}

	return bld.Complete(r)
}
