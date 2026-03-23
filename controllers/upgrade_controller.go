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
	"slices"
	"strings"
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

	// NodePolicyStateManagers holds per-NicNodePolicy upgrade state managers.
	// Keyed by "NicNodePolicy-<name>".
	NodePolicyStateManagers map[string]upgrade.ClusterUpgradeStateManager

	// newStateManagerFn creates a ClusterUpgradeStateManager for a given requestor ID.
	// Injected for testability; defaults to newNodePolicyStateManager.
	newStateManagerFn func(requestorID string) (upgrade.ClusterUpgradeStateManager, error)
}

const plannedRequeueInterval = time.Minute * 2

// UpgradeStateAnnotation is kept for backwards cleanup TODO: drop in 2 releases
const UpgradeStateAnnotation = "nvidia.com/ofed-upgrade-state"

//nolint:lll
// +kubebuilder:rbac:groups=mellanox.com,resources=nicclusterpolicies;nicclusterpolicies/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mellanox.com,resources=nicnodepolicies;nicnodepolicies/status,verbs=get;list;watch
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

	// Cleanup old annotations, leftover from the old versions of network-operator
	// TODO drop in 2 releases
	if err := r.removeNodeUpgradeStateAnnotations(ctx); err != nil {
		return ctrl.Result{}, err
	}

	needsRequeue := false

	// Handle NicClusterPolicy upgrade
	ncpRequeue, err := r.reconcileClusterPolicyUpgrade(ctx, reqLogger)
	if err != nil {
		return ctrl.Result{}, err
	}
	if ncpRequeue {
		needsRequeue = true
	}

	// Handle NicNodePolicy upgrades
	requeue, err := r.reconcileNodePolicyUpgrades(ctx, reqLogger)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		needsRequeue = true
	}

	if needsRequeue {
		return ctrl.Result{Requeue: true, RequeueAfter: plannedRequeueInterval}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileClusterPolicyUpgrade handles the NicClusterPolicy OFED upgrade flow.
// Returns true if an active upgrade is in progress and periodic requeue is needed.
func (r *UpgradeReconciler) reconcileClusterPolicyUpgrade(ctx context.Context, reqLogger logr.Logger) (bool, error) {
	nicClusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{}
	err := r.Get(ctx, types.NamespacedName{Name: consts.NicClusterPolicyResourceName}, nicClusterPolicy)

	if err != nil {
		if errors.IsNotFound(err) {
			return false, r.cleanupUpgradeResources(ctx)
		}
		return false, err
	}

	if nicClusterPolicy.Spec.OFEDDriver == nil ||
		nicClusterPolicy.Spec.OFEDDriver.OfedUpgradePolicy == nil ||
		!nicClusterPolicy.Spec.OFEDDriver.OfedUpgradePolicy.AutoUpgrade {
		reqLogger.V(consts.LogLevelInfo).Info("OFED Upgrade Policy is disabled for NicClusterPolicy, skipping")
		return false, r.cleanupUpgradeResources(ctx)
	}

	upgradePolicy := nicClusterPolicy.Spec.OFEDDriver.OfedUpgradePolicy
	state, err := r.StateManager.BuildState(ctx,
		config.FromEnv().State.NetworkOperatorResourceNamespace,
		map[string]string{consts.OfedDriverLabel: "", consts.DSOwnerLabel: mellanoxv1alpha1.NicClusterPolicyCRDName})
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to build cluster upgrade state for NicClusterPolicy")
		return false, err
	}

	reqLogger.V(consts.LogLevelInfo).Info("Applying upgrade state for NicClusterPolicy")
	driverUpgradePolicy := mellanoxv1alpha1.GetDriverUpgradePolicy(upgradePolicy)
	return true, r.StateManager.ApplyState(ctx, state, driverUpgradePolicy)
}

