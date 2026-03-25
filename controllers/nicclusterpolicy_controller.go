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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/clustertype"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/docadriverimages"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/state"
	"github.com/Mellanox/network-operator/pkg/staticconfig"
)

// NicClusterPolicyReconciler reconciles a NicClusterPolicy object
type NicClusterPolicyReconciler struct {
	client.Client
	Scheme                   *runtime.Scheme
	ClusterTypeProvider      clustertype.Provider
	StaticConfigProvider     staticconfig.Provider
	MigrationCh              chan struct{}
	DocaDriverImagesProvider docadriverimages.Provider

	stateManager state.Manager
}

// In case of adding support for additional types, also update in GetSupportedGVKs func in pkg/state/state_skel.go

//nolint:lll
// +kubebuilder:rbac:groups=mellanox.com,resources=nicclusterpolicies;nicclusterpolicies/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mellanox.com,resources=nicclusterpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=security.openshift.io,resourceNames=privileged,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=get;create;patch;update
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;pods;pods/status;services;services/finalizers;endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups="",resources=pods/eviction,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=pods/finalizers,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.openshift.io,resources=proxies;clusterversions,verbs=get;list;watch
// +kubebuilder:rbac:groups=nv-ipam.nvidia.com,resources=ippools,verbs=get;list;watch;create;deletecollection;
// +kubebuilder:rbac:groups=nv-ipam.nvidia.com,resources=ippools/status,verbs=get;update;patch;
// +kubebuilder:rbac:groups=nv-ipam.nvidia.com,resources=cidrpools,verbs=get;list;watch;create;deletecollection;
// +kubebuilder:rbac:groups=nv-ipam.nvidia.com,resources=cidrpools/status,verbs=get;update;patch;
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers;certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams,verbs=get;list;watch
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicconfigurationtemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicconfigurationtemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicconfigurationtemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicdevices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicdevices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicdevices/finalizers,verbs=update
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicfirmwaretemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicfirmwaretemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicfirmwaretemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicfirmwaresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicfirmwaresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicfirmwaresources/finalizers,verbs=update
//+kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicinterfacenametemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=configuration.net.nvidia.com,resources=nicinterfacenametemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/status,verbs=get
// +kubebuilder:rbac:groups=spectrumx.nvidia.com,resources=spectrumxrailpoolconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spectrumx.nvidia.com,resources=spectrumxrailpoolconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=spectrumx.nvidia.com,resources=spectrumxrailpoolconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=ovsnetworks;sriovnetworknodepolicies;sriovnetworkpoolconfigs,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=sriovnetwork.openshift.io,resources=sriovnetworknodestates,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//nolint:gocognit,funlen
func (r *NicClusterPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Wait for migration flow to finish
	select {
	case <-r.MigrationCh:
	case <-ctx.Done():
		return ctrl.Result{}, fmt.Errorf("canceled")
	}
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelInfo).Info("Reconciling NicClusterPolicy")

	// Fetch the NicClusterPolicy instance
	instance := &mellanoxv1alpha1.NicClusterPolicy{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			shouldRequeue, err := r.handleMOFEDWaitLabelsNoConfig(ctx)
			if err != nil {
				reqLogger.V(consts.LogLevelError).Error(err, "Fail to clear Mofed label on CR deletion.")
				return reconcile.Result{}, err
			}
			if shouldRequeue {
				return r.requeue()
			}

			shouldRequeue, err = r.handleWaitLabelsNoConfig(
				ctx, consts.NicConfigurationDaemonLabel, nodeinfo.NodeLabelWaitNicConfig)
			if err != nil {
				reqLogger.V(consts.LogLevelError).Error(err, "Fail to clear NIC Configuration wait label on CR deletion.")
				return reconcile.Result{}, err
			}
			if shouldRequeue {
				return r.requeue()
			}
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.V(consts.LogLevelError).Error(err, "Error occurred on GET CRD request from API server.")
		return reconcile.Result{}, err
	}

	if req.Name != consts.NicClusterPolicyResourceName {
		err := r.handleUnsupportedInstance(ctx, instance)
		return reconcile.Result{}, err
	}

	// Create a new State service catalog
	sc := state.NewInfoCatalog()
	sc.Add(state.InfoTypeClusterType, r.ClusterTypeProvider)
	sc.Add(state.InfoTypeStaticConfig, r.StaticConfigProvider)

	if instance.Spec.OFEDDriver != nil {
		if err := setupOFEDCatalog(ctx, r.Client, instance.Spec.OFEDDriver,
			r.DocaDriverImagesProvider, sc, nil); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		r.DocaDriverImagesProvider.SetImageSpec(nil)
	}

	// Sync state and update status
	managerStatus := r.stateManager.SyncState(ctx, instance, sc)
	r.updateCrStatus(ctx, instance, managerStatus)

	shouldRequeueMofed, err := r.handleMOFEDWaitLabels(ctx, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	shouldRequeueNicConfig := false
	// If NIC Configuration Operator is not configured, handle NIC Configuration wait labels
	if instance.Spec.NicConfigurationOperator == nil {
		shouldRequeueNicConfig, err = r.handleWaitLabelsNoConfig(
			ctx, consts.NicConfigurationDaemonLabel, nodeinfo.NodeLabelWaitNicConfig)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	if shouldRequeueMofed || shouldRequeueNicConfig || managerStatus.Status != state.SyncStateReady {
		return r.requeue()
	}

	return ctrl.Result{}, nil
}

// requeue triggers resync with configured requeue delay.
func (r *NicClusterPolicyReconciler) requeue() (reconcile.Result, error) {
	return requeueWithDelay()
}

// handleMOFEDWaitLabels updates node labels to mark whether device plugins should wait for OFED.
// If OFED is not configured in NCP, delegates to the NNP-aware fallback.
// Returns true if requeue is required.
func (r *NicClusterPolicyReconciler) handleMOFEDWaitLabels(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy) (bool, error) {
	reqLogger := log.FromContext(ctx)
	if cr.Spec.OFEDDriver == nil {
		reqLogger.V(consts.LogLevelDebug).Info("no OFED config in the policy, check OFED wait label on nodes")
		return r.handleMOFEDWaitLabelsNoConfig(ctx)
	}
	if err := handleOFEDWaitLabelsForPods(ctx, r.Client,
		map[string]string{consts.OfedDriverLabel: ""}); err != nil {
		return false, err
	}
	return false, nil
}

// handleMOFEDWaitLabelsNoConfig handles mofed.wait labels when OFED is NOT configured in NCP.
// It is NNP-aware: nodes managed by NicNodePolicies with ofedDriver are skipped
// (the NNP controller owns their mofed.wait label).
// For remaining nodes:
//   - nodes with a leftover OFED pod → mofed.wait=true (pod is terminating)
//   - nodes with Mellanox NIC but no pod → mofed.wait=false
//   - nodes without Mellanox NIC → label removed
//
// Returns true if requeue is required (leftover pods still terminating).
func (r *NicClusterPolicyReconciler) handleMOFEDWaitLabelsNoConfig(ctx context.Context) (bool, error) {
	reqLogger := log.FromContext(ctx)

	// Get nodes managed by NNPs with OFED — we must not touch their mofed.wait label
	nnpNodes, err := getNodesManagedByNNPsWithOFED(ctx, r.Client)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "failed to get NNP-managed nodes, proceeding without exclusion")
		// Don't fail — proceed without exclusion to avoid blocking label cleanup
		nnpNodes = nil
	}

	// List all OFED pods regardless of owner
	nodesWithPod := map[string]struct{}{}
	pods := &corev1.PodList{}
	if err := r.Client.List(ctx, pods, client.MatchingLabels{consts.OfedDriverLabel: ""}); err != nil {
		return false, errors.Wrap(err, "failed to list OFED pods")
	}
	for i := range pods.Items {
		if pods.Items[i].Spec.NodeName != "" {
			nodesWithPod[pods.Items[i].Spec.NodeName] = struct{}{}
		}
	}

	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return false, errors.Wrap(err, "failed to list nodes")
	}

	ncpNodesWithPod := 0
	for i := range nodes.Items {
		node := &nodes.Items[i]

		// Skip nodes managed by NNP — their mofed.wait is handled by the NNP controller
		if nnpNodes[node.Name] {
			continue
		}

		labelValue := ""
		if _, hasPod := nodesWithPod[node.Name]; hasPod {
			labelValue = "true"
			ncpNodesWithPod++
		} else if node.GetLabels()[nodeinfo.NodeLabelMlnxNIC] == "true" {
			labelValue = "false"
		}
		if err := setNodeLabel(ctx, r.Client, node.Name, nodeinfo.NodeLabelWaitOFED, labelValue); err != nil {
			return false, err
		}
	}

	if ncpNodesWithPod > 0 {
		reqLogger.V(consts.LogLevelDebug).Info(
			"no OFED spec in NicClusterPolicy but there are OFED pods on non-NNP nodes, requeue")
		return true, nil
	}
	return false, nil
}

