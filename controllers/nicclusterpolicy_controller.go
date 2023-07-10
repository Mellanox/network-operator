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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	mellanoxv1alpha1 "github.com/Mellanox/network-operator/api/v1alpha1"
	"github.com/Mellanox/network-operator/pkg/config"
	"github.com/Mellanox/network-operator/pkg/consts"
	"github.com/Mellanox/network-operator/pkg/nodeinfo"
	"github.com/Mellanox/network-operator/pkg/state"
)

// NicClusterPolicyReconciler reconciles a NicClusterPolicy object
type NicClusterPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	stateManager state.Manager
}

// In case of adding support for additional types, also update in getSupportedGVKs func in pkg/state/state_skel.go
//nolint
// +kubebuilder:rbac:groups=mellanox.com,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resourceNames=privileged,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=podsecuritypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch;update
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;pods;pods/status;services;services/finalizers;endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups="",resources=pods/eviction,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=whereabouts.cni.cncf.io,resources=ippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=whereabouts.cni.cncf.io,resources=overlappingrangeipreservations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.openshift.io,resources=proxies,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NicClusterPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
			err := r.clearMofedWaitLabel(ctx)
			if err != nil {
				reqLogger.V(consts.LogLevelError).Error(err, "Fail to clear Mofed label on CR deletion.")
				return reconcile.Result{}, err
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
	if instance.Spec.OFEDDriver != nil || instance.Spec.RdmaSharedDevicePlugin != nil ||
		instance.Spec.SriovDevicePlugin != nil {
		// Create node infoProvider and add to the service catalog
		reqLogger.V(consts.LogLevelInfo).Info("Creating Node info provider")
		nodeList := &corev1.NodeList{}
		err = r.List(ctx, nodeList, nodeinfo.MellanoxNICListOptions...)
		if err != nil {
			// Failed to get node list
			reqLogger.V(consts.LogLevelError).Error(err, "Error occurred on LIST nodes request from API server.")
			return reconcile.Result{}, err
		}
		nodePtrList := make([]*corev1.Node, len(nodeList.Items))
		nodeNames := make([]*string, len(nodeList.Items))
		for i := range nodePtrList {
			nodePtrList[i] = &nodeList.Items[i]
			nodeNames[i] = &nodeList.Items[i].Name
		}
		reqLogger.V(consts.LogLevelDebug).Info("Node info provider with", "Nodes:", nodeNames)
		infoProvider := nodeinfo.NewProvider(nodePtrList)
		sc.Add(state.InfoTypeNodeInfo, infoProvider)
	}
	// Sync state and update status
	managerStatus := r.stateManager.SyncState(ctx, instance, sc)
	r.updateCrStatus(ctx, instance, managerStatus)

	err = r.updateNodeLabels(ctx, instance)
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

// updateNodeLabels updates nodes labels to mark device plugins should wait for OFED pod
// Set nvidia.com/ofed.wait=false if OFED is not deployed.
func (r *NicClusterPolicyReconciler) updateNodeLabels(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy) error {
	if cr.Spec.OFEDDriver != nil {
		pods := &corev1.PodList{}
		podLabel := "mofed-" + cr.Spec.OFEDDriver.Version
		_ = r.Client.List(ctx, pods, client.MatchingLabels{"driver-pod": podLabel})
		for i := range pods.Items {
			pod := pods.Items[i]
			labelValue := "true"
			// We assume that OFED pod contains only one container to simplify the logic.
			// We can revisit this logic in the future if needed
			if len(pod.Status.ContainerStatuses) != 0 && pod.Status.ContainerStatuses[0].Ready {
				labelValue = "false"
			}
			patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:%q}}}`, nodeinfo.NodeLabelWaitOFED, labelValue))
			err := r.Client.Patch(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pod.Spec.NodeName,
				},
			}, client.RawPatch(types.StrategicMergePatchType, patch))

			if err != nil {
				return errors.Wrapf(err, "unable to patch %s label for node %s", nodeinfo.NodeLabelWaitOFED,
					pod.Spec.NodeName)
			}
		}
	} else {
		return r.clearMofedWaitLabel(ctx)
	}

	return nil
}

// clearMofedWaitLabel set "network.nvidia.com/operator.mofed.wait" to false
// on Nodes with Mellanox NICs
func (r *NicClusterPolicyReconciler) clearMofedWaitLabel(ctx context.Context) error {
	// We deploy OFED and Device plugins only on a nodes with Mellanox NICs
	nodes := &corev1.NodeList{}

	err := r.Client.List(ctx, nodes, client.MatchingLabels{nodeinfo.NodeLabelMlnxNIC: "true"})
	if err != nil {
		return errors.Wrap(err, "failed to list nodes")
	}

	for i := range nodes.Items {
		patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:"false"}}}`, nodeinfo.NodeLabelWaitOFED))
		err := r.Client.Patch(ctx, &nodes.Items[i], client.RawPatch(types.StrategicMergePatchType, patch))
		if err != nil {
			return errors.Wrapf(err, "unable to patch %s node label for node %s",
				nodeinfo.NodeLabelWaitOFED, nodes.Items[i].Name)
		}
	}
	return nil
}

//nolint:dupl
func (r *NicClusterPolicyReconciler) updateCrStatus(
	ctx context.Context, cr *mellanoxv1alpha1.NicClusterPolicy, status state.Results) {
	reqLogger := log.FromContext(ctx)
NextResult:
	for _, stateStatus := range status.StatesStatus {
		// basically iterate over results and add/update crStatus.AppliedStates
		for i := range cr.Status.AppliedStates {
			if cr.Status.AppliedStates[i].Name == stateStatus.StateName {
				cr.Status.AppliedStates[i].State = mellanoxv1alpha1.State(stateStatus.Status)
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

	// send status update request to k8s API
	reqLogger.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.Name, "namespace", cr.Namespace, "Result:", cr.Status)
	err := r.Status().Update(ctx, cr)
	if err != nil {
		reqLogger.V(consts.LogLevelError).Error(err, "Failed to update CR status")
	}
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
	stateManager, err := state.NewManager(mellanoxv1alpha1.NicClusterPolicyCRDName, mgr.GetClient(), mgr.GetScheme(),
		setupLog.WithName("StateManager"))
	if err != nil {
		setupLog.V(consts.LogLevelError).Error(err, "Error creating state manager.")
		return err
	}
	r.stateManager = stateManager

	ctl := ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxv1alpha1.NicClusterPolicy{}).
		// Watch for changes to primary resource NicClusterPolicy
		Watches(&source.Kind{Type: &mellanoxv1alpha1.NicClusterPolicy{}}, &handler.EnqueueRequestForObject{})

	// we always add object with a same(static) key to the queue to reduce
	// reconciliation count
	updateEnqueue := handler.Funcs{
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name: consts.NicClusterPolicyResourceName,
			}})
		},
	}

	// Watch for "feature.node.kubernetes.io/pci-15b3.present" label applying
	nodePredicates := builder.WithPredicates(MlnxLabelChangedPredicate{})
	ctl = ctl.Watches(&source.Kind{Type: &corev1.Node{}}, updateEnqueue, nodePredicates)

	ws := stateManager.GetWatchSources()

	for i := range ws {
		setupLog.V(consts.LogLevelInfo).Info("Watching", "Kind", fmt.Sprintf("%T", ws[i].Type))
		ctl = ctl.Watches(ws[i], &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &mellanoxv1alpha1.NicClusterPolicy{},
		}, builder.WithPredicates(IgnoreSameContentPredicate{}))
	}

	return ctl.Complete(r)
}
