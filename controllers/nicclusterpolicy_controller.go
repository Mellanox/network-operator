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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
	Log    logr.Logger
	Scheme *runtime.Scheme

	stateManager state.Manager
}

// +kubebuilder:rbac:groups=mellanox.com,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resourceNames=privileged,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch;update
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;pods;pods/status;services;services/finalizers;endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NicClusterPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := r.Log.WithValues("nicclusterpolicy", req.NamespacedName)
	reqLogger.V(consts.LogLevelInfo).Info("Reconciling NicClusterPolicy")

	// Fetch the NicClusterPolicy instance
	instance := &mellanoxv1alpha1.NicClusterPolicy{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.V(consts.LogLevelError).Info("Error occurred on GET CRD request from API server.", "error:", err)
		return reconcile.Result{}, err
	}

	if req.Name != consts.NicClusterPolicyResourceName {
		err := r.handleUnsupportedInstance(instance, req, reqLogger)
		return reconcile.Result{}, err
	}

	// Create a new State service catalog
	sc := state.NewInfoCatalog()
	if instance.Spec.OFEDDriver != nil || instance.Spec.NVPeerDriver != nil ||
		instance.Spec.RdmaSharedDevicePlugin != nil {
		// Create node infoProvider and add to the service catalog
		reqLogger.V(consts.LogLevelInfo).Info("Creating Node info provider")
		nodeList := &corev1.NodeList{}
		err = r.List(context.TODO(), nodeList, nodeinfo.MellanoxNICListOptions...)
		if err != nil {
			// Failed to get node list
			reqLogger.V(consts.LogLevelError).Info("Error occurred on LIST nodes request from API server.", "error:", err)
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
	// Create manager
	managerStatus, err := r.stateManager.SyncState(instance, sc)
	r.updateCrStatus(instance, managerStatus)

	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.updateNodeLabels(instance)
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
func (r *NicClusterPolicyReconciler) updateNodeLabels(cr *mellanoxv1alpha1.NicClusterPolicy) error {
	if cr.Spec.OFEDDriver != nil {
		pods := &corev1.PodList{}
		podLabel := "mofed-" + cr.Spec.OFEDDriver.Version
		_ = r.Client.List(context.TODO(), pods, client.MatchingLabels{"driver-pod": podLabel})
		for i := range pods.Items {
			pod := pods.Items[i]
			labelValue := "true"
			// We assume that OFED pod contains only one container to simplify the logic.
			// We can revisit this logic in the future if needed
			if len(pod.Status.ContainerStatuses) != 0 && pod.Status.ContainerStatuses[0].Ready {
				labelValue = "false"
			}
			patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, nodeinfo.NodeLabelWaitOFED, labelValue))
			err := r.Client.Patch(context.TODO(), &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pod.Spec.NodeName,
				},
			}, client.RawPatch(types.StrategicMergePatchType, patch))

			if err != nil {
				return errors.New("unable to update node label")
			}
		}
	} else {
		nodes := &corev1.NodeList{}
		// We deploy OFED and Device plugins only on a nodes with Mellanox NICs
		err := r.Client.List(context.TODO(), nodes, client.MatchingLabels{nodeinfo.NodeLabelMlnxNIC: "true"})
		if err != nil {
			return errors.New("unable to get nodes")
		}

		for i := range nodes.Items {
			nodes.Items[i].Labels[nodeinfo.NodeLabelWaitOFED] = "false"
			err = r.Client.Update(context.TODO(), &nodes.Items[i])
			if err != nil {
				return errors.New("unable to update node label")
			}
		}
	}

	return nil
}

//nolint:dupl
func (r *NicClusterPolicyReconciler) updateCrStatus(cr *mellanoxv1alpha1.NicClusterPolicy, status state.Results) {
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
	r.Log.V(consts.LogLevelInfo).Info(
		"Updating status", "Custom resource name", cr.Name, "namespace", cr.Namespace, "Result:", cr.Status)
	err := r.Status().Update(context.TODO(), cr)
	if err != nil {
		r.Log.V(consts.LogLevelError).Info("Failed to update CR status", "error:", err)
	}
}

func (r *NicClusterPolicyReconciler) handleUnsupportedInstance(instance *mellanoxv1alpha1.NicClusterPolicy,
	request reconcile.Request, reqLogger logr.Logger) error {
	reqLogger.V(consts.LogLevelWarning).Info("unsupported NicClusterPolicy instance", "instance name:", request.Name)
	reqLogger.V(consts.LogLevelWarning).Info("NicClusterPolicy supports instance with predefined name",
		"supported instance name:", consts.NicClusterPolicyResourceName)

	instance.Status.State = mellanoxv1alpha1.StateIgnore
	instance.Status.Reason = fmt.Sprintf("Unsupported NicClusterPolicy instance %s. Only instance with name %s is"+
		" supported", instance.Name, consts.NicClusterPolicyResourceName)

	err := r.Status().Update(context.TODO(), instance)
	if err != nil {
		r.Log.V(consts.LogLevelError).Info("Failed to update CR status", "error:", err)
	}

	return err
}

// SetupWithManager sets up the controller with the Manager.
//nolint:dupl
func (r *NicClusterPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create state manager
	stateManager, err := state.NewManager(mellanoxv1alpha1.NicClusterPolicyCRDName, mgr.GetClient(), mgr.GetScheme())
	if err != nil {
		// Error creating stateManager
		r.Log.V(consts.LogLevelError).Info("Error creating state manager.", "error:", err)
		panic("Failed to create State manager")
	}
	r.stateManager = stateManager

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&mellanoxv1alpha1.NicClusterPolicy{}).
		// Watch for changes to primary resource NicClusterPolicy
		Watches(&source.Kind{Type: &mellanoxv1alpha1.NicClusterPolicy{}}, &handler.EnqueueRequestForObject{})

	// Watch for changes to secondary resource DaemonSet and requeue the owner NicClusterPolicy
	ws := stateManager.GetWatchSources()
	r.Log.V(consts.LogLevelInfo).Info("Watch Sources", "Kind:", ws)
	for i := range ws {
		builder = builder.Watches(ws[i], &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &mellanoxv1alpha1.NicClusterPolicy{},
		})
	}

	return builder.Complete(r)
}