// handleWaitLabelsNoConfig handles wait labels for for scenarios when given component is
// not configured in NicClusterPolicy and does the following:
// - sets given label to false on Nodes with NVIDIA NICs
// - removes given label from nodes which have no NVIDIA NICs anymore
// - sets given label to true if detects a pod with a specific label on the node (probably in the terminating state).
// returns true if requeue (resync) is required
//
//nolint:lll
func (r *NicClusterPolicyReconciler) handleWaitLabelsNoConfig(ctx context.Context, podLabel, waitLabel string) (bool, error) {
	reqLogger := log.FromContext(ctx)
	nodesWithPod := map[string]struct{}{}
	pods := &corev1.PodList{}
	if err := r.Client.List(ctx, pods, client.MatchingLabels{podLabel: ""}); err != nil {
		return false, errors.Wrap(err, "failed to list pods")
	}
	for i := range pods.Items {
		pod := pods.Items[i]
		if pod.Spec.NodeName != "" {
			nodesWithPod[pod.Spec.NodeName] = struct{}{}
		}
	}
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return false, errors.Wrap(err, "failed to list nodes")
	}
	for i := range nodes.Items {
		node := nodes.Items[i]
		labelValue := ""
		if _, hasPod := nodesWithPod[node.Name]; hasPod {
			labelValue = "true"
		} else if node.GetLabels()[nodeinfo.NodeLabelMlnxNIC] == "true" {
			labelValue = "false"
		}
		if err := setNodeLabel(ctx, r.Client, node.Name, waitLabel, labelValue); err != nil {
			return false, err
		}
	}
	if len(nodesWithPod) > 0 {
		// There is no given component spec in the NicClusterPolicy, but some pods are on nodes.
		// These Pods should be eventually removed from the cluster,
		// and we will need to update the given wait label for nodes.
		// Here, we trigger resync explicitly to ensure that we will always handle the removal of the Pod.
		// This explicit resync is required because we don't watch for Pods and can't rely on the DaemonSet
		// update in this case (cache with Pods can be outdated when we handle DaemonSet removal event).
		reqLogger.V(consts.LogLevelDebug).Info(
			"no given component spec in NicClusterPolicy but there are pods on nodes, requeue", "component", podLabel)
		return true, nil
	}
	return false, nil
}