// reconcileNodePolicyUpgrades handles OFED upgrade for all NicNodePolicies independently.
// Returns true if any policy needs requeueing.
func (r *UpgradeReconciler) reconcileNodePolicyUpgrades(ctx context.Context,
	reqLogger logr.Logger) (bool, error) {
	nodePolicyList := &mellanoxv1alpha1.NicNodePolicyList{}
	if err := r.List(ctx, nodePolicyList); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if r.NodePolicyStateManagers == nil {
		r.NodePolicyStateManagers = make(map[string]upgrade.ClusterUpgradeStateManager)
	}

	// Track active policy keys to clean up stale managers
	activePolicyKeys := make(map[string]bool)
	needsRequeue := false

	for i := range nodePolicyList.Items {
		np := &nodePolicyList.Items[i]
		policyKey := mellanoxv1alpha1.NicNodePolicyCRDName + "-" + np.Name

		if np.Spec.OFEDDriver == nil ||
			np.Spec.OFEDDriver.OfedUpgradePolicy == nil ||
			!np.Spec.OFEDDriver.OfedUpgradePolicy.AutoUpgrade {
			reqLogger.V(consts.LogLevelInfo).Info("OFED Upgrade Policy disabled for NicNodePolicy",
				"policy", np.Name)
			// Clean up NodeMaintenance objects and upgrade labels for this policy
			if _, ok := r.NodePolicyStateManagers[policyKey]; ok {
				if err := r.cleanupNodePolicyUpgradeResources(ctx, policyKey); err != nil {
					reqLogger.V(consts.LogLevelError).Error(err,
						"Failed to clean up upgrade resources for disabled policy", "policy", np.Name)
					needsRequeue = true
					continue // keep manager in map, retry next reconcile
				}
			}
			delete(r.NodePolicyStateManagers, policyKey)
			continue
		}

		activePolicyKeys[policyKey] = true
		needsRequeue = true

		// Get or create state manager for this policy
		sm, ok := r.NodePolicyStateManagers[policyKey]
		if !ok {
			var err error
			sm, err = r.getOrCreateNodePolicyStateManager(policyKey)
			if err != nil {
				reqLogger.V(consts.LogLevelError).Error(err,
					"Failed to create state manager for NicNodePolicy", "policy", np.Name)
				return false, err
			}
			r.NodePolicyStateManagers[policyKey] = sm
		}

		state, err := sm.BuildState(ctx,
			config.FromEnv().State.NetworkOperatorResourceNamespace,
			map[string]string{consts.OfedDriverLabel: "", consts.DSOwnerLabel: policyKey})
		if err != nil {
			reqLogger.V(consts.LogLevelError).Error(err,
				"Failed to build upgrade state for NicNodePolicy", "policy", np.Name)
			return false, err
		}

		reqLogger.V(consts.LogLevelInfo).Info("Applying upgrade state for NicNodePolicy",
			"policy", np.Name)
		driverUpgradePolicy := mellanoxv1alpha1.GetDriverUpgradePolicy(np.Spec.OFEDDriver.OfedUpgradePolicy)
		if err := sm.ApplyState(ctx, state, driverUpgradePolicy); err != nil {
			reqLogger.V(consts.LogLevelError).Error(err,
				"Failed to apply upgrade state for NicNodePolicy", "policy", np.Name)
			return false, err
		}
	}

	// Clean up stale state managers for deleted policies
	for key := range r.NodePolicyStateManagers {
		if !activePolicyKeys[key] {
			if err := r.cleanupNodePolicyUpgradeResources(ctx, key); err != nil {
				reqLogger.V(consts.LogLevelError).Error(err,
					"Failed to clean up upgrade resources for deleted policy", "policy", key)
				needsRequeue = true
				continue // keep manager in map, retry next reconcile
			}
			delete(r.NodePolicyStateManagers, key)
		}
	}

	return needsRequeue, nil
}

// getOrCreateNodePolicyStateManager creates a new ClusterUpgradeStateManager for a NicNodePolicy.
func (r *UpgradeReconciler) getOrCreateNodePolicyStateManager(
	policyKey string) (upgrade.ClusterUpgradeStateManager, error) {
	if r.newStateManagerFn != nil {
		return r.newStateManagerFn(policyKey)
	}
	return newNodePolicyStateManager(policyKey)
}

// newNodePolicyStateManager creates a ClusterUpgradeStateManager with a policy-specific requestor ID.
func newNodePolicyStateManager(policyKey string) (upgrade.ClusterUpgradeStateManager, error) {
	upgradeLogger := ctrl.Log.WithName("controllers").WithName("Upgrade").WithName(policyKey)
	requestorOpts := upgrade.GetRequestorOptsFromEnvs()
	// Create a policy-specific requestor ID
	requestorOpts.MaintenanceOPRequestorID = requestorOpts.MaintenanceOPRequestorID + "-" + policyKey
	return upgrade.NewClusterUpgradeStateManager(
		upgradeLogger,
		ctrl.GetConfigOrDie(),
		nil,
		upgrade.StateOptions{Requestor: requestorOpts})
}

