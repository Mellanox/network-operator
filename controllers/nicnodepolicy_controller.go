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
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/policyoverlap"
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
			// Owned objects are automatically garbage collected.
			// Clean up mofed.wait labels for any terminating OFED pods owned by this NNP.
			return r.handleDeletion(ctx, req)
		}
		// Error reading the object - requeue the request.
		reqLogger.V(consts.LogLevelError).Error(err, "Error occurred on GET CRD request from API server.")
		return reconcile.Result{}, err
	}

	// Check for node selector overlap with other NicNodePolicies at runtime
	// (catches cases where nodes are re-labeled after admission)
	if overlapErr := r.checkNodeOverlap(ctx, instance); overlapErr != nil {
		reqLogger.V(consts.LogLevelError).Error(overlapErr, "Node selector overlap detected")
		instance.Status.State = mellanoxv1alpha1.StateError
		instance.Status.Reason = overlapErr.Error()
		r.updateCrStatus(ctx, instance, state.Results{Status: state.SyncStateError})
		return r.requeue()
	}

	// Create a new State service catalog
	sc := state.NewInfoCatalog()
	sc.Add(state.InfoTypeClusterType, r.ClusterTypeProvider)

	if instance.Spec.OFEDDriver != nil {
		if err := setupOFEDCatalog(ctx, r.Client, instance.Spec.OFEDDriver,
			r.DocaDriverImagesProvider, sc, instance.Spec.NodeSelector); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		r.DocaDriverImagesProvider.SetImageSpec(nil)
	}

	// Sync state and update status
	managerStatus := r.stateManager.SyncState(ctx, instance, sc)
	r.updateCrStatus(ctx, instance, managerStatus)

	// Manage mofed.wait labels on nodes where this NNP's OFED pods are running
	if _, err := r.handleMOFEDWaitLabels(ctx, instance); err != nil {
		return reconcile.Result{}, err
	}

	if managerStatus.Status != state.SyncStateReady {
		return r.requeue()
	}

	return ctrl.Result{}, nil
}

// requeue triggers resync with configured requeue delay.
func (r *NicNodePolicyReconciler) requeue() (reconcile.Result, error) {
	return requeueWithDelay()
}

func (r *NicNodePolicyReconciler) updateCrStatus(
	ctx context.Context, cr *mellanoxv1alpha1.NicNodePolicy, status state.Results) {
	updatePolicyCRStatus(ctx, r, cr, status)
}

// handleMOFEDWaitLabels manages mofed.wait labels on nodes where this NNP's OFED pods run.
// If this NNP does not have ofedDriver, it is a no-op.
func (r *NicNodePolicyReconciler) handleMOFEDWaitLabels(
	ctx context.Context, instance *mellanoxv1alpha1.NicNodePolicy) (bool, error) {
	if instance.Spec.OFEDDriver == nil {
		return false, nil
	}
	dsOwner := mellanoxv1alpha1.NicNodePolicyCRDName + "-" + instance.Name
	if err := handleOFEDWaitLabelsForPods(ctx, r.Client, map[string]string{
		consts.OfedDriverLabel: "",
		consts.DSOwnerLabel:    dsOwner,
	}); err != nil {
		return false, err
	}
	return false, nil
}

// handleDeletion cleans up mofed.wait labels when a NicNodePolicy is deleted.
// If OFED pods owned by this NNP are still terminating, sets mofed.wait=true and requeues.
func (r *NicNodePolicyReconciler) handleDeletion(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	dsOwner := mellanoxv1alpha1.NicNodePolicyCRDName + "-" + req.Name

	pods := &corev1.PodList{}
	if err := r.Client.List(ctx, pods, client.MatchingLabels{
		consts.OfedDriverLabel: "",
		consts.DSOwnerLabel:    dsOwner,
	}); err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "failed to list OFED pods for deleted NNP")
		return reconcile.Result{}, nil
	}

	if len(pods.Items) > 0 {
		reqLogger.V(consts.LogLevelInfo).Info("NNP deleted but OFED pods still terminating, setting mofed.wait=true",
			"policy", req.Name, "podCount", len(pods.Items))
		for i := range pods.Items {
			if pods.Items[i].Spec.NodeName != "" {
				if err := setNodeLabel(ctx, r.Client, pods.Items[i].Spec.NodeName,
					nodeinfo.NodeLabelWaitOFED, "true"); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
		return r.requeue()
	}

	return reconcile.Result{}, nil
}

// checkNodeOverlap lists all NicNodePolicies and detects if any nodes are selected by multiple policies.
func (r *NicNodePolicyReconciler) checkNodeOverlap(ctx context.Context,
	instance *mellanoxv1alpha1.NicNodePolicy) error {
	nodePolicyList := &mellanoxv1alpha1.NicNodePolicyList{}
	if err := r.List(ctx, nodePolicyList); err != nil {
		return fmt.Errorf("failed to list NicNodePolicies: %w", err)
	}

	if len(nodePolicyList.Items) < 2 {
		return nil
	}

	overlaps, err := policyoverlap.DetectNodeOverlap(ctx, r.Client, nodePolicyList.Items)
	if err != nil {
		return fmt.Errorf("failed to detect node overlap: %w", err)
	}
	if len(overlaps) > 0 {
		return fmt.Errorf("%s", policyoverlap.FormatNodeOverlap(overlaps))
	}
	return nil
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

	bld = watchStateSources(bld, mgr, setupLog, stateManager, &mellanoxv1alpha1.NicNodePolicy{})

	// Watch Node objects for label changes so re-labeling triggers re-reconciliation
	// and overlap detection runs again
	bld = bld.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			// Re-reconcile all NicNodePolicies when a node changes
			policyList := &mellanoxv1alpha1.NicNodePolicyList{}
			if err := mgr.GetClient().List(ctx, policyList); err != nil {
				return nil
			}
			var requests []reconcile.Request
			for _, p := range policyList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&p),
				})
			}
			return requests
		}),
		builder.WithPredicates(NodeLabelChangePredicate{}),
	)

	return bld.Complete(r)
}