//nolint:dupl
func (r *NicClusterPolicyReconciler) updateCrStatus(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy, status state.Results) {
	updatePolicyCRStatus(ctx, r, cr, status)
}

func (r *NicClusterPolicyReconciler) handleUnsupportedInstance(
	ctx context.Context, instance *mellanoxv1alpha1.NicClusterPolicy) error {
	reqLogger := log.FromContext(ctx)
	reqLogger.V(consts.LogLevelWarning).Info("unsupported NicClusterPolicy instance name")
	reqLogger.V(consts.LogLevelWarning).Info("NicClusterPolicy supports instance with predefined name",
		"supported instance name:", consts.NicClusterPolicyResourceName)

	instance.Status.State = mellanoxv1alpha1.StateIgnore
	instance.Status.Reason = fmt.Sprintf("Unsupported NicClusterPolicy instance %s. Only instance with name %s is"+
		" supported", instance.Name, consts.NicClusterPolicyResourceName)

	err := r.Status().Update(ctx, instance)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to update CR status")
	}

	return err
}

// SetupWithManager sets up the controller with the Manager.
//
//nolint:dupl
func (r *NicClusterPolicyReconciler) SetupWithManager(mgr ctrl.Manager, setupLog logr.Logger) error {
	// Create state manager
	stateManager, err := state.NewManager(mellanoxv1alpha1.NicClusterPolicyCRDName, mgr.GetClient(),
		setupLog.WithName("StateManager"))
	if err != nil {
		setupLog.V(consts.LogLevelError).Error(err, "Error creating state manager.")
		return err
	}
	r.stateManager = stateManager

	// Define the controller builder
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxv1alpha1.NicClusterPolicy{}).
		// Watch for changes to primary resource NicClusterPolicy
		// The EnqueueRequestForObject handler is fine for the primary resource if no complex mapping is needed.
		// If generation checks are needed, WithEventFilter can be used on For().
		Watches(&mellanoxv1alpha1.NicClusterPolicy{}, &handler.EnqueueRequestForObject{})

	// we always add object with a same(static) key to the queue to reduce
	// reconciliation count
	updateEnqueue := handler.Funcs{
		UpdateFunc: func(_ context.Context, _ event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name: consts.NicClusterPolicyResourceName,
			}})
		},
	}

	// Predicates for Node events
	nodeEventPredicates := predicate.Or(
		MlnxLabelChangedPredicate{},
		NodeTaintChangedPredicate{},
	)

	// Add a watch for Node resources with the combined predicates
	bld = bld.Watches(
		&corev1.Node{},
		updateEnqueue, // Use the handler.Funcs defined above
		builder.WithPredicates(nodeEventPredicates), // Wrap predicates for WatchesOption
	)

	bld = watchStateSources(bld, mgr, setupLog, stateManager, &mellanoxv1alpha1.NicClusterPolicy{})

	// Watch NicNodePolicy changes so we recalculate NNP-managed node exclusions for mofed.wait
	bld = bld.Watches(
		&mellanoxv1alpha1.NicNodePolicy{},
		updateEnqueue,
		builder.WithPredicates(predicate.GenerationChangedPredicate{}),
	)

	return bld.Complete(r)
}
