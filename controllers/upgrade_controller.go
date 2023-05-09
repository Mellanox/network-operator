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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/NVIDIA/k8s-operator-libs/pkg/upgrade"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
)

// UpgradeReconciler reconciles OFED Daemon Sets for upgrade
type UpgradeReconciler struct {
	client.Client
	Log                      logr.Logger
	Scheme                   *runtime.Scheme
	StateManager             *upgrade.ClusterUpgradeStateManager
	NodeUpgradeStateProvider upgrade.NodeUpgradeStateProvider
}

const plannedRequeueInterval = time.Minute * 2

// UpgradeStateAnnotation is kept for backwards cleanup TODO: drop in 2 releases
const UpgradeStateAnnotation = "nvidia.com/ofed-upgrade-state"

//nolint
// +kubebuilder:rbac:groups=mellanox.com,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets;controllerrevisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UpgradeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("upgrade", req.NamespacedName)
	reqLogger.V(consts.LogLevelInfo).Info("Reconciling Upgrade")

	nicClusterPolicy := &mellanoxv1alpha1.NicClusterPolicy{}
	err := r.Get(ctx, types.NamespacedName{Name: consts.NicClusterPolicyResourceName}, nicClusterPolicy)

	if err != nil {
		if errors.IsNotFound(err) {
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
		err = r.removeNodeUpgradeStateLabels(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	upgradePolicy := nicClusterPolicy.Spec.OFEDDriver.OfedUpgradePolicy

	state, err := r.BuildState(ctx)
	if err != nil {
		r.Log.V(consts.LogLevelError).Error(err, "Failed to build cluster upgrade state")
		return ctrl.Result{}, err
	}

	reqLogger.V(consts.LogLevelInfo).Info("Propagate state to state manager")
	reqLogger.V(consts.LogLevelDebug).Info("Current cluster upgrade state", "state", state)
	driverUpgradePolicy := mellanoxv1alpha1.GetDriverUpgradePolicy(upgradePolicy)
	err = r.StateManager.ApplyState(ctx, state, driverUpgradePolicy)
	if err != nil {
		r.Log.V(consts.LogLevelError).Error(err, "Failed to apply cluster upgrade state")
		return ctrl.Result{}, err
	}

	// In some cases if node state changes fail to apply, upgrade process
	// might become stuck until the new reconcile loop is scheduled.
	// Since node/ds/nicclusterpolicy updates from outside of the upgrade flow
	// are not guaranteed, for safety reconcile loop should be requeued every few minutes.
	return ctrl.Result{Requeue: true, RequeueAfter: plannedRequeueInterval}, nil
}

// removeNodeUpgradeStateLabels loops over nodes in the cluster and removes upgrade.UpgradeStateLabel
// It is used for cleanup when autoUpgrade feature gets disabled
func (r *UpgradeReconciler) removeNodeUpgradeStateLabels(ctx context.Context) error {
	r.Log.Info("Resetting node upgrade labels from all nodes")

	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)
	if err != nil {
		r.Log.Error(err, "Failed to get node list to reset upgrade labels")
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
				r.Log.V(consts.LogLevelError).Error(
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
	r.Log.V(consts.LogLevelInfo).Info("Resetting node upgrade annotations from all nodes")

	nodeList := &corev1.NodeList{}
	err := r.List(ctx, nodeList)
	if err != nil {
		r.Log.V(consts.LogLevelError).Error(err, "Failed to get node list to reset upgrade annotations")
		return err
	}
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		_, present := node.Annotations[UpgradeStateAnnotation]
		if present {
			delete(node.Annotations, UpgradeStateAnnotation)
			err = r.Update(ctx, node)
			if err != nil {
				r.Log.V(consts.LogLevelError).Error(
					err, "Failed to reset upgrade annotation from node", "node", node)
				return err
			}
		}
	}
	return nil
}

// BuildState creates a snapshot of the current OFED upgrade state in the cluster (upgrade.ClusterUpgradeState)
// It creates mappings between nodes and their upgrade state
// Nodes are grouped together with the driver POD running on them and the daemon set, controlling this pod
// This state is then used as an input for the upgrade.ClusterUpgradeStateManager
func (r *UpgradeReconciler) BuildState(ctx context.Context) (*upgrade.ClusterUpgradeState, error) {
	r.Log.V(consts.LogLevelInfo).Info("Building state")

	upgradeState := upgrade.NewClusterUpgradeState()

	daemonSets, err := r.getDriverDaemonSets(ctx)
	if err != nil {
		r.Log.V(consts.LogLevelError).Error(err, "Failed to get driver daemon set list")
		return nil, err
	}

	r.Log.V(consts.LogLevelDebug).Info("Got driver daemon sets", "length", len(daemonSets))

	// Get list of driver pods
	podList := &corev1.PodList{}

	err = r.List(ctx, podList,
		client.InNamespace(config.FromEnv().State.NetworkOperatorResourceNamespace),
		client.MatchingLabels{consts.OfedDriverLabel: ""})
	if err != nil {
		return nil, err
	}

	filteredPodList := []corev1.Pod{}
	for _, ds := range daemonSets {
		dsPods := r.getPodsOwnedbyDs(ds, podList.Items)
		if int(ds.Status.DesiredNumberScheduled) != len(dsPods) {
			r.Log.V(consts.LogLevelInfo).Info("Driver daemon set has Unscheduled pods", "name", ds.Name)
			return nil, fmt.Errorf("DS should not have Unscheduled pods")
		}
		filteredPodList = append(filteredPodList, dsPods...)
	}

	upgradeStateLabel := upgrade.GetUpgradeStateLabelKey()

	for i := range filteredPodList {
		pod := &filteredPodList[i]
		ownerDaemonSet := daemonSets[pod.OwnerReferences[0].UID]
		nodeState, err := r.buildNodeUpgradeState(ctx, pod, ownerDaemonSet)
		if err != nil {
			r.Log.V(consts.LogLevelError).Error(err, "Failed to build node upgrade state for pod", "pod", pod)
			return nil, err
		}
		nodeStateLabel := nodeState.Node.Labels[upgradeStateLabel]
		upgradeState.NodeStates[nodeStateLabel] = append(
			upgradeState.NodeStates[nodeStateLabel], nodeState)
	}

	return &upgradeState, nil
}

// buildNodeUpgradeState creates a mapping between a node,
// the driver POD running on them and the daemon set, controlling this pod
func (r *UpgradeReconciler) buildNodeUpgradeState(
	ctx context.Context, pod *corev1.Pod, ds *appsv1.DaemonSet) (*upgrade.NodeUpgradeState, error) {
	node, err := r.NodeUpgradeStateProvider.GetNode(ctx, pod.Spec.NodeName)
	if err != nil {
		r.Log.V(consts.LogLevelError).Error(err, "Failed to get node", "node", pod.Spec.NodeName)
		return nil, err
	}

	upgradeStateLabel := upgrade.GetUpgradeStateLabelKey()
	r.Log.V(consts.LogLevelInfo).Info("Node hosting a driver pod",
		"node", node.Name, "state", node.Labels[upgradeStateLabel])

	return &upgrade.NodeUpgradeState{Node: node, DriverPod: pod, DriverDaemonSet: ds}, nil
}

// getDriverDaemonSets retrieves DaemonSets labeled with OfedDriverLabel and returns UID->DaemonSet map
func (r *UpgradeReconciler) getDriverDaemonSets(ctx context.Context) (map[types.UID]*appsv1.DaemonSet, error) {
	// Get list of driver pods
	daemonSetList := &appsv1.DaemonSetList{}

	err := r.List(ctx, daemonSetList,
		client.InNamespace(config.FromEnv().State.NetworkOperatorResourceNamespace),
		client.MatchingLabels{consts.OfedDriverLabel: ""})
	if err != nil {
		r.Log.V(consts.LogLevelError).Error(err, "Failed to get daemon set list")
		return nil, err
	}

	daemonSetMap := make(map[types.UID]*appsv1.DaemonSet)
	for i := range daemonSetList.Items {
		daemonSet := &daemonSetList.Items[i]
		daemonSetMap[daemonSet.UID] = daemonSet
	}

	return daemonSetMap, nil
}

// getPodsOwnedbyDs gets a list of pods return a list of the pods owned by the specified DaemonSet
func (r *UpgradeReconciler) getPodsOwnedbyDs(ds *appsv1.DaemonSet, pods []corev1.Pod) []corev1.Pod {
	dsPodList := []corev1.Pod{}
	for i := range pods {
		pod := &pods[i]
		if pod.OwnerReferences == nil || len(pod.OwnerReferences) < 1 {
			r.Log.V(consts.LogLevelWarning).Info("OFED Driver Pod has no owner DaemonSet", "pod", pod)
			continue
		}
		r.Log.V(consts.LogLevelDebug).Info("Pod", "pod", pod.Name, "owner", pod.OwnerReferences[0].Name)

		if ds.UID == pod.OwnerReferences[0].UID {
			dsPodList = append(dsPodList, *pod)
		} else {
			r.Log.V(consts.LogLevelWarning).Info("OFED Driver Pod is not owned by an OFED Driver DaemonSet",
				"pod", pod, "actual owner", pod.OwnerReferences[0])
		}
	}
	return dsPodList
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl
func (r *UpgradeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// we always add object with a same(static) key to the queue to reduce
	// reconciliation count
	qHandler := func(q workqueue.RateLimitingInterface) {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: "ofed-upgrade-reconcile-namespace",
			Name:      "ofed-upgrade-reconcile-name",
		}})
	}

	createUpdateDeleteEnqueue := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			qHandler(q)
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			qHandler(q)
		},
		DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
			qHandler(q)
		},
	}

	createUpdateEnqueue := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			qHandler(q)
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			qHandler(q)
		},
	}

	// react on events only for OFED daemon set
	daemonSetPredicates := builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
		labels := object.GetLabels()
		_, ok := labels[consts.OfedDriverLabel]
		return ok
	}))

	// Watch for spec and annotation changes
	nodePredicates := builder.WithPredicates(predicate.AnnotationChangedPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxv1alpha1.NicClusterPolicy{}).
		// set MaxConcurrentReconciles to 1, by default it is already 1, but
		// we set it explicitly here to indicate that we rely on this default behavior
		// UpgradeReconciler contains logic which is not concurrent friendly
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(&source.Kind{Type: &mellanoxv1alpha1.NicClusterPolicy{}}, createUpdateDeleteEnqueue).
		Watches(&source.Kind{Type: &corev1.Node{}}, createUpdateEnqueue, nodePredicates).
		Watches(&source.Kind{Type: &appsv1.DaemonSet{}}, createUpdateDeleteEnqueue, daemonSetPredicates).
		Complete(r)
}