// cleanupUpgradeResources cleans up existing nodeMaintenance and state label upgrade resources
// upon NicClusterPolicy deletion, OfedUpgradePolicy removal or disabled autoUpgrade feature.
// Nodes managed by NicNodePolicies with active OFED upgrades are excluded from cleanup.
func (r *UpgradeReconciler) cleanupUpgradeResources(ctx context.Context) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Starting nodeMaintenance, state label upgrade resources cleanup")

	// Get nodes managed by NNPs with OFED — don't clean their upgrade labels
	nnpNodes, err := getNodesManagedByNNPsWithOFED(ctx, r.Client)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err,
			"Failed to get NNP-managed nodes for upgrade cleanup, proceeding without exclusion")
		nnpNodes = nil
	}

	// Clean up node upgrade state labels (excluding NNP-managed nodes)
	if err := r.removeNodeUpgradeStateLabels(ctx, nnpNodes); err != nil {
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

// removeUpgradeStateLabelFromNodes removes upgrade.UpgradeStateLabel from the given nodes.
func (r *UpgradeReconciler) removeUpgradeStateLabelFromNodes(ctx context.Context, nodes []*corev1.Node) error {
	upgradeStateLabel := upgrade.GetUpgradeStateLabelKey()
	for _, node := range nodes {
		if _, present := node.Labels[upgradeStateLabel]; present {
			delete(node.Labels, upgradeStateLabel)
			if err := r.Update(ctx, node); err != nil {
				log.FromContext(ctx).V(consts.LogLevelError).Error(
					err, "Failed to reset upgrade label from node", "node", node.Name)
				return err
			}
		}
	}
	return nil
}

// removeNodeUpgradeStateLabels removes upgrade.UpgradeStateLabel from all cluster nodes,
// skipping nodes in excludeNodes (managed by NNP upgrade controllers).
func (r *UpgradeReconciler) removeNodeUpgradeStateLabels(
	ctx context.Context, excludeNodes map[string]bool) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Resetting node upgrade labels from all nodes")

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		reqLogger.Error(err, "Failed to get node list to reset upgrade labels")
		return err
	}

	nodes := make([]*corev1.Node, 0, len(nodeList.Items))
	for i := range nodeList.Items {
		if !excludeNodes[nodeList.Items[i].Name] {
			nodes = append(nodes, &nodeList.Items[i])
		}
	}
	return r.removeUpgradeStateLabelFromNodes(ctx, nodes)
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

// cleanupNodePolicyUpgradeResources cleans up upgrade state labels and NodeMaintenance objects
// for a specific NicNodePolicy that has been deleted or had its upgrade policy disabled.
func (r *UpgradeReconciler) cleanupNodePolicyUpgradeResources(ctx context.Context, policyKey string) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Cleaning up upgrade resources for NicNodePolicy", "policy", policyKey)

	// Step 1: Remove upgrade state labels from this policy's nodes (always, regardless of mode)
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.MatchingLabels{
		consts.OfedDriverLabel: "",
		consts.DSOwnerLabel:    policyKey,
	}); err != nil {
		return fmt.Errorf("failed to list OFED pods for policy %s: %w", policyKey, err)
	}

	nodes := make([]*corev1.Node, 0, len(pods.Items))
	for i := range pods.Items {
		nodeName := pods.Items[i].Spec.NodeName
		if nodeName == "" {
			continue
		}
		node := &corev1.Node{}
		if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	if err := r.removeUpgradeStateLabelFromNodes(ctx, nodes); err != nil {
		return err
	}

	// Step 2: Clean up NodeMaintenance objects (requestor mode only)
	requestorOpts := upgrade.GetRequestorOptsFromEnvs()
	if !requestorOpts.UseMaintenanceOperator {
		return nil
	}

	policyRequestorID := requestorOpts.MaintenanceOPRequestorID + "-" + policyKey

	nodeMaintenanceList := &maintenancev1alpha1.NodeMaintenanceList{}
	if err := r.List(ctx, nodeMaintenanceList,
		client.InNamespace(requestorOpts.MaintenanceOPRequestorNS)); err != nil {
		return fmt.Errorf("failed to list NodeMaintenance objects: %w", err)
	}

	for i := range nodeMaintenanceList.Items {
		nm := &nodeMaintenanceList.Items[i]
		if nm.Spec.RequestorID == policyRequestorID {
			reqLogger.V(consts.LogLevelInfo).Info("Deleting NodeMaintenance object for deleted policy",
				"name", nm.Name, "nodeName", nm.Spec.NodeName, "policy", policyKey)
			if err := r.Delete(ctx, nm); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete NodeMaintenance %s: %w", nm.Name, err)
			}
		}
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
		Watches(&mellanoxv1alpha1.NicNodePolicy{}, createUpdateDeleteEnqueue).
		Watches(&corev1.Node{}, createUpdateEnqueue, nodePredicates).
		Watches(&appsv1.DaemonSet{}, createUpdateDeleteEnqueue, daemonSetPredicates)

	// Conditionally add Watches for NodeMaintenance if UseMaintenanceOperator is true
	requestorOpts := upgrade.GetRequestorOptsFromEnvs()
	if requestorOpts.UseMaintenanceOperator {
		baseRequestorID := requestorOpts.MaintenanceOPRequestorID
		nodeMaintenancePredicate := upgrade.NewConditionChangedPredicate(log, baseRequestorID)
		// Match NodeMaintenance objects owned by NCP (exact match) or any NNP (prefix match)
		requestorIDPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
			nm, ok := object.(*maintenancev1alpha1.NodeMaintenance)
			if !ok {
				return false
			}
			return nm.Spec.RequestorID == baseRequestorID ||
				strings.HasPrefix(nm.Spec.RequestorID, baseRequestorID+"-") ||
				slices.Contains(nm.Spec.AdditionalRequestors, baseRequestorID)
		})
		mngr = mngr.Watches(&maintenancev1alpha1.NodeMaintenance{}, createUpdateDeleteEnqueue,
			builder.WithPredicates(nodeMaintenancePredicate, requestorIDPredicate))
	}

	return mngr.Complete(r)
}
