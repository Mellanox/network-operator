/*
Copyright 2022 NVIDIA CORPORATION & AFFILIATES

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

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
)

// UpgradeReconciler reconciles OFED Daemon Sets for upgrade
type UpgradeReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	StateManager upgrade.ClusterUpgradeStateManager
	MigrationCh  chan struct{}
}

const plannedRequeueInterval = time.Minute * 2

// UpgradeStateAnnotation is kept for backwards cleanup TODO: drop in 2 releases
const UpgradeStateAnnotation = "nvidia.com/ofed-upgrade-state"

//nolint:lll
// +kubebuilder:rbac:groups=mellanox.com,resources=nicclusterpolicies;nicclusterpolicies/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets;controllerrevisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UpgradeReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	// Wait for migration flow to finish
	select {
	case <-r.MigrationCh:
	case <-ctx.Done():
		return ctrl.Result{}, fmt.Errorf("canceled")
	}
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Reconciling Upgrade")

	nicClusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{}
	err := r.Get(ctx, types.NamespacedName{Name: consts.NicClusterPolicyResourceName}, nicClusterPolicy)

	if err != nil {
		if errors.IsNotFound(err) {
			// Cleanup existing upgrade related resources
			if err := r.cleanupUpgradeResources(ctx); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Cleanup old annotations, leftover from the old versions of network-operator
	// TODO drop in 2 releases
	err = r.removeNodeUpgradeStateAnnotations(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if nicClusterPolicy.Spec.OFEDDriver == nil ||
		nicClusterPolicy.Spec.OFEDDriver.OfedUpgradePolicy == nil ||
		!nicClusterPolicy.Spec.OFEDDriver.OfedUpgradePolicy.AutoUpgrade {
		reqLogger.V(consts.LogLevelInfo).Info("OFED Upgrade Policy is disabled, skipping driver upgrade")
		// Cleanup existing upgrade related resources
		if err := r.cleanupUpgradeResources(ctx); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	upgradePolicy := nicClusterPolicy.Spec.OFEDDriver.OfedUpgradePolicy

	state, err := r.StateManager.BuildState(ctx,
		config.FromEnv().State.NetworkOperatorResourceNamespace,
		map[string]string{consts.OfedDriverLabel: ""})
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to build cluster upgrade state")
		return ctrl.Result{}, err
	}

	reqLogger.V(consts.LogLevelInfo).Info("Propagate state to state manager")
	reqLogger.V(consts.LogLevelDebug).Info("Current cluster upgrade state", "state", state)
	driverUpgradePolicy := mellanoxv1alpha1.GetDriverUpgradePolicy(upgradePolicy)
	err = r.StateManager.ApplyState(ctx, state, driverUpgradePolicy)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to apply cluster upgrade state")
		return ctrl.Result{}, err
	}

	// In some cases if node state changes fail to apply, upgrade process
	// might become stuck until the new reconcile loop is scheduled.
	// Since node/ds/nicclusterpolicy updates from outside of the upgrade flow
	// are not guaranteed, for safety reconcile loop should be requeued every few minutes.
	return ctrl.Result{Requeue: true, RequeueAfter: plannedRequeueInterval}, nil
}

// cleanupUpgradeResources cleans up existing nodeMaintenance and state label upgrade resources
// upon NicClusterPolicy deletion, OfedUpgradePolicy removal or disabled autoUpgrade feature
func (r *UpgradeReconciler) cleanupUpgradeResources(ctx context.Context) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Starting nodeMaintenance, state label upgrade resources cleanup")

	// Clean up node upgrade state labels
	if err := r.removeNodeUpgradeStateLabels(ctx); err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to remove node upgrade state labels")
		return err
	}

	// Clean up NodeMaintenance objects
	if err := r.cleanupNodeMaintenanceObjects(ctx); err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to cleanup NodeMaintenance objects")
		return err
	}

	return nil
}

// removeNodeUpgradeStateLabels loops over nodes in the cluster and removes upgrade.UpgradeStateLabel
// It is used for cleanup when autoUpgrade feature gets disabled
func (r *UpgradeReconciler) removeNodeUpgradeStateLabels(ctx context.Context) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Resetting node upgrade labels from all nodes")

	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)
	if err != nil {
		reqLogger.Error(err, "Failed to get node list to reset upgrade labels")
		return err
	}

	upgradeStateLabel := upgrade.GetUpgradeStateLabelKey()

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		_, present := node.Labels[upgradeStateLabel]
		if present {
			delete(node.Labels, upgradeStateLabel)
			err = r.Update(ctx, node)
			if err != nil {
				reqLogger.V(consts.LogLevelError).Error(
					err, "Failed to reset upgrade annotation from node", "node", node)
				return err
			}
		}
	}
	return nil
}

// removeNodeUpgradeStateAnnotations loops over nodes in the cluster and removes UpgradeStateAnnotation
// It is used now only to clean up leftover annotations from previous versions of network-operator
// TODO drop in 2 releases
func (r *UpgradeReconciler) removeNodeUpgradeStateAnnotations(ctx context.Context) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Resetting node upgrade annotations from all nodes")

	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to get node list to reset upgrade annotations")
		return err
	}
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		_, present := node.Annotations[UpgradeStateAnnotation]
		if present {
			delete(node.Annotations, UpgradeStateAnnotation)
			err = r.Update(ctx, node)
			if err != nil {
				reqLogger.V(consts.LogLevelError).Error(
					err, "Failed to reset upgrade annotation from node", "node", node)
				return err
			}
		}
	}
	return nil
}

// cleanupNodeMaintenanceObjects removes all NodeMaintenance objects created by the network operator
// It is called when NicClusterPolicy is deleted to ensure proper cleanup
func (r *UpgradeReconciler) cleanupNodeMaintenanceObjects(ctx context.Context) error {
	reqLogger := log.FromContext(ctx)
	// Get requestor configuration from environment
	requestorOpts := upgrade.GetRequestorOptsFromEnvs()
	// Only proceed if requestor mode is enabled
	if !requestorOpts.UseMaintenanceOperator {
		reqLogger.V(consts.LogLevelDebug).Info("Requestor mode not enabled, skipping NodeMaintenance cleanup")
		return nil
	}
	reqLogger.V(consts.LogLevelInfo).Info("Cleaning up NodeMaintenance objects",
		"requestorID", requestorOpts.MaintenanceOPRequestorID)

	// List all NodeMaintenance objects in the target namespace
	nodeMaintenanceList := &maintenancev1alpha1.NodeMaintenanceList{}
	err := r.List(ctx, nodeMaintenanceList,
		[]client.ListOption{client.InNamespace(requestorOpts.MaintenanceOPRequestorNS)}...)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to list NodeMaintenance objects")
		return err
	}

	// Delete NodeMaintenance objects that belong to our requestor ID
	ownedNMCount := 0
	for i := range nodeMaintenanceList.Items {
		nm := &nodeMaintenanceList.Items[i]

		// Only delete objects created by our requestor ID
		if nm.Spec.RequestorID == requestorOpts.MaintenanceOPRequestorID {
			reqLogger.V(consts.LogLevelInfo).Info("Deleting NodeMaintenance object",
				"name", nm.Name, "namespace", nm.Namespace, "nodeName", nm.Spec.NodeName)

			err := r.Delete(ctx, nm)
			if err != nil {
				if errors.IsNotFound(err) {
					// Object was already deleted, continue
					reqLogger.V(consts.LogLevelDebug).Info("NodeMaintenance object already deleted",
						"name", nm.Name, "namespace", nm.Namespace)
					continue
				}
				reqLogger.V(consts.LogLevelError).Error(err,
					"Failed to delete NodeMaintenance object",
					"name", nm.Name, "namespace", nm.Namespace)
				return err
			}
			ownedNMCount++
		}
	}

	if ownedNMCount > 0 {
		reqLogger.V(consts.LogLevelInfo).Info("Successfully cleaned up NodeMaintenance objects",
			"count", ownedNMCount)
	} else {
		reqLogger.V(consts.LogLevelInfo).Info("No NodeMaintenance objects found for cleanup")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl
func (r *UpgradeReconciler) SetupWithManager(log logr.Logger, mgr ctrl.Manager) error {
	// we always add object with a same(static) key to the queue to reduce
	// reconciliation count
	qHandler := func(q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: "ofed-upgrade-reconcile-namespace",
			Name:      "ofed-upgrade-reconcile-name",
		}})
	}

	createUpdateDeleteEnqueue := handler.Funcs{
		CreateFunc: func(_ context.Context, _ event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			qHandler(q)
		},
		UpdateFunc: func(_ context.Context, _ event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			qHandler(q)
		},
		DeleteFunc: func(_ context.Context, _ event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			qHandler(q)
		},
	}

	createUpdateEnqueue := handler.Funcs{
		CreateFunc: func(_ context.Context, _ event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			qHandler(q)
		},
		UpdateFunc: func(_ context.Context, _ event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			qHandler(q)
		},
	}

	// react on events only for OFED daemon set
	daemonSetPredicates := builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
		labels := object.GetLabels()
		_, ok := labels[consts.OfedDriverLabel]
		return ok
	}))

	// react only on label and annotation changes
	nodePredicates := builder.WithPredicates(
		predicate.Or(predicate.AnnotationChangedPredicate{},
			predicate.LabelChangedPredicate{}))

	mngr := ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxv1alpha1.NicClusterPolicy{}).
		Named("Upgrade").
		// set MaxConcurrentReconciles to 1, by default it is already 1, but
		// we set it explicitly here to indicate that we rely on this default behavior
		// UpgradeReconciler contains logic which is not concurrent friendly
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(&mellanoxv1alpha1.NicClusterPolicy{}, createUpdateDeleteEnqueue).
		Watches(&corev1.Node{}, createUpdateEnqueue, nodePredicates).
		Watches(&appsv1.DaemonSet{}, createUpdateDeleteEnqueue, daemonSetPredicates)

	// Conditionally add Watches for NodeMaintenance if UseMaintenanceOperator is true
	requestorOpts := upgrade.GetRequestorOptsFromEnvs()
	if requestorOpts.UseMaintenanceOperator {
		nodeMaintenancePredicate := upgrade.NewConditionChangedPredicate(log,
			requestorOpts.MaintenanceOPRequestorID)
		requestorIDPredicate := upgrade.NewRequestorIDPredicate(log,
			requestorOpts.MaintenanceOPRequestorID)
		mngr = mngr.Watches(&maintenancev1alpha1.NodeMaintenance{}, createUpdateDeleteEnqueue,
			builder.WithPredicates(nodeMaintenancePredicate, requestorIDPredicate))
	}

	return mngr.Complete(r)
}
